// Package fastly contains the public API operations implemented by sift.
package fastly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const tokenURL = "https://api.fastly.com/tokens/self"
const maxResponseBytes = 1 << 20

// TokenInfo preserves the Bash projection, including missing fields as null
// and the original JSON types. Unselected response fields are never emitted.
type TokenInfo struct {
	Name      json.RawMessage `json:"name"`
	Scope     json.RawMessage `json:"scope"`
	ExpiresAt json.RawMessage `json:"expires_at"`
}

// Client uses only the public Fastly host. HTTP is injectable for offline tests.
// Requests have a 60-second deadline and do not follow redirects or retry.
type Client struct{ HTTP *http.Client }

func (c Client) Whoami(ctx context.Context, token string) (TokenInfo, error) {
	var info TokenInfo
	if token == "" {
		return info, errors.New("FASTLY_API_TOKEN is not set")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return info, requestError(err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	req.Header.Set("Fastly-Key", token)
	req.Header.Set("Accept", "application/json")
	client := http.Client{}
	if c.HTTP != nil {
		client = *c.HTTP
	}
	// Never forward Fastly-Key to a redirect destination, even on the same host.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return info, requestError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 401:
			return info, errors.New("Fastly API returned HTTP 401: authentication failed (FASTLY_API_TOKEN)")
		case 403:
			return info, errors.New("Fastly API returned HTTP 403: authorization failed")
		default:
			return info, fmt.Errorf("Fastly API returned HTTP %d for GET /tokens/self", resp.StatusCode)
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return info, requestError(err)
	}
	if len(body) > maxResponseBytes {
		return info, errors.New("Fastly API response exceeded size limit")
	}
	if !json.Valid(body) {
		return info, errors.New("Fastly API returned malformed JSON")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return info, errors.New("Fastly API returned unexpected response shape: expected an object")
	}
	// Like jq '{name, scope, expires_at}', absent keys become null.
	info = TokenInfo{object["name"], object["scope"], object["expires_at"]}
	return info, nil
}

// Never include transport errors, response bodies or header values in diagnostics.
func requestError(err error) error {
	if errors.Is(err, context.Canceled) {
		return errors.New("Fastly API request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("Fastly API request timed out")
	}
	return errors.New("Fastly API request failed")
}
