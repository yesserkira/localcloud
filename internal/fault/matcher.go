package fault

import (
	"strings"
	"time"

	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Match tests whether a fault rule matches an incoming request.
func Match(rule *timeline.FaultRule, method, path, service, host string, headers map[string]string) bool {
	if !rule.Enabled {
		return false
	}

	// Check safety limits
	if rule.Safety.MaxHits > 0 && rule.HitCount >= rule.Safety.MaxHits {
		return false
	}
	if rule.Safety.ExpiresAfter != "" {
		dur, err := time.ParseDuration(rule.Safety.ExpiresAfter)
		if err == nil && time.Since(rule.CreatedAt) > dur {
			return false
		}
	}

	m := rule.Match

	if m.Service != "" && !strings.EqualFold(m.Service, service) {
		return false
	}
	if m.Method != "" && !strings.EqualFold(m.Method, method) {
		return false
	}
	if m.Path != "" && m.Path != path {
		return false
	}
	if m.PathPrefix != "" && !strings.HasPrefix(path, m.PathPrefix) {
		return false
	}
	if m.Host != "" && !strings.EqualFold(m.Host, host) {
		return false
	}

	// Header matching
	for k, v := range m.Headers {
		actual, ok := headers[k]
		if !ok || !strings.EqualFold(actual, v) {
			return false
		}
	}

	return true
}

// FindFirstMatch returns the first matching enabled rule, or nil.
func FindFirstMatch(rules []timeline.FaultRule, method, path, service, host string, headers map[string]string) *timeline.FaultRule {
	for i := range rules {
		if Match(&rules[i], method, path, service, host, headers) {
			return &rules[i]
		}
	}
	return nil
}

// ValidateRule checks a FaultRule for configuration errors.
func ValidateRule(rule *timeline.FaultRule) []string {
	var errs []string

	if rule.Name == "" {
		errs = append(errs, "name is required")
	}

	switch rule.Kind {
	case timeline.FaultKindDelayResponse:
		if rule.Action.DelayMs <= 0 {
			errs = append(errs, "delay_response requires action.delayMs > 0")
		}
	case timeline.FaultKindForceHTTPStatus:
		if rule.Action.StatusCode < 100 || rule.Action.StatusCode > 599 {
			errs = append(errs, "force_http_status requires valid action.statusCode (100-599)")
		}
	case timeline.FaultKindDropOutbound:
		// No additional params needed
	case timeline.FaultKindMutateJSON:
		if rule.Action.BodyJSON == nil || len(rule.Action.BodyJSON) == 0 {
			errs = append(errs, "mutate_json_response requires action.bodyJson")
		}
	case timeline.FaultKindSimulateTimeout:
		if rule.Action.DelayMs <= 0 {
			errs = append(errs, "simulate_timeout requires action.delayMs > 0")
		}
	case timeline.FaultKindDelayQueue, timeline.FaultKindBlockEmail:
		// Valid kinds with no additional validation
	default:
		errs = append(errs, "unknown fault kind: "+rule.Kind)
	}

	switch rule.Scope {
	case timeline.FaultScopeLive, timeline.FaultScopeReplay, timeline.FaultScopeBoth:
	default:
		errs = append(errs, "scope must be live, replay, or both")
	}

	// Safety limits validation
	if rule.Safety.MaxHits < 0 {
		errs = append(errs, "safety.maxHits must be >= 0")
	}
	if rule.Safety.ExpiresAfter != "" {
		if _, err := time.ParseDuration(rule.Safety.ExpiresAfter); err != nil {
			errs = append(errs, "safety.expiresAfter must be a valid duration (e.g. 30m, 1h)")
		}
	}

	return errs
}
