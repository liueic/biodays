package mailer_test

import (
	"strings"
	"testing"
	"time"

	"biodays/server/internal/mailer"
	"biodays/server/internal/model"

	"github.com/stretchr/testify/require"
)

func TestRendererEscapesUserContentInHTML(t *testing.T) {
	renderer := mailer.NewRenderer("biodays", "https://biodays.test")
	msg, err := renderer.Render(model.MailTypeAdminNewFeedback, mailer.TemplateData{
		FeedbackID: "abc",
		Title:      `<script>alert("x")</script>`,
		Content:    `<img src=x onerror=alert(1)>`,
		AdminURL:   "https://biodays.test/admin/feedback/abc",
	})
	require.NoError(t, err)
	require.NotContains(t, msg.HTML, "<script>")
	require.NotContains(t, msg.HTML, "<img src=x")
	require.Contains(t, msg.HTML, "&lt;script&gt;")
	require.NotContains(t, msg.Subject, "\n")
}

func TestSMTPSenderValidation(t *testing.T) {
	base := mailer.SMTPConfig{
		Host: "smtp.example.com", Port: 587, TLSMode: "starttls",
		From: "notify@example.com", Timeout: time.Second,
	}
	_, err := mailer.NewSMTPSender(base)
	require.NoError(t, err)

	implicit := base
	implicit.TLSMode = "implicit_tls"
	implicit.Port = 465
	_, err = mailer.NewSMTPSender(implicit)
	require.NoError(t, err)

	cases := map[string]func(*mailer.SMTPConfig){
		"host":  func(c *mailer.SMTPConfig) { c.Host = "" },
		"port":  func(c *mailer.SMTPConfig) { c.Port = 0 },
		"tls":   func(c *mailer.SMTPConfig) { c.TLSMode = "plain" },
		"from":  func(c *mailer.SMTPConfig) { c.From = "" },
		"email": func(c *mailer.SMTPConfig) { c.From = "not-an-email" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := base
			mutate(&cfg)
			_, err := mailer.NewSMTPSender(cfg)
			require.Error(t, err)
			require.False(t, strings.Contains(err.Error(), "password"))
		})
	}
}
