package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func cmdReplay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scenarioID := fs.String("scenario", "", "Scenario ID to replay (required)")
	baseURL := fs.String("base-url", "", "Target base URL (required)")
	skipUnsafe := fs.Bool("skip-unsafe", false, "Skip non-idempotent methods (POST, PUT, DELETE, PATCH)")
	confirmUnsafe := fs.Bool("confirm-unsafe", false, "Allow non-idempotent methods")
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *scenarioID == "" {
		fmt.Fprintln(stderr, "localcloud replay: --scenario is required")
		return 1
	}
	if *baseURL == "" {
		fmt.Fprintln(stderr, "localcloud replay: --base-url is required")
		return 1
	}

	body := map[string]any{
		"baseUrl":       *baseURL,
		"skipUnsafe":    *skipUnsafe,
		"confirmUnsafe": *confirmUnsafe,
	}
	jsonBody, _ := json.Marshal(body)

	url := fmt.Sprintf("http://%s/api/scenarios/%s/replay", *addr, *scenarioID)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonBody)))
	if err != nil {
		fmt.Fprintf(stderr, "localcloud replay: cannot reach agent at %s: %v\n", *addr, err)
		return 1
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		errData, _ := result["error"].(map[string]any)
		msg := "unknown error"
		if errData != nil {
			if m, ok := errData["message"].(string); ok {
				msg = m
			}
		}
		fmt.Fprintf(stderr, "localcloud replay: %s\n", msg)
		return 1
	}

	total, _ := result["total"].(float64)
	passed, _ := result["passed"].(float64)
	failed, _ := result["failed"].(float64)
	skipped, _ := result["skipped"].(float64)
	runID, _ := result["runId"].(string)

	fmt.Fprintf(stdout, "Replay completed: %s\n", runID)
	fmt.Fprintf(stdout, "  Total:   %d\n", int(total))
	fmt.Fprintf(stdout, "  Passed:  %d\n", int(passed))
	fmt.Fprintf(stdout, "  Failed:  %d\n", int(failed))
	if int(skipped) > 0 {
		fmt.Fprintf(stdout, "  Skipped: %d\n", int(skipped))
	}

	// Print diffs if any
	diffs, _ := result["diffs"].([]any)
	for _, d := range diffs {
		diff, ok := d.(map[string]any)
		if !ok {
			continue
		}
		match, _ := diff["statusMatch"].(bool)
		method, _ := diff["method"].(string)
		path, _ := diff["path"].(string)
		origStatus, _ := diff["originalStatus"].(float64)
		replayStatus, _ := diff["replayStatus"].(float64)
		errMsg, _ := diff["error"].(string)

		status := "PASS"
		if !match || errMsg != "" {
			status = "FAIL"
		}

		fmt.Fprintf(stdout, "\n  %s %s %s  original=%d replay=%d",
			status, method, path, int(origStatus), int(replayStatus))
		if errMsg != "" {
			fmt.Fprintf(stdout, "  error=%s", errMsg)
		}
	}
	fmt.Fprintln(stdout)

	if int(failed) > 0 {
		return 1
	}
	return 0
}
