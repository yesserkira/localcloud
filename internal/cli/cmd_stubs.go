package cli

import (
	"fmt"
	"io"
)

func cmdStudio(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "localcloud: 'studio' is not yet implemented")
	return 1
}
