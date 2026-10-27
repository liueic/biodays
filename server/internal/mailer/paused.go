package mailer

import (
	"context"
	"errors"
)

// ErrMailPaused is returned by PausedSender when delivery is switched off or
// blocked by configuration. It is a temporary condition by classification:
// the worker retries the task later, exactly like the pre-settings behaviour
// of not running a worker at all.
var ErrMailPaused = errors.New("mail delivery is paused")

// PausedSender occupies the sender slot while SMTP is disabled or its
// configuration is incomplete, keeping queued tasks pending instead of
// failing them.
type PausedSender struct {
	Reason string
}

func (p PausedSender) Send(context.Context, Message) error {
	if p.Reason != "" {
		return temporary(errors.New(p.Reason))
	}
	return ErrMailPaused
}

// Interface guard: PausedSender must satisfy the worker's Sender contract.
var _ Sender = PausedSender{}
