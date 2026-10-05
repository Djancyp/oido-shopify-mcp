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
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
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

// tokenCache holds the client-credentials token between tool calls. The server
// is a long-lived process and Shopify tokens last 24h, so one exchange serves
// many calls.
var tokenCache struct {
	sync.Mutex
	key     string
	token   string
	expires time.Time
}

// tokenRefreshMargin renews a token slightly early so it never expires mid-call.
const tokenRefreshMargin = 5 * time.Minute

// clientCredentialsToken exchanges a Dev Dashboard app's client id/secret for an
// Admin API token. Only works when the app and store share a Shopify org.
func clientCredentialsToken(ctx context.Context, hc *http.Client, store, id, secret string) (string, error) {
	key := store + "|" + id + "|" + secret
	tokenCache.Lock()
	defer tokenCache.Unlock()
	if tokenCache.key == key && tokenCache.token != "" && time.Now().Before(tokenCache.expires) {
		return tokenCache.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {id}, "client_secret": {secret}}
	tok, ttl, err := exchangeToken(ctx, hc, fmt.Sprintf("https://%s/admin/oauth/access_token", store), form)
	if err != nil {
		return "", err
	}
	tokenCache.key, tokenCache.token = key, tok
	tokenCache.expires = time.Now().Add(ttl - tokenRefreshMargin)
	return tok, nil
}

func exchangeToken(ctx context.Context, hc *http.Client, endpoint string, form url.Values) (string, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := hc.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("token exchange %s: %s — check SHOPIFY_CLIENT_ID/SECRET, that the app is installed on the store, and that the app and store are in the same Shopify organization", resp.Status, truncate(string(body)))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.AccessToken == "" {
		return "", 0, fmt.Errorf("token exchange: unexpected response")
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= tokenRefreshMargin {
		ttl = 24 * time.Hour
	}
	return out.AccessToken, ttl, nil
}

// newClient reads the store settings and resolves an Admin API token: either a
// static SHOPIFY_ACCESS_TOKEN (legacy custom app) or SHOPIFY_CLIENT_ID +
// SHOPIFY_CLIENT_SECRET (Dev Dashboard app, exchanged for a 24h token).
// endpointOverride redirects GraphQL requests; set only by tests.
var endpointOverride string

func newClient(ctx context.Context) (*client, error) {
	rawStore := os.Getenv("SHOPIFY_STORE")
	tok := strings.TrimSpace(os.Getenv("SHOPIFY_ACCESS_TOKEN"))
	clientID := strings.TrimSpace(os.Getenv("SHOPIFY_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("SHOPIFY_CLIENT_SECRET"))
	if strings.TrimSpace(rawStore) == "" {
		return nil, fmt.Errorf("not configured: SHOPIFY_STORE is empty — set it to your store domain, e.g. acme.myshopify.com")
	}
	if tok == "" && (clientID == "" || clientSecret == "") {
		return nil, fmt.Errorf("not configured: set SHOPIFY_CLIENT_ID + SHOPIFY_CLIENT_SECRET (Dev Dashboard app → Settings), or SHOPIFY_ACCESS_TOKEN for an existing legacy custom app")
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
	hc := &http.Client{Timeout: httpTimeout}
	if tok == "" {
		if tok, err = clientCredentialsToken(ctx, hc, store, clientID, clientSecret); err != nil {
			return nil, err
		}
	}
	endpoint := fmt.Sprintf("https://%s/admin/api/%s/graphql.json", store, ver)
	if endpointOverride != "" {
		endpoint = endpointOverride
	}
	return &client{
		endpoint: endpoint,
		version:  ver,
		token:    tok,
		http:     hc,
	}, nil
}

type gqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

// gql runs one query or mutation and returns the `data` field as indented JSON,
// truncated for the model's context.
func (c *client) gql(ctx context.Context, query string, vars map[string]any) (string, error) {
	data, err := c.gqlData(ctx, query, vars)
	if err != nil {
		return "", err
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode data: %w", err)
	}
	return truncate(string(pretty)), nil
}

// gqlData runs one query or mutation and returns the decoded `data` field.
// GraphQL-level errors and mutation userErrors are returned as Go errors, since
// Shopify reports them with HTTP 200.
func (c *client) gqlData(ctx context.Context, query string, vars map[string]any) (any, error) {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Shopify-Access-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("shopify %s: %s", resp.Status, truncate(string(body)))
	}
	return decodeResponse(body)
}

func decodeResponse(body []byte) (any, error) {
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
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
		return nil, fmt.Errorf("shopify: %s", strings.Join(msgs, "; "))
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil, fmt.Errorf("shopify returned no data")
	}
	var data any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, fmt.Errorf("decode data: %w", err)
	}
	if msg := userErrors(data); msg != "" {
		return nil, fmt.Errorf("shopify rejected the request: %s", msg)
	}
	return data, nil
}

// parseResponse decodes a response body into indented, truncated JSON.
func parseResponse(body []byte) (string, error) {
	data, err := decodeResponse(body)
	if err != nil {
		return "", err
	}
	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode data: %w", err)
	}
	return truncate(string(pretty)), nil
}

// userErrors finds any non-empty `userErrors` array anywhere in a mutation
// result and renders it as "field: message".
func userErrors(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for key, val := range t {
			if !strings.HasSuffix(strings.ToLower(key), "usererrors") {
				continue
			}
			arr, ok := val.([]any)
			if !ok || len(arr) == 0 {
				continue
			}
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

// gids converts a list of ids to global ids of one kind.
func gids(kind string, ids []string) ([]string, error) {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		g, err := gid(kind, id)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// idempotent adds the @idempotent directive after call in doc when the API
// version requires it for inventory mutations.
func (c *client) idempotent(doc, call string) string {
	if !needsIdempotency(c.version) {
		return doc
	}
	return strings.Replace(doc, call, fmt.Sprintf("%s @idempotent(key: %q)", call, idempotencyKey()), 1)
}
