package snapshot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) *Snapshot {
	t.Helper()
	s, err := Load(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCompare(t *testing.T) {
	before, after := fixture(t, "before"), fixture(t, "after")
	r, err := Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Changed || r.Provenance.APIRequestsMade || r.Command != "diff" || r.SchemaVersion != "1.0" {
		t.Fatalf("incorrect report: %+v", r)
	}
	d := r.Changes.WorkspaceRules
	if d.Counts != (Counts{1, 1, 1}) || d.Added[0].ID != "rule-added" || d.Removed[0].ID != "rule-removed" || !reflect.DeepEqual(d.Modified[0].ChangedFields, []string{"enabled"}) {
		t.Fatalf("incorrect rule delta: %+v", d)
	}
	if !reflect.DeepEqual(r.Changes.Workspace.ChangedFields, []string{"mode", "optional"}) {
		t.Fatal("missing/null change lost")
	}
	if _, ok := r.Changes.Workspace.Before["optional"]; ok {
		t.Fatal("missing field inserted")
	}
	if v, ok := r.Changes.Workspace.After["optional"]; !ok || v != nil {
		t.Fatal("explicit null lost")
	}
	if r.Changes.Thresholds.Counts != (Counts{}) {
		t.Fatal("equivalent numeric spelling treated as drift")
	}
	payload, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(payload, []byte(`9007199254740993`)) || !bytes.Contains(payload, []byte(`"added":[]`)) {
		t.Fatal("numeric or empty array contract lost")
	}
	for i := 0; i < 3; i++ {
		again, _ := Compare(before, after)
		b, _ := json.Marshal(again)
		if !bytes.Equal(payload, b) {
			t.Fatal("nondeterministic report")
		}
	}
	var human bytes.Buffer
	if err := r.WriteHuman(&human); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"NGWAF snapshot diff", "[WARN]", "+1  -1  ~1", "rule-a\tenabled\tSynthetic rule", "list entries were not included"} {
		if !strings.Contains(human.String(), want) {
			t.Fatalf("human output missing %q", want)
		}
	}
	// Collection ordering and capture metadata alone do not constitute drift.
	same := fixture(t, "before")
	rows := at(same.document, "configuration", "rules", "workspace").([]any)
	rows[0], rows[1] = rows[1], rows[0]
	same.document["captured_at"] = "later"
	unchanged, err := Compare(before, same)
	if err != nil || unchanged.Changed {
		t.Fatal("ordering/metadata-only drift")
	}
}

func TestInvalidSnapshots(t *testing.T) {
	original, err := os.ReadFile("testdata/before.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, from, to string }{
		{"schema", `"1.0"`, `"2.0"`},
		{"missing customer id", `"id": "customer-test"`, `"name_only": "customer-test"`},
		{"missing collection", `"account_applying": [{"reference_id": "account-rule", "enabled": true}]`, `"account_applying": null`},
		{"duplicate identity", `"rule-removed"`, `"rule-a"`},
		{"missing identity", `"id": "rule-removed"`, `"unrecognized": "rule-removed"`},
		{"duplicate key", `"mode": "log"`, `"mode": "log", "mode": "block"`},
		{"malformed", `"snapshot_schema_version":`, `"snapshot_schema_version"`},
		{"multiple values", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(string(original), tc.from, tc.to, 1)
			if tc.name == "multiple values" {
				body = string(original) + " {}"
			}
			path := filepath.Join(t.TempDir(), "invalid.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("accepted invalid snapshot")
			}
		})
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("accepted missing file")
	}
	for _, kind := range []string{"customer", "workspace"} {
		b := fixture(t, "before")
		b.document[kind].(object)["id"] = "other"
		if _, err := Compare(fixture(t, "before"), b); err == nil || !strings.Contains(err.Error(), "cross-"+kind) {
			t.Fatalf("accepted cross-%s comparison", kind)
		}
	}
}
