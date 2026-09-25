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

	from, to, err := parseWindow("", "", now)
	if err != nil || !from.Equal(date("2026-09-23")) || !to.Equal(date("2026-09-23")) {
		t.Errorf("defaults = %v..%v, %v; want yesterday..yesterday", from, to, err)
	}
	from, to, err = parseWindow("2026-09-20", "", now)
	if err != nil || !from.Equal(date("2026-09-20")) || !to.Equal(date("2026-09-23")) {
		t.Errorf("from only = %v..%v, %v", from, to, err)
	}
	from, to, err = parseWindow("2026-09-20", "2026-09-21", now)
	if err != nil || !from.Equal(date("2026-09-20")) || !to.Equal(date("2026-09-21")) {
		t.Errorf("both = %v..%v, %v", from, to, err)
	}
	for name, in := range map[string][2]string{
		"reversed":   {"2026-09-22", "2026-09-21"},
		"bad from":   {"22/9", "2026-09-23"},
		"bad to":     {"2026-09-22", "yesterday"},
		"from later": {"2026-09-25", ""},
	} {
		if _, _, err := parseWindow(in[0], in[1], now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseArgs(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)

	inv, err := parseArgs([]string{"arbetsmiljoverket", "--from", "2026-09-20", "--to", "2026-09-21"}, now)
	if err != nil || inv.source != "arbetsmiljoverket" || inv.feed.Name != "inspection-notices" || inv.feed.EventType != "WORK_ENVIRONMENT_INSPECTION_NOTICE" || inv.from.Format(dateLayout) != "2026-09-20" {
		t.Errorf("default feed = %+v, %v", inv, err)
	}
	inv, err = parseArgs([]string{"arbetsmiljoverket", "--feed", "recurring-inspection-failures", "--from", "2026-09-20"}, now)
	if err != nil || inv.feed.Name != "recurring-inspection-failures" || inv.feed.EventType != "WORK_EQUIPMENT_INSPECTION_FAILED" ||
		inv.from.Format(dateLayout) != "2026-09-20" || inv.to.Format(dateLayout) != "2026-09-23" {
		t.Errorf("certificate feed = %+v, %v", inv, err)
	}
	inv, err = parseArgs([]string{"stockholm", "--from", "2026-09-01", "--to", "2026-09-25"}, now)
	if err != nil || inv.source != "stockholm" || inv.feed.Name != "" || inv.from.Format(dateLayout) != "2026-09-01" || inv.to.Format(dateLayout) != "2026-09-25" {
		t.Errorf("stockholm = %+v, %v", inv, err)
	}
	inv, err = parseArgs([]string{"stockholm"}, now)
	if err != nil || inv.from.Format(dateLayout) != "2026-09-23" || inv.to.Format(dateLayout) != "2026-09-23" {
		t.Errorf("stockholm defaults = %+v, %v", inv, err)
	}
	_, err = parseArgs([]string{"arbetsmiljoverket", "--feed", "sanction-orders"}, now)
	if err == nil || !strings.Contains(err.Error(), "sanction-orders") || !strings.Contains(err.Error(), "recurring-inspection-failures") {
		t.Errorf("unknown feed error = %v", err)
	}
	for name, args := range map[string][]string{
		"no source":             {},
		"other source":          {"bolagsverket"},
		"bad date":              {"arbetsmiljoverket", "--feed", "inspection-notices", "--to", "yesterday"},
		"unknown flag":          {"arbetsmiljoverket", "--window", "7d"},
		"stockholm has no feed": {"stockholm", "--feed", "x"},
		"stockholm bad window":  {"stockholm", "--from", "2026-09-26", "--to", "2026-09-25"},
	} {
		if _, err := parseArgs(args, now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
