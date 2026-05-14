package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func cmdRecord(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "Scenario name (required)")
	desc := fs.String("desc", "", "Scenario description")
	tags := fs.String("tags", "", "Comma-separated tags")
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *name == "" {
		fmt.Fprintln(stderr, "localcloud record: --name is required")
		return 1
	}

	var tagList []string
	if *tags != "" {
		for _, t := range strings.Split(*tags, ",") {
			tagList = append(tagList, strings.TrimSpace(t))
		}
	}

	body := map[string]any{
		"name":        *name,
		"description": *desc,
		"tags":        tagList,
	}
	jsonBody, _ := json.Marshal(body)

	url := fmt.Sprintf("http://%s/api/scenarios/start", *addr)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonBody)))
	if err != nil {
		fmt.Fprintf(stderr, "localcloud record: cannot reach agent at %s: %v\n", *addr, err)
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
		fmt.Fprintf(stderr, "localcloud record: %s\n", msg)
		return 1
	}

	fmt.Fprintf(stdout, "Recording started: %s\n", *name)
	if id, ok := result["id"].(string); ok {
		fmt.Fprintf(stdout, "  Scenario ID: %s\n", id)
	}
	fmt.Fprintln(stdout, "  All captured events will be tagged to this scenario.")
	fmt.Fprintln(stdout, "  Run 'localcloud stop' to finish recording.")
	return 0
}

func cmdStop(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	url := fmt.Sprintf("http://%s/api/scenarios/stop", *addr)
	resp, err := http.Post(url, "application/json", strings.NewReader("{}"))
	if err != nil {
		fmt.Fprintf(stderr, "localcloud stop: cannot reach agent at %s: %v\n", *addr, err)
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
		fmt.Fprintf(stderr, "localcloud stop: %s\n", msg)
		return 1
	}

	name, _ := result["name"].(string)
	eventCount, _ := result["eventCount"].(float64)
	replayable, _ := result["replayableCount"].(float64)

	fmt.Fprintf(stdout, "Recording stopped: %s\n", name)
	fmt.Fprintf(stdout, "  Events captured: %d\n", int(eventCount))
	fmt.Fprintf(stdout, "  Replayable:      %d\n", int(replayable))
	return 0
}
