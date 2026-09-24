// Package email is the outbound email abstraction: a Sender interface, the
// SMTP implementation, a logging sender for local runs without SMTP, a
// recorder for tests and the embedded message templates.
package email

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
)

// Message is one outbound email. Text is required; HTML is optional and is
// sent as a multipart/alternative when present.
type Message struct {
	To      []string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers messages. Application code depends on this interface,
// never on a provider.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// NewSender builds the configured sender.
func NewSender(cfg config.EmailConfig, logger *slog.Logger) (Sender, error) {
	switch cfg.Provider {
	case "smtp":
		return NewSMTPSender(cfg.SMTP, cfg.From), nil
	case "log":
		return &LogSender{Logger: logger}, nil
	}
	return nil, fmt.Errorf("email: unknown provider %q", cfg.Provider)
}

// LogSender writes messages to the log instead of delivering them.
type LogSender struct {
	Logger *slog.Logger
}

// Send logs that a message would have been sent. Only metadata is logged:
// bodies carry verification and reset links and must never reach the logs.
func (s *LogSender) Send(ctx context.Context, msg Message) error {
	s.Logger.InfoContext(ctx, "email (log sender)", "to", msg.To, "subject", msg.Subject, "text_bytes", len(msg.Text), "html_bytes", len(msg.HTML))
	return nil
}

// Recorder captures messages for tests.
type Recorder struct {
	mu       sync.Mutex
	messages []Message
	// Err, when set, is returned by Send instead of recording.
	Err error
}

// Send records msg.
func (r *Recorder) Send(ctx context.Context, msg Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.messages = append(r.messages, msg)
	return nil
}

// Messages returns a copy of everything sent so far.
func (r *Recorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.messages...)
}

// Fail makes subsequent Send calls return err (nil restores delivery).
func (r *Recorder) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Err = err
}

// Reset forgets recorded messages.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = nil
}
