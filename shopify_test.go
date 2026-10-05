package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNormalizeStore(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{"acme", "acme.myshopify.com", false},
		{"Acme.myshopify.com", "acme.myshopify.com", false},
		{"https://acme.myshopify.com/admin/products", "acme.myshopify.com", false},
		{" acme-shop.myshopify.com/ ", "acme-shop.myshopify.com", false},
		{"shop.example.com", "", true},
		{"evil.com/.myshopify.com", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeStore(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("normalizeStore(%q) = %q, %v; want %q (err=%v)", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestGid(t *testing.T) {
	tests := []struct {
		kind, in, want string
		wantErr        bool
	}{
		{"Product", "123", "gid://shopify/Product/123", false},
		{"Product", "gid://shopify/Product/9", "gid://shopify/Product/9", false},
		{"Order", " 42 ", "gid://shopify/Order/42", false},
		{"Order", "", "", true},
		{"Order", "12a", "", true},
		{"Order", "#1001", "", true},
	}
	for _, tt := range tests {
		got, err := gid(tt.kind, tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("gid(%q, %q) = %q, %v; want %q (err=%v)", tt.kind, tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestPageSize(t *testing.T) {
	for in, want := range map[int]int{-1: 20, 0: 20, 5: 5, 100: 100, 500: 100} {
		if got := pageSize(in); got != want {
			t.Errorf("pageSize(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestNeedsIdempotency(t *testing.T) {
	for v, want := range map[string]bool{
		"2025-10": false, "2026-01": false, "2026-04": true,
		"2026-07": true, "2027-01": true, "unstable": true,
	} {
		if got := needsIdempotency(v); got != want {
			t.Errorf("needsIdempotency(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestParseResponse(t *testing.T) {
	t.Run("data is indented", func(t *testing.T) {
		out, err := parseResponse([]byte(`{"data":{"shop":{"name":"Acme"}}}`))
		if err != nil || !strings.Contains(out, `"name": "Acme"`) {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
	t.Run("graphql errors", func(t *testing.T) {
		_, err := parseResponse([]byte(`{"errors":[{"message":"Access denied for orders field.","extensions":{"code":"ACCESS_DENIED"}}]}`))
		if err == nil || !strings.Contains(err.Error(), "Access denied") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("throttled hint", func(t *testing.T) {
		_, err := parseResponse([]byte(`{"errors":[{"message":"Throttled","extensions":{"code":"THROTTLED"}}]}`))
		if err == nil || !strings.Contains(err.Error(), "rate limited") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("userErrors become an error", func(t *testing.T) {
		_, err := parseResponse([]byte(`{"data":{"productCreate":{"product":null,"userErrors":[{"field":["product","title"],"message":"can't be blank"}]}}}`))
		if err == nil || !strings.Contains(err.Error(), "product.title: can't be blank") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("empty userErrors is success", func(t *testing.T) {
		out, err := parseResponse([]byte(`{"data":{"productCreate":{"product":{"id":"gid://shopify/Product/1"},"userErrors":[]}}}`))
		if err != nil || !strings.Contains(out, "Product/1") {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
	t.Run("null data", func(t *testing.T) {
		if _, err := parseResponse([]byte(`{"data":null}`)); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("long output truncated", func(t *testing.T) {
		big := `{"data":{"x":"` + strings.Repeat("a", maxBody*2) + `"}}`
		out, err := parseResponse([]byte(big))
		if err != nil || !strings.Contains(out, "truncated") || len(out) > maxBody+200 {
			t.Fatalf("len=%d err=%v", len(out), err)
		}
	})
}

func TestClientGQL(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("X-Shopify-Access-Token")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"data":{"shop":{"name":"Acme"}}}`)
	}))
	defer srv.Close()

	c := &client{endpoint: srv.URL, version: defaultAPIVersion, token: "shpat_x", http: srv.Client()}
	out, err := c.gql(context.Background(), `query { shop { name } }`, map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "shpat_x" {
		t.Errorf("token header = %q", gotAuth)
	}
	if gotBody["query"] != `query { shop { name } }` {
		t.Errorf("query = %v", gotBody["query"])
	}
	if !strings.Contains(out, "Acme") {
		t.Errorf("out = %q", out)
	}

	t.Run("non-2xx", func(t *testing.T) {
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"errors":"[API] Invalid API key or access token"}`)
		}))
		defer bad.Close()
		c := &client{endpoint: bad.URL, token: "x", http: bad.Client()}
		_, err := c.gql(context.Background(), `query { shop { name } }`, nil)
		if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Invalid API key") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestNewClient(t *testing.T) {
	t.Run("missing store", func(t *testing.T) {
		t.Setenv("SHOPIFY_STORE", "")
		t.Setenv("SHOPIFY_ACCESS_TOKEN", "t")
		if _, err := newClient(); err == nil || !strings.Contains(err.Error(), "SHOPIFY_STORE") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing token", func(t *testing.T) {
		t.Setenv("SHOPIFY_STORE", "acme")
		t.Setenv("SHOPIFY_ACCESS_TOKEN", "")
		if _, err := newClient(); err == nil || !strings.Contains(err.Error(), "SHOPIFY_ACCESS_TOKEN") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("default version and endpoint", func(t *testing.T) {
		t.Setenv("SHOPIFY_STORE", "acme")
		t.Setenv("SHOPIFY_ACCESS_TOKEN", "t")
		t.Setenv("SHOPIFY_API_VERSION", "")
		c, err := newClient()
		if err != nil {
			t.Fatal(err)
		}
		if want := "https://acme.myshopify.com/admin/api/" + defaultAPIVersion + "/graphql.json"; c.endpoint != want {
			t.Errorf("endpoint = %q, want %q", c.endpoint, want)
		}
	})
	t.Run("bad version", func(t *testing.T) {
		t.Setenv("SHOPIFY_STORE", "acme")
		t.Setenv("SHOPIFY_ACCESS_TOKEN", "t")
		t.Setenv("SHOPIFY_API_VERSION", "../x")
		if _, err := newClient(); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestHandlersValidateBeforeNetwork(t *testing.T) {
	// No env set: any call that reached newClient would say "not configured".
	t.Setenv("SHOPIFY_STORE", "")
	h := &handler{}
	ctx := context.Background()
	tests := []struct {
		name string
		call func() (string, bool)
		want string
	}{
		{"create needs title", func() (string, bool) {
			r, _, _ := h.ProductCreate(ctx, nil, ProductCreateArgs{})
			return text(r), r.IsError
		}, "title is required"},
		{"create bad status", func() (string, bool) {
			r, _, _ := h.ProductCreate(ctx, nil, ProductCreateArgs{Title: "x", Status: "LIVE"})
			return text(r), r.IsError
		}, "status must be"},
		{"update needs a field", func() (string, bool) {
			r, _, _ := h.ProductUpdate(ctx, nil, ProductUpdateArgs{ID: "1"})
			return text(r), r.IsError
		}, "nothing to update"},
		{"order update needs a field", func() (string, bool) {
			r, _, _ := h.OrderUpdate(ctx, nil, OrderUpdateArgs{ID: "1"})
			return text(r), r.IsError
		}, "nothing to update"},
		{"inventory zero delta", func() (string, bool) {
			r, _, _ := h.InventoryAdjust(ctx, nil, InventoryAdjustArgs{InventoryItemID: "1", LocationID: "2"})
			return text(r), r.IsError
		}, "delta must be non-zero"},
		{"bad order id", func() (string, bool) {
			r, _, _ := h.Order(ctx, nil, IDArgs{ID: "#1001"})
			return text(r), r.IsError
		}, "invalid order id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, isErr := tt.call()
			if !isErr || !strings.Contains(got, tt.want) {
				t.Fatalf("got %q (isErr=%v), want error containing %q", got, isErr, tt.want)
			}
		})
	}
}

func text(r *mcp.CallToolResult) string {
	if len(r.Content) == 0 {
		return ""
	}
	if tc, ok := r.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}
