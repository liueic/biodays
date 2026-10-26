package store

import (
	"context"
	"errors"
	"time"

	"biodays/server/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

var (
	ErrFeedbackNotFound  = errors.New("feedback not found")
	ErrVersionConflict   = errors.New("feedback version conflict")
	ErrRequestConflict   = errors.New("request id already used with different content")
	ErrTokenInvalid      = errors.New("token invalid")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenConsumed     = errors.New("token already used")
	ErrOutboxNotFound    = errors.New("mail task not found")
	ErrChallengeNotFound = errors.New("challenge not found")
	ErrChallengeConsumed = errors.New("challenge already consumed")
)

// FeedbackStore owns the feedback tables (submissions, sessions, tokens, mail).
type FeedbackStore struct {
	db *gorm.DB
}

// OpenFeedback opens feedback.db on SQLite. It is deliberately separate from
// Store so story rebuilds never touch user submissions.
func OpenFeedback(path string) (*FeedbackStore, error) {
	db, err := gorm.Open(sqlite.Open(sqliteDSN(path)), feedbackGormConfig())
	if err != nil {
		return nil, err
	}
	// WAL allows concurrent readers; writes still serialise. Four connections
	// cover HTTP writes plus the single mail worker.
	if err := setPool(db, 4, 2); err != nil {
		return nil, err
	}
	return newFeedbackStore(db)
}

// OpenFeedbackPostgres opens the feedback tables in the PostgreSQL database
// named by dsn. Stories live in the same database; table names do not overlap.
func OpenFeedbackPostgres(dsn string) (*FeedbackStore, error) {
	db, err := gorm.Open(postgres.Open(dsn), feedbackGormConfig())
	if err != nil {
		return nil, err
	}
	if err := setPool(db, 10, 5); err != nil {
		return nil, err
	}
	return newFeedbackStore(db)
}

func feedbackGormConfig() *gorm.Config {
	return &gorm.Config{
		TranslateError: true,
		Logger: gormlogger.New(discardWriter{}, gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	}
}

func newFeedbackStore(db *gorm.DB) (*FeedbackStore, error) {
	s := &FeedbackStore{db: db}
	if err := s.db.AutoMigrate(
		&model.Feedback{}, &model.FeedbackEvent{}, &model.AdminSession{},
		&model.SubscriptionToken{}, &model.MailOutbox{}, &model.AbuseBucket{},
		&model.FeedbackChallenge{}, &model.FeedbackDeletion{},
		&model.AdminUser{}, &model.AppSetting{},
	); err != nil {
		return nil, err
	}
	// The index repair is SQLite-specific; AutoMigrate already creates the
	// composite unique index on PostgreSQL.
	if s.db.Dialector.Name() != "postgres" {
		// Early builds created a unique index on feedback_events.seq alone, which
		// allowed only one feedback per database. Replace it with (feedback_id, seq).
		if err := s.repairFeedbackEventIndex(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *FeedbackStore) repairFeedbackEventIndex() error {
	var columns []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.db.Raw(`PRAGMA index_info('idx_feedback_events_seq')`).Scan(&columns).Error; err != nil || len(columns) == 0 {
		return nil
	}
	if len(columns) != 1 || columns[0].Name != "seq" {
		return nil
	}
	if err := s.db.Exec(`DROP INDEX idx_feedback_events_seq`).Error; err != nil {
		return err
	}
	return s.db.Exec(`CREATE UNIQUE INDEX idx_feedback_events_seq ON feedback_events(feedback_id, seq)`).Error
}

// discardWriter keeps GORM's default logger quiet without changing the wire format.
type discardWriter struct{}

func (discardWriter) Printf(string, ...any) {}

func (s *FeedbackStore) Close() error {
	pool, err := s.db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}

func (s *FeedbackStore) Ping(ctx context.Context) error {
	var result int
	return s.db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error
}

// BackupTo writes a consistent snapshot with VACUUM INTO, including WAL data.
func (s *FeedbackStore) BackupTo(ctx context.Context, path string) error {
	if s.db.Dialector.Name() == "postgres" {
		return ErrBackupUnsupported
	}
	return s.db.WithContext(ctx).Exec("VACUUM INTO ?", path).Error
}

type CreateSubmissionInput struct {
	Feedback    *model.Feedback
	Event       *model.FeedbackEvent
	Tokens      []model.SubscriptionToken
	Outbox      []model.MailOutbox
	ChallengeID string
	Now         time.Time
}

// CreateSubmission writes the feedback, event, tokens and mail jobs in one
// transaction. SMTP is never touched here.
func (s *FeedbackStore) CreateSubmission(ctx context.Context, in CreateSubmissionInput) error {
	now := in.Now.UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.ChallengeID != "" {
			result := tx.Model(&model.FeedbackChallenge{}).
				Where("id = ? AND consumed_at IS NULL AND expires_at > ?", in.ChallengeID, now).
				Updates(map[string]any{"consumed_at": now, "feedback_id": in.Feedback.ID})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrChallengeConsumed
			}
		}
		if err := tx.Create(in.Feedback).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrRequestConflict
			}
			return err
		}
		if in.Event != nil {
			in.Event.FeedbackID = in.Feedback.ID
			if err := tx.Create(in.Event).Error; err != nil {
				return err
			}
		}
		for i := range in.Tokens {
			if err := tx.Create(&in.Tokens[i]).Error; err != nil {
				return err
			}
		}
		for i := range in.Outbox {
			in.Outbox[i].FeedbackID = in.Feedback.ID
			if in.Outbox[i].CreatedAt.IsZero() {
				in.Outbox[i].CreatedAt = in.Feedback.CreatedAt
			}
			if in.Outbox[i].UpdatedAt.IsZero() {
				in.Outbox[i].UpdatedAt = in.Feedback.CreatedAt
			}
			if in.Outbox[i].NextAttemptAt.IsZero() {
				in.Outbox[i].NextAttemptAt = in.Feedback.CreatedAt
			}
			if err := tx.Create(&in.Outbox[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *FeedbackStore) GetFeedbackByRequestID(ctx context.Context, requestID string) (model.Feedback, error) {
	var fb model.Feedback
	err := s.db.WithContext(ctx).Where("request_id = ?", requestID).First(&fb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Feedback{}, ErrFeedbackNotFound
	}
	return fb, err
}

func (s *FeedbackStore) CreateChallenge(ctx context.Context, challenge *model.FeedbackChallenge) error {
	return s.db.WithContext(ctx).Create(challenge).Error
}

func (s *FeedbackStore) GetChallenge(ctx context.Context, id string) (model.FeedbackChallenge, error) {
	var challenge model.FeedbackChallenge
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&challenge).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.FeedbackChallenge{}, ErrChallengeNotFound
	}
	return challenge, err
}

func (s *FeedbackStore) CountFeedbackByContentHash(ctx context.Context, hash string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Feedback{}).
		Where("content_hash = ? AND created_at >= ?", hash, since.UTC()).
		Count(&count).Error
	return count, err
}

func (s *FeedbackStore) CountFeedbackByEmailDomain(ctx context.Context, domain string, since time.Time) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Feedback{}).
		Where("email_domain = ? AND created_at >= ?", domain, since.UTC()).
		Count(&count).Error
	return count, err
}

