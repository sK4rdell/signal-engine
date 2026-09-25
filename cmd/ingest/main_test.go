package main

import (
	"strings"
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

func TestParseArgs(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)

	feed, w, err := parseArgs([]string{"arbetsmiljoverket", "--from", "2026-09-20", "--to", "2026-09-21"}, now)
	if err != nil || feed.Name != "inspection-notices" || feed.EventType != "WORK_ENVIRONMENT_INSPECTION_NOTICE" || w.From.Format(dateLayout) != "2026-09-20" {
		t.Errorf("default feed = %+v, %v, %v", feed, w, err)
	}
	feed, _, err = parseArgs([]string{"arbetsmiljoverket", "--feed", "inspection-notices"}, now)
	if err != nil || feed.Name != "inspection-notices" {
		t.Errorf("explicit inspection notices = %+v, %v", feed, err)
	}
	feed, w, err = parseArgs([]string{"arbetsmiljoverket", "--feed", "recurring-inspection-failures", "--from", "2026-09-20"}, now)
	if err != nil || feed.Name != "recurring-inspection-failures" || feed.EventType != "WORK_EQUIPMENT_INSPECTION_FAILED" ||
		w.From.Format(dateLayout) != "2026-09-20" || w.To.Format(dateLayout) != "2026-09-23" {
		t.Errorf("certificate feed = %+v, %v, %v", feed, w, err)
	}
	_, _, err = parseArgs([]string{"arbetsmiljoverket", "--feed", "sanction-orders"}, now)
	if err == nil || !strings.Contains(err.Error(), "sanction-orders") || !strings.Contains(err.Error(), "recurring-inspection-failures") {
		t.Errorf("unknown feed error = %v", err)
	}
	for name, args := range map[string][]string{
		"no source":    {},
		"other source": {"bolagsverket"},
		"bad date":     {"arbetsmiljoverket", "--feed", "inspection-notices", "--to", "yesterday"},
		"unknown flag": {"arbetsmiljoverket", "--window", "7d"},
	} {
		if _, _, err := parseArgs(args, now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
