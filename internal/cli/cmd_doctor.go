package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"github.com/localcloud-dev/localcloud/internal/config"
)

func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	cfgPath := "localcloud.yml"
	verbose := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 < len(args) {
				i++
				cfgPath = args[i]
			}
		case "--verbose":
			verbose = true
		case "--help", "-h":
			fmt.Fprintln(stdout, `Usage: localcloud doctor [flags]

Flags:
  --config <path>    Config file path (default: localcloud.yml)
  --verbose          Show detailed check output`)
			return 0
		}
	}

	passed := 0
	warned := 0
	failed := 0

	check := func(name string, fn func() (string, bool)) {
		msg, ok := fn()
		if ok {
			fmt.Fprintf(stdout, "  ✓ %s: %s\n", name, msg)
			passed++
		} else {
			fmt.Fprintf(stdout, "  ✗ %s: %s\n", name, msg)
			failed++
		}
	}

	warn := func(name string, fn func() (string, bool)) {
		msg, ok := fn()
		if ok {
			fmt.Fprintf(stdout, "  ✓ %s: %s\n", name, msg)
			passed++
		} else {
			fmt.Fprintf(stdout, "  ⚠ %s: %s\n", name, msg)
			warned++
		}
	}

	fmt.Fprintln(stdout, "LocalCloud Doctor")
	fmt.Fprintln(stdout, "")

	// Config
	check("Config file", func() (string, bool) {
		_, err := os.Stat(cfgPath)
		if err != nil {
			return fmt.Sprintf("%s not found; run localcloud init", cfgPath), false
		}
		return cfgPath, true
	})

	var cfg *config.Config
	check("Config valid", func() (string, bool) {
		var err error
		cfg, err = config.LoadFile(cfgPath)
		if err != nil {
			return err.Error(), false
		}
		errs := cfg.Validate()
		if len(errs) > 0 {
			return errs[0].Error(), false
		}
		return "ok", true
	})

	// Docker
	warn("Docker available", func() (string, bool) {
		path, err := exec.LookPath("docker")
		if err != nil {
			return "docker not found in PATH", false
		}
		if verbose {
			return path, true
		}
		return "found", true
	})

	warn("Docker Compose", func() (string, bool) {
		cmd := exec.Command("docker", "compose", "version")
		out, err := cmd.Output()
		if err != nil {
			return "docker compose not available", false
		}
		if verbose {
			return string(out), true
		}
		return "found", true
	})

	// Ports
	if cfg != nil {
		checkPort := func(name, addr string) {
			warn(name, func() (string, bool) {
				ln, err := net.Listen("tcp", addr)
				if err != nil {
					return fmt.Sprintf("%s in use or unavailable", addr), false
				}
				ln.Close()
				return fmt.Sprintf("%s available", addr), true
			})
		}

		agentAddr := fmt.Sprintf("%s:%d", cfg.Agent.Bind, cfg.Agent.Port)
		studioAddr := fmt.Sprintf("%s:%d", cfg.Agent.Bind, cfg.Agent.StudioPort)
		checkPort("Agent port", agentAddr)
		checkPort("Studio port", studioAddr)
	}

	// Binding safety
	if cfg != nil {
		warn("Network binding", func() (string, bool) {
			if cfg.Agent.Bind == "0.0.0.0" {
				return "agent bound to 0.0.0.0; local traffic may be visible on network", false
			}
			return "loopback only", true
		})
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "Results: %d passed, %d warnings, %d failed\n", passed, warned, failed)

	if failed > 0 {
		return 2
	}
	return 0
}
