package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cassiejk/sift/internal/fastly"
)

// Run executes sift with the process environment and a background context.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

// RunContext allows the executable to propagate signal cancellation.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return run(ctx, args, stdout, stderr, os.Getenv, nil)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string, httpClient *http.Client) int {
	if len(args) > 0 && args[0] == "diff" {
		return runDiff(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "customer" {
		if len(args) != 2 || args[1] != "whoami" {
			fmt.Fprintln(stderr, "sift: usage: sift customer whoami (no arguments or options)")
			return 2
		}
		info, err := (fastly.Client{HTTP: httpClient}).Whoami(ctx, getenv("FASTLY_API_TOKEN"))
		if err != nil {
			fmt.Fprintln(stderr, "sift:", err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			fmt.Fprintln(stderr, "sift: could not write JSON output")
			return 1
		}
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(stdout, "sift — Fastly security inspection (Go rewrite, early scaffold)\nUsage: sift [help|version]\n       sift customer whoami\n       sift diff <before.json> <after.json> [--json] [--check]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "sift: unexpected arguments; run: sift help")
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, "sift — Fastly security inspection (Go rewrite, early scaffold)\nUsage: sift [help|version]\n       sift customer whoami\n       sift diff <before.json> <after.json> [--json] [--check]")
		return 0
	case "version":
		fmt.Fprintln(stdout, "sift dev (unreleased)")
		return 0
	default:
		fmt.Fprintln(stderr, "sift: unknown command; run: sift help")
		return 2
	}
}
