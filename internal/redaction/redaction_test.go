package redaction

import (
	"testing"
)

func TestRedactHeaders(t *testing.T) {
	p := DefaultPolicy()
	headers := map[string]string{
		"Authorization": "Bearer secret123",
		"Content-Type":  "application/json",
		"Cookie":        "session=abc",
		"X-Custom":      "safe",
	}

	count := p.RedactHeaders(headers)
	if count != 2 {
		t.Fatalf("expected 2 redacted, got %d", count)
	}
	if headers["Authorization"] != "[REDACTED]" {
		t.Fatalf("authorization not redacted: %s", headers["Authorization"])
	}
	if headers["Cookie"] != "[REDACTED]" {
		t.Fatalf("cookie not redacted: %s", headers["Cookie"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Fatal("content-type should not be redacted")
	}
}

func TestRedactHeadersCaseInsensitive(t *testing.T) {
	p := DefaultPolicy()
	headers := map[string]string{
		"AUTHORIZATION": "token",
		"x-api-key":     "key123",
	}

	count := p.RedactHeaders(headers)
	if count != 2 {
		t.Fatalf("expected 2, got %d", count)
	}
}

func TestRedactHeadersNil(t *testing.T) {
	p := DefaultPolicy()
	count := p.RedactHeaders(nil)
	if count != 0 {
		t.Fatal("expected 0")
	}
}

func TestRedactJSONBody(t *testing.T) {
	p := DefaultPolicy()
	body := `{"email":"alex@example.test","password":"secret123","name":"Alex"}`

	redacted, count := p.RedactJSONBody(body)
	if count != 1 {
		t.Fatalf("expected 1 redacted field, got %d", count)
	}
	if !contains(redacted, "[REDACTED]") {
		t.Fatalf("password not redacted in: %s", redacted)
	}
	if contains(redacted, "secret123") {
		t.Fatalf("password value still present in: %s", redacted)
	}
}

func TestRedactJSONBodyNoMatch(t *testing.T) {
	p := DefaultPolicy()
	body := `{"email":"alex@example.test","name":"Alex"}`

	redacted, count := p.RedactJSONBody(body)
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
	if redacted != body {
		t.Fatal("body should be unchanged")
	}
}

func TestRedactJSONBodyInvalid(t *testing.T) {
	p := DefaultPolicy()
	body := "not json"

	redacted, count := p.RedactJSONBody(body)
	if count != 0 || redacted != body {
		t.Fatal("invalid JSON should pass through unchanged")
	}
}

func TestTruncatePreview(t *testing.T) {
	p := &Policy{BodyMaxBytes: 10}
	result := p.TruncatePreview("hello world, this is long")
	if len(result) != 10 {
		t.Fatalf("expected 10 bytes, got %d", len(result))
	}
}

func TestTruncatePreviewShort(t *testing.T) {
	p := &Policy{BodyMaxBytes: 100}
	input := "short"
	result := p.TruncatePreview(input)
	if result != input {
		t.Fatal("short body should not be truncated")
	}
}

func TestScanForSecrets(t *testing.T) {
	warnings := ScanForSecrets(`{"auth":"Bearer eyJtoken"}`)
	if len(warnings) == 0 {
		t.Fatal("expected warnings for bearer token")
	}
}

func TestScanForSecretsClean(t *testing.T) {
	warnings := ScanForSecrets(`{"email":"alex@example.test","name":"Alex"}`)
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
