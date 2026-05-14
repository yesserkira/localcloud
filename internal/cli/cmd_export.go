package cli

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func cmdExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scenarioID := fs.String("scenario", "", "Scenario ID to export (required)")
	output := fs.String("output", "", "Output file path (default: stdout)")
	addr := fs.String("addr", "127.0.0.1:41778", "Agent API address")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *scenarioID == "" {
		fmt.Fprintln(stderr, "localcloud export: --scenario is required")
		return 1
	}

	url := fmt.Sprintf("http://%s/api/scenarios/%s/export", *addr, *scenarioID)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		fmt.Fprintf(stderr, "localcloud export: cannot reach agent at %s: %v\n", *addr, err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := make([]byte, 1024)
		n, _ := resp.Body.Read(buf)
		fmt.Fprintf(stderr, "localcloud export: %s\n", string(buf[:n]))
		return 1
	}

	var w io.Writer = stdout
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(stderr, "localcloud export: cannot create file: %v\n", err)
			return 1
		}
		defer f.Close()
		w = f
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	if *output != "" {
		fmt.Fprintf(stderr, "Exported to %s\n", *output)
	}
	return 0
}
