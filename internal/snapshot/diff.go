// Package snapshot compares local NGWAF configuration snapshots without API access.
package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strings"
)

type object = map[string]any

// Snapshot retains unmodified JSON values, including absent keys and exact numbers.
type Snapshot struct {
	document    object
	workspace   object
	collections map[string]map[string]object
}

var collectionPaths = []struct {
	name string
	path []string
}{
	{"workspace_rules", []string{"rules", "workspace"}},
	{"account_rules_applying", []string{"rules", "account_applying"}},
	{"thresholds", []string{"thresholds"}},
	{"workspace_lists", []string{"lists", "workspace"}},
	{"account_lists", []string{"lists", "account"}},
}

// Load validates one schema 1.0 JSON object. Errors do not echo file contents.
func Load(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("snapshot is not readable")
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber()
	value, err := readValue(dec)
	if err != nil {
		return nil, errors.New("invalid snapshot JSON")
	}
	if _, err = dec.Token(); err != io.EOF {
		return nil, errors.New("snapshot must contain exactly one JSON object")
	}
	doc, ok := value.(object)
	if !ok {
		return nil, errors.New("snapshot must be a JSON object")
	}
	return validate(doc)
}

// Decode tokens explicitly to reject ambiguous duplicate object keys.
func readValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); ok {
		switch delim {
		case '{':
			m := object{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("invalid key")
				}
				if _, exists := m[name]; exists {
					return nil, errors.New("duplicate key")
				}
				m[name], err = readValue(dec)
				if err != nil {
					return nil, err
				}
			}
			_, err = dec.Token()
			return m, err
		case '[':
			a := []any{}
			for dec.More() {
				v, err := readValue(dec)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			_, err = dec.Token()
			return a, err
		}
		return nil, errors.New("unexpected delimiter")
	}
	return tok, nil
}

func at(m object, path ...string) any {
	var v any = m
	for _, key := range path {
		obj, ok := v.(object)
		if !ok {
			return nil
		}
		v = obj[key]
	}
	return v
}

func validate(doc object) (*Snapshot, error) {
	invalid := errors.New("invalid or unsupported ngwaf snapshot")
	if doc["snapshot_schema_version"] != "1.0" {
		return nil, invalid
	}
	for _, kind := range []string{"customer", "workspace"} {
		id, ok := at(doc, kind, "id").(string)
		if !ok || id == "" {
			return nil, invalid
		}
	}
	config, ok := doc["configuration"].(object)
	if !ok {
		return nil, invalid
	}
	ws, ok := config["workspace"].(object)
	if !ok {
		return nil, invalid
	}
	if v := at(doc, "safety", "list_entries_included"); v != nil {
		if _, ok := v.(bool); !ok {
			return nil, invalid
		}
	}
	s := &Snapshot{doc, ws, map[string]map[string]object{}}
	for _, c := range collectionPaths {
		rows, ok := at(config, c.path...).([]any)
		if !ok {
			return nil, invalid
		}
		index := map[string]object{}
		for _, row := range rows {
			item, ok := row.(object)
			if !ok {
				return nil, invalid
			}
			id, err := identity(item)
			if err != nil {
				return nil, invalid
			}
			if _, exists := index[id]; exists {
				return nil, invalid
			}
			index[id] = item
		}
		s.collections[c.name] = index
	}
	return s, nil
}

func identity(item object) (string, error) {
	for _, key := range []string{"id", "reference_id", "name", "description"} {
		v := item[key]
		if v == nil || v == false {
			continue
		} // jq // skips null and false, not empty strings.
		id, ok := v.(string)
		if !ok || id == "" {
			return "", errors.New("invalid identity")
		}
		return id, nil
	}
	return "", errors.New("missing identity")
}