func (s *FeedbackStore) GetFeedback(ctx context.Context, id string) (model.Feedback, error) {
	var fb model.Feedback
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&fb).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Feedback{}, ErrFeedbackNotFound
	}
	return fb, err
}

type FeedbackListFilter struct {
	Kind     string
	Status   string
	Query    string
	Page     int
	PageSize int
}

func (f FeedbackListFilter) apply(db *gorm.DB) *gorm.DB {
	if f.Kind != "" {
		db = db.Where("kind = ?", f.Kind)
	}
	if f.Status != "" {
		db = db.Where("status = ?", f.Status)
	}
	if f.Query != "" {
		pattern := "%" + escapeLike(f.Query) + "%"
		db = db.Where(`(title LIKE ? ESCAPE '\' OR content LIKE ? ESCAPE '\' OR nickname LIKE ? ESCAPE '\')`,
			pattern, pattern, pattern)
	}
	return db
}

func (s *FeedbackStore) ListFeedback(ctx context.Context, f FeedbackListFilter) ([]model.Feedback, int64, error) {
	var total int64
	if err := f.apply(s.db.WithContext(ctx).Model(&model.Feedback{})).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Feedback
	query := f.apply(s.db.WithContext(ctx).Model(&model.Feedback{}))
	if err := query.Order("created_at DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *FeedbackStore) ListEvents(ctx context.Context, feedbackID string) ([]model.FeedbackEvent, error) {
	var events []model.FeedbackEvent
	err := s.db.WithContext(ctx).Where("feedback_id = ?", feedbackID).Order("seq ASC").Find(&events).Error
	return events, err
}

type FeedbackUpdate struct {
	ID           string
	Version      int
	Status       string
	InternalNote *string
	PublicReply  *string
	Actor        string
	RecordEvent  bool
	Event        *model.FeedbackEvent
	Tokens       []model.SubscriptionToken
	Outbox       []model.MailOutbox
	Now          time.Time
}

// UpdateFeedback applies an optimistic-locked admin edit and queues any mail
// in the same transaction.
func (s *FeedbackStore) UpdateFeedback(ctx context.Context, u FeedbackUpdate) (model.Feedback, error) {
	var updated model.Feedback
	now := u.Now.UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current model.Feedback
		if err := tx.Where("id = ?", u.ID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFeedbackNotFound
			}
			return err
		}
		if current.Version != u.Version {
			return ErrVersionConflict
		}

		updates := map[string]any{
			"version":    current.Version + 1,
			"updated_at": now,
		}
		if u.Status != "" && u.Status != current.Status {
			updates["status"] = u.Status
			if u.Status == model.FeedbackStatusPending {
				updates["closed_at"] = nil
			} else if u.Status == model.FeedbackStatusCompleted || u.Status == model.FeedbackStatusRejected || u.Status == model.FeedbackStatusSpam {
				if current.ClosedAt == nil {
					updates["closed_at"] = now
				}
			}
		}
		if u.InternalNote != nil {
			updates["internal_note"] = *u.InternalNote
		}
		if u.PublicReply != nil {
			updates["public_reply"] = *u.PublicReply
		}

		result := tx.Model(&model.Feedback{}).Where("id = ? AND version = ?", u.ID, u.Version).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVersionConflict
		}

		if u.RecordEvent {
			var maxSeq int
			if err := tx.Model(&model.FeedbackEvent{}).Where("feedback_id = ?", u.ID).
				Select("COALESCE(MAX(seq), 0)").Scan(&maxSeq).Error; err != nil {
				return err
			}
			event := model.FeedbackEvent{}
			if u.Event != nil {
				event = *u.Event
			}
			event.FeedbackID = u.ID
			event.Seq = maxSeq + 1
			event.FromStatus = current.Status
			if event.ToStatus == "" {
				event.ToStatus = current.Status
			}
			if u.InternalNote != nil {
				event.InternalNote = *u.InternalNote
			}
			if u.PublicReply != nil {
				event.PublicReply = *u.PublicReply
			}
			event.Actor = u.Actor
			event.CreatedAt = now
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
		}

		for i := range u.Tokens {
			u.Tokens[i].FeedbackID = u.ID
			u.Tokens[i].CreatedAt = now
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&u.Tokens[i]).Error; err != nil {
				return err
			}
		}

		for i := range u.Outbox {
			u.Outbox[i].FeedbackID = u.ID
			u.Outbox[i].CreatedAt = now
			u.Outbox[i].UpdatedAt = now
			if u.Outbox[i].NextAttemptAt.IsZero() {
				u.Outbox[i].NextAttemptAt = now
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&u.Outbox[i]).Error; err != nil {
				return err
			}
		}

		return tx.Where("id = ?", u.ID).First(&updated).Error
	})
	return updated, err
}

