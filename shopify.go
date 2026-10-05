package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const httpTimeout = 30 * time.Second

// maxBody caps how much JSON one tool returns, so a long product list can't
// blow up the model's context.
const maxBody = 12000

// defaultAPIVersion is used when SHOPIFY_API_VERSION is unset.
const defaultAPIVersion = "2026-07"

// idempotentSince is the first API version where inventory mutations require an
// @idempotent key.
const idempotentSince = "2026-04"

var (
	storeRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.myshopify\.com$`)
	versionRe = regexp.MustCompile(`^\d{4}-(01|04|07|10)$|^unstable$`)
)

// client issues Admin GraphQL requests to one Shopify store.
type client struct {
	endpoint string
	version  string
	token    string
	http     *http.Client
}

// normalizeStore accepts "acme", "acme.myshopify.com" or a pasted URL and
// returns the bare myshopify.com host.
func normalizeStore(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimRight(strings.SplitN(s, "/", 2)[0], "/")
	if s != "" && !strings.Contains(s, ".") {
		s += ".myshopify.com"
	}
	if !storeRe.MatchString(s) {
		return "", fmt.Errorf("SHOPIFY_STORE must be the store's myshopify.com domain, e.g. acme.myshopify.com")
	}
	return s, nil
}

func newClient() (*client, error) {
	rawStore := os.Getenv("SHOPIFY_STORE")
	tok := strings.TrimSpace(os.Getenv("SHOPIFY_ACCESS_TOKEN"))
	if strings.TrimSpace(rawStore) == "" {
		return nil, fmt.Errorf("not configured: SHOPIFY_STORE is empty — set it to your store domain, e.g. acme.myshopify.com")
	}
	if tok == "" {
		return nil, fmt.Errorf("not configured: SHOPIFY_ACCESS_TOKEN is empty — create a custom app in Shopify admin (Settings → Apps → Develop apps) and paste its Admin API access token")
	}
	store, err := normalizeStore(rawStore)
	if err != nil {
		return nil, err
	}
	ver := strings.TrimSpace(os.Getenv("SHOPIFY_API_VERSION"))
	if ver == "" {
		ver = defaultAPIVersion
	}
	if !versionRe.MatchString(ver) {
		return nil, fmt.Errorf("SHOPIFY_API_VERSION %q is not a valid version, expected e.g. %s", ver, defaultAPIVersion)
	}
	return &client{
		endpoint: fmt.Sprintf("https://%s/admin/api/%s/graphql.json", store, ver),
		version:  ver,
		token:    tok,
		http:     &http.Client{Timeout: httpTimeout},
	}, nil
}

type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

// gql runs one query or mutation and returns the `data` field as indented JSON.
// GraphQL-level errors and mutation userErrors are returned as Go errors, since
// Shopify reports them with HTTP 200.
func (c *client) gql(ctx context.Context, query string, vars map[string]any) (string, error) {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Shopify-Access-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("shopify %s: %s", resp.Status, truncate(string(body)))
	}
	return parseResponse(body)
}

func parseResponse(body []byte) (string, error) {
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(env.Errors) > 0 {
		msgs := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			m := e.Message
			if e.Extensions.Code == "THROTTLED" {
				m += " (rate limited — retry shortly or request fewer items)"
			}
			msgs = append(msgs, m)
		}
		return "", fmt.Errorf("shopify: %s", strings.Join(msgs, "; "))
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return "", fmt.Errorf("shopify returned no data")
	}
	var data any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("decode data: %w", err)
	}
	if msg := userErrors(data); msg != "" {
		return "", fmt.Errorf("shopify rejected the request: %s", msg)
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return truncate(string(env.Data)), nil
	}
	return truncate(string(pretty)), nil
}

// userErrors finds any non-empty `userErrors` array anywhere in a mutation
// result and renders it as "field: message".
func userErrors(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if arr, ok := t["userErrors"].([]any); ok && len(arr) > 0 {
			parts := make([]string, 0, len(arr))
			for _, e := range arr {
				m, _ := e.(map[string]any)
				msg, _ := m["message"].(string)
				if f, ok := m["field"].([]any); ok && len(f) > 0 {
					names := make([]string, 0, len(f))
					for _, n := range f {
						names = append(names, fmt.Sprint(n))
					}
					msg = strings.Join(names, ".") + ": " + msg
				}
				parts = append(parts, msg)
			}
			return strings.Join(parts, "; ")
		}
		for _, child := range t {
			if s := userErrors(child); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := userErrors(child); s != "" {
				return s
			}
		}
	}
	return ""
}

func truncate(s string) string {
	if len(s) <= maxBody {
		return s
	}
	return s[:maxBody] + "\n… truncated; narrow the query or lower `first`"
}

// gid turns a bare numeric ID into a Shopify global ID. Full gids pass through.
func gid(kind, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%s id is required", strings.ToLower(kind))
	}
	if strings.HasPrefix(id, "gid://") {
		return id, nil
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("invalid %s id %q: use a numeric id or a gid://shopify/%s/... id", strings.ToLower(kind), id, kind)
		}
	}
	return fmt.Sprintf("gid://shopify/%s/%s", kind, id), nil
}

// pageSize clamps the requested page size to Shopify's 1–100 range.
func pageSize(n int) int {
	switch {
	case n <= 0:
		return 20
	case n > 100:
		return 100
	}
	return n
}

// idempotencyKey returns a random key for mutations that need @idempotent.
func idempotencyKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprint(time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// needsIdempotency reports whether the configured API version requires the
// @idempotent directive on inventory mutations. YYYY-MM strings compare
// correctly as text; "unstable" sorts after digits and is treated as new.
func needsIdempotency(version string) bool {
	return version >= idempotentSince
}
