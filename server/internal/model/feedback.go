package model

import "time"

const (
	FeedbackKindTopic    = "topic"
	FeedbackKindResource = "resource"
	FeedbackKindFeature  = "feature"
	FeedbackKindIssue    = "issue"
)

var FeedbackKinds = []string{FeedbackKindTopic, FeedbackKindResource, FeedbackKindFeature, FeedbackKindIssue}

const (
	FeedbackStatusPending   = "pending"
	FeedbackStatusReviewing = "reviewing"
	FeedbackStatusAccepted  = "accepted"
	FeedbackStatusCompleted = "completed"
	FeedbackStatusRejected  = "rejected"
	FeedbackStatusSpam      = "spam"
)

var FeedbackStatuses = []string{
	FeedbackStatusPending, FeedbackStatusReviewing, FeedbackStatusAccepted,
	FeedbackStatusCompleted, FeedbackStatusRejected, FeedbackStatusSpam,
}

// NotifiableStatuses are the only status changes a confirmed subscriber hears about.
var NotifiableStatuses = []string{FeedbackStatusAccepted, FeedbackStatusCompleted, FeedbackStatusRejected}

func IsNotifiableStatus(status string) bool {
	return status == FeedbackStatusAccepted || status == FeedbackStatusCompleted || status == FeedbackStatusRejected
}

const (
	SubscriptionNone                = "none"
	SubscriptionPendingConfirmation = "pending_confirmation"
	SubscriptionConfirmed           = "confirmed"
	SubscriptionUnsubscribed        = "unsubscribed"
)

const (
	TokenPurposeConfirm     = "confirm"
	TokenPurposeUnsubscribe = "unsubscribe"
)

const (
	VerificationMethodPow       = "pow"
	VerificationMethodTurnstile = "turnstile"
	VerificationMethodNone      = "none"
)

const (
	SpamVerdictAllow  = "allow"
	SpamVerdictReview = "review"
	SpamVerdictSpam   = "spam"
)

const (
	SpamSourceHeuristic = "heuristic"
	SpamSourceAI        = "ai"
	SpamSourceAIError   = "ai_error"
	SpamSourceAIBudget  = "ai_budget"
)

const (
	MailTypeAdminNewFeedback    = "admin_new_feedback"
	MailTypeSubscriptionConfirm = "subscription_confirm"
	MailTypeUserStatusUpdate    = "user_status_update"
	MailTypeSMTPTest            = "smtp_test"
)

const (
	MailStatusPending   = "pending"
	MailStatusSending   = "sending"
	MailStatusSent      = "sent"
	MailStatusFailed    = "failed"
	MailStatusUnknown   = "unknown"
	MailStatusCancelled = "cancelled"
)

// Feedback is one anonymous submission. Contact details live only in this
// database; story IDs are plain strings so the two databases stay independent.
type Feedback struct {
	ID                 string     `gorm:"primaryKey;size:32"`
	RequestID          string     `gorm:"uniqueIndex;not null;size:64"`
	RequestBodyHash    string     `gorm:"not null;size:64"`
	Kind               string     `gorm:"not null;size:16;index:idx_feedback_kind_created,priority:1"`
	Title              string     `gorm:"not null;size:160"`
	Content            string     `gorm:"type:text;not null"`
	SourceURL          string     `gorm:"size:2048;not null;default:''"`
	StoryID            string     `gorm:"size:32;not null;default:''"`
	Nickname           string     `gorm:"size:160;not null;default:''"`
	Email              string     `gorm:"size:254;not null;default:''"`
	Status             string     `gorm:"not null;size:16;index:idx_feedback_status_created,priority:1"`
	InternalNote       string     `gorm:"type:text;not null;default:''"`
	PublicReply        string     `gorm:"type:text;not null;default:''"`
	Version            int        `gorm:"not null;default:1"`
	SubscriptionStatus string     `gorm:"not null;size:32;default:none"`
	ConfirmationEmail  string     `gorm:"not null;size:16;default:none"`
	ConsentVersion     string     `gorm:"size:32;not null;default:''"`
	ConsentedAt        *time.Time `gorm:"index"`
	ConfirmedAt        *time.Time `gorm:"index"`
	UnsubscribedAt     *time.Time `gorm:"index"`
	ContentHash        string     `gorm:"not null;size:64;default:'';index:idx_feedback_content_hash"`
	EmailDomain        string     `gorm:"not null;size:254;default:'';index:idx_feedback_email_domain"`
	SpamScore          int        `gorm:"not null;default:0"`
	SpamVerdict        string     `gorm:"not null;size:16;default:''"`
	SpamSignals        string     `gorm:"not null;size:512;default:''"`
	SpamSource         string     `gorm:"not null;size:32;default:''"`
	SpamModel          string     `gorm:"not null;size:128;default:''"`
	SpamConfidence     float64    `gorm:"not null;default:0"`
	VerificationMethod string     `gorm:"not null;size:16;default:''"`
	VerifiedAt         *time.Time `gorm:"index"`
	CreatedAt          time.Time  `gorm:"not null;index:idx_feedback_status_created,priority:2,sort:desc;index:idx_feedback_kind_created,priority:2,sort:desc;index:idx_feedback_created"`
	UpdatedAt          time.Time  `gorm:"not null"`
	ClosedAt           *time.Time `gorm:"index"`
}

