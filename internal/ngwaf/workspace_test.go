package ngwaf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/cassiejk/sift/internal/fastly"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const customerBody = `{"id":"customer-test","name":"Synthetic customer"}`
const singlePage = `{"meta":{"total":1},"data":[{"id":"workspace-test","name":"Production","customer_id":"customer-test"}]}`
const detailBody = `{"id":"workspace-test","name":"Production","customer_id":"customer-test","workspace_uid":"","mode":"block","attack_signal_thresholds":{"immediate":0},"default_blocking_response_code":403}`

func TestInspectWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name, input, customer, page, detail, diagnostic string
		wantCalls, failAt, status                       int
	}{
		{name: "exact id", input: "workspace-test", wantCalls: 3},
		{name: "exact ascii name", input: "pRODUCTION", wantCalls: 3},
		{name: "customer mismatch", input: "workspace-test", customer: `{"id":"other","name":"Other"}`, diagnostic: "customer-scope", wantCalls: 1},
		{name: "conflicting owner", input: "workspace-test", detail: strings.Replace(detailBody, "customer-test", "other", 1), diagnostic: "ownership", wantCalls: 3},
		{name: "unknown explicit owner", input: "workspace-test", detail: strings.Replace(detailBody, `"customer-test"`, `null`, 1), diagnostic: "ownership", wantCalls: 3},
		{name: "detail identity", input: "workspace-test", detail: strings.Replace(detailBody, `"id":"workspace-test"`, `"id":"other"`, 1), diagnostic: "identity mismatch", wantCalls: 3},
		{name: "inventory ownership", input: "workspace-test", page: strings.Replace(singlePage, "customer-test", "other", 1), diagnostic: "ownership", wantCalls: 2},
		{name: "incomplete page", input: "workspace-test", page: strings.Replace(singlePage, `"total":1`, `"total":2`, 1), diagnostic: "incomplete", wantCalls: 2},
		{name: "missing data", input: "workspace-test", page: `{"meta":{"total":0}}`, diagnostic: "data array", wantCalls: 2},
		{name: "missing total", input: "workspace-test", page: `{"data":[]}`, diagnostic: "total", wantCalls: 2},
		{name: "duplicate id", input: "workspace-test", page: `{"meta":{"total":2},"data":[{"id":"workspace-test","name":"Production"},{"id":"workspace-test","name":"Other"}]}`, diagnostic: "duplicate identity", wantCalls: 2},
		{name: "ambiguous name", input: "Production", page: `{"meta":{"total":2},"data":[{"id":"one","name":"Production"},{"id":"two","name":"production"}]}`, diagnostic: "ambiguous", wantCalls: 2},
		{name: "partial name", input: "Prod", diagnostic: "not found", wantCalls: 2},
		{name: "auth failure", input: "workspace-test", status: 401, failAt: 1, diagnostic: "HTTP 401", wantCalls: 1},
		{name: "detail failure", input: "workspace-test", status: 403, failAt: 3, diagnostic: "HTTP 403", wantCalls: 3},
		{name: "malformed detail", input: "workspace-test", detail: `{"token":"synthetic-secret"`, diagnostic: "malformed JSON", wantCalls: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := fastly.Client{HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.URL.Scheme != "https" || r.URL.Host != "api.fastly.com" || r.Header.Get("Fastly-Key") != "synthetic-secret" {
					t.Fatal("incorrect public request")
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				var body string
				switch calls {
				case 1:
					if r.URL.Path != "/customer/customer-test" || r.URL.RawQuery != "" {
						t.Fatal("unexpected customer request")
					}
					body = tc.customer
					if body == "" {
						body = customerBody
					}
				case 2:
					if r.URL.Path != "/ngwaf/v1/workspaces" || r.URL.Query().Get("customer_id") != "customer-test" || r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("page") != "1" {
						t.Fatal("unexpected inventory request")
					}
					body = tc.page
					if body == "" {
						body = singlePage
					}
				case 3:
					if r.URL.Path != "/ngwaf/v1/workspaces/workspace-test" || r.URL.Query().Get("customer_id") != "customer-test" {
						t.Fatal("unexpected detail request")
					}
					body = tc.detail
					if body == "" {
						body = detailBody
					}
				default:
					t.Fatal("extra API request")
				}
				status := 200
				if tc.failAt == calls {
					status = tc.status
					body = "synthetic-secret"
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})}}
			report, err := InspectWorkspace(context.Background(), client, "synthetic-secret", "customer-test", tc.input, nil)
			if calls != tc.wantCalls {
				t.Fatalf("calls=%d want %d", calls, tc.wantCalls)
			}
			if tc.diagnostic != "" {
				if err == nil || !strings.Contains(err.Error(), tc.diagnostic) || strings.Contains(err.Error(), "synthetic-secret") {
					t.Fatalf("incorrect error %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if report.WorkspaceID != "workspace-test" || report.Workspace != "Production" || report.WorkspaceInput != tc.input || string(report.WorkspaceUID) != `""` || string(report.ImmediateBlocking) != "0" || report.Provenance.DataClass != "configuration_snapshot" {
				t.Fatalf("incorrect projection: %+v", report)
			}
		})
	}
}

