package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomerSelection(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		env                          map[string]string
		body                         string
		readErr                      error
		wantID, wantSource, wantPath string
		fail                         bool
	}{
		{name: "environment wins", env: map[string]string{"FASTLY_CUSTOMER_ID": "customer-test"}, wantID: "customer-test", wantSource: "environment"},
		{name: "xdg file", env: map[string]string{"XDG_CONFIG_HOME": "/synthetic-config"}, body: "customer-test\n\n", wantID: "customer-test", wantSource: "selected_file", wantPath: filepath.Join("/synthetic-config", "fastly-ngwaf", "customer_id")},
		{name: "home fallback", env: map[string]string{"HOME": "/synthetic-home"}, body: " customer-test \n", wantID: " customer-test ", wantSource: "selected_file", wantPath: filepath.Join("/synthetic-home", ".config", "fastly-ngwaf", "customer_id")},
		{name: "missing", env: map[string]string{"HOME": "/synthetic-home"}, readErr: os.ErrNotExist, fail: true},
		{name: "unreadable", env: map[string]string{"HOME": "/synthetic-home"}, readErr: errors.New("sensitive file error"), fail: true},
		{name: "empty", env: map[string]string{"HOME": "/synthetic-home"}, body: "\n", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			id, source, err := CustomerSelection(func(k string) string { return tc.env[k] }, func(path string) ([]byte, error) {
				reads++
				if tc.wantPath != "" && path != tc.wantPath {
					t.Fatalf("path=%q", path)
				}
				return []byte(tc.body), tc.readErr
			})
			if (err != nil) != tc.fail || id != tc.wantID || source != tc.wantSource {
				t.Fatalf("id=%q source=%q err=%v", id, source, err)
			}
			if tc.wantSource == "environment" && reads != 0 {
				t.Fatal("read file despite env selection")
			}
		})
	}
}
