package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
		contains string
	}{
		{"help", []string{"--help"}, 0, "Usage: sift"},
		{"version", []string{"version"}, 0, "sift dev"},
		{"unknown", []string{"nonexistent"}, 2, "unknown command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, err bytes.Buffer
			got := Run(tc.args, &out, &err)
			if got != tc.want || !strings.Contains(out.String()+err.String(), tc.contains) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", got, out.String(), err.String())
			}
		})
	}
}
