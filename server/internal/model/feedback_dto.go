package model

import (
	"strings"
	"time"
)

type FeedbackKindOption struct {
	Value string `json:"value" example:"topic"`
	Label string `json:"label" example:"想看话题"`
}

type FeedbackLimits struct {
	TitleMin     int `json:"title_min" example:"1"`
	TitleMax     int `json:"title_max" example:"80"`
	ContentMin   int `json:"content_min" example:"10"`
	ContentMax   int `json:"content_max" example:"5000"`
	NicknameMax  int `json:"nickname_max" example:"40"`
	SourceURLMax int `json:"source_url_max" example:"2048"`
}

type VerificationOptionsDTO struct {
	TurnstileEnabled bool   `json:"turnstile_enabled"`
	TurnstileSiteKey string `json:"turnstile_site_key,omitempty"`
	AIAssisted       bool   `json:"ai_assisted"`
}

type FeedbackOptionsDTO struct {
	Enabled              bool                   `json:"enabled"`
	SubscriptionsEnabled bool                   `json:"subscriptions_enabled"`
	Kinds                []FeedbackKindOption   `json:"kinds"`
	Limits               FeedbackLimits         `json:"limits"`
	Verification         VerificationOptionsDTO `json:"verification"`
}

type SubmitReceiptDTO struct {
	ID                 string    `json:"id" example:"9f2c1d7e8a4b"`
	Status             string    `json:"status" example:"pending"`
	SubscriptionStatus string    `json:"subscription_status" example:"none"`
	ConfirmationEmail  string    `json:"confirmation_email" example:"queued"`
	CreatedAt          time.Time `json:"created_at"`
}

type FeedbackListItemDTO struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	Title              string    `json:"title"`
	Summary            string    `json:"summary"`
	Status             string    `json:"status"`
	SubscriptionStatus string    `json:"subscription_status"`
	StoryID            string    `json:"story_id,omitempty"`
	HasEmail           bool      `json:"has_email"`
	HasNickname        bool      `json:"has_nickname"`
	SpamScore          int       `json:"spam_score"`
	SpamVerdict        string    `json:"spam_verdict"`
	Version            int       `json:"version"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type FeedbackEventDTO struct {
	Seq          int       `json:"seq"`
	FromStatus   string    `json:"from_status"`
	ToStatus     string    `json:"to_status"`
	InternalNote string    `json:"internal_note"`
	PublicReply  string    `json:"public_reply"`
	Actor        string    `json:"actor"`
	CreatedAt    time.Time `json:"created_at"`
}

type MailTaskDTO struct {
	ID             string     `json:"id"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	NextAttemptAt  time.Time  `json:"next_attempt_at"`
	LastError      string     `json:"last_error,omitempty"`
	SMTPAcceptedAt *time.Time `json:"smtp_accepted_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type FeedbackDetailDTO struct {
	ID                 string             `json:"id"`
	Kind               string             `json:"kind"`
	Title              string             `json:"title"`
	Content            string             `json:"content"`
	SourceURL          string             `json:"source_url"`
	StoryID            string             `json:"story_id"`
	Nickname           string             `json:"nickname"`
	Email              string             `json:"email"`
	EmailVerified      bool               `json:"email_verified"`
	Status             string             `json:"status"`
	InternalNote       string             `json:"internal_note"`
	PublicReply        string             `json:"public_reply"`
	Version            int                `json:"version"`
	SubscriptionStatus string             `json:"subscription_status"`
	ConsentVersion     string             `json:"consent_version"`
	ConsentedAt        *time.Time         `json:"consented_at,omitempty"`
	ConfirmedAt        *time.Time         `json:"confirmed_at,omitempty"`
	UnsubscribedAt     *time.Time         `json:"unsubscribed_at,omitempty"`
	SpamScore          int                `json:"spam_score"`
	SpamVerdict        string             `json:"spam_verdict"`
	SpamSignals        []string           `json:"spam_signals"`
	SpamSource         string             `json:"spam_source"`
	SpamModel          string             `json:"spam_model"`
	SpamConfidence     float64            `json:"spam_confidence"`
	VerificationMethod string             `json:"verification_method"`
	VerifiedAt         *time.Time         `json:"verified_at,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	ClosedAt           *time.Time         `json:"closed_at,omitempty"`
	Events             []FeedbackEventDTO `json:"events"`
	Mail               []MailTaskDTO      `json:"mail"`
}

