package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestFromContext_IncludesAttributesAddedLater(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "info", "json")

	ctx := NewScope(context.Background(), logger)
	Add(ctx, "request_id", "abc")

	type otherKey struct{}
	derived := context.WithValue(ctx, otherKey{}, 1)
	Add(derived, "user_id", "u1")

	FromContext(ctx).Info("hello", "extra", 1)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("decode log line: %v (%q)", err, buf.String())
	}
	for key, want := range map[string]any{"request_id": "abc", "user_id": "u1", "msg": "hello", "extra": float64(1)} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v", key, line[key], want)
		}
	}
}

func TestFromContext_WithoutScopeReturnsDefault(t *testing.T) {
	if FromContext(context.Background()) == nil {
		t.Fatal("expected a logger")
	}
	Add(context.Background(), "ignored", true) // must not panic
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, "debug", "text").Debug("dbg")
	if !bytes.Contains(buf.Bytes(), []byte("msg=dbg")) {
		t.Fatalf("unexpected text output %q", buf.String())
	}
}
