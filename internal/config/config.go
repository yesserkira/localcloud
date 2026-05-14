package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root localcloud.yml configuration.
type Config struct {
	Version int           `yaml:"version" json:"version"`
	Project ProjectConfig `yaml:"project" json:"project"`
	Agent   AgentConfig   `yaml:"agent" json:"agent"`
	Compose ComposeConfig `yaml:"compose,omitempty" json:"compose,omitempty"`
	Services map[string]ServiceConfig `yaml:"services,omitempty" json:"services,omitempty"`
	Redaction RedactionConfig `yaml:"redaction,omitempty" json:"redaction,omitempty"`
	Recording RecordingConfig `yaml:"recording,omitempty" json:"recording,omitempty"`
	Replay   ReplayConfig    `yaml:"replay,omitempty" json:"replay,omitempty"`
	Faults   FaultsConfig    `yaml:"faults,omitempty" json:"faults,omitempty"`
}

type ProjectConfig struct {
	Name    string `yaml:"name" json:"name"`
	DataDir string `yaml:"dataDir" json:"dataDir"`
}

type AgentConfig struct {
	Bind              string `yaml:"bind" json:"bind"`
	Port              int    `yaml:"port" json:"port"`
	StudioPort        int    `yaml:"studioPort" json:"studioPort"`
	Database          string `yaml:"database" json:"database"`
	CorrelationHeader string `yaml:"correlationHeader" json:"correlationHeader"`
	AllowNetworkBind  bool   `yaml:"allowNetworkBind,omitempty" json:"allowNetworkBind,omitempty"`
}

type ComposeConfig struct {
	Files       []string `yaml:"files,omitempty" json:"files,omitempty"`
	ProjectName string   `yaml:"projectName,omitempty" json:"projectName,omitempty"`
	AutoStart   bool     `yaml:"autoStart,omitempty" json:"autoStart,omitempty"`
}

type ServiceConfig struct {
	Type       string            `yaml:"type" json:"type"`
	BaseURL    string            `yaml:"baseUrl,omitempty" json:"baseUrl,omitempty"`
	Container  string            `yaml:"container,omitempty" json:"container,omitempty"`
	HealthPath string            `yaml:"healthPath,omitempty" json:"healthPath,omitempty"`
	DSN        string            `yaml:"dsn,omitempty" json:"dsn,omitempty"`
	Addr       string            `yaml:"addr,omitempty" json:"addr,omitempty"`
	APIUrl     string            `yaml:"apiUrl,omitempty" json:"apiUrl,omitempty"`
	SMTPHost   string            `yaml:"smtpHost,omitempty" json:"smtpHost,omitempty"`
	SMTPPort   int               `yaml:"smtpPort,omitempty" json:"smtpPort,omitempty"`
	Endpoint   string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	AccessKey  string            `yaml:"accessKey,omitempty" json:"accessKey,omitempty"`
	SecretKey  string            `yaml:"secretKey,omitempty" json:"secretKey,omitempty"`
	Capture    CaptureConfig     `yaml:"capture,omitempty" json:"capture,omitempty"`
}

type CaptureConfig struct {
	Mode           string   `yaml:"mode,omitempty" json:"mode,omitempty"`
	Inbound        bool     `yaml:"inbound,omitempty" json:"inbound,omitempty"`
	Outbound       bool     `yaml:"outbound,omitempty" json:"outbound,omitempty"`
	ProxyPort      int      `yaml:"proxyPort,omitempty" json:"proxyPort,omitempty"`
	ReplayBaseURL  string   `yaml:"replayBaseUrl,omitempty" json:"replayBaseUrl,omitempty"`
	Schemas        []string `yaml:"schemas,omitempty" json:"schemas,omitempty"`
	Tables         []string `yaml:"tables,omitempty" json:"tables,omitempty"`
	RedactColumns  []string `yaml:"redactColumns,omitempty" json:"redactColumns,omitempty"`
	Queues         []string `yaml:"queues,omitempty" json:"queues,omitempty"`
	Commands       []string `yaml:"commands,omitempty" json:"commands,omitempty"`
	RedactJSONPaths []string `yaml:"redactJsonPaths,omitempty" json:"redactJsonPaths,omitempty"`
	PollInterval   string   `yaml:"pollInterval,omitempty" json:"pollInterval,omitempty"`
	RedactBody     bool     `yaml:"redactBody,omitempty" json:"redactBody,omitempty"`
	Buckets        []string `yaml:"buckets,omitempty" json:"buckets,omitempty"`
	Services       []string `yaml:"services,omitempty" json:"services,omitempty"`
}