type MailStatusDTO struct {
	SMTPEnabled      bool     `json:"smtp_enabled"`
	Configured       bool     `json:"configured"`
	Missing          []string `json:"missing"`
	NotifyConfigured bool     `json:"notify_configured"`
	// Error explains why delivery is currently paused (disabled, incomplete
	// config, or unreadable secret); empty when healthy.
	Error        string           `json:"error,omitempty"`
	Counts       map[string]int64 `json:"counts"`
	RecentErrors []MailTaskDTO    `json:"recent_errors"`
}

func FeedbackListItem(fb Feedback) FeedbackListItemDTO {
	summary := []rune(fb.Content)
	if len(summary) > 120 {
		summary = append(summary[:120], '…')
	}
	return FeedbackListItemDTO{
		ID: fb.ID, Kind: fb.Kind, Title: fb.Title, Summary: string(summary),
		Status: fb.Status, SubscriptionStatus: fb.SubscriptionStatus,
		StoryID: fb.StoryID, HasEmail: fb.Email != "", HasNickname: fb.Nickname != "",
		SpamScore: fb.SpamScore, SpamVerdict: fb.SpamVerdict,
		Version: fb.Version, CreatedAt: fb.CreatedAt, UpdatedAt: fb.UpdatedAt,
	}
}

func splitSignals(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func FeedbackDetail(fb Feedback, events []FeedbackEvent, mail []MailOutbox) FeedbackDetailDTO {
	detail := FeedbackDetailDTO{
		ID: fb.ID, Kind: fb.Kind, Title: fb.Title, Content: fb.Content,
		SourceURL: fb.SourceURL, StoryID: fb.StoryID, Nickname: fb.Nickname,
		Email: fb.Email, EmailVerified: fb.ConfirmedAt != nil,
		Status: fb.Status, InternalNote: fb.InternalNote, PublicReply: fb.PublicReply,
		Version: fb.Version, SubscriptionStatus: fb.SubscriptionStatus,
		ConsentVersion: fb.ConsentVersion, ConsentedAt: fb.ConsentedAt,
		ConfirmedAt: fb.ConfirmedAt, UnsubscribedAt: fb.UnsubscribedAt,
		SpamScore: fb.SpamScore, SpamVerdict: fb.SpamVerdict,
		SpamSignals: splitSignals(fb.SpamSignals), SpamSource: fb.SpamSource,
		SpamModel: fb.SpamModel, SpamConfidence: fb.SpamConfidence,
		VerificationMethod: fb.VerificationMethod, VerifiedAt: fb.VerifiedAt,
		CreatedAt: fb.CreatedAt, UpdatedAt: fb.UpdatedAt, ClosedAt: fb.ClosedAt,
		Events: make([]FeedbackEventDTO, 0, len(events)),
		Mail:   make([]MailTaskDTO, 0, len(mail)),
	}
	for _, event := range events {
		detail.Events = append(detail.Events, FeedbackEventDTO{
			Seq: event.Seq, FromStatus: event.FromStatus, ToStatus: event.ToStatus,
			InternalNote: event.InternalNote, PublicReply: event.PublicReply,
			Actor: event.Actor, CreatedAt: event.CreatedAt,
		})
	}
	for _, task := range mail {
		detail.Mail = append(detail.Mail, MailTaskDTO{
			ID: task.ID, Type: task.Type, Status: task.Status, Attempts: task.Attempts,
			NextAttemptAt: task.NextAttemptAt, LastError: task.LastError,
			SMTPAcceptedAt: task.SMTPAcceptedAt, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		})
	}
	return detail
}