// JSON equality ignores object ordering and equivalent numeric spellings,
// while retaining array order, field presence, booleans and nulls.
func equal(a, b any) bool {
	switch x := a.(type) {
	case json.Number:
		y, ok := b.(json.Number)
		if !ok {
			return false
		}
		if x == y {
			return true
		}
		r, ok := new(big.Rat).SetString(string(x))
		if !ok {
			return false
		}
		s, ok := new(big.Rat).SetString(string(y))
		return ok && r.Cmp(s) == 0
	case object:
		y, ok := b.(object)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, exists := y[k]
			if !exists || !equal(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i, v := range x {
			if !equal(v, y[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

func fields(a, b object) []string {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	out := []string{}
	for k := range keys {
		av, ae := a[k]
		bv, be := b[k]
		if ae != be || !equal(av, bv) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

type Entry struct {
	ID    string `json:"id"`
	Value object `json:"value"`
}
type Modified struct {
	ID            string   `json:"id"`
	ChangedFields []string `json:"changed_fields"`
	Before        object   `json:"before"`
	After         object   `json:"after"`
}
type Counts struct {
	Added    int `json:"added"`
	Removed  int `json:"removed"`
	Modified int `json:"modified"`
}
type Delta struct {
	Added    []Entry    `json:"added"`
	Removed  []Entry    `json:"removed"`
	Modified []Modified `json:"modified"`
	Counts   Counts     `json:"counts"`
}
type WorkspaceChange struct {
	Changed       bool     `json:"changed"`
	ChangedFields []string `json:"changed_fields"`
	Before        object   `json:"before"`
	After         object   `json:"after"`
}
type Changes struct {
	Workspace      WorkspaceChange `json:"workspace"`
	WorkspaceRules Delta           `json:"workspace_rules"`
	AccountRules   Delta           `json:"account_rules_applying"`
	Thresholds     Delta           `json:"thresholds"`
	WorkspaceLists Delta           `json:"workspace_lists"`
	AccountLists   Delta           `json:"account_lists"`
}
type Capture struct {
	CapturedAt  any `json:"captured_at"`
	ToolVersion any `json:"tool_version"`
}
type ListEntries struct {
	BeforeIncluded bool `json:"before_included"`
	AfterIncluded  bool `json:"after_included"`
}
type Provenance struct {
	DataClass       string `json:"data_class"`
	APIRequestsMade bool   `json:"API_requests_made"`
	ScopeValidation string `json:"scope_validation"`
}

// Report follows the Bash local diff JSON envelope.
type Report struct {
	Command       string      `json:"command"`
	SchemaVersion string      `json:"snapshot_schema_version"`
	Customer      object      `json:"customer"`
	Workspace     object      `json:"workspace"`
	Before        Capture     `json:"before"`
	After         Capture     `json:"after"`
	ListEntries   ListEntries `json:"list_entries"`
	Changes       Changes     `json:"changes"`
	Provenance    Provenance  `json:"provenance"`
	Changed       bool        `json:"changed"`
}

func delta(a, b map[string]object) Delta {
	d := Delta{Added: []Entry{}, Removed: []Entry{}, Modified: []Modified{}}
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	ids := make([]string, 0, len(keys))
	for k := range keys {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	for _, id := range ids {
		av, ae := a[id]
		bv, be := b[id]
		switch {
		case !ae:
			d.Added = append(d.Added, Entry{id, bv})
		case !be:
			d.Removed = append(d.Removed, Entry{id, av})
		case !equal(av, bv):
			d.Modified = append(d.Modified, Modified{id, fields(av, bv), av, bv})
		}
	}
	d.Counts = Counts{len(d.Added), len(d.Removed), len(d.Modified)}
	return d
}

// Compare requires equal customer/workspace IDs; it does not verify API ownership.
func Compare(a, b *Snapshot) (Report, error) {
	var r Report
	if at(a.document, "customer", "id") != at(b.document, "customer", "id") {
		return r, errors.New("refusing cross-customer snapshot comparison")
	}
	if at(a.document, "workspace", "id") != at(b.document, "workspace", "id") {
		return r, errors.New("refusing cross-workspace snapshot comparison")
	}
	r = Report{Command: "diff", SchemaVersion: "1.0", Customer: a.document["customer"].(object), Workspace: a.document["workspace"].(object), Before: Capture{a.document["captured_at"], a.document["tool_version"]}, After: Capture{b.document["captured_at"], b.document["tool_version"]}, ListEntries: ListEntries{at(a.document, "safety", "list_entries_included") == true, at(b.document, "safety", "list_entries_included") == true}, Provenance: Provenance{"local_snapshot_comparison", false, "customer and workspace IDs must match"}}
	r.Changes.Workspace = WorkspaceChange{!equal(a.workspace, b.workspace), fields(a.workspace, b.workspace), a.workspace, b.workspace}
	r.Changed = r.Changes.Workspace.Changed
	targets := []*Delta{&r.Changes.WorkspaceRules, &r.Changes.AccountRules, &r.Changes.Thresholds, &r.Changes.WorkspaceLists, &r.Changes.AccountLists}
	for i, c := range collectionPaths {
		*targets[i] = delta(a.collections[c.name], b.collections[c.name])
		counts := targets[i].Counts
		r.Changed = r.Changed || counts.Added+counts.Removed+counts.Modified > 0
	}
	return r, nil
}

// WriteHuman renders the Bash report's labels and counts without terminal color
// or an external column formatter. Write failures propagate to the CLI.
func (r Report) WriteHuman(w io.Writer) error {
	var b strings.Builder
	text := func(v any) string {
		if v == nil {
			return "null"
		}
		if s, ok := v.(string); ok {
			return s
		}
		data, _ := json.Marshal(v)
		return string(data)
	}
	fmt.Fprintln(&b, "NGWAF snapshot diff")
	fmt.Fprintf(&b, "%s (%s) · %s (%s)\n", text(r.Customer["name"]), text(r.Customer["id"]), text(r.Workspace["name"]), text(r.Workspace["id"]))
	fmt.Fprintf(&b, "%s → %s\n\n", text(r.Before.CapturedAt), text(r.After.CapturedAt))
	status := func(label, detail string, changed bool) {
		badge := "PASS"
		if changed {
			badge = "WARN"
		}
		fmt.Fprintf(&b, "[%s]  %-24s %s\n", badge, label, detail)
	}
	detail := "unchanged"
	if r.Changes.Workspace.Changed {
		detail = "changed: " + strings.Join(r.Changes.Workspace.ChangedFields, ", ")
	}
	status("Workspace settings", detail, r.Changes.Workspace.Changed)
	rows := []struct {
		label string
		d     Delta
	}{{"Workspace rules", r.Changes.WorkspaceRules}, {"Applicable account rules", r.Changes.AccountRules}, {"Thresholds", r.Changes.Thresholds}, {"Workspace lists", r.Changes.WorkspaceLists}, {"Account lists", r.Changes.AccountLists}}
	for _, row := range rows {
		c := row.d.Counts
		changed := c.Added+c.Removed+c.Modified > 0
		detail := "unchanged"
		if changed {
			detail = fmt.Sprintf("+%d  -%d  ~%d", c.Added, c.Removed, c.Modified)
		}
		status(row.label, detail, changed)
	}
	if len(r.Changes.WorkspaceRules.Modified) > 0 {
		fmt.Fprintln(&b, "\nModified workspace rules\nRULE ID\tCHANGED FIELDS\tDESCRIPTION")
		for _, m := range r.Changes.WorkspaceRules.Modified {
			desc := m.After["description"]
			if desc == nil || desc == false {
				desc = m.Before["description"]
			}
			if desc == nil || desc == false {
				desc = ""
			}
			// Match jq @tsv escaping for cells, including embedded newlines and tabs.
			escape := strings.NewReplacer("\\", "\\\\", "\t", "\\t", "\n", "\\n", "\r", "\\r")
			fmt.Fprintf(&b, "%s\t%s\t%s\n", escape.Replace(m.ID), escape.Replace(strings.Join(m.ChangedFields, ",")), escape.Replace(text(desc)))
		}
	}
	if !r.ListEntries.BeforeIncluded && !r.ListEntries.AfterIncluded {
		fmt.Fprintln(&b, "\nNote: list entries were not included in either snapshot; list metadata and reported entry-count changes remain comparable.")
	}
	_, err := io.WriteString(w, b.String())
	return err
}