// DeleteFeedback removes contact details, tokens, events and unsent mail while
// keeping a minimal audit row and already-sent delivery metadata.
func (s *FeedbackStore) DeleteFeedback(ctx context.Context, id, actor string, now time.Time) error {
	now = now.UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var fb model.Feedback
		if err := tx.Where("id = ?", id).First(&fb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFeedbackNotFound
			}
			return err
		}
		if err := tx.Where("feedback_id = ?", id).Delete(&model.FeedbackEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("feedback_id = ?", id).Delete(&model.SubscriptionToken{}).Error; err != nil {
			return err
		}
		if err := tx.Where("feedback_id = ? AND status IN ?", id, []string{
			model.MailStatusPending, model.MailStatusSending, model.MailStatusFailed, model.MailStatusCancelled,
		}).Delete(&model.MailOutbox{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.MailOutbox{}).Where("feedback_id = ?", id).
			Updates(map[string]any{"payload": "", "recipient": ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.FeedbackDeletion{FeedbackID: id, Actor: actor, CreatedAt: now}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&model.Feedback{}).Error
	})
}

func (s *FeedbackStore) CreateSession(ctx context.Context, session *model.AdminSession) error {
	return s.db.WithContext(ctx).Create(session).Error
}

// GetAdminUser returns the configured administrator account, or nil when none
// exists yet (fresh deployment, `biodays admin set-password` not run).
func (s *FeedbackStore) GetAdminUser(ctx context.Context) (*model.AdminUser, error) {
	var user model.AdminUser
	err := s.db.WithContext(ctx).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpsertAdminUser stores the single administrator account.
func (s *FeedbackStore) UpsertAdminUser(ctx context.Context, user *model.AdminUser) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Single-row semantics: replacing the account must not leave a second row.
		if err := tx.Where("1 = 1").Delete(&model.AdminUser{}).Error; err != nil {
			return err
		}
		return tx.Create(user).Error
	})
}

