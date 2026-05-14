package redis

import (
	"testing"
)

func TestParseQuotedStrings(t *testing.T) {
	cases := []struct {
		input    string
		expected []string
	}{
		{`"LPUSH" "email_jobs" "{\"type\":\"welcome\"}"`, []string{"LPUSH", "email_jobs", `{"type":"welcome"}`}},
		{`"GET" "key"`, []string{"GET", "key"}},
		{`"SET" "k" "v"`, []string{"SET", "k", "v"}},
		{``, nil},
	}

	for _, tc := range cases {
		result := parseQuotedStrings(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("input=%q: expected %d parts, got %d: %v", tc.input, len(tc.expected), len(result), result)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("input=%q: part[%d] expected %q, got %q", tc.input, i, tc.expected[i], result[i])
			}
		}
	}
}

func TestRedactPayload(t *testing.T) {
	a := &Adapter{
		redactJSONPaths: []string{"$.password", "$.token"},
	}

	input := `{"email":"test@example.test","password":"secret","token":"abc"}`
	result := a.redactPayload(input)

	if result == input {
		t.Fatal("expected redaction to change the payload")
	}

	// Check that password and token are redacted
	if containsStr(result, `"secret"`) {
		t.Fatalf("password not redacted: %s", result)
	}
	if containsStr(result, `"abc"`) {
		t.Fatalf("token not redacted: %s", result)
	}
}

func TestRedactPayloadNoMatch(t *testing.T) {
	a := &Adapter{
		redactJSONPaths: []string{"$.password"},
	}

	input := `{"email":"test@example.test"}`
	result := a.redactPayload(input)
	if result != input {
		t.Fatal("should not change when no fields match")
	}
}

func TestRedactPayloadInvalidJSON(t *testing.T) {
	a := &Adapter{
		redactJSONPaths: []string{"$.password"},
	}

	input := "not json"
	result := a.redactPayload(input)
	if result != input {
		t.Fatal("invalid JSON should pass through")
	}
}

func TestIsRelevantCommand(t *testing.T) {
	a := &Adapter{queues: []string{"email_jobs"}}

	cases := []struct {
		entry    *monitorEntry
		expected bool
	}{
		{&monitorEntry{command: "LPUSH", key: "email_jobs"}, true},
		{&monitorEntry{command: "BRPOP", key: "email_jobs"}, true},
		{&monitorEntry{command: "LPUSH", key: "other_queue"}, false},
		{&monitorEntry{command: "GET", key: "email_jobs"}, false},
	}

	for _, tc := range cases {
		result := a.isRelevantCommand(tc.entry)
		if result != tc.expected {
			t.Errorf("%s %s: expected %v, got %v", tc.entry.command, tc.entry.key, tc.expected, result)
		}
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
