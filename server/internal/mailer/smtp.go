package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// Message is a rendered email ready for delivery.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
	Headers map[string]string
}

// DeliveryError classifies failures for the outbox worker. Permanent errors
// are never retried; ambiguous errors mean the server may have accepted the
// message, so the task is parked as unknown for a human to decide.
type DeliveryError struct {
	Permanent bool
	Ambiguous bool
	Err       error
}

func (e *DeliveryError) Error() string { return e.Err.Error() }
func (e *DeliveryError) Unwrap() error { return e.Err }

func permanent(err error) error {
	return &DeliveryError{Permanent: true, Err: err}
}

func temporary(err error) error {
	return &DeliveryError{Err: err}
}

func ambiguous(err error) error {
	return &DeliveryError{Ambiguous: true, Err: err}
}

type SMTPConfig struct {
	Host     string
	Port     int
	TLSMode  string
	Username string
	Password string
	From     string
	Timeout  time.Duration
	SiteName string
}

// SMTPSender sends through a single configured relay with certificate and
// protocol-read timeouts. It never uses user addresses as the envelope sender.
type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.Host == "" {
		return nil, errors.New("smtp host is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("smtp port %d is invalid", cfg.Port)
	}
	if cfg.TLSMode != "starttls" && cfg.TLSMode != "implicit_tls" {
		return nil, fmt.Errorf("smtp tls mode %q is invalid", cfg.TLSMode)
	}
	if cfg.From == "" {
		return nil, errors.New("smtp from address is required")
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		return nil, fmt.Errorf("smtp from address: %w", err)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &SMTPSender{cfg: cfg}, nil
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return permanent(fmt.Errorf("invalid recipient: %w", err))
	}
	// The sender address is fixed by configuration; user input never reaches it.
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return permanent(fmt.Errorf("invalid configured from address: %w", err))
	}

	options := []gomail.Option{
		gomail.WithTimeout(s.cfg.Timeout),
		gomail.WithTLSConfig(&tls.Config{
			ServerName: s.cfg.Host,
			MinVersion: tls.VersionTLS12,
		}),
	}
	if s.cfg.TLSMode == "implicit_tls" {
		options = append(options, gomail.WithSSL(), gomail.WithPort(s.cfg.Port))
	} else {
		options = append(options, gomail.WithPort(s.cfg.Port), gomail.WithTLSPolicy(gomail.TLSMandatory))
	}
	if s.cfg.Username != "" {
		options = append(options,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(s.cfg.Username),
			gomail.WithPassword(s.cfg.Password),
		)
	}

	client, err := gomail.NewClient(s.cfg.Host, options...)
	if err != nil {
		return permanent(fmt.Errorf("smtp client: %w", err))
	}

	dialCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	if err := client.DialWithContext(dialCtx); err != nil {
		return classifyDialError(err)
	}

	gm := gomail.NewMsg(gomail.WithNoDefaultUserAgent())
	if err := gm.From(from.Address); err != nil {
		_ = client.Close()
		return permanent(err)
	}
	if err := gm.To(msg.To); err != nil {
		_ = client.Close()
		return permanent(err)
	}
	gm.Subject(sanitizeHeader(msg.Subject))
	gm.SetBodyString(gomail.TypeTextPlain, msg.Text)
	gm.AddAlternativeString(gomail.TypeTextHTML, msg.HTML)
	for key, value := range msg.Headers {
		gm.SetGenHeader(gomail.Header(key), sanitizeHeader(value))
	}

	sendErr := client.Send(gm)
	// Close errors after a successful send only affect the QUIT exchange; the
	// message was already accepted, so do not retry it.
	_ = client.Close()
	if sendErr == nil {
		return nil
	}
	return classifySendError(sendErr)
}

func classifyDialError(err error) error {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return temporary(err)
	}
	lower := strings.ToLower(err.Error())
	for _, marker := range []string{"certificate", "x509", "authentication", "auth failed", "535", "530", "unknown authority"} {
		if strings.Contains(lower, marker) {
			return permanent(err)
		}
	}
	return temporary(err)
}

func classifySendError(err error) error {
	var sendErr *gomail.SendError
	if errors.As(err, &sendErr) {
		switch sendErr.Reason {
		case gomail.ErrWriteContent, gomail.ErrSMTPDataClose, gomail.ErrSMTPData, gomail.ErrConnCheck, gomail.ErrAmbiguous:
			// The DATA phase may have completed; the server's verdict is unknown.
			return ambiguous(err)
		}
		if sendErr.IsTemp() {
			return temporary(err)
		}
		return permanent(err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ambiguous(err)
	}
	return temporary(err)
}

func sanitizeHeader(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
}
