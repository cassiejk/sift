package cli

import (
	"fmt"
	"io"
)

// Run implements only local scaffolding commands. No Fastly requests are made.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stdout, "sift — Fastly security inspection (Go rewrite, early scaffold)\nUsage: sift [help|version]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "sift: unexpected arguments; run: sift help")
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, "sift — Fastly security inspection (Go rewrite, early scaffold)\nUsage: sift [help|version]")
		return 0
	case "version":
		fmt.Fprintln(stdout, "sift dev (unreleased)")
		return 0
	default:
		fmt.Fprintf(stderr, "sift: unknown command %q; run: sift help\n", args[0])
		return 2
	}
}
