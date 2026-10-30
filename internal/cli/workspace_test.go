package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestWorkspaceCLI(t *testing.T) {
	for _, tc := range []struct {
		name            string
		args            []string
		token, customer string
		code, calls     int
	}{
		{"success", []string{"ngwaf", "workspace", "Production"}, "synthetic-secret", "customer-test", 0, 3},
		{"missing token", []string{"ngwaf", "workspace", "Production"}, "", "customer-test", 1, 0},
		{"missing selection", []string{"ngwaf", "workspace", "Production"}, "synthetic-secret", "", 1, 0},
		{"extra args", []string{"ngwaf", "workspace", "Production", "--json"}, "synthetic-secret", "customer-test", 2, 0},
		{"missing args", []string{"ngwaf", "workspace"}, "synthetic-secret", "customer-test", 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, envReads := 0, 0
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				body := ""
				switch r.URL.Path {
				case "/customer/customer-test":
					body = `{"id":"customer-test","name":"Synthetic"}`
				case "/ngwaf/v1/workspaces":
					body = `{"meta":{"total":1},"data":[{"id":"workspace-test","name":"Production"}]}`
				case "/ngwaf/v1/workspaces/workspace-test":
					body = `{"id":"workspace-test","workspace_uid":null,"mode":"log"}`
				default:
					t.Fatal("unexpected request")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})}
			var out, errOut bytes.Buffer
			code := run(context.Background(), tc.args, &out, &errOut, func(k string) string {
				envReads++
				switch k {
				case "FASTLY_API_TOKEN":
					return tc.token
				case "FASTLY_CUSTOMER_ID":
					return tc.customer
				}
				return ""
			}, client)
			if code != tc.code || calls != tc.calls {
				t.Fatalf("code=%d calls=%d stderr=%q", code, calls, errOut.String())
			}
			if tc.code == 0 {
				if !strings.Contains(out.String(), `"customer_scoped": true`) || !strings.Contains(out.String(), `"workspace_uid": null`) || !strings.Contains(errOut.String(), "scope verified") || !strings.Contains(out.String(), `"selection_source": "environment"`) || !strings.Contains(errOut.String(), "inventory progress: 1 pages, 1/1 rows") {
					t.Fatal("missing output/scope")
				}
			} else if out.Len() != 0 || errOut.Len() == 0 {
				t.Fatal("incorrect failure streams")
			}
			if tc.code == 2 && envReads != 0 {
				t.Fatal("syntax checked after environment access")
			}
			if strings.Contains(out.String()+errOut.String(), "synthetic-secret") {
				t.Fatal("credential leak")
			}
		})
	}
}
