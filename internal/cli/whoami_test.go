package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWhoami(t *testing.T) {
	const secret = "synthetic-token-do-not-print"
	for _, tc := range []struct {
		name, token, body, diagnostic, output string
		status, code, calls                   int
		args                                  []string
		transportErr                          error
	}{
		{name: "success", token: secret, status: 200, body: `{"name":"Synthetic token","scope":"global:read","expires_at":null,"token":"` + secret + `"}`, output: "{\n  \"name\": \"Synthetic token\",\n  \"scope\": \"global:read\",\n  \"expires_at\": null\n}\n", calls: 1},
		{name: "missing fields", token: secret, status: 200, body: `{}`, output: "{\n  \"name\": null,\n  \"scope\": null,\n  \"expires_at\": null\n}\n", calls: 1},
		{name: "missing credentials", code: 1, diagnostic: "FASTLY_API_TOKEN is not set"},
		{name: "unauthorized", token: secret, status: 401, body: secret, code: 1, calls: 1, diagnostic: "HTTP 401: authentication failed"},
		{name: "forbidden", token: secret, status: 403, body: secret, code: 1, calls: 1, diagnostic: "HTTP 403: authorization failed"},
		{name: "api failure", token: secret, status: 503, body: secret, code: 1, calls: 1, diagnostic: "HTTP 503"},
		{name: "redirect", token: secret, status: 302, code: 1, calls: 1, diagnostic: "HTTP 302"},
		{name: "malformed", token: secret, status: 200, body: `{"token":"` + secret, code: 1, calls: 1, diagnostic: "malformed JSON"},
		{name: "array", token: secret, status: 200, body: `[]`, code: 1, calls: 1, diagnostic: "unexpected response shape"},
		{name: "null", token: secret, status: 200, body: `null`, code: 1, calls: 1, diagnostic: "unexpected response shape"},
		{name: "multiple values", token: secret, status: 200, body: `{} {}`, code: 1, calls: 1, diagnostic: "malformed JSON"},
		{name: "oversized", token: secret, status: 200, body: strings.Repeat(" ", (1<<20)+1), code: 1, calls: 1, diagnostic: "size limit"},
		{name: "transport redaction", token: secret, transportErr: errors.New(secret), code: 1, calls: 1, diagnostic: "request failed"},
		{name: "unexpected argument", token: secret, args: []string{"customer", "whoami", secret}, code: 2, diagnostic: "usage:"},
		{name: "unexpected flag", args: []string{"customer", "whoami", "--json"}, code: 2, diagnostic: "usage:"},
		{name: "customer only", args: []string{"customer"}, code: 2, diagnostic: "usage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, envReads := 0, 0
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.URL.String() != "https://api.fastly.com/tokens/self" || r.Body != nil {
					t.Fatal("unexpected request")
				}
				if r.Header.Get("Fastly-Key") != secret || r.Header.Get("Accept") != "application/json" {
					t.Fatal("incorrect authentication headers")
				}
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) > 60*time.Second {
					t.Fatal("missing bounded deadline")
				}
				if tc.transportErr != nil {
					return nil, tc.transportErr
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Location": []string{"https://other.invalid/"}}, Body: io.NopCloser(strings.NewReader(tc.body)), Request: r}, nil
			})}
			args := tc.args
			if args == nil {
				args = []string{"customer", "whoami"}
			}
			var out, errOut bytes.Buffer
			code := run(context.Background(), args, &out, &errOut, func(key string) string {
				envReads++
				if key != "FASTLY_API_TOKEN" {
					t.Fatal("unexpected environment lookup")
				}
				return tc.token
			}, client)
			if code != tc.code || calls != tc.calls {
				t.Fatalf("code=%d calls=%d; want %d/%d", code, calls, tc.code, tc.calls)
			}
			if tc.code == 2 && envReads != 0 {
				t.Fatal("arguments not checked before credentials")
			}
			if out.String() != tc.output || (tc.diagnostic == "" && errOut.Len() != 0) || !strings.Contains(errOut.String(), tc.diagnostic) {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", out.String(), errOut.String())
			}
			if strings.Contains(out.String()+errOut.String(), secret) {
				t.Fatal("credential leaked")
			}
		})
	}
}

func TestWhoamiCancellation(t *testing.T) {
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if expired {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}
		client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
			cancel()
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}
		var out, errOut bytes.Buffer
		code := run(ctx, []string{"customer", "whoami"}, &out, &errOut, func(string) string { return "synthetic" }, client)
		cancel()
		want := "canceled"
		if expired {
			want = "timed out"
		}
		if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), want) {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
		}
	}
}
