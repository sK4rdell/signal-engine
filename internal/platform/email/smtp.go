package email

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"betemplate/internal/platform/config"
)

// SMTPSender delivers over SMTP with opportunistic STARTTLS and optional
// PLAIN authentication. Development targets Mailpit (no TLS, no auth); the
// same code talks to a real provider once host, port, user and password are
// configured. With RequireTLS the sender refuses to continue, and in
// particular refuses to send credentials, when STARTTLS is not offered.
type SMTPSender struct {
	cfg  config.SMTPConfig
	from string
}

// NewSMTPSender builds an SMTP sender.
func NewSMTPSender(cfg config.SMTPConfig, from string) *SMTPSender {
	return &SMTPSender{cfg: cfg, from: from}
}

// Send delivers one message over a fresh connection.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("email: message has no recipients")
	}
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))

	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("email: dial %s: %w", addr, err)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(60 * time.Second)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return fmt.Errorf("email: set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("email: smtp handshake: %w", err)
	}
	defer func() { _ = client.Close() }()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("email: starttls: %w", err)
		}
	} else if s.cfg.RequireTLS {
		return fmt.Errorf("email: smtp server %s does not offer STARTTLS", s.cfg.Host)
	}

	if s.cfg.User != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("email: smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("email: mail from: %w", err)
	}
	for _, to := range msg.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("email: rcpt to: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: data: %w", err)
	}
	if _, err := io.WriteString(w, Encode(s.from, msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("email: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: finish: %w", err)
	}
	return client.Quit()
}

// Encode renders the RFC 5322 wire form of msg. Text-only messages are a
// single UTF-8 text/plain body; with HTML the message becomes
// multipart/alternative with the plain text first.
func Encode(from string, msg Message) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject) + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: <" + randomID() + "@" + hostOf(from) + ">\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTML == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		writeQP(&b, msg.Text)
		return b.String()
	}

	boundary := "=_betemplate_" + randomID()
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	writeQP(&b, msg.Text)
	b.WriteString("\r\n--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	writeQP(&b, msg.HTML)
	b.WriteString("\r\n--" + boundary + "--\r\n")
	return b.String()
}

// writeQP keeps every wire line under the SMTP limit regardless of the
// source line length.
func writeQP(b *strings.Builder, body string) {
	qp := quotedprintable.NewWriter(b)
	_, _ = io.WriteString(qp, body)
	_ = qp.Close()
}

func randomID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("email: crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(buf)
}

func hostOf(address string) string {
	if i := strings.LastIndex(address, "@"); i >= 0 && i < len(address)-1 {
		return strings.TrimSuffix(address[i+1:], ">")
	}
	return "localhost"
}
