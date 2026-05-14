package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func cmdFault(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printFaultUsage(stderr)
		return 1
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "list":
		return faultList(subArgs, stdout, stderr)
	case "create":
		return faultCreate(subArgs, stdout, stderr)
	case "enable":
		return faultSetEnabled(subArgs, true, stdout, stderr)
	case "disable":
		return faultSetEnabled(subArgs, false, stdout, stderr)
	case "delete":
		return faultDelete(subArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "localcloud fault: unknown subcommand %q\n", sub)
		printFaultUsage(stderr)
		return 1
	}
}

func printFaultUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: localcloud fault <subcommand> [flags]

Subcommands:
  list      List all fault rules
  create    Create a new fault rule
  enable    Enable a fault rule by ID
  disable   Disable a fault rule by ID
  delete    Delete a fault rule by ID`)
}

func faultList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fault list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	url := fmt.Sprintf("http://%s/api/fault-rules", *addr)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud fault list: cannot reach agent at %s: %v\n", *addr, err)
		return 1
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	items, _ := result["items"].([]any)
	if len(items) == 0 {
		fmt.Fprintln(stdout, "No fault rules configured.")
		return 0
	}

	fmt.Fprintf(stdout, "%-12s %-20s %-10s %-25s %-8s %s\n",
		"ID", "NAME", "KIND", "MATCH", "ENABLED", "HITS")
	for _, item := range items {
		rule, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id, _ := rule["id"].(string)
		name, _ := rule["name"].(string)
		kind, _ := rule["kind"].(string)
		enabled, _ := rule["enabled"].(bool)
		hitCount, _ := rule["hitCount"].(float64)

		match, _ := rule["match"].(map[string]any)
		matchStr := formatMatch(match)

		enabledStr := "no"
		if enabled {
			enabledStr = "yes"
		}

		// Truncate ID for display
		if len(id) > 12 {
			id = id[:12]
		}

		fmt.Fprintf(stdout, "%-12s %-20s %-10s %-25s %-8s %d\n",
			id, name, kind, matchStr, enabledStr, int(hitCount))
	}
	return 0
}

func faultCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fault create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Rule name (required)")
	kind := fs.String("kind", "", "Fault kind (required): delay_response, force_http_status, drop_outbound_request, mutate_json_response, simulate_timeout")
	scope := fs.String("scope", "both", "Scope: live, replay, or both")
	service := fs.String("service", "", "Target service name")
	method := fs.String("method", "", "HTTP method to match")
	pathPrefix := fs.String("path-prefix", "", "Path prefix to match")
	statusCode := fs.Int("status-code", 0, "Status code for force_http_status")
	delayMs := fs.Int("delay-ms", 0, "Delay in ms for delay_response/simulate_timeout")
	reason := fs.String("reason", "", "Error reason message")
	maxHits := fs.Int("max-hits", 0, "Safety: max number of hits (0=unlimited)")
	expiresAfter := fs.String("expires-after", "", "Safety: auto-expire duration (e.g. 30m, 1h)")
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *name == "" || *kind == "" {
		fmt.Fprintln(stderr, "localcloud fault create: --name and --kind are required")
		return 1
	}

	rule := map[string]any{
		"name":    *name,
		"kind":    *kind,
		"scope":   *scope,
		"enabled": true,
		"match":   map[string]any{},
		"action":  map[string]any{},
		"safety":  map[string]any{},
	}

	matchMap := rule["match"].(map[string]any)
	if *service != "" {
		matchMap["service"] = *service
	}
	if *method != "" {
		matchMap["method"] = *method
	}
	if *pathPrefix != "" {
		matchMap["pathPrefix"] = *pathPrefix
	}

	actionMap := rule["action"].(map[string]any)
	if *statusCode > 0 {
		actionMap["statusCode"] = *statusCode
	}
	if *delayMs > 0 {
		actionMap["delayMs"] = *delayMs
	}
	if *reason != "" {
		actionMap["reason"] = *reason
	}

	safetyMap := rule["safety"].(map[string]any)
	if *maxHits > 0 {
		safetyMap["maxHits"] = *maxHits
	}
	if *expiresAfter != "" {
		safetyMap["expiresAfter"] = *expiresAfter
	}

	jsonBody, _ := json.Marshal(rule)
	url := fmt.Sprintf("http://%s/api/fault-rules", *addr)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonBody)))
	if err != nil {
		fmt.Fprintf(stderr, "localcloud fault create: cannot reach agent at %s: %v\n", *addr, err)
		return 1
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusCreated {
		errData, _ := result["error"].(map[string]any)
		msg := "unknown error"
		if errData != nil {
			if m, ok := errData["message"].(string); ok {
				msg = m
			}
		}
		fmt.Fprintf(stderr, "localcloud fault create: %s\n", msg)
		return 1
	}

	id, _ := result["id"].(string)
	fmt.Fprintf(stdout, "Fault rule created: %s (%s)\n", *name, id)
	return 0
}

func faultSetEnabled(args []string, enabled bool, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fault enable/disable", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "localcloud fault enable/disable: rule ID is required")
		return 1
	}
	ruleID := fs.Arg(0)

	body, _ := json.Marshal(map[string]any{"enabled": enabled})
	url := fmt.Sprintf("http://%s/api/fault-rules/%s", *addr, ruleID)

	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(string(body)))
	if err != nil {
		fmt.Fprintf(stderr, "localcloud fault: %v\n", err)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud fault: cannot reach agent at %s: %v\n", *addr, err)
		return 1
	}
	defer resp.Body.Close()

	action := "enabled"
	if !enabled {
		action = "disabled"
	}

	if resp.StatusCode == http.StatusOK {
		fmt.Fprintf(stdout, "Fault rule %s: %s\n", action, ruleID)
		return 0
	}

	fmt.Fprintf(stderr, "localcloud fault: failed to update rule (status %d)\n", resp.StatusCode)
	return 1
}

func faultDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fault delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "localcloud fault delete: rule ID is required")
		return 1
	}
	ruleID := fs.Arg(0)

	url := fmt.Sprintf("http://%s/api/fault-rules/%s", *addr, ruleID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud fault: %v\n", err)
		return 1
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud fault: cannot reach agent at %s: %v\n", *addr, err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Fprintf(stdout, "Fault rule deleted: %s\n", ruleID)
		return 0
	}

	fmt.Fprintf(stderr, "localcloud fault: failed to delete rule (status %d)\n", resp.StatusCode)
	return 1
}

func formatMatch(m map[string]any) string {
	if m == nil {
		return "*"
	}
	var parts []string
	if s, ok := m["service"].(string); ok && s != "" {
		parts = append(parts, "svc="+s)
	}
	if s, ok := m["method"].(string); ok && s != "" {
		parts = append(parts, s)
	}
	if s, ok := m["pathPrefix"].(string); ok && s != "" {
		parts = append(parts, s+"*")
	}
	if s, ok := m["path"].(string); ok && s != "" {
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return "*"
	}
	result := strings.Join(parts, " ")
	if len(result) > 25 {
		result = result[:22] + "..."
	}
	return result
}
