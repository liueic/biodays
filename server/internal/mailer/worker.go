package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"biodays/server/internal/model"
	"biodays/server/internal/security"
	"biodays/server/internal/store"
)

var retryDelays = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
	120 * time.Minute,
}

const maxAttempts = 5

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Worker drains mail_outbox in-process. One worker is enough: leases guard
// against duplicate claims across restarts.
type Worker struct {
	store       *store.FeedbackStore
	sender      Sender
	renderer    Renderer
	logger      *slog.Logger
	now         func() time.Time
	interval    time.Duration
	lease       time.Duration
	abuseSecret []byte
	notifyTo    string
}

func NewWorker(st *store.FeedbackStore, sender Sender, renderer Renderer, logger *slog.Logger, now func() time.Time, abuseSecret []byte, notifyTo string) *Worker {
	return &Worker{
		store:       st,
		sender:      sender,
		renderer:    renderer,
		logger:      logger,
		now:         now,
		interval:    15 * time.Second,
		lease:       2 * time.Minute,
		abuseSecret: abuseSecret,
		notifyTo:    notifyTo,
	}
}

// Run processes until ctx is cancelled. The caller owns shutdown waiting.
func (w *Worker) Run(ctx context.Context) {
	if _, err := w.store.RecoverExpiredLeases(ctx, w.now()); err != nil {
		w.logger.Error("recover mail leases", "error", err)
	}
	if _, err := w.store.BackfillOutboxRecipient(ctx, model.MailTypeAdminNewFeedback, w.notifyTo, w.now()); err != nil {
		w.logger.Error("backfill admin mail recipient", "error", err)
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		if _, err := w.ProcessOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("mail worker tick", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessOnce claims and delivers a batch. Exported for tests.
func (w *Worker) ProcessOnce(ctx context.Context) (int, error) {
	now := w.now().UTC()
	if _, err := w.store.RecoverExpiredLeases(ctx, now); err != nil {
		return 0, err
	}
	tasks, err := w.store.ClaimOutbox(ctx, now, w.lease, 10)
	if err != nil {
		return 0, err
	}
	sent := 0
	for i := range tasks {
		if ctx.Err() != nil {
			return sent, ctx.Err()
		}
		if w.process(ctx, &tasks[i]) {
			sent++
		}
	}
	return sent, nil
}

func (w *Worker) process(ctx context.Context, task *model.MailOutbox) bool {
	now := w.now().UTC()
	if task.Recipient == "" {
		w.cancel(ctx, task, "no recipient configured")
		return false
	}

	data := TemplateData{}
	if task.Payload != "" {
		if err := json.Unmarshal([]byte(task.Payload), &data); err != nil {
			w.fail(ctx, task, "mail payload is corrupt")
			return false
		}
	}
	if data.SiteName == "" {
		data.SiteName = w.renderer.SiteName
	}
	if data.SiteURL == "" {
		data.SiteURL = w.renderer.SiteURL
	}
	if data.EmailFingerprint == "" && data.Email != "" && len(w.abuseSecret) > 0 {
		data.EmailFingerprint = security.HMACKey(w.abuseSecret, "email", strings.ToLower(data.Email))
	}

	if task.Type != model.MailTypeSMTPTest {
		fb, err := w.store.GetFeedback(ctx, task.FeedbackID)
		if err != nil {
			if errors.Is(err, store.ErrFeedbackNotFound) {
				w.cancel(ctx, task, "feedback deleted")
			} else {
				w.retry(ctx, task, now, err)
			}
			return false
		}
		switch task.Type {
		case model.MailTypeSubscriptionConfirm:
			if !w.prepareConfirmation(ctx, task, &fb, &data, now) {
				return false
			}
		case model.MailTypeUserStatusUpdate:
			if !w.prepareStatusUpdate(ctx, task, &fb, &data) {
				return false
			}
		}
	}

	msg, err := w.renderer.Render(task.Type, data)
	if err != nil {
		w.fail(ctx, task, "render mail: "+err.Error())
		return false
	}
	msg.To = task.Recipient

	if err := w.sender.Send(ctx, msg); err != nil {
		w.handleSendError(ctx, task, err, now)
		return false
	}
	if err := w.store.FinishOutbox(ctx, task.ID, w.now(), now); err != nil {
		w.logger.Error("mark mail sent", "task", task.ID, "error", err)
		return false
	}
	return true
}

func (w *Worker) prepareConfirmation(ctx context.Context, task *model.MailOutbox, fb *model.Feedback, data *TemplateData, now time.Time) bool {
	if fb.SubscriptionStatus == model.SubscriptionConfirmed {
		w.cancel(ctx, task, "subscription already confirmed")
		return false
	}
	if fb.SubscriptionStatus != model.SubscriptionPendingConfirmation {
		w.cancel(ctx, task, "subscription is "+fb.SubscriptionStatus)
		return false
	}

	expired := false
	if data.TokenHash != "" {
		token, err := w.store.GetToken(ctx, data.TokenHash, model.TokenPurposeConfirm)
		switch {
		case err == nil:
			expired = !token.ExpiresAt.After(now) || token.ConsumedAt != nil
		case errors.Is(err, store.ErrTokenInvalid):
			expired = true
		default:
			w.retry(ctx, task, now, err)
			return false
		}
	} else {
		expired = true
	}
	if !expired {
		return true
	}

	raw, err := security.RandomToken()
	if err != nil {
		w.retry(ctx, task, now, err)
		return false
	}
	hash := security.HashToken(raw)
	fingerprint := data.EmailFingerprint
	if fingerprint == "" {
		fingerprint = security.HMACKey(w.abuseSecret, "email", strings.ToLower(fb.Email))
	}
	token := &model.SubscriptionToken{
		TokenHash:        hash,
		FeedbackID:       fb.ID,
		Purpose:          model.TokenPurposeConfirm,
		EmailFingerprint: fingerprint,
		ExpiresAt:        now.Add(24 * time.Hour),
		CreatedAt:        now,
	}
	if err := w.store.CreateToken(ctx, token, true); err != nil {
		w.retry(ctx, task, now, err)
		return false
	}
	if err := w.store.CancelOtherConfirmTasks(ctx, fb.ID, task.ID, "superseded by regenerated token", now); err != nil {
		w.retry(ctx, task, now, err)
		return false
	}
	data.TokenHash = hash
	data.ConfirmURL = ConfirmURL(data.SiteURL, raw)
	encoded, err := json.Marshal(data)
	if err != nil {
		w.retry(ctx, task, now, err)
		return false
	}
	if err := w.store.UpdateOutboxPayload(ctx, task.ID, string(encoded), now); err != nil {
		w.retry(ctx, task, now, err)
		return false
	}
	return true
}

func (w *Worker) prepareStatusUpdate(ctx context.Context, task *model.MailOutbox, fb *model.Feedback, data *TemplateData) bool {
	if fb.SubscriptionStatus != model.SubscriptionConfirmed {
		w.cancel(ctx, task, "subscription no longer confirmed")
		return false
	}
	if !model.IsNotifiableStatus(fb.Status) {
		w.cancel(ctx, task, "status is not notifiable")
		return false
	}
	if data.UnsubscribeURL == "" {
		w.cancel(ctx, task, "missing unsubscribe link")
		return false
	}
	// Render the current state, not a stale snapshot from when the task queued.
	data.FeedbackID = fb.ID
	data.Title = fb.Title
	data.Status = fb.Status
	data.StatusLabel = StatusLabel(fb.Status)
	data.PublicReply = fb.PublicReply
	return true
}

func (w *Worker) handleSendError(ctx context.Context, task *model.MailOutbox, err error, now time.Time) {
	var delivery *DeliveryError
	if errors.As(err, &delivery) {
		switch {
		case delivery.Ambiguous:
			if markErr := w.store.MarkOutboxUnknown(ctx, task.ID, err.Error(), now); markErr != nil {
				w.logger.Error("mark mail unknown", "task", task.ID, "error", markErr)
			}
			return
		case delivery.Permanent:
			w.fail(ctx, task, err.Error())
			return
		}
	}
	w.retry(ctx, task, now, err)
}

func (w *Worker) retry(ctx context.Context, task *model.MailOutbox, now time.Time, err error) {
	attempts := task.Attempts + 1
	if attempts >= maxAttempts {
		w.fail(ctx, task, "retries exhausted: "+err.Error())
		return
	}
	delay := retryDelays[min(attempts-1, len(retryDelays)-1)]
	if storeErr := w.store.RetryOutboxLater(ctx, task.ID, err.Error(), attempts, now.Add(delay), now); storeErr != nil {
		w.logger.Error("schedule mail retry", "task", task.ID, "error", storeErr)
	}
}

func (w *Worker) fail(ctx context.Context, task *model.MailOutbox, reason string) {
	if err := w.store.MarkOutboxFailed(ctx, task.ID, reason, w.now()); err != nil {
		w.logger.Error("mark mail failed", "task", task.ID, "error", err)
	}
}

func (w *Worker) cancel(ctx context.Context, task *model.MailOutbox, reason string) {
	if err := w.store.CancelOutbox(ctx, task.ID, reason, w.now()); err != nil {
		w.logger.Error("cancel mail", "task", task.ID, "error", err)
	}
}

func ConfirmURL(siteURL, raw string) string {
	return strings.TrimRight(siteURL, "/") + "/feedback/confirm#token=" + raw
}

func UnsubscribeURL(siteURL, raw string) string {
	return strings.TrimRight(siteURL, "/") + "/feedback/unsubscribe#token=" + raw
}
