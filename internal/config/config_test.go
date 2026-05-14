package config

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	errs := cfg.Validate()
	if len(errs) > 0 {
		t.Fatalf("default config should be valid, got: %v", errs)
	}
}

func TestDemoSaaSConfig(t *testing.T) {
	cfg := DemoSaaSConfig()
	errs := cfg.Validate()
	if len(errs) > 0 {
		t.Fatalf("demo-saas config should be valid, got: %v", errs)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	cfg := DemoSaaSConfig()
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfg2, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg2.Project.Name != cfg.Project.Name {
		t.Fatalf("roundtrip project name mismatch: %s vs %s", cfg.Project.Name, cfg2.Project.Name)
	}
}

func TestValidateMissingProjectName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.Name = ""
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for missing project name")
	}
	if !strings.Contains(errs[0].Error(), "project.name") {
		t.Fatalf("unexpected error: %v", errs[0])
	}
}

func TestValidateInvalidBind(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Bind = "0.0.0.0"
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Fatal("expected error for non-loopback bind")
	}
}

func TestValidateAllowNetworkBind(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.Bind = "0.0.0.0"
	cfg.Agent.AllowNetworkBind = true
	errs := cfg.Validate()
	for _, e := range errs {
		if strings.Contains(e.Error(), "agent.bind") {
			t.Fatalf("should allow 0.0.0.0 with AllowNetworkBind, got: %v", e)
		}
	}
}

func TestValidateSamePort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agent.StudioPort = cfg.Agent.Port
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "must differ") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for same port")
	}
}

func TestValidatePostgresMissingDSN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services = map[string]ServiceConfig{
		"db": {Type: "postgres", Capture: CaptureConfig{Tables: []string{"users"}}},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "dsn is required") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for missing DSN")
	}
}

func TestValidatePostgresMissingTables(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services = map[string]ServiceConfig{
		"db": {Type: "postgres", DSN: "postgres://x@localhost/db"},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "explicit tables") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for missing tables")
	}
}

func TestValidateRedisMissingAddr(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services = map[string]ServiceConfig{
		"cache": {Type: "redis"},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "addr is required") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for missing redis addr")
	}
}

func TestValidateMailpitNonLocal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services = map[string]ServiceConfig{
		"mail": {Type: "mailpit", APIUrl: "http://external.host:8025"},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "must use localhost") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for non-local mailpit")
	}
}

func TestValidateMailpitPollIntervalTooLow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services = map[string]ServiceConfig{
		"mail": {Type: "mailpit", APIUrl: "http://localhost:8025", Capture: CaptureConfig{PollInterval: "100ms"}},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "minimum is 500ms") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for poll interval too low")
	}
}

func TestValidateMinIOLiteralSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services = map[string]ServiceConfig{
		"s3": {Type: "minio", Endpoint: "http://localhost:9000", SecretKey: "mysecret"},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "literal value") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for literal secret")
	}
}

func TestValidateDuplicateFaultRuleNames(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Faults.Rules = []FaultRuleConfig{
		{Name: "rule1", Kind: "delay_response", Scope: "replay"},
		{Name: "rule1", Kind: "force_http_status", Scope: "live"},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate name") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for duplicate fault rule names")
	}
}

func TestValidateEnabledFaultWithoutSafety(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Faults.Rules = []FaultRuleConfig{
		{Name: "rule1", Kind: "delay_response", Scope: "replay", Enabled: true},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "must define safety") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for enabled rule without safety")
	}
}

func TestValidateRecordingMaxDurationTooLong(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Recording.MaxDuration = "2h"
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "≤ 1h") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for maxDuration > 1h")
	}
}

func TestValidateAutoStopGreaterThanMaxDuration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Recording.MaxDuration = "5m"
	cfg.Recording.AutoStopOnIdle = "6m"
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "less than recording.maxDuration") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for autoStopOnIdle >= maxDuration")
	}
}

func TestValidateInvalidVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Version = 99
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "version must be 1") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for invalid version")
	}
}

func TestValidateInvalidFaultKind(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Faults.Rules = []FaultRuleConfig{
		{Name: "bad", Kind: "invalid_kind", Scope: "replay"},
	}
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "invalid") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected error for invalid fault kind")
	}
}

func TestParseMinimalYAML(t *testing.T) {
	yaml := `
version: 1
project:
  name: test-app
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Agent.Bind != "127.0.0.1" {
		t.Fatalf("expected default bind, got %q", cfg.Agent.Bind)
	}
	if cfg.Agent.Port != 41777 {
		t.Fatalf("expected default port, got %d", cfg.Agent.Port)
	}
}