type RedactionConfig struct {
	Headers       []string `yaml:"headers,omitempty" json:"headers,omitempty"`
	JSONPaths     []string `yaml:"jsonPaths,omitempty" json:"jsonPaths,omitempty"`
	BodyMaxBytes  int      `yaml:"bodyMaxBytes,omitempty" json:"bodyMaxBytes,omitempty"`
	StoreRawBodies bool    `yaml:"storeRawBodies,omitempty" json:"storeRawBodies,omitempty"`
}

type RecordingConfig struct {
	IncludeSources            []string `yaml:"includeSources,omitempty" json:"includeSources,omitempty"`
	IncludeUncorrelatedEvents bool     `yaml:"includeUncorrelatedEvents,omitempty" json:"includeUncorrelatedEvents,omitempty"`
	DefaultTags               []string `yaml:"defaultTags,omitempty" json:"defaultTags,omitempty"`
	MaxDuration               string   `yaml:"maxDuration,omitempty" json:"maxDuration,omitempty"`
	AutoStopOnIdle            string   `yaml:"autoStopOnIdle,omitempty" json:"autoStopOnIdle,omitempty"`
}

type ReplayConfig struct {
	DefaultTargetBaseURL          string   `yaml:"defaultTargetBaseUrl,omitempty" json:"defaultTargetBaseUrl,omitempty"`
	RequireConfirmationForMethods []string `yaml:"requireConfirmationForMethods,omitempty" json:"requireConfirmationForMethods,omitempty"`
	RegenerateCorrelationIDs      bool     `yaml:"regenerateCorrelationIds,omitempty" json:"regenerateCorrelationIds,omitempty"`
	Timeout                       string   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

type FaultsConfig struct {
	Enabled bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Rules   []FaultRuleConfig `yaml:"rules,omitempty" json:"rules,omitempty"`
}

type FaultRuleConfig struct {
	Name    string         `yaml:"name" json:"name"`
	Enabled bool           `yaml:"enabled" json:"enabled"`
	Kind    string         `yaml:"kind" json:"kind"`
	Scope   string         `yaml:"scope" json:"scope"`
	Match   map[string]any `yaml:"match" json:"match"`
	Action  map[string]any `yaml:"action" json:"action"`
	Safety  map[string]any `yaml:"safety,omitempty" json:"safety,omitempty"`
}

// LoadFile reads and parses a localcloud.yml config file.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: cannot read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses YAML bytes into a Config.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: invalid YAML: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

// Marshal serializes config to YAML bytes.
func (c *Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

func (c *Config) applyDefaults() {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Project.DataDir == "" {
		c.Project.DataDir = ".localcloud"
	}
	if c.Agent.Bind == "" {
		c.Agent.Bind = "127.0.0.1"
	}
	if c.Agent.Port == 0 {
		c.Agent.Port = 41777
	}
	if c.Agent.StudioPort == 0 {
		c.Agent.StudioPort = 41778
	}
	if c.Agent.Database == "" {
		c.Agent.Database = ".localcloud/localcloud.db"
	}
	if c.Agent.CorrelationHeader == "" {
		c.Agent.CorrelationHeader = "x-localcloud-correlation-id"
	}
	if c.Redaction.BodyMaxBytes == 0 {
		c.Redaction.BodyMaxBytes = 8192
	}
	if len(c.Redaction.Headers) == 0 {
		c.Redaction.Headers = DefaultRedactHeaders()
	}
	if c.Recording.MaxDuration == "" {
		c.Recording.MaxDuration = "10m"
	}
	if c.Replay.Timeout == "" {
		c.Replay.Timeout = "10s"
	}
	if len(c.Replay.RequireConfirmationForMethods) == 0 {
		c.Replay.RequireConfirmationForMethods = []string{"POST", "PUT", "PATCH", "DELETE"}
	}
	c.Replay.RegenerateCorrelationIDs = true
}

// DefaultRedactHeaders returns the default header denylist for redaction.
func DefaultRedactHeaders() []string {
	return []string{
		"authorization",
		"proxy-authorization",
		"cookie",
		"set-cookie",
		"x-api-key",
		"x-auth-token",
		"stripe-signature",
	}
}

// Validate checks the config for errors. Returns a slice of all detected problems.
func (c *Config) Validate() []error {
	var errs []error

	if c.Version != 1 {
		errs = append(errs, fmt.Errorf("config: version must be 1, got %d", c.Version))
	}
	if c.Project.Name == "" {
		errs = append(errs, fmt.Errorf("config: project.name is required"))
	}

	// Agent bind validation
	if !c.Agent.AllowNetworkBind && c.Agent.Bind != "127.0.0.1" && c.Agent.Bind != "localhost" && c.Agent.Bind != "::1" {
		errs = append(errs, fmt.Errorf("config: agent.bind must be 127.0.0.1 or localhost in V1; got %q", c.Agent.Bind))
	}
	if c.Agent.Port < 1 || c.Agent.Port > 65535 {
		errs = append(errs, fmt.Errorf("config: agent.port must be 1-65535, got %d", c.Agent.Port))
	}
	if c.Agent.StudioPort < 1 || c.Agent.StudioPort > 65535 {
		errs = append(errs, fmt.Errorf("config: agent.studioPort must be 1-65535, got %d", c.Agent.StudioPort))
	}
	if c.Agent.Port == c.Agent.StudioPort {
		errs = append(errs, fmt.Errorf("config: agent.port and agent.studioPort must differ"))
	}

	// Service validation
	for name, svc := range c.Services {
		errs = append(errs, c.validateService(name, svc)...)
	}

	// Fault rule validation
	ruleNames := map[string]bool{}
	for i, rule := range c.Faults.Rules {
		if rule.Name == "" {
			errs = append(errs, fmt.Errorf("config: faults.rules[%d].name is required", i))
		} else if ruleNames[rule.Name] {
			errs = append(errs, fmt.Errorf("config: faults.rules duplicate name %q", rule.Name))
		} else {
			ruleNames[rule.Name] = true
		}
		if !isValidFaultKind(rule.Kind) {
			errs = append(errs, fmt.Errorf("config: faults.rules[%d].kind %q is invalid", i, rule.Kind))
		}
		if !isValidFaultScope(rule.Scope) {
			errs = append(errs, fmt.Errorf("config: faults.rules[%d].scope %q is invalid", i, rule.Scope))
		}
		if rule.Enabled {
			if rule.Safety == nil || (rule.Safety["maxHits"] == nil && rule.Safety["expiresAfter"] == nil) {
				errs = append(errs, fmt.Errorf("config: enabled fault rule %q must define safety.maxHits or safety.expiresAfter", rule.Name))
			}
		}
	}

	// Recording duration validation
	if c.Recording.MaxDuration != "" {
		dur, err := time.ParseDuration(c.Recording.MaxDuration)
		if err != nil {
			errs = append(errs, fmt.Errorf("config: recording.maxDuration %q is invalid: %v", c.Recording.MaxDuration, err))
		} else if dur > time.Hour {
			errs = append(errs, fmt.Errorf("config: recording.maxDuration must be ≤ 1h"))
		}
	}
	if c.Recording.AutoStopOnIdle != "" && c.Recording.MaxDuration != "" {
		idle, err1 := time.ParseDuration(c.Recording.AutoStopOnIdle)
		max, err2 := time.ParseDuration(c.Recording.MaxDuration)
		if err1 == nil && err2 == nil && idle >= max {
			errs = append(errs, fmt.Errorf("config: recording.autoStopOnIdle must be less than recording.maxDuration"))
		}
	}

	return errs
}

func (c *Config) validateService(name string, svc ServiceConfig) []error {
	var errs []error

	validTypes := map[string]bool{
		"http": true, "postgres": true, "redis": true,
		"mailpit": true, "minio": true, "localstack": true,
		"worker": true, "stripe": true,
	}
	if !validTypes[svc.Type] {
		errs = append(errs, fmt.Errorf("config: services.%s.type %q is unsupported", name, svc.Type))
	}

	switch svc.Type {
	case "postgres":
		if svc.DSN == "" {
			errs = append(errs, fmt.Errorf("config: services.%s.dsn is required for postgres", name))
		}
		if svc.Capture.Mode != "" && svc.Capture.Mode != "audit_trigger" && svc.Capture.Mode != "disabled" {
			errs = append(errs, fmt.Errorf("config: services.%s.capture.mode must be audit_trigger or disabled", name))
		}
		if len(svc.Capture.Tables) == 0 && svc.Capture.Mode != "disabled" {
			errs = append(errs, fmt.Errorf("config: services.%s postgres capture requires explicit tables; wildcard capture is not supported in V1", name))
		}
	case "redis":
		if svc.Addr == "" {
			errs = append(errs, fmt.Errorf("config: services.%s.addr is required for redis", name))
		}
	case "mailpit":
		if svc.APIUrl == "" {
			errs = append(errs, fmt.Errorf("config: services.%s.apiUrl is required for mailpit", name))
		}
		if svc.APIUrl != "" && !isLocalURL(svc.APIUrl) {
			errs = append(errs, fmt.Errorf("config: services.%s.apiUrl must use localhost, 127.0.0.1, or an explicitly allowed local host", name))
		}
		if svc.Capture.PollInterval != "" {
			dur, err := time.ParseDuration(svc.Capture.PollInterval)
			if err != nil {
				errs = append(errs, fmt.Errorf("config: services.%s.capture.pollInterval %q is invalid", name, svc.Capture.PollInterval))
			} else if dur < 500*time.Millisecond {
				errs = append(errs, fmt.Errorf("config: services.%s.capture.pollInterval minimum is 500ms", name))
			}
		}
	case "minio":
		if svc.Endpoint == "" {
			errs = append(errs, fmt.Errorf("config: services.%s.endpoint is required for minio", name))
		}
		if svc.SecretKey != "" && !strings.HasPrefix(svc.SecretKey, "env:") {
			errs = append(errs, fmt.Errorf("config: services.%s.secretKey contains a literal value; use env:MINIO_ROOT_PASSWORD or pass --allow-literal-secrets", name))
		}
	case "localstack":
		if svc.Endpoint == "" {
			errs = append(errs, fmt.Errorf("config: services.%s.endpoint is required for localstack", name))
		}
	}

	return errs
}

func isLocalURL(u string) bool {
	// Simple check for localhost/127.0.0.1/::1 in the URL
	for _, local := range []string{"localhost", "127.0.0.1", "::1", "[::1]"} {
		if strings.Contains(u, local) {
			return true
		}
	}
	// Also accept bare IPs that resolve to loopback
	host := u
	if strings.Contains(host, "://") {
		parts := strings.SplitN(host, "://", 2)
		host = parts[1]
	}
	host = strings.SplitN(host, "/", 2)[0]
	host = strings.SplitN(host, ":", 2)[0]
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isValidFaultKind(k string) bool {
	switch k {
	case "delay_response", "force_http_status", "drop_outbound_request",
		"mutate_json_response", "simulate_timeout", "delay_queue_processing",
		"block_email_delivery":
		return true
	}
	return false
}

func isValidFaultScope(s string) bool {
	switch s {
	case "live", "replay", "both":
		return true
	}
	return false
}

// DefaultConfig returns a minimal default configuration.
func DefaultConfig() *Config {
	cfg := &Config{
		Project: ProjectConfig{Name: "my-app"},
	}
	cfg.applyDefaults()
	return cfg
}

// DemoSaaSConfig returns a complete demo-saas example configuration.
func DemoSaaSConfig() *Config {
	cfg := &Config{
		Project: ProjectConfig{
			Name:    "demo-saas",
			DataDir: ".localcloud",
		},
		Agent: AgentConfig{
			Bind:              "127.0.0.1",
			Port:              41777,
			StudioPort:        41778,
			Database:          ".localcloud/localcloud.db",
			CorrelationHeader: "x-localcloud-correlation-id",
		},
		Compose: ComposeConfig{
			Files:       []string{"docker-compose.yml"},
			ProjectName: "localcloud-demo-saas",
			AutoStart:   true,
		},
		Services: map[string]ServiceConfig{
			"api": {
				Type:       "http",
				BaseURL:    "http://localhost:3000",
				Container:  "demo-api",
				HealthPath: "/health",
				Capture: CaptureConfig{
					Inbound:       true,
					Outbound:      true,
					ProxyPort:     41800,
					ReplayBaseURL: "http://localhost:3000",
				},
			},
			"worker": {
				Type:      "worker",
				Container: "demo-worker",
			},
			"postgres": {
				Type: "postgres",
				DSN:  "postgres://localcloud:localcloud@localhost:5432/demo?sslmode=disable",
				Capture: CaptureConfig{
					Mode:    "audit_trigger",
					Schemas: []string{"public"},
					Tables:  []string{"users"},
					RedactColumns: []string{"users.password_hash", "users.reset_token"},
				},
			},
			"redis": {
				Type: "redis",
				Addr: "localhost:6379",
				Capture: CaptureConfig{
					Mode:   "monitor",
					Queues: []string{"email_jobs", "default"},
				},
			},
			"mailpit": {
				Type:     "mailpit",
				APIUrl:   "http://localhost:8025",
				SMTPHost: "localhost",
				SMTPPort: 1025,
				Capture: CaptureConfig{
					PollInterval: "1s",
					RedactBody:   true,
				},
			},
		},
		Redaction: RedactionConfig{
			Headers:        DefaultRedactHeaders(),
			JSONPaths:      []string{"$.password", "$.token", "$.access_token", "$.refresh_token"},
			BodyMaxBytes:   8192,
			StoreRawBodies: false,
		},
		Recording: RecordingConfig{
			IncludeUncorrelatedEvents: true,
			DefaultTags:              []string{"demo"},
			MaxDuration:              "10m",
		},
		Replay: ReplayConfig{
			DefaultTargetBaseURL:          "http://localhost:3000",
			RequireConfirmationForMethods: []string{"POST", "PUT", "PATCH", "DELETE"},
			RegenerateCorrelationIDs:      true,
			Timeout:                       "10s",
		},
		Faults: FaultsConfig{
			Enabled: true,
			Rules: []FaultRuleConfig{
				{
					Name:    "force-signup-500",
					Enabled: false,
					Kind:    "force_http_status",
					Scope:   "replay",
					Match:   map[string]any{"service": "api", "method": "POST", "path": "/signup"},
					Action:  map[string]any{"statusCode": 500, "bodyJson": map[string]any{"error": "forced by LocalCloud"}},
					Safety:  map[string]any{"maxHits": 5},
				},
				{
					Name:    "block-welcome-email",
					Enabled: false,
					Kind:    "block_email_delivery",
					Scope:   "both",
					Match:   map[string]any{"service": "mailpit", "emailToContains": "@example.test"},
					Action:  map[string]any{"reason": "blocked by LocalCloud fault rule"},
					Safety:  map[string]any{"maxHits": 10},
				},
			},
		},
	}
	cfg.applyDefaults()
	return cfg
}
