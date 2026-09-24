package main

import (
	"testing"
	"time"
)

func TestParseWindow(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)
	date := func(s string) time.Time {
		d, err := time.Parse(dateLayout, s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}

	w, err := parseWindow("", "", now)
	if err != nil || !w.From.Equal(date("2026-09-23")) || !w.To.Equal(date("2026-09-23")) {
		t.Errorf("defaults = %v..%v, %v; want yesterday..yesterday", w.From, w.To, err)
	}
	w, err = parseWindow("2026-09-20", "", now)
	if err != nil || !w.From.Equal(date("2026-09-20")) || !w.To.Equal(date("2026-09-23")) {
		t.Errorf("from only = %v..%v, %v", w.From, w.To, err)
	}
	w, err = parseWindow("2026-09-20", "2026-09-21", now)
	if err != nil || !w.From.Equal(date("2026-09-20")) || !w.To.Equal(date("2026-09-21")) {
		t.Errorf("both = %v..%v, %v", w.From, w.To, err)
	}
	for name, in := range map[string][2]string{
		"reversed":   {"2026-09-22", "2026-09-21"},
		"bad from":   {"22/9", "2026-09-23"},
		"bad to":     {"2026-09-22", "yesterday"},
		"from later": {"2026-09-25", ""},
	} {
		if _, err := parseWindow(in[0], in[1], now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
