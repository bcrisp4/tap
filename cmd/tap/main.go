// Command tap is the Tap feed reader binary.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/bcrisp4/tap/internal/version"
)

func main() {
	os.Exit(run(os.Args, os.Stdout))
}

func run(args []string, out io.Writer) int {
	for _, a := range args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Fprintf(out, "tap %s\n", version.String())
			return 0
		}
	}
	return 0
}