// GetAppSetting reads a runtime setting; empty string when absent.
func (s *FeedbackStore) GetAppSetting(ctx context.Context, key string) (string, error) {
	var setting model.AppSetting
	err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// SetAppSetting writes a runtime setting.
func (s *FeedbackStore) SetAppSetting(ctx context.Context, key, value string, now time.Time) error {
	setting := model.AppSetting{Key: key, Value: value, UpdatedAt: now.UTC()}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&setting).Error
}

func (s *FeedbackStore) GetSession(ctx context.Context, tokenHash string, now time.Time) (model.AdminSession, error) {
	var session model.AdminSession
	err := s.db.WithContext(ctx).Where("token_hash = ? AND expires_at > ?", tokenHash, now.UTC()).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AdminSession{}, ErrFeedbackNotFound
	}
	return session, err
}

func (s *FeedbackStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).Delete(&model.AdminSession{}).Error
}

func (s *FeedbackStore) CleanupExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("expires_at < ?", now.UTC()).Delete(&model.AdminSession{})
	return result.RowsAffected, result.Error
}

// RevokeAllSessions is used after restoring a backup: old cookies must not
// survive a rollback.
func (s *FeedbackStore) RevokeAllSessions(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&model.AdminSession{}).Error
}

// PauseOutboxAfterRestore parks every unfinished task as unknown, because a
// restore can roll a sent task back to pending and blind retries may duplicate
// mail. An administrator must review before retrying.
func (s *FeedbackStore) PauseOutboxAfterRestore(ctx context.Context, now time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Where("status IN ?", []string{model.MailStatusPending, model.MailStatusSending, model.MailStatusFailed}).
		Updates(map[string]any{
			"status":      model.MailStatusUnknown,
			"lease_until": nil,
			"last_error":  "database restored; verify delivery before retrying",
			"updated_at":  now.UTC(),
		})
	return result.RowsAffected, result.Error
}

// DeleteOtherCredentialSessions invalidates sessions after the configured
// username/hash changes. Call at startup and right after login.
func (s *FeedbackStore) DeleteOtherCredentialSessions(ctx context.Context, version string) error {
	return s.db.WithContext(ctx).Where("credential_version <> ?", version).Delete(&model.AdminSession{}).Error
}

func (s *FeedbackStore) CreateToken(ctx context.Context, token *model.SubscriptionToken, invalidatePurpose bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if invalidatePurpose {
			if err := tx.Where("feedback_id = ? AND purpose = ? AND consumed_at IS NULL",
				token.FeedbackID, token.Purpose).Delete(&model.SubscriptionToken{}).Error; err != nil {
				return err
			}
		}
		return tx.Create(token).Error
	})
}

func (s *FeedbackStore) GetToken(ctx context.Context, hash, purpose string) (model.SubscriptionToken, error) {
	var token model.SubscriptionToken
	err := s.db.WithContext(ctx).Where("token_hash = ? AND purpose = ?", hash, purpose).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.SubscriptionToken{}, ErrTokenInvalid
	}
	return token, err
}

func (s *FeedbackStore) InvalidateTokens(ctx context.Context, feedbackID, purpose string) error {
	return s.db.WithContext(ctx).Where("feedback_id = ? AND purpose = ? AND consumed_at IS NULL",
		feedbackID, purpose).Delete(&model.SubscriptionToken{}).Error
}

// ConsumeToken validates and burns a token atomically, returning its feedback.
func (s *FeedbackStore) ConsumeToken(ctx context.Context, hash, purpose string, now time.Time) (model.SubscriptionToken, model.Feedback, error) {
	now = now.UTC()
	var token model.SubscriptionToken
	var fb model.Feedback
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("token_hash = ? AND purpose = ?", hash, purpose).First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTokenInvalid
			}
			return err
		}
		if token.ConsumedAt != nil {
			return ErrTokenConsumed
		}
		if !token.ExpiresAt.After(now) {
			return ErrTokenExpired
		}
		result := tx.Model(&model.SubscriptionToken{}).
			Where("id = ? AND consumed_at IS NULL", token.ID).
			Update("consumed_at", now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrTokenConsumed
		}
		if err := tx.Where("id = ?", token.FeedbackID).First(&fb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFeedbackNotFound
			}
			return err
		}
		return nil
	})
	return token, fb, err
}

