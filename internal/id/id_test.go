package id

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	id := New()
	if len(id) != 26 {
		t.Fatalf("expected 26 char ULID, got %d chars: %s", len(id), id)
	}
}

func TestNewWithPrefix(t *testing.T) {
	id := NewWithPrefix("evt")
	if !strings.HasPrefix(id, "evt_") {
		t.Fatalf("expected evt_ prefix, got %s", id)
	}
}

func TestUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := New()
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestAllPrefixes(t *testing.T) {
	cases := []struct {
		fn     func() string
		prefix string
	}{
		{Event, "evt_"},
		{Run, "run_"},
		{Scenario, "scn_"},
		{ReplayRun, "rpr_"},
		{FaultRule, "flt_"},
		{ConfigSnapshot, "cfg_"},
		{Correlation, "cor_"},
	}
	for _, tc := range cases {
		id := tc.fn()
		if !strings.HasPrefix(id, tc.prefix) {
			t.Errorf("expected prefix %s, got %s", tc.prefix, id)
		}
	}
}
