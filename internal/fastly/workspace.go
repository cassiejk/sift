package fastly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
)

// Object preserves API field presence, null and original JSON numeric values.
type Object = map[string]json.RawMessage

func (c Client) Customer(ctx context.Context, token, id string) (Object, error) {
	return c.getObject(ctx, token, "/customer/"+url.PathEscape(id), nil)
}
func (c Client) WorkspacePage(ctx context.Context, token, customer string, page int) (Object, error) {
	return c.getObject(ctx, token, "/ngwaf/v1/workspaces", url.Values{"customer_id": {customer}, "limit": {"100"}, "page": {strconv.Itoa(page)}})
}
func (c Client) WorkspaceDetail(ctx context.Context, token, customer, id string) (Object, error) {
	return c.getObject(ctx, token, "/ngwaf/v1/workspaces/"+url.PathEscape(id), url.Values{"customer_id": {customer}})
}
func (c Client) getObject(ctx context.Context, token, path string, q url.Values) (Object, error) {
	body, err := c.get(ctx, token, path, q)
	if err != nil {
		return nil, err
	}
	return DecodeObject(body)
}

// DecodeObject rejects duplicate keys, including nested keys, rather than
// guessing which contradictory identity or pagination metadata is authoritative.
func DecodeObject(body []byte) (Object, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var check func() error
	check = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for dec.More() {
					k, err := dec.Token()
					if err != nil {
						return err
					}
					key, ok := k.(string)
					if !ok || seen[key] {
						return errors.New("ambiguous JSON object")
					}
					seen[key] = true
					if err := check(); err != nil {
						return err
					}
				}
			case '[':
				for dec.More() {
					if err := check(); err != nil {
						return err
					}
				}
			default:
				return errors.New("unexpected delimiter")
			}
			_, err = dec.Token()
			return err
		}
		return nil
	}
	var obj Object
	if !json.Valid(body) || check() != nil || json.Unmarshal(body, &obj) != nil || obj == nil {
		return nil, errors.New("Fastly API returned invalid or ambiguous object")
	}
	return obj, nil
}