func (s *FeedbackStore) ClaimOutbox(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]model.MailOutbox, error) {
	now = now.UTC()
	var items []model.MailOutbox
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Model(&model.MailOutbox{}).
			Where("status = ? AND next_attempt_at <= ?", model.MailStatusPending, now).
			Order("next_attempt_at ASC").Limit(limit).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Model(&model.MailOutbox{}).Where("id IN ?", ids).Updates(map[string]any{
			"status":      model.MailStatusSending,
			"lease_until": now.Add(lease),
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Order("next_attempt_at ASC").Find(&items).Error
	})
	return items, err
}

// RecoverExpiredLeases marks tasks whose worker vanished as unknown; their
// delivery state cannot be assumed.
func (s *FeedbackStore) RecoverExpiredLeases(ctx context.Context, now time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Where("status = ? AND lease_until IS NOT NULL AND lease_until < ?", model.MailStatusSending, now.UTC()).
		Updates(map[string]any{
			"status":      model.MailStatusUnknown,
			"lease_until": nil,
			"updated_at":  now.UTC(),
			"last_error":  "worker lease expired; delivery state unknown",
		})
	return result.RowsAffected, result.Error
}

func (s *FeedbackStore) FinishOutbox(ctx context.Context, id string, acceptedAt, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":           model.MailStatusSent,
		"payload":          "",
		"lease_until":      nil,
		"smtp_accepted_at": acceptedAt.UTC(),
		"last_error":       "",
		"updated_at":       now.UTC(),
	}).Error
}

func (s *FeedbackStore) MarkOutboxUnknown(ctx context.Context, id, reason string, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":      model.MailStatusUnknown,
		"lease_until": nil,
		"last_error":  reason,
		"updated_at":  now.UTC(),
	}).Error
}

func (s *FeedbackStore) RetryOutboxLater(ctx context.Context, id, reason string, attempts int, next time.Time, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":          model.MailStatusFailed,
		"attempts":        attempts,
		"next_attempt_at": next.UTC(),
		"lease_until":     nil,
		"last_error":      reason,
		"updated_at":      now.UTC(),
	}).Error
}

func (s *FeedbackStore) MarkOutboxFailed(ctx context.Context, id, reason string, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":      model.MailStatusFailed,
		"lease_until": nil,
		"last_error":  reason,
		"updated_at":  now.UTC(),
	}).Error
}

func (s *FeedbackStore) CancelOutbox(ctx context.Context, id, reason string, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":      model.MailStatusCancelled,
		"payload":     "",
		"lease_until": nil,
		"last_error":  reason,
		"updated_at":  now.UTC(),
	}).Error
}

func (s *FeedbackStore) CancelPendingByFeedback(ctx context.Context, feedbackID string, types []string, reason string, now time.Time) error {
	query := s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Where("feedback_id = ? AND status IN ?", feedbackID, []string{model.MailStatusPending, model.MailStatusFailed})
	if len(types) > 0 {
		query = query.Where("type IN ?", types)
	}
	return query.Updates(map[string]any{
		"status":      model.MailStatusCancelled,
		"payload":     "",
		"updated_at":  now.UTC(),
		"last_error":  reason,
		"lease_until": nil,
	}).Error
}

func (s *FeedbackStore) UpdateOutboxPayload(ctx context.Context, id, payload string, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).Where("id = ? AND status = ?", id, model.MailStatusSending).
		Updates(map[string]any{"payload": payload, "updated_at": now.UTC()}).Error
}

func (s *FeedbackStore) ResetOutboxForRetry(ctx context.Context, id string, now time.Time) error {
	result := s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Where("id = ? AND status IN ?", id, []string{model.MailStatusFailed, model.MailStatusUnknown}).
		Updates(map[string]any{
			"status":          model.MailStatusPending,
			"next_attempt_at": now.UTC(),
			"lease_until":     nil,
			"updated_at":      now.UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOutboxNotFound
	}
	return nil
}

func (s *FeedbackStore) GetOutbox(ctx context.Context, id string) (model.MailOutbox, error) {
	var item model.MailOutbox
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MailOutbox{}, ErrOutboxNotFound
	}
	return item, err
}

type OutboxFilter struct {
	Type     string
	Status   string
	Page     int
	PageSize int
}

