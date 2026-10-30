package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cassiejk/sift/internal/config"
	"github.com/cassiejk/sift/internal/fastly"
	"github.com/cassiejk/sift/internal/ngwaf"
)

func runWorkspace(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string, client *http.Client) int {
	if len(args) != 2 || args[0] != "workspace" || args[1] == "" || strings.HasPrefix(args[1], "-") {
		fmt.Fprintln(stderr, "sift: usage: sift ngwaf workspace <workspace> (exact ID or exact name; no flags)")
		return 2
	}
	token := getenv("FASTLY_API_TOKEN")
	if token == "" {
		fmt.Fprintln(stderr, "sift: FASTLY_API_TOKEN is not set")
		return 1
	}
	customer, source, err := config.CustomerSelection(getenv, os.ReadFile)
	if err != nil {
		fmt.Fprintln(stderr, "sift:", err)
		return 1
	}
	fmt.Fprintln(stderr, "sift: verifying customer and retrieving complete workspace inventory (maximum 1000 pages; 120s deadline)")
	report, err := ngwaf.InspectWorkspace(ctx, fastly.Client{HTTP: client}, token, customer, args[1], func(pages, rows, total int) {
		if pages == 1 || pages%10 == 0 || rows == total {
			fmt.Fprintf(stderr, "sift: workspace inventory progress: %d pages, %d/%d rows\n", pages, rows, total)
		}
	})
	if err != nil {
		fmt.Fprintln(stderr, "sift:", err)
		return 1
	}
	report.Provenance.CustomerIdentity.SelectionSource = source
	fmt.Fprintf(stderr, "sift: customer and workspace scope verified (source=%s)\n", source)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(stderr, "sift: could not write workspace output")
		return 1
	}
	return 0
}
