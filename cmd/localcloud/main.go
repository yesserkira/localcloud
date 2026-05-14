package main

import (
	"os"

	"github.com/localcloud-dev/localcloud/internal/cli"
)

// Set by build flags: go build -ldflags "-X main.version=0.1.0"
var version = "dev"

func main() {
	code := cli.Run(os.Args[1:], version)
	os.Exit(code)
}
