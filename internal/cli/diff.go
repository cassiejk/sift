package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cassiejk/sift/internal/snapshot"
)

func runDiff(args []string, stdout, stderr io.Writer) int {
	usage := func() int {
		fmt.Fprintln(stderr, "sift: usage: sift diff <before.json> <after.json> [--json] [--check]")
		return 2
	}
	if len(args) < 2 {
		return usage()
	}
	asJSON, check := false, false
	for _, arg := range args[2:] {
		switch arg {
		case "--json":
			asJSON = true
		case "--check":
			check = true
		default:
			return usage()
		}
	}
	before, err := snapshot.Load(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "sift: before snapshot:", err)
		return 1
	}
	after, err := snapshot.Load(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "sift: after snapshot:", err)
		return 1
	}
	report, err := snapshot.Compare(before, after)
	if err != nil {
		fmt.Fprintln(stderr, "sift:", err)
		return 1
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(report)
	} else {
		err = report.WriteHuman(stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "sift: could not write diff output")
		return 1
	}
	if check && report.Changed {
		return 1
	}
	return 0
}
