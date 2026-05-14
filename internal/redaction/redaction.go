package redaction

import (
	"encoding/json"
	"strings"
)

const redactedMarker = "[REDACTED]"

// Policy defines what to redact from captured data.
type Policy struct {
	Headers      []string // lowercase header names
	JSONPaths    []string // top-level JSON field paths like "$.password"
	BodyMaxBytes int
}

// DefaultPolicy returns the default redaction policy.
func DefaultPolicy() *Policy {
	return &Policy{
		Headers: []string{
			"authorization", "proxy-authorization",
			"cookie", "set-cookie",
			"x-api-key", "x-auth-token", "stripe-signature",
		},
		JSONPaths: []string{
			"$.password", "$.token", "$.access_token", "$.refresh_token",
		},
		BodyMaxBytes: 8192,
	}
}

// RedactHeaders applies redaction to a header map in-place.
// Returns the count of redacted headers.
func (p *Policy) RedactHeaders(headers map[string]string) int {
	if headers == nil {
		return 0
	}
	count := 0
	deny := make(map[string]bool, len(p.Headers))
	for _, h := range p.Headers {
		deny[strings.ToLower(h)] = true
	}
	for k, v := range headers {
		if deny[strings.ToLower(k)] && v != redactedMarker {
			headers[k] = redactedMarker
			count++
		}
	}
	return count
}

// RedactJSONBody applies top-level field redaction to a JSON body string.
// Returns the redacted JSON and the count of redacted fields.
func (p *Policy) RedactJSONBody(body string) (string, int) {
	if body == "" {
		return body, 0
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		// Not valid JSON — can't redact fields
		return body, 0
	}

	count := 0
	for _, path := range p.JSONPaths {
		field := strings.TrimPrefix(path, "$.")
		if field == path {
			continue // not a simple top-level path
		}
		// Handle nested paths (only one level deep for V1)
		parts := strings.SplitN(field, ".", 2)
		if len(parts) == 1 {
			if _, ok := obj[field]; ok {
				obj[field] = redactedMarker
				count++
			}
		}
	}

	if count == 0 {
		return body, 0
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return body, 0
	}
	return string(out), count
}

// TruncatePreview truncates a body string to the configured max bytes.
func (p *Policy) TruncatePreview(body string) string {
	if p.BodyMaxBytes <= 0 || len(body) <= p.BodyMaxBytes {
		return body
	}
	return body[:p.BodyMaxBytes]
}

// ScanForSecrets checks if a string likely contains unreacted secrets.
// Returns a list of warnings. Used during export validation.
func ScanForSecrets(s string) []string {
	var warnings []string
	lower := strings.ToLower(s)

	patterns := []struct {
		substr string
		msg    string
	}{
		{"bearer ", "possible bearer token"},
		{"basic ", "possible basic auth credential"},
		{"password", "possible password value"},
		{"secret", "possible secret value"},
		{"api_key", "possible API key"},
		{"apikey", "possible API key"},
		{"private_key", "possible private key"},
	}

	for _, p := range patterns {
		if strings.Contains(lower, p.substr) {
			warnings = append(warnings, p.msg)
		}
	}
	return warnings
}
