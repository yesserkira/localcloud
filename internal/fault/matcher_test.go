package fault

import (
	"testing"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

func TestMatchBasic(t *testing.T) {
	rule := &timeline.FaultRule{
		Enabled: true,
		Kind:    timeline.FaultKindForceHTTPStatus,
		Match: timeline.FaultMatch{
			Service:    "api",
			Method:     "POST",
			PathPrefix: "/signup",
		},
		Action: timeline.FaultAction{StatusCode: 500},
		Safety: timeline.FaultSafety{},
	}

	if !Match(rule, "POST", "/signup", "api", "", nil) {
		t.Error("expected match for POST /signup on api")
	}
	if Match(rule, "GET", "/signup", "api", "", nil) {
		t.Error("should not match GET method")
	}
	if Match(rule, "POST", "/signup", "worker", "", nil) {
		t.Error("should not match wrong service")
	}
	if Match(rule, "POST", "/health", "api", "", nil) {
		t.Error("should not match wrong path prefix")
	}
}

func TestMatchDisabled(t *testing.T) {
	rule := &timeline.FaultRule{
		Enabled: false,
		Kind:    timeline.FaultKindDelayResponse,
		Match:   timeline.FaultMatch{},
	}
	if Match(rule, "GET", "/", "", "", nil) {
		t.Error("disabled rule should not match")
	}
}

func TestMatchMaxHits(t *testing.T) {
	rule := &timeline.FaultRule{
		Enabled:  true,
		Kind:     timeline.FaultKindDelayResponse,
		Match:    timeline.FaultMatch{},
		HitCount: 10,
		Safety:   timeline.FaultSafety{MaxHits: 10},
	}
	if Match(rule, "GET", "/", "", "", nil) {
		t.Error("should not match when hit count >= max hits")
	}
}

func TestMatchExpired(t *testing.T) {
	rule := &timeline.FaultRule{
		Enabled:   true,
		Kind:      timeline.FaultKindDelayResponse,
		Match:     timeline.FaultMatch{},
		CreatedAt: time.Now().Add(-2 * time.Hour),
		Safety:    timeline.FaultSafety{ExpiresAfter: "1h"},
	}
	if Match(rule, "GET", "/", "", "", nil) {
		t.Error("should not match when expired")
	}
}

func TestMatchHeaders(t *testing.T) {
	rule := &timeline.FaultRule{
		Enabled: true,
		Kind:    timeline.FaultKindForceHTTPStatus,
		Match: timeline.FaultMatch{
			Headers: map[string]string{"X-Test": "yes"},
		},
	}

	if !Match(rule, "GET", "/", "", "", map[string]string{"X-Test": "yes"}) {
		t.Error("expected match with matching header")
	}
	if Match(rule, "GET", "/", "", "", map[string]string{"X-Test": "no"}) {
		t.Error("should not match wrong header value")
	}
	if Match(rule, "GET", "/", "", "", nil) {
		t.Error("should not match missing header")
	}
}

func TestFindFirstMatch(t *testing.T) {
	rules := []timeline.FaultRule{
		{Enabled: false, Kind: timeline.FaultKindDelayResponse, Match: timeline.FaultMatch{}},
		{Enabled: true, Kind: timeline.FaultKindForceHTTPStatus, Match: timeline.FaultMatch{Method: "GET"}, Action: timeline.FaultAction{StatusCode: 503}},
		{Enabled: true, Kind: timeline.FaultKindDelayResponse, Match: timeline.FaultMatch{}, Action: timeline.FaultAction{DelayMs: 100}},
	}

	got := FindFirstMatch(rules, "GET", "/", "", "", nil)
	if got == nil {
		t.Fatal("expected match")
	}
	if got.Kind != timeline.FaultKindForceHTTPStatus {
		t.Errorf("expected force_http_status, got %s", got.Kind)
	}
}

func TestFindFirstMatchNoMatch(t *testing.T) {
	rules := []timeline.FaultRule{
		{Enabled: true, Kind: timeline.FaultKindForceHTTPStatus, Match: timeline.FaultMatch{Service: "worker"}},
	}
	got := FindFirstMatch(rules, "GET", "/", "api", "", nil)
	if got != nil {
		t.Error("expected no match")
	}
}

func TestValidateRule(t *testing.T) {
	tests := []struct {
		name    string
		rule    timeline.FaultRule
		wantErr bool
	}{
		{
			name: "valid delay",
			rule: timeline.FaultRule{
				Name: "delay", Kind: timeline.FaultKindDelayResponse, Scope: "both",
				Action: timeline.FaultAction{DelayMs: 500},
			},
		},
		{
			name: "valid force status",
			rule: timeline.FaultRule{
				Name: "503", Kind: timeline.FaultKindForceHTTPStatus, Scope: "live",
				Action: timeline.FaultAction{StatusCode: 503},
			},
		},
		{
			name: "missing name",
			rule: timeline.FaultRule{
				Kind: timeline.FaultKindDelayResponse, Scope: "both",
				Action: timeline.FaultAction{DelayMs: 500},
			},
			wantErr: true,
		},
		{
			name: "delay without ms",
			rule: timeline.FaultRule{
				Name: "bad", Kind: timeline.FaultKindDelayResponse, Scope: "both",
			},
			wantErr: true,
		},
		{
			name: "invalid status code",
			rule: timeline.FaultRule{
				Name: "bad", Kind: timeline.FaultKindForceHTTPStatus, Scope: "both",
				Action: timeline.FaultAction{StatusCode: 999},
			},
			wantErr: true,
		},
		{
			name: "unknown kind",
			rule: timeline.FaultRule{
				Name: "bad", Kind: "unknown_kind", Scope: "both",
			},
			wantErr: true,
		},
		{
			name: "invalid scope",
			rule: timeline.FaultRule{
				Name: "bad", Kind: timeline.FaultKindDropOutbound, Scope: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid expires duration",
			rule: timeline.FaultRule{
				Name: "bad", Kind: timeline.FaultKindDropOutbound, Scope: "both",
				Safety: timeline.FaultSafety{ExpiresAfter: "not-a-duration"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateRule(&tt.rule)
			if tt.wantErr && len(errs) == 0 {
				t.Error("expected validation errors")
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}