// FeedbackEvent records every status transition. Notes are snapshots so the
// audit trail survives later edits to the feedback row.
type FeedbackEvent struct {
	ID           uint      `gorm:"primaryKey"`
	FeedbackID   string    `gorm:"not null;size:32;index:idx_feedback_events_feedback;uniqueIndex:idx_feedback_events_seq,priority:1"`
	Seq          int       `gorm:"not null;uniqueIndex:idx_feedback_events_seq,priority:2"`
	FromStatus   string    `gorm:"not null;size:16"`
	ToStatus     string    `gorm:"not null;size:16"`
	InternalNote string    `gorm:"type:text;not null;default:''"`
	PublicReply  string    `gorm:"type:text;not null;default:''"`
	Actor        string    `gorm:"not null;size:32"`
	CreatedAt    time.Time `gorm:"not null"`
}

type AdminSession struct {
	TokenHash         string    `gorm:"primaryKey;size:64"`
	CredentialVersion string    `gorm:"not null;size:64;index"`
	CSRFHash          string    `gorm:"not null;size:64"`
	CreatedAt         time.Time `gorm:"not null"`
	ExpiresAt         time.Time `gorm:"not null;index:idx_admin_sessions_expires"`
}

// SubscriptionToken stores only a hash. Purpose separates confirmation from
// unsubscribe links; both are single-use and tied to one feedback row.
type SubscriptionToken struct {
	ID               uint       `gorm:"primaryKey"`
	TokenHash        string     `gorm:"not null;uniqueIndex;size:64"`
	FeedbackID       string     `gorm:"not null;size:32;index:idx_subscription_tokens_feedback"`
	Purpose          string     `gorm:"not null;size:16;index"`
	EmailFingerprint string     `gorm:"not null;size:64"`
	ExpiresAt        time.Time  `gorm:"not null;index:idx_subscription_tokens_expires"`
	ConsumedAt       *time.Time `gorm:"index"`
	CreatedAt        time.Time  `gorm:"not null"`
}

// MailOutbox is the durable mail queue. Payload holds the rendered message and
// any sensitive link; it is cleared as soon as SMTP accepts the message.
type MailOutbox struct {
	ID             string     `gorm:"primaryKey;size:32"`
	FeedbackID     string     `gorm:"not null;size:32;index"`
	EventID        *uint      `gorm:"index"`
	Type           string     `gorm:"not null;size:32;index"`
	EventKey       string     `gorm:"not null;uniqueIndex;size:128"`
	Recipient      string     `gorm:"not null;size:254"`
	Template       string     `gorm:"not null;size:32"`
	Payload        string     `gorm:"type:text"`
	Status         string     `gorm:"not null;size:16;index:idx_outbox_claim,priority:1"`
	Attempts       int        `gorm:"not null;default:0"`
	NextAttemptAt  time.Time  `gorm:"not null;index:idx_outbox_claim,priority:2"`
	LeaseUntil     *time.Time `gorm:"index"`
	LastError      string     `gorm:"type:text;not null;default:''"`
	SMTPAcceptedAt *time.Time `gorm:"index"`
	CreatedAt      time.Time  `gorm:"not null"`
	UpdatedAt      time.Time  `gorm:"not null"`
}

// AdminUser is the single administrator account, stored in the database and
// managed with `biodays admin set-password`. Environment variables only seed
// it on first boot for backward compatibility.
type AdminUser struct {
	Username     string    `gorm:"primaryKey;size:64"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// AppSetting holds small runtime-generated secrets (e.g. the abuse HMAC
// key) so a deployment works without hand-crafted environment values.
type AppSetting struct {
	Key       string    `gorm:"primaryKey;size:64"`
	Value     string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// AbuseBucket is a fixed-window counter keyed by HMAC, never by raw IP/email.
type AbuseBucket struct {
	ID          uint      `gorm:"primaryKey"`
	KeyHash     string    `gorm:"not null;size:64;uniqueIndex:idx_abuse_key_window,priority:1"`
	WindowStart time.Time `gorm:"not null;uniqueIndex:idx_abuse_key_window,priority:2"`
	// Hits is deliberately not named "count": bare count is an aggregate
	// function name in PostgreSQL and raw SQL would need quoting everywhere.
	Hits      int       `gorm:"not null;default:0"`
	ExpiresAt time.Time `gorm:"not null;index:idx_abuse_expires"`
}

// FeedbackChallenge is a single-use proof-of-work challenge bound to one
// client IP. Only HMACs are stored; the raw IP never reaches the database.
type FeedbackChallenge struct {
	ID         string     `gorm:"primaryKey;size:32"`
	IPHash     string     `gorm:"not null;size:64;index:idx_feedback_challenges_ip_expires,priority:1"`
	Salt       string     `gorm:"not null;size:64"`
	Signature  string     `gorm:"not null;size:64"`
	Difficulty int        `gorm:"not null;default:4"`
	MinSeconds int        `gorm:"not null;default:3"`
	IssuedAt   time.Time  `gorm:"not null"`
	ExpiresAt  time.Time  `gorm:"not null;index:idx_feedback_challenges_ip_expires,priority:2;index:idx_feedback_challenges_expires"`
	ConsumedAt *time.Time `gorm:"index"`
	FeedbackID string     `gorm:"not null;size:32;default:''"`
}

// FeedbackDeletion is the minimal post-deletion audit record: no content and
// no contact details.
type FeedbackDeletion struct {
	ID         uint      `gorm:"primaryKey"`
	FeedbackID string    `gorm:"not null;size:32;index"`
	Actor      string    `gorm:"not null;size:32"`
	CreatedAt  time.Time `gorm:"not null"`
}
