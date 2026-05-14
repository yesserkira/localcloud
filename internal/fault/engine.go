package fault

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/localcloud-dev/localcloud/internal/storage"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Engine manages active fault rules and applies them to HTTP requests.
type Engine struct {
	faults *storage.FaultRuleRepository
	logger *slog.Logger
}

// NewEngine creates a fault injection engine.
func NewEngine(db *storage.DB, logger *slog.Logger) *Engine {
	return &Engine{
		faults: storage.NewFaultRuleRepository(db),
		logger: logger,
	}
}

// FaultResult describes the fault action to apply (returned by Check).
type FaultResult struct {
	Rule   *timeline.FaultRule
	Action timeline.FaultAction
}

// Check evaluates all enabled rules against the request and returns
// the first match, or nil if no fault applies.
func (e *Engine) Check(ctx context.Context, method, path, service, host string, headers map[string]string) (*FaultResult, error) {
	rules, err := e.faults.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("fault: list rules: %w", err)
	}

	matched := FindFirstMatch(rules, method, path, service, host, headers)
	if matched == nil {
		return nil, nil
	}

	// Increment hit count
	if err := e.faults.IncrementHitCount(ctx, matched.ID); err != nil {
		e.logger.Error("fault: increment hit count", "ruleID", matched.ID, "err", err)
	}

	e.logger.Info("fault matched",
		"rule", matched.Name,
		"kind", matched.Kind,
		"method", method,
		"path", path,
	)

	return &FaultResult{Rule: matched, Action: matched.Action}, nil
}

// Apply executes the fault action on an HTTP response writer.
// Returns true if the request was fully handled (caller should not proxy).
func Apply(w http.ResponseWriter, result *FaultResult) bool {
	if result == nil {
		return false
	}

	rule := result.Rule
	action := result.Action

	switch rule.Kind {
	case timeline.FaultKindDelayResponse:
		// Delay then let the request proceed normally
		time.Sleep(time.Duration(action.DelayMs) * time.Millisecond)
		return false

	case timeline.FaultKindForceHTTPStatus:
		w.Header().Set("X-LocalCloud-Fault", rule.Name)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(action.StatusCode)
		body := map[string]any{
			"error":  action.Reason,
			"fault":  rule.Name,
			"status": action.StatusCode,
		}
		json.NewEncoder(w).Encode(body)
		return true

	case timeline.FaultKindDropOutbound:
		// Close connection immediately
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, err := hj.Hijack()
			if err == nil {
				conn.Close()
				return true
			}
		}
		// Fallback: return 502
		w.Header().Set("X-LocalCloud-Fault", rule.Name)
		w.WriteHeader(http.StatusBadGateway)
		return true

	case timeline.FaultKindMutateJSON:
		w.Header().Set("X-LocalCloud-Fault", rule.Name)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(action.BodyJSON)
		return true

	case timeline.FaultKindSimulateTimeout:
		// Wait until the delay expires, then close connection
		time.Sleep(time.Duration(action.DelayMs) * time.Millisecond)
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, err := hj.Hijack()
			if err == nil {
				conn.Close()
				return true
			}
		}
		w.WriteHeader(http.StatusGatewayTimeout)
		return true

	default:
		return false
	}
}
