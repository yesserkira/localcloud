package cli

import (
	"fmt"
	"io"
	"os"
)

// version is set during Run and available to all subcommands in this package.
var version string

// Run is the CLI entrypoint. It parses args and dispatches to commands.
// Returns the process exit code.
func Run(args []string, ver string) int {
	version = ver
	return run(args, version, os.Stdout, os.Stderr)
}

func run(args []string, ver string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout, ver)
		return 0
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "localcloud %s\n", ver)
		return 0
	case "help", "--help", "-h":
		printUsage(stdout, ver)
		return 0
	case "init":
		return cmdInit(cmdArgs, stdout, stderr)
	case "up":
		return cmdUp(cmdArgs, stdout, stderr)
	case "down":
		return cmdDown(cmdArgs, stdout, stderr)
	case "status":
		return cmdStatus(cmdArgs, stdout, stderr)
	case "studio":
		return cmdStudio(cmdArgs, stdout, stderr)
	case "record":
		return cmdRecord(cmdArgs, stdout, stderr)
	case "stop":
		return cmdStop(cmdArgs, stdout, stderr)
	case "replay":
		return cmdReplay(cmdArgs, stdout, stderr)
	case "export":
		return cmdExport(cmdArgs, stdout, stderr)
	case "doctor":
		return cmdDoctor(cmdArgs, stdout, stderr)
	case "fault":
		return cmdFault(cmdArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "localcloud: unknown command %q\n", cmd)
		fmt.Fprintf(stderr, "Run 'localcloud help' for usage.\n")
		return 1
	}
}

func printUsage(w io.Writer, version string) {
	fmt.Fprintf(w, `localcloud %s — local development control plane

Usage:
  localcloud <command> [flags]

Commands:
  init        Create localcloud.yml and project scaffolding
  up          Start agent, adapters, and optionally Docker Compose stack
  down        Stop agent and optionally Compose services
  status      Show agent, adapter, and service health
  studio      Open Studio dashboard in browser
  record      Start recording a named scenario
  stop        Stop active recording
  replay      Replay a scenario's captured requests
  export      Export a scenario as portable JSON
  doctor      Diagnose config, ports, Docker, and adapters
  fault       Manage fault injection rules
  version     Print version

Run 'localcloud <command> --help' for command-specific help.
`, version)
}
