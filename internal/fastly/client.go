// Package fastly contains the public API operations implemented by sift.
package fastly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

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
	body, err := c.get(ctx, token, "/tokens/self", nil)
	if err != nil {
		return TokenInfo{}, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return TokenInfo{}, errors.New("Fastly API returned unexpected response shape: expected an object")
	}
	return TokenInfo{object["name"], object["scope"], object["expires_at"]}, nil
}

// get is limited to this client's fixed public host and internal endpoint paths.
func (c Client) get(ctx context.Context, token, path string, query url.Values) ([]byte, error) {
	if token == "" {
		return nil, errors.New("FASTLY_API_TOKEN is not set")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, requestError(err)
	}
	endpoint := "https://api.fastly.com" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("invalid Fastly API request")
	}
	req.Header.Set("Fastly-Key", token)
	req.Header.Set("Accept", "application/json")
	client := http.Client{}
	if c.HTTP != nil {
		client = *c.HTTP
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, requestError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		switch resp.StatusCode {
		case 401:
			return nil, errors.New("Fastly API returned HTTP 401: authentication failed (FASTLY_API_TOKEN)")
		case 403:
			return nil, errors.New("Fastly API returned HTTP 403: authorization failed")
		default:
			return nil, fmt.Errorf("Fastly API returned HTTP %d", resp.StatusCode)
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, requestError(err)
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("Fastly API response exceeded size limit")
	}
	if !json.Valid(body) {
		return nil, errors.New("Fastly API returned malformed JSON")
	}
	return body, nil
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
