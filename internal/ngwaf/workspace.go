// Package ngwaf inspects customer-scoped public NGWAF configuration.
package ngwaf

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cassiejk/sift/internal/fastly"
)

const MaxWorkspacePages = 1000
const InspectionTimeout = 120 * time.Second

type WorkspaceReport struct {
	Workspace                   string          `json:"workspace"`
	WorkspaceID                 string          `json:"workspace_id"`
	WorkspaceUID                json.RawMessage `json:"workspace_uid"`
	WorkspaceInput              string          `json:"workspace_input"`
	Mode                        json.RawMessage `json:"mode"`
	ImmediateBlocking           json.RawMessage `json:"immediate_blocking"`
	DefaultBlockingResponseCode json.RawMessage `json:"default_blocking_response_code"`
	Provenance                  Provenance      `json:"provenance"`
}
type Provenance struct {
	Endpoint          string                    `json:"endpoint"`
	DataClass         string                    `json:"data_class"`
	CustomerScoped    bool                      `json:"customer_scoped"`
	CustomerIdentity  CustomerIdentityEvidence  `json:"customer_identity"`
	WorkspaceIdentity WorkspaceIdentityEvidence `json:"workspace_identity"`
	Inventory         InventoryEvidence         `json:"inventory"`
	Requests          RequestEvidence           `json:"requests"`
}

// Selected identity is input; confirmed identity is established by the customer GET.
type CustomerIdentityEvidence struct {
	SelectedID      string `json:"selected_id"`
	ConfirmedID     string `json:"api_confirmed_id"`
	SelectionSource string `json:"selection_source"`
	Endpoint        string `json:"endpoint"`
}
type WorkspaceIdentityEvidence struct {
	ResolvedBy          string `json:"resolved_by"`
	InventoryMembership bool   `json:"customer_inventory_membership"`
	DetailIDVerified    bool   `json:"detail_id_verified"`
	OwnershipBasis      string `json:"ownership_basis"`
	DetailOwnerPresent  bool   `json:"detail_customer_id_present"`
}
type InventoryEvidence struct {
	Endpoint      string `json:"endpoint"`
	CustomerID    string `json:"customer_id"`
	PagesFetched  int    `json:"pages_fetched"`
	RowsFetched   int    `json:"rows_fetched"`
	ReportedTotal int    `json:"reported_total"`
	Completeness  string `json:"completeness"`
}
type RequestEvidence struct {
	LogicalRequests    int `json:"logical_requests"`
	MaxLogicalRequests int `json:"max_logical_requests"`
}

func validID(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsFunc(s, unicode.IsControl)
}
func requiredString(obj fastly.Object, key string) (string, error) {
	var v string
	if json.Unmarshal(obj[key], &v) != nil || v == "" {
		return "", errors.New("missing or malformed identity metadata")
	}
	return v, nil
}
func verifyOwner(obj fastly.Object, customer string) error {
	if _, present := obj["customer_id"]; present {
		id, err := requiredString(obj, "customer_id")
		if err != nil || id != customer {
			return errors.New("workspace customer ownership is conflicting or unavailable")
		}
	}
	return nil
}
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