func (s *FeedbackStore) ListOutbox(ctx context.Context, f OutboxFilter) ([]model.MailOutbox, int64, error) {
	query := s.db.WithContext(ctx).Model(&model.MailOutbox{})
	if f.Type != "" {
		query = query.Where("type = ?", f.Type)
	}
	if f.Status != "" {
		query = query.Where("status = ?", f.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.MailOutbox
	err := query.Order("created_at DESC").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&items).Error
	return items, total, err
}

func (s *FeedbackStore) ListOutboxByFeedback(ctx context.Context, feedbackID string) ([]model.MailOutbox, error) {
	var items []model.MailOutbox
	err := s.db.WithContext(ctx).Where("feedback_id = ?", feedbackID).Order("created_at ASC").Find(&items).Error
	return items, err
}

func (s *FeedbackStore) RecentOutboxErrors(ctx context.Context, limit int) ([]model.MailOutbox, error) {
	var items []model.MailOutbox
	err := s.db.WithContext(ctx).
		Where("status IN ?", []string{model.MailStatusFailed, model.MailStatusUnknown}).
		Order("updated_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (s *FeedbackStore) OutboxCounts(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string
		Total  int64
	}
	var rows []row
	if err := s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Select("status, COUNT(*) AS total").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, r := range rows {
		counts[r.Status] = r.Total
	}
	return counts, nil
}

// ConsumeRateLimit increments a fixed-window bucket and reports whether the
// request stays within limit. Keys must already be HMACed by the caller.
func (s *FeedbackStore) ConsumeRateLimit(ctx context.Context, keyHash string, windowStart, expiresAt time.Time, limit int) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	allowed := true
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(
			`INSERT INTO abuse_buckets (key_hash, window_start, hits, expires_at) VALUES (?, ?, 1, ?)
			 ON CONFLICT(key_hash, window_start) DO UPDATE SET hits = abuse_buckets.hits + 1, expires_at = excluded.expires_at`,
			keyHash, windowStart.UTC(), expiresAt.UTC(),
		)
		if res.Error != nil {
			return res.Error
		}
		var hits int
		if err := tx.Raw(`SELECT hits FROM abuse_buckets WHERE key_hash = ? AND window_start = ?`,
			keyHash, windowStart.UTC()).Scan(&hits).Error; err != nil {
			return err
		}
		if hits > limit {
			allowed = false
		}
		return nil
	})
	return allowed, err
}

type CleanupReport struct {
	Sessions          int64
	ConfirmTokens     int64
	AbuseBuckets      int64
	Challenges        int64
	SentOutbox        int64
	ContactScrubbed   int64
	TokensInvalidated int64
}

// Cleanup applies the documented retention policy. It is safe to run repeatedly.
func (s *FeedbackStore) Cleanup(ctx context.Context, now time.Time) (CleanupReport, error) {
	now = now.UTC()
	var report CleanupReport
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("expires_at < ?", now).Delete(&model.AdminSession{})
		report.Sessions, _ = res.RowsAffected, res.Error
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("purpose = ? AND expires_at < ?", model.TokenPurposeConfirm, now).Delete(&model.SubscriptionToken{})
		report.ConfirmTokens = res.RowsAffected
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("expires_at < ?", now).Delete(&model.AbuseBucket{})
		report.AbuseBuckets = res.RowsAffected
		if res.Error != nil {
			return res.Error
		}

		res = tx.Where("expires_at < ?", now).Delete(&model.FeedbackChallenge{})
		report.Challenges = res.RowsAffected
		if res.Error != nil {
			return res.Error
		}

		res = tx.Model(&model.MailOutbox{}).Where("status = ? AND payload <> ''", model.MailStatusSent).Update("payload", "")
		report.SentOutbox = res.RowsAffected
		if res.Error != nil {
			return res.Error
		}
		if err := tx.Where("status IN ? AND updated_at < ?",
			[]string{model.MailStatusSent, model.MailStatusCancelled, model.MailStatusFailed, model.MailStatusUnknown},
			now.AddDate(0, 0, -30)).Delete(&model.MailOutbox{}).Error; err != nil {
			return err
		}

		cutoff := now.AddDate(0, 0, -90)
		var closed []model.Feedback
		if err := tx.Where("closed_at IS NOT NULL AND closed_at < ? AND (email <> '' OR nickname <> '')", cutoff).
			Find(&closed).Error; err != nil {
			return err
		}
		for _, fb := range closed {
			if err := tx.Model(&model.Feedback{}).Where("id = ?", fb.ID).Updates(map[string]any{
				"email":               "",
				"email_domain":        "",
				"nickname":            "",
				"subscription_status": model.SubscriptionUnsubscribed,
				"unsubscribed_at":     now,
			}).Error; err != nil {
				return err
			}
			res = tx.Where("feedback_id = ?", fb.ID).Delete(&model.SubscriptionToken{})
			report.TokensInvalidated += res.RowsAffected
			if res.Error != nil {
				return res.Error
			}
			report.ContactScrubbed++
		}
		return nil
	})
	return report, err
}