// A two-page fixture exercises actual limit=100 traversal, exact-ID precedence
// over a colliding name, missing owner/UID fields and pagination conflicts.
func TestWorkspacePages(t *testing.T) {
	for _, mode := range []string{"complete", "total changed", "repeated page"} {
		t.Run(mode, func(t *testing.T) {
			rows := []map[string]string{}
			for i := 0; i < 100; i++ {
				rows = append(rows, map[string]string{"id": fmt.Sprintf("w-%d", i), "name": fmt.Sprintf("Name %d", i)})
			}
			rows[0]["name"] = "workspace-test"
			encoded, _ := json.Marshal(rows)
			calls := 0
			client := fastly.Client{HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				body := ""
				switch r.URL.Path {
				case "/customer/customer-test":
					body = customerBody
				case "/ngwaf/v1/workspaces":
					p := r.URL.Query().Get("page")
					if p == "1" {
						body = `{"meta":{"total":101},"data":` + string(encoded) + `}`
					} else if p == "2" {
						total := 101
						if mode == "total changed" {
							total = 102
						}
						if mode == "repeated page" {
							body = `{"meta":{"total":101},"data":` + string(encoded) + `}`
						} else {
							body = `{"meta":{"total":` + strconv.Itoa(total) + `},"data":[{"id":"workspace-test","name":"Production"}]}`
						}
					} else {
						t.Fatal("extra page")
					}
				case "/ngwaf/v1/workspaces/workspace-test":
					body = `{"id":"workspace-test","mode":"log"}`
				default:
					t.Fatal("unexpected endpoint")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})}}
			report, err := InspectWorkspace(context.Background(), client, "synthetic-secret", "customer-test", "workspace-test", nil)
			if mode == "complete" {
				if err != nil || calls != 4 || report.Workspace != "Production" || report.WorkspaceUID != nil {
					t.Fatalf("calls=%d err=%v report=%+v", calls, err, report)
				}
			} else if err == nil || calls != 3 {
				t.Fatalf("accepted inconsistent pages: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestWorkspaceP0Evidence(t *testing.T) {
	for _, cancelAtDetail := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		calls, updates := 0, 0
		client := fastly.Client{HTTP: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			body := customerBody
			if calls == 2 {
				body = singlePage
			}
			if calls == 3 {
				body = detailBody
				if cancelAtDetail {
					cancel()
				}
			}
			if calls > 3 {
				t.Fatal("unexpected request")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})}}
		report, err := InspectWorkspace(ctx, client, "synthetic-secret", "customer-test", "Production", func(pages, rows, total int) {
			updates++
			if pages != 1 || rows != 1 || total != 1 {
				t.Fatal("progress does not match actual retrieval")
			}
		})
		cancel()
		if calls != 3 || updates != 1 {
			t.Fatalf("calls=%d updates=%d", calls, updates)
		}
		if cancelAtDetail {
			if err != context.Canceled || report.Provenance.CustomerScoped {
				t.Fatal("canceled inspection produced verified report")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		// Assert serialized provenance, not just in-memory fields.
		body, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			Provenance Provenance `json:"provenance"`
		}
		if err := json.Unmarshal(body, &output); err != nil {
			t.Fatal(err)
		}
		p := output.Provenance
		if p.CustomerIdentity.SelectedID != "customer-test" || p.CustomerIdentity.ConfirmedID != "customer-test" || p.CustomerIdentity.Endpoint != "GET /customer/{customer_id}" {
			t.Fatal("missing selected vs confirmed identity evidence")
		}
		if p.WorkspaceIdentity.ResolvedBy != "name" || !p.WorkspaceIdentity.InventoryMembership || !p.WorkspaceIdentity.DetailIDVerified || !p.WorkspaceIdentity.DetailOwnerPresent {
			t.Fatal("missing workspace identity evidence")
		}
		if p.Inventory.PagesFetched != 1 || p.Inventory.RowsFetched != 1 || p.Inventory.ReportedTotal != 1 || p.Inventory.Completeness != "complete_against_reported_total" || p.Inventory.Endpoint != "GET /ngwaf/v1/workspaces" {
			t.Fatal("incorrect completeness evidence")
		}
		if p.Requests.LogicalRequests != 3 || p.Requests.MaxLogicalRequests != 1002 {
			t.Fatal("incorrect request accounting")
		}
	}
}