// InspectWorkspace validates customer identity, complete customer-scoped inventory
// membership and exact detail identity. Missing optional owner fields rely on the
// public customer_id-filtered inventory contract; explicit conflicts always fail.
func InspectWorkspace(ctx context.Context, client fastly.Client, token, customer, input string, progress func(pages, rows, total int)) (WorkspaceReport, error) {
	var report WorkspaceReport
	if !validID(customer) || input == "" {
		return report, errors.New("invalid customer or workspace selector")
	}
	ctx, cancel := context.WithTimeout(ctx, InspectionTimeout)
	defer cancel()
	body, err := client.Customer(ctx, token, customer)
	if err != nil {
		return report, err
	}
	returned, err := requiredString(body, "id")
	if err != nil || returned != customer {
		return report, errors.New("customer-scope validation mismatch")
	}
	if _, err := requiredString(body, "name"); err != nil {
		return report, errors.New("customer-scope response missing name")
	}
	type workspace struct{ id, name string }
	inventory := []workspace{}
	seen := map[string]bool{}
	total := -1
	complete := false
	pagesFetched := 0
	for page := 1; page <= MaxWorkspacePages; page++ {
		obj, err := client.WorkspacePage(ctx, token, customer, page)
		if err != nil {
			return report, err
		}
		if err := verifyOwner(obj, customer); err != nil {
			return report, err
		}
		meta, err := fastly.DecodeObject(obj["meta"])
		if err != nil {
			return report, errors.New("workspace pagination requires valid total metadata")
		}
		count, err := strconv.Atoi(string(meta["total"]))
		if err != nil || count < 0 {
			return report, errors.New("workspace pagination requires a nonnegative integer total")
		}
		if total >= 0 && count != total {
			return report, errors.New("workspace pagination total changed")
		}
		total = count
		var rows []json.RawMessage
		raw, exists := obj["data"]
		if !exists || string(raw) == "null" || json.Unmarshal(raw, &rows) != nil || rows == nil || len(rows) > 100 {
			return report, errors.New("workspace pagination returned invalid data array")
		}
		for _, raw := range rows {
			row, err := fastly.DecodeObject(raw)
			if err != nil {
				return report, err
			}
			id, err := requiredString(row, "id")
			if err != nil || !validID(id) {
				return report, errors.New("workspace inventory has invalid identity")
			}
			name, err := requiredString(row, "name")
			if err != nil {
				return report, err
			}
			if seen[id] {
				return report, errors.New("workspace inventory contains duplicate identity")
			}
			seen[id] = true
			if err := verifyOwner(row, customer); err != nil {
				return report, err
			}
			inventory = append(inventory, workspace{id, name})
		}
		if len(inventory) > total {
			return report, errors.New("workspace pagination exceeded advertised total")
		}
		pagesFetched = page
		if progress != nil {
			progress(page, len(inventory), total)
		}
		if len(inventory) == total {
			complete = true
			break
		}
		if len(rows) < 100 {
			return report, errors.New("workspace pagination incomplete: short page before total")
		}
	}
	if !complete {
		return report, errors.New("workspace pagination safety limit exceeded")
	}
	resolvedBy := "id"
	matches := []workspace{}
	for _, w := range inventory {
		if w.id == input {
			matches = append(matches, w)
		}
	}
	if len(matches) == 0 {
		resolvedBy = "name"
		for _, w := range inventory {
			if asciiLower(w.name) == asciiLower(input) {
				matches = append(matches, w)
			}
		}
	}
	if len(matches) == 0 {
		return report, errors.New("workspace not found for selected customer")
	}
	if len(matches) != 1 {
		return report, errors.New("workspace name is ambiguous for selected customer")
	}
	selected := matches[0]
	detail, err := client.WorkspaceDetail(ctx, token, customer, selected.id)
	if err != nil {
		return report, err
	}
	id, err := requiredString(detail, "id")
	if err != nil || id != selected.id {
		return report, errors.New("workspace detail identity mismatch")
	}
	if err := verifyOwner(detail, customer); err != nil {
		return report, err
	}
	if _, exists := detail["name"]; exists {
		name, err := requiredString(detail, "name")
		if err != nil || name != selected.name {
			return report, errors.New("workspace detail name conflicts with inventory")
		}
	}
	if raw, present := detail["workspace_uid"]; present && string(raw) != "null" {
		var uid string
		if json.Unmarshal(raw, &uid) != nil {
			return report, errors.New("workspace UID is malformed")
		}
	}
	var immediate json.RawMessage
	if raw, present := detail["attack_signal_thresholds"]; present && string(raw) != "null" {
		thresholds, err := fastly.DecodeObject(raw)
		if err != nil {
			return report, errors.New("workspace thresholds are malformed")
		}
		immediate = thresholds["immediate"]
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	_, detailOwnerPresent := detail["customer_id"]
	report = WorkspaceReport{
		Workspace: selected.name, WorkspaceID: selected.id, WorkspaceUID: detail["workspace_uid"], WorkspaceInput: input,
		Mode: detail["mode"], ImmediateBlocking: immediate, DefaultBlockingResponseCode: detail["default_blocking_response_code"],
		Provenance: Provenance{
			Endpoint: "GET /ngwaf/v1/workspaces/{workspace_id}", DataClass: "configuration_snapshot", CustomerScoped: true,
			CustomerIdentity:  CustomerIdentityEvidence{customer, returned, "caller", "GET /customer/{customer_id}"},
			WorkspaceIdentity: WorkspaceIdentityEvidence{resolvedBy, true, true, "validated_customer_filtered_inventory_and_exact_detail_id", detailOwnerPresent},
			Inventory:         InventoryEvidence{"GET /ngwaf/v1/workspaces", customer, pagesFetched, len(inventory), total, "complete_against_reported_total"},
			Requests:          RequestEvidence{pagesFetched + 2, MaxWorkspacePages + 2},
		},
	}
	return report, nil
}