type ConfirmOutcome string

const (
	ConfirmOutcomeConfirmed    ConfirmOutcome = "confirmed"
	ConfirmOutcomeAlready      ConfirmOutcome = "already_confirmed"
	ConfirmOutcomeUnsubscribed ConfirmOutcome = "unsubscribed"
)

type ConfirmInput struct {
	TokenHash  string
	Now        time.Time
	UnsubToken *model.SubscriptionToken
	Outbox     []model.MailOutbox
}

// ConfirmSubscription consumes the token, flips the subscription and queues
// the unsubscribe token and any catch-up status mail in one transaction.
func (s *FeedbackStore) ConfirmSubscription(ctx context.Context, in ConfirmInput) (model.Feedback, ConfirmOutcome, error) {
	now := in.Now.UTC()
	var fb model.Feedback
	outcome := ConfirmOutcomeConfirmed
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token model.SubscriptionToken
		if err := tx.Where("token_hash = ? AND purpose = ?", in.TokenHash, model.TokenPurposeConfirm).
			First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTokenInvalid
			}
			return err
		}
		if err := tx.Where("id = ?", token.FeedbackID).First(&fb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFeedbackNotFound
			}
			return err
		}
		if token.ConsumedAt != nil {
			if fb.SubscriptionStatus == model.SubscriptionUnsubscribed {
				outcome = ConfirmOutcomeUnsubscribed
			} else {
				outcome = ConfirmOutcomeAlready
			}
			return nil
		}
		if !token.ExpiresAt.After(now) {
			return ErrTokenExpired
		}
		if err := tx.Model(&model.SubscriptionToken{}).Where("id = ? AND consumed_at IS NULL", token.ID).
			Update("consumed_at", now).Error; err != nil {
			return err
		}
		if fb.SubscriptionStatus == model.SubscriptionUnsubscribed || fb.UnsubscribedAt != nil {
			outcome = ConfirmOutcomeUnsubscribed
			return nil
		}
		updates := map[string]any{
			"subscription_status": model.SubscriptionConfirmed,
			"confirmed_at":        now,
			"updated_at":          now,
		}
		if err := tx.Model(&model.Feedback{}).Where("id = ?", fb.ID).Updates(updates).Error; err != nil {
			return err
		}
		if in.UnsubToken != nil {
			in.UnsubToken.FeedbackID = fb.ID
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(in.UnsubToken).Error; err != nil {
				return err
			}
		}
		for i := range in.Outbox {
			in.Outbox[i].FeedbackID = fb.ID
			in.Outbox[i].CreatedAt = now
			in.Outbox[i].UpdatedAt = now
			if in.Outbox[i].NextAttemptAt.IsZero() {
				in.Outbox[i].NextAttemptAt = now
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&in.Outbox[i]).Error; err != nil {
				return err
			}
		}
		return tx.Where("id = ?", fb.ID).First(&fb).Error
	})
	return fb, outcome, err
}

type UnsubscribeOutcome string

const (
	UnsubscribeOutcomeUnsubscribed UnsubscribeOutcome = "unsubscribed"
	UnsubscribeOutcomeAlready      UnsubscribeOutcome = "already_unsubscribed"
)

