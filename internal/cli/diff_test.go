package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDiff(t *testing.T) {
	before, after := "../snapshot/testdata/before.json", "../snapshot/testdata/after.json"
	for _, tc := range []struct {
		name       string
		args       []string
		code       int
		jsonOutput bool
		changed    bool
	}{
		{"unchanged", []string{before, before, "--json", "--check"}, 0, true, false},
		{"changed", []string{before, after, "--json"}, 0, true, true},
		{"check changed", []string{before, after, "--check", "--json"}, 1, true, true},
		{"human", []string{before, after}, 0, false, true},
		{"missing operand", []string{before}, 2, false, false},
		{"unsupported flag", []string{before, after, "--output"}, 2, false, false},
		{"extra operand", []string{before, after, "extra"}, 2, false, false},
		{"missing file", []string{before, "nonexistent.json", "--json"}, 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { t.Fatal("diff attempted HTTP"); return nil, nil })}
			code := run(context.Background(), append([]string{"diff"}, tc.args...), &out, &errOut, func(string) string { t.Fatal("diff read environment"); return "" }, client)
			if code != tc.code {
				t.Fatalf("code=%d want %d", code, tc.code)
			}
			if tc.jsonOutput {
				var r struct {
					Changed    bool `json:"changed"`
					Provenance struct {
						API bool `json:"API_requests_made"`
					} `json:"provenance"`
				}
				if err := json.Unmarshal(out.Bytes(), &r); err != nil {
					t.Fatal(err)
				}
				if r.Changed != tc.changed || r.Provenance.API || errOut.Len() != 0 {
					t.Fatal("incorrect JSON/check output")
				}
			} else if tc.name == "human" {
				if !strings.Contains(out.String(), "NGWAF snapshot diff") || errOut.Len() != 0 {
					t.Fatal("missing human output")
				}
			} else if out.Len() != 0 || errOut.Len() == 0 {
				t.Fatal("failure polluted stdout or omitted diagnostic")
			}
		})
	}
}
