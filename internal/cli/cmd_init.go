package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/localcloud-dev/localcloud/internal/config"
)

func cmdInit(args []string, stdout, stderr io.Writer) int {
	cfgPath := "localcloud.yml"
	force := false
	example := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force", "-f":
			force = true
		case "--config":
			if i+1 < len(args) {
				i++
				cfgPath = args[i]
			}
		case "--example":
			if i+1 < len(args) {
				i++
				example = args[i]
			}
		case "--help", "-h":
			fmt.Fprintln(stdout, `Usage: localcloud init [flags]

Flags:
  --example <name>   Generate example project (e.g. demo-saas)
  --config <path>    Config file path (default: localcloud.yml)
  --force            Overwrite existing config`)
			return 0
		}
	}

	if !force {
		if _, err := os.Stat(cfgPath); err == nil {
			fmt.Fprintf(stderr, "localcloud: %s already exists. Use --force to overwrite.\n", cfgPath)
			return 2
		}
	}

	cfg := config.DefaultConfig()
	if example == "demo-saas" {
		cfg = config.DemoSaaSConfig()
	}

	data, err := cfg.Marshal()
	if err != nil {
		fmt.Fprintf(stderr, "localcloud: failed to generate config: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil && cfgPath != "localcloud.yml" {
		fmt.Fprintf(stderr, "localcloud: failed to create directory: %v\n", err)
		return 1
	}

	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "localcloud: failed to write %s: %v\n", cfgPath, err)
		return 1
	}

	if err := os.MkdirAll(".localcloud", 0o755); err != nil {
		fmt.Fprintf(stderr, "localcloud: warning: could not create .localcloud/: %v\n", err)
	}

	fmt.Fprintln(stdout, "LocalCloud initialized")
	fmt.Fprintf(stdout, "  config: %s\n", cfgPath)
	if example != "" {
		fmt.Fprintf(stdout, "  example: %s\n", example)
	}
	fmt.Fprintln(stdout, "  next: localcloud up")
	return 0
}