// Unsubscribe is idempotent and cancels not-yet-sent user mail immediately.
func (s *FeedbackStore) Unsubscribe(ctx context.Context, hash string, now time.Time) (model.Feedback, UnsubscribeOutcome, error) {
	now = now.UTC()
	var fb model.Feedback
	outcome := UnsubscribeOutcomeUnsubscribed
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var token model.SubscriptionToken
		if err := tx.Where("token_hash = ? AND purpose = ?", hash, model.TokenPurposeUnsubscribe).
			First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTokenInvalid
			}
			return err
		}
		if err := tx.Where("id = ?", token.FeedbackID).First(&fb).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFeedbackNotFound
			}
			return err
		}
		if token.ConsumedAt != nil || fb.SubscriptionStatus == model.SubscriptionUnsubscribed {
			if token.ConsumedAt == nil {
				if err := tx.Model(&model.SubscriptionToken{}).Where("id = ?", token.ID).
					Update("consumed_at", now).Error; err != nil {
					return err
				}
			}
			outcome = UnsubscribeOutcomeAlready
			return nil
		}
		if err := tx.Model(&model.SubscriptionToken{}).Where("id = ? AND consumed_at IS NULL", token.ID).
			Update("consumed_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Feedback{}).Where("id = ?", fb.ID).Updates(map[string]any{
			"subscription_status": model.SubscriptionUnsubscribed,
			"unsubscribed_at":     now,
			"updated_at":          now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.MailOutbox{}).
			Where("feedback_id = ? AND type = ? AND status IN ?", fb.ID, model.MailTypeUserStatusUpdate,
				[]string{model.MailStatusPending, model.MailStatusFailed}).
			Updates(map[string]any{
				"status":      model.MailStatusCancelled,
				"payload":     "",
				"lease_until": nil,
				"last_error":  "subscription unsubscribed",
				"updated_at":  now,
			}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", fb.ID).First(&fb).Error
	})
	return fb, outcome, err
}

// QueueConfirmation replaces any outstanding confirmation token and queues the
// matching email atomically.
func (s *FeedbackStore) QueueConfirmation(ctx context.Context, token *model.SubscriptionToken, outbox *model.MailOutbox, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("feedback_id = ? AND purpose = ? AND consumed_at IS NULL",
			token.FeedbackID, model.TokenPurposeConfirm).Delete(&model.SubscriptionToken{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.MailOutbox{}).
			Where("feedback_id = ? AND type = ? AND status IN ?", token.FeedbackID,
				model.MailTypeSubscriptionConfirm, []string{model.MailStatusPending, model.MailStatusFailed}).
			Updates(map[string]any{
				"status": model.MailStatusCancelled, "payload": "", "last_error": "superseded by resend",
				"lease_until": nil, "updated_at": now.UTC(),
			}).Error; err != nil {
			return err
		}
		if err := tx.Create(token).Error; err != nil {
			return err
		}
		outbox.FeedbackID = token.FeedbackID
		outbox.CreatedAt = now
		outbox.UpdatedAt = now
		if outbox.NextAttemptAt.IsZero() {
			outbox.NextAttemptAt = now
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(outbox).Error
	})
}

// CancelOtherConfirmTasks supersedes sibling confirmation jobs after the
// worker regenerates an expired token.
func (s *FeedbackStore) CancelOtherConfirmTasks(ctx context.Context, feedbackID, keepID, reason string, now time.Time) error {
	return s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Where("feedback_id = ? AND type = ? AND id <> ? AND status IN ?", feedbackID,
			model.MailTypeSubscriptionConfirm, keepID,
			[]string{model.MailStatusPending, model.MailStatusFailed}).
		Updates(map[string]any{
			"status": model.MailStatusCancelled, "payload": "", "last_error": reason,
			"lease_until": nil, "updated_at": now.UTC(),
		}).Error
}

func (s *FeedbackStore) InsertOutbox(ctx context.Context, item *model.MailOutbox, now time.Time) error {
	item.CreatedAt = now
	item.UpdatedAt = now
	if item.NextAttemptAt.IsZero() {
		item.NextAttemptAt = now
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error
}

func (s *FeedbackStore) RateLimitCount(ctx context.Context, keyHash string, windowStart time.Time) (int, error) {
	var hits int
	err := s.db.WithContext(ctx).Raw(
		`SELECT hits FROM abuse_buckets WHERE key_hash = ? AND window_start = ?`,
		keyHash, windowStart.UTC()).Scan(&hits).Error
	return hits, err
}

func (s *FeedbackStore) ClearRateLimit(ctx context.Context, keyHash string, windowStart time.Time) error {
	return s.db.WithContext(ctx).Where("key_hash = ? AND window_start = ?", keyHash, windowStart.UTC()).
		Delete(&model.AbuseBucket{}).Error
}

// BackfillOutboxRecipient repairs tasks queued while SMTP/notify address was
// not yet configured.
func (s *FeedbackStore) BackfillOutboxRecipient(ctx context.Context, mailType, recipient string, now time.Time) (int64, error) {
	if recipient == "" {
		return 0, nil
	}
	result := s.db.WithContext(ctx).Model(&model.MailOutbox{}).
		Where("type = ? AND recipient = '' AND status = ?", mailType, model.MailStatusPending).
		Updates(map[string]any{"recipient": recipient, "updated_at": now.UTC()})
	return result.RowsAffected, result.Error
}
