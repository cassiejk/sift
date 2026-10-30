// Package config reads explicit customer selection without modifying configuration.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CustomerSelection follows Bash precedence and reads its selection file only
// when FASTLY_CUSTOMER_ID is empty. It never reads credential files or writes.
func CustomerSelection(getenv func(string) string, readFile func(string) ([]byte, error)) (id, source string, err error) {
	if id = getenv("FASTLY_CUSTOMER_ID"); id != "" {
		return id, "environment", nil
	}
	root := getenv("XDG_CONFIG_HOME")
	if root == "" {
		home := getenv("HOME")
		if home == "" {
			return "", "", errors.New("no customer selected: set FASTLY_CUSTOMER_ID")
		}
		root = filepath.Join(home, ".config")
	}
	body, err := readFile(filepath.Join(root, "fastly-ngwaf", "customer_id"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", errors.New("no customer selected: set FASTLY_CUSTOMER_ID")
		}
		return "", "", errors.New("customer selection file is not readable")
	}
	// Bash command substitution removes trailing newlines, not surrounding spaces.
	id = strings.TrimRight(string(body), "\n")
	if id == "" {
		return "", "", errors.New("no customer selected: set FASTLY_CUSTOMER_ID")
	}
	return id, "selected_file", nil
}
