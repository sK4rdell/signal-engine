package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sK4rdell/signal-engine/internal/platform/config"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
)

func TestRender_AllTemplates(t *testing.T) {
	data := map[string]string{"Link": "https://app.test/verify?token=abc&x=1", "ExpiresIn": "24 hours"}
	want := map[Template]struct{ subject, phrase string }{
		TemplateVerifyEmail:   {"Verify your email address", "Confirm your email address"},
		TemplateResetPassword: {"Reset your password", "password reset"},
		TemplateWelcome:       {"Welcome!", "account is ready"},
	}
	for tmpl, w := range want {
		msg, err := Render(tmpl, []string{"a@example.com"}, data)
		if err != nil {
			t.Fatalf("Render(%s): %v", tmpl, err)
		}
		if msg.Subject != w.subject || !strings.Contains(msg.Text, w.phrase) || !strings.Contains(msg.HTML, w.phrase) {
			t.Errorf("%s: subject %q text %q", tmpl, msg.Subject, msg.Text)
		}
		if !strings.Contains(msg.Text, "https://app.test/verify?token=abc&x=1") {
			t.Errorf("%s: link missing from text %q", tmpl, msg.Text)
		}
		if !strings.Contains(msg.HTML, "https://app.test/verify?token=abc&amp;x=1") {
			t.Errorf("%s: html should escape the link: %s", tmpl, msg.HTML)
		}
		if msg.To[0] != "a@example.com" {
			t.Errorf("%s: to = %v", tmpl, msg.To)
		}
	}
	if _, err := Render(Template("missing"), nil, data); err == nil {
		t.Error("unknown template should fail")
	}
}

func TestEncode_TextOnlyAndMultipart(t *testing.T) {
	plain := Encode("no-reply@example.com", Message{To: []string{"a@example.com"}, Subject: "Hej då", Text: "Line 1\nLine 2\n"})
	for _, want := range []string{"From: no-reply@example.com\r\n", "To: a@example.com\r\n", "Subject: =?utf-8?q?Hej_d=C3=A5?=\r\n", "Content-Type: text/plain; charset=UTF-8", "Line 1\r\nLine 2"} {
		if !strings.Contains(plain, want) {
			t.Errorf("plain message missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "multipart") {
		t.Error("text-only message must not be multipart")
	}

	long := strings.Repeat("x", 2000)
	multi := Encode("no-reply@example.com", Message{To: []string{"a@example.com", "b@example.com"}, Subject: "s", Text: "t", HTML: "<p>" + long + "</p>"})
	if !strings.Contains(multi, "multipart/alternative") || !strings.Contains(multi, "text/html") || !strings.Contains(multi, "To: a@example.com, b@example.com") {
		t.Errorf("multipart headers missing:\n%s", multi)
	}
	for _, line := range strings.Split(multi, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("wire line exceeds SMTP limit: %d bytes", len(line))
		}
	}
}

func TestRecorder(t *testing.T) {
	var r Recorder
	if err := r.Send(context.Background(), Message{Subject: "a"}); err != nil {
		t.Fatal(err)
	}
	r.Fail(errors.New("down"))
	if err := r.Send(context.Background(), Message{Subject: "b"}); err == nil {
		t.Error("expected failure")
	}
	r.Fail(nil)
	_ = r.Send(context.Background(), Message{Subject: "c"})
	got := r.Messages()
	if len(got) != 2 || got[0].Subject != "a" || got[1].Subject != "c" {
		t.Errorf("messages = %+v", got)
	}
	r.Reset()
	if len(r.Messages()) != 0 {
		t.Error("Reset should clear")
	}
}

func TestLogSender_NeverLogsBodiesOrTokens(t *testing.T) {
	var buf bytes.Buffer
	s := &LogSender{Logger: logging.New(&buf, "debug", "json")}
	const token = "S3cr3tT0kenValue_abcdefghijklmnopqrstuvwxyz"
	msg := Message{
		To:      []string{"a@example.com"},
		Subject: "Verify your email address",
		Text:    "Open https://app.test/verify-email?token=" + token,
		HTML:    "<a href=\"https://app.test/verify-email?token=" + token + "\">x</a>",
	}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	logs := buf.String()
	if !strings.Contains(logs, "a@example.com") || !strings.Contains(logs, "Verify your email address") {
		t.Errorf("metadata missing from %s", logs)
	}
	for _, forbidden := range []string{token, "token=", "app.test/verify-email", "<a href"} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("log contains %q: %s", forbidden, logs)
		}
	}
}

func TestNewSender(t *testing.T) {
	s, err := NewSender(config.EmailConfig{Provider: "log"}, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*LogSender); !ok {
		t.Errorf("sender = %T", s)
	}
	if err := s.Send(context.Background(), Message{To: []string{"a@example.com"}}); err != nil {
		t.Error(err)
	}
	if _, err := NewSender(config.EmailConfig{Provider: "pigeon"}, logging.Discard()); err == nil {
		t.Error("unknown provider should fail")
	}
}

// TestSMTPSender_DeliversToMailpit is an integration test against the
// compose Mailpit. It runs when TEST_SMTP_ADDR and TEST_MAILPIT_URL are set.
func TestSMTPSender_DeliversToMailpit(t *testing.T) {
	smtpAddr := os.Getenv("TEST_SMTP_ADDR")
	mailpitURL := os.Getenv("TEST_MAILPIT_URL")
	if smtpAddr == "" || mailpitURL == "" {
		t.Skip("TEST_SMTP_ADDR / TEST_MAILPIT_URL not set")
	}
	host, portStr, ok := strings.Cut(smtpAddr, ":")
	if !ok {
		t.Fatalf("TEST_SMTP_ADDR %q must be host:port", smtpAddr)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatal(err)
	}

	subject := "smtp integration test " + time.Now().Format(time.RFC3339Nano)
	sender := NewSMTPSender(config.SMTPConfig{Host: host, Port: port}, "no-reply@example.com")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := sender.Send(ctx, Message{To: []string{"rcpt@example.com"}, Subject: subject, Text: "hello åäö", HTML: "<p>hello</p>"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(mailpitURL, "/")+"/api/v1/search?query="+url.QueryEscape(`subject:"`+subject+`"`), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mailpit search: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var result struct {
		Messages []struct {
			Subject string `json:"Subject"`
			To      []struct {
				Address string `json:"Address"`
			} `json:"To"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode mailpit response %s: %v", body, err)
	}
	if len(result.Messages) != 1 || result.Messages[0].Subject != subject || result.Messages[0].To[0].Address != "rcpt@example.com" {
		t.Errorf("mailpit result = %s", body)
	}

	strict := NewSMTPSender(config.SMTPConfig{Host: host, Port: port, RequireTLS: true}, "no-reply@example.com")
	if err := strict.Send(ctx, Message{To: []string{"rcpt@example.com"}, Subject: "x", Text: "x"}); err == nil {
		t.Error("RequireTLS must refuse a server without STARTTLS")
	}
}
