package service

import (
	"biodays/server/internal/antispam"
	"biodays/server/internal/settings"
)

// managerSource adapts *settings.Manager to the SettingsSource the feedback
// flow consumes. Each Effective call flattens the current atomic snapshot.
type managerSource struct {
	manager *settings.Manager
	siteURL string
}

func NewManagerSettingsSource(manager *settings.Manager, siteURL string) SettingsSource {
	return managerSource{manager: manager, siteURL: siteURL}
}

func (m managerSource) Effective() EffectiveSettings {
	snap := m.manager.Snapshot()
	doc := snap.Document

	missing := make([]string, 0, 4)
	if doc.SMTP.Enabled {
		missing = append(missing, doc.SMTP.MissingConfig()...)
	}

	var verifier antispam.TurnstileVerifier
	if snap.Turnstile != nil {
		verifier = snap.Turnstile
	}

	return EffectiveSettings{
		Enabled:              true, // the subsystem only exists when enabled
		SubscriptionsEnabled: doc.Subscriptions,
		SMTPEnabled:          doc.SMTP.Enabled,
		SMTPConfigured:       len(missing) == 0,
		SMTPMissing:          missing,
		SMTPError:            snap.SMTPError,
		SiteURL:              m.siteURL,
		NotifyTo:             doc.SMTP.NotifyTo,
		RateLimits:           doc.RateLimits,
		TurnstileEnabled:     doc.Turnstile.Enabled,
		TurnstileSiteKey:     doc.Turnstile.SiteKey,
		TurnstileVerifier:    verifier,
		SpamEnabled:          doc.Spam.Enabled,
		SpamAIAssisted:       doc.AISpam.Enabled && snap.AIToken != "",
		SpamJudge:            snap.SpamJudge,
	}
}
