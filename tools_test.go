package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	gProduct   = "gid://shopify/Product/1"
	gVariant   = "gid://shopify/ProductVariant/2"
	gLocation  = "gid://shopify/Location/3"
	gItem      = "gid://shopify/InventoryItem/4"
	gCustomer  = "gid://shopify/Customer/5"
	gCollect   = "gid://shopify/Collection/6"
	gOrder     = "gid://shopify/Order/7"
	gPage      = "gid://shopify/Page/8"
	gBlog      = "gid://shopify/Blog/9"
	gTheme     = "gid://shopify/OnlineStoreTheme/10"
	gOwnerProd = "gid://shopify/Product/1"
)

// samples holds one realistic call per tool. TestEveryToolIsSampled keeps it
// complete; TestToolsSendValidGraphQL replays it against a fake server.
var samples = map[string]string{
	"shopify_shop":                      `{}`,
	"shopify_shop_policies":             `{}`,
	"shopify_shop_policy_update":        `{"type":"refund_policy","body":"30 days"}`,
	"shopify_locations":                 `{}`,
	"shopify_markets":                   `{}`,
	"shopify_publications":              `{}`,
	"shopify_graphql":                   `{"query":"query { shop { name } }","variables_json":"{}"}`,
	"shopify_schema":                    `{"search":"fulfillment"}`,
	"shopify_themes":                    `{}`,
	"shopify_theme_files":               `{"theme_id":"10","filenames":["templates/index.json"]}`,
	"shopify_theme_file_save":           `{"theme_id":"10","files":[{"filename":"assets/x.css","content":"a{}"}]}`,
	"shopify_theme_file_delete":         `{"theme_id":"10","filenames":["assets/x.css"]}`,
	"shopify_theme_create":              `{"source_url":"https://example.com/t.zip","name":"T"}`,
	"shopify_theme_publish":             `{"id":"10"}`,
	"shopify_products":                  `{"query":"status:active","first":5}`,
	"shopify_product":                   `{"id":"1"}`,
	"shopify_product_create":            `{"title":"Tee","tags":["a"]}`,
	"shopify_product_update":            `{"id":"1","title":"Tee 2","seo_title":"T","seo_description":"D","status":"active"}`,
	"shopify_product_set":               `{"title":"Tee","status":"ACTIVE","options":[{"name":"Size","values":["S","M"]},{"name":"Color","values":["Red"]}],"variants":[{"option_values":["S","Red"],"price":"10.00","sku":"A","inventory_quantity":5,"location_id":"3","weight_grams":200,"image_url":"https://example.com/a.jpg"},{"option_values":["M","Red"],"price":"12.00","track_inventory":false}],"images":[{"url":"https://example.com/a.jpg","alt":"a"}],"collection_ids":["6"],"seo_title":"T"}`,
	"shopify_product_delete":            `{"id":"1"}`,
	"shopify_variants_create":           `{"product_id":"1","variants":[{"option_values":[{"option_name":"Size","name":"L"}],"price":"9","sku":"L","inventory_quantity":2,"location_id":"3","image_url":"https://example.com/a.jpg"}]}`,
	"shopify_variants_update":           `{"product_id":"1","variants":[{"id":"2","price":"11","compare_at_price":"15","sku":"X","barcode":"123"}]}`,
	"shopify_variants_delete":           `{"product_id":"1","variant_ids":["2"]}`,
	"shopify_publish":                   `{"id":"1","publication_id":"11"}`,
	"shopify_collections":               `{}`,
	"shopify_collection":                `{"id":"6"}`,
	"shopify_collection_create":         `{"title":"Sale","sort_order":"best_selling","image_url":"https://example.com/c.jpg","rules":[{"field":"tag","value":"sale"},{"field":"vendor","relation":"contains","value":"Acme"}],"match_any":true}`,
	"shopify_collection_update":         `{"id":"6","title":"Sale 2","sort_order":"manual"}`,
	"shopify_collection_products":       `{"id":"6","add_product_ids":["1"],"remove_product_ids":["2"]}`,
	"shopify_collection_delete":         `{"id":"6"}`,
	"shopify_inventory_levels":          `{"id":"4"}`,
	"shopify_inventory_set":             `{"inventory_item_id":"4","location_id":"3","quantity":10}`,
	"shopify_inventory_adjust":          `{"inventory_item_id":"4","location_id":"3","delta":-2}`,
	"shopify_pages":                     `{}`,
	"shopify_page_save":                 `{"title":"About","body_html":"<p>x</p>","handle":"about"}`,
	"shopify_page_delete":               `{"id":"8"}`,
	"shopify_blogs":                     `{}`,
	"shopify_blog_create":               `{"title":"News"}`,
	"shopify_articles":                  `{}`,
	"shopify_article_save":              `{"blog_id":"9","title":"Hi","author_name":"Jo","body_html":"<p>x</p>","tags":["a"],"image_url":"https://example.com/a.jpg","image_alt":"a"}`,
	"shopify_article_delete":            `{"id":"12"}`,
	"shopify_menus":                     `{}`,
	"shopify_menu_save":                 `{"title":"Main","handle":"main-menu","items":[{"title":"Home","type":"frontpage"},{"title":"Shop","type":"collection","resource_id":"6","children":[{"title":"Docs","type":"http","url":"https://example.com"}]}]}`,
	"shopify_menu_delete":               `{"id":"13"}`,
	"shopify_redirects":                 `{}`,
	"shopify_redirect_create":           `{"path":"/old","target":"/new"}`,
	"shopify_redirect_delete":           `{"id":"14"}`,
	"shopify_files":                     `{}`,
	"shopify_file_create":               `{"files":[{"url":"https://example.com/a.png","alt":"a"},{"url":"https://example.com/a.pdf"}]}`,
	"shopify_metafields_set":            `{"metafields":[{"owner_id":"` + gOwnerProd + `","namespace":"custom","key":"material","type":"single_line_text_field","value":"cotton"}]}`,
	"shopify_discounts":                 `{}`,
	"shopify_discount_code_create":      `{"title":"Spring","code":"SPRING10","percentage":10,"applies_to":"collections","collection_ids":["6"],"min_subtotal":"50","usage_limit":100,"once_per_customer":true,"ends_at":"2027-01-01T00:00:00Z"}`,
	"shopify_discount_automatic_create": `{"title":"Auto","amount":"5","applies_to":"products","product_ids":["1"],"combines_with_shipping_discounts":true}`,
	"shopify_discount_deactivate":       `{"id":"15"}`,
	"shopify_gift_card_create":          `{"amount":"25","currency":"EUR","customer_id":"5","expires_on":"2027-01-01"}`,
	"shopify_delivery_profiles":         `{}`,
	"shopify_shipping_rate_add":         `{"zone_name":"EU","country_codes":["de","fr"],"rate_name":"Standard","price":"4.90","currency":"EUR","min_order_subtotal":"50"}`,
	"shopify_orders":                    `{"query":"financial_status:paid"}`,
	"shopify_order":                     `{"id":"7"}`,
	"shopify_order_update":              `{"id":"7","note":"n","tags":["vip"]}`,
	"shopify_order_cancel":              `{"id":"7","reason":"customer","refund_to_original_payment":true,"notify_customer":true,"staff_note":"x"}`,
	"shopify_order_close":               `{"id":"7"}`,
	"shopify_order_mark_paid":           `{"id":"7"}`,
	"shopify_fulfillment_orders":        `{"id":"7"}`,
	"shopify_fulfill":                   `{"order_id":"7","tracking_number":"T1","tracking_url":"https://example.com/t","tracking_company":"DHL"}`,
	"shopify_draft_order_create":        `{"currency":"EUR","email":"a@b.co","line_items":[{"variant_id":"2","quantity":1},{"title":"Setup","price":"20","quantity":2}],"customer_id":"5","discount_percent":10,"discount_title":"VIP","tags":["b2b"]}`,
	"shopify_draft_order_complete":      `{"id":"16"}`,
	"shopify_draft_order_invoice_send":  `{"id":"16","to":"a@b.co","subject":"S","message":"M"}`,
	"shopify_customers":                 `{"query":"email:a@b.co"}`,
	"shopify_customer":                  `{"id":"5"}`,
	"shopify_customer_create":           `{"email":"a@b.co","first_name":"A","accepts_marketing":true}`,
	"shopify_customer_update":           `{"id":"5","tags":["x"]}`,
	"shopify_customer_delete":           `{"id":"5"}`,
}

type captured struct {
	Name      string         `json:"name"`
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// fakeShopify serves canned GraphQL answers rich enough for the multi-step
// tools (publications, delivery profiles, fulfillment orders) and records every
// request.
func fakeShopify(t *testing.T, rec *[]captured, mu *sync.Mutex, current *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		mu.Lock()
		*rec = append(*rec, captured{Name: *current, Query: req.Query, Variables: req.Variables})
		mu.Unlock()

		var data string
		switch {
		case strings.Contains(req.Query, "publications(first"):
			data = `{"publications":{"nodes":[{"id":"gid://shopify/Publication/11","name":"Online Store"}]}}`
		case strings.Contains(req.Query, "deliveryProfiles(first"):
			data = `{"deliveryProfiles":{"nodes":[{"id":"gid://shopify/DeliveryProfile/20","name":"General","default":true,"profileLocationGroups":[{"locationGroup":{"id":"gid://shopify/DeliveryLocationGroup/21"}}]}]}}`
		case strings.Contains(req.Query, "fulfillmentOrders(first"):
			data = `{"order":{"id":"x","fulfillmentOrders":{"nodes":[{"id":"gid://shopify/FulfillmentOrder/30","status":"OPEN"},{"id":"gid://shopify/FulfillmentOrder/31","status":"CLOSED"}]}}}`
		case strings.Contains(req.Query, "currencyCode } }") && strings.Contains(req.Query, "shop {"):
			data = `{"shop":{"currencyCode":"EUR"}}`
		case strings.Contains(req.Query, "__schema"):
			data = `{"__schema":{"queryType":{"fields":[]},"mutationType":{"fields":[{"name":"fulfillmentCreate","args":[{"name":"fulfillment","type":{"kind":"NON_NULL","ofType":{"kind":"INPUT_OBJECT","name":"FulfillmentInput"}}}],"type":{"kind":"OBJECT","name":"FulfillmentCreatePayload"}}]}}}`
		default:
			data = `{"ok":true}`
		}
		io.WriteString(w, `{"data":`+data+`}`)
	}))
}

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := newServer().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func TestEveryToolIsSampled(t *testing.T) {
	cs := connect(t)
	for _, n := range toolNames(t, cs) {
		if _, ok := samples[n]; !ok {
			t.Errorf("tool %s has no sample call", n)
		}
	}
	registered := map[string]bool{}
	for _, n := range toolNames(t, cs) {
		registered[n] = true
	}
	for n := range samples {
		if !registered[n] {
			t.Errorf("sample for unknown tool %s", n)
		}
	}
}

func TestToolsSendValidGraphQL(t *testing.T) {
	var (
		rec     []captured
		mu      sync.Mutex
		current string
	)
	srv := fakeShopify(t, &rec, &mu, &current)
	defer srv.Close()
	endpointOverride = srv.URL
	t.Cleanup(func() { endpointOverride = "" })
	t.Setenv("SHOPIFY_STORE", "acme")
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "t")
	t.Setenv("SHOPIFY_API_VERSION", "2026-07")

	cs := connect(t)
	for _, name := range toolNames(t, cs) {
		current = name
		var args map[string]any
		if err := json.Unmarshal([]byte(samples[name]), &args); err != nil {
			t.Fatalf("%s: bad sample: %v", name, err)
		}
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if res.IsError {
			t.Errorf("%s returned error: %v", name, res.Content[0].(*mcp.TextContent).Text)
		}
	}

	if out := os.Getenv("VALIDATE_OUT"); out != "" {
		b, _ := json.MarshalIndent(rec, "", " ")
		if err := os.WriteFile(out, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestInputErrors checks bad input is rejected before any request is sent.
func TestInputErrors(t *testing.T) {
	var (
		rec     []captured
		mu      sync.Mutex
		current string
	)
	srv := fakeShopify(t, &rec, &mu, &current)
	defer srv.Close()
	endpointOverride = srv.URL
	t.Cleanup(func() { endpointOverride = "" })
	t.Setenv("SHOPIFY_STORE", "acme")
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "t")

	cs := connect(t)
	tests := []struct{ tool, args, want string }{
		{"shopify_product_create", `{"title":""}`, "title is required"},
		{"shopify_product_create", `{"title":"x","status":"LIVE"}`, "status must be"},
		{"shopify_product_update", `{"id":"1"}`, "nothing to update"},
		{"shopify_product_set", `{"title":"x","variants":[{"option_values":["a"]}]}`, "variants need options"},
		{"shopify_product_set", `{"title":"x","options":[{"name":"S","values":["a"]}],"variants":[{"option_values":["a","b"]}]}`, "expected 1"},
		{"shopify_product_set", `{"title":"x","options":[{"name":"S","values":["a"]}],"variants":[{"option_values":["a"],"inventory_quantity":1}]}`, "location_id is required"},
		{"shopify_collection_create", `{"title":"x","rules":[{"field":"color","value":"r"}]}`, "field must be"},
		{"shopify_collection_create", `{"title":"x","rules":[{"field":"tag","relation":"EQUALS","value":"r"}]}`, "not valid for tag"},
		{"shopify_inventory_adjust", `{"inventory_item_id":"1","location_id":"2","delta":0}`, "delta must be non-zero"},
		{"shopify_order", `{"id":"#1001"}`, "invalid order id"},
		{"shopify_order_cancel", `{"id":"1","reason":"bored"}`, "reason must be one of"},
		{"shopify_menu_save", `{"title":"m","handle":"m","items":[{"title":"x","type":"collection"}]}`, "needs a resource_id"},
		{"shopify_menu_save", `{"title":"m","handle":"m","items":[{"title":"x","type":"http"}]}`, "needs a url"},
		{"shopify_redirect_create", `{"path":"old","target":"/new"}`, "must start with /"},
		{"shopify_metafields_set", `{"metafields":[{"owner_id":"5","namespace":"a","key":"b","type":"c","value":"d"}]}`, "full gid"},
		{"shopify_discount_code_create", `{"title":"d","code":"X","percentage":10,"amount":"5"}`, "exactly one"},
		{"shopify_discount_code_create", `{"title":"d","percentage":10}`, "code is required"},
		{"shopify_discount_code_create", `{"title":"d","code":"X","percentage":150}`, "between 0 and 100"},
		{"shopify_shipping_rate_add", `{"zone_name":"z","rate_name":"r","price":"1"}`, "country_codes or rest_of_world"},
		{"shopify_draft_order_create", `{"line_items":[{"quantity":1}]}`, "variant_id"},
		{"shopify_customer_create", `{"first_name":"x"}`, "email or phone"},
		{"shopify_page_save", `{"body_html":"x"}`, "title is required"},
		{"shopify_article_save", `{"title":"x"}`, "blog_id are required"},
		{"shopify_graphql", `{"query":"query{shop{name}}","variables_json":"[1]"}`, "JSON object"},
		{"shopify_schema", `{}`, "pass search or type"},
		{"shopify_shop_policy_update", `{"type":"nope","body":"x"}`, "type must be one of"},
	}
	for _, tt := range tests {
		var args map[string]any
		json.Unmarshal([]byte(tt.args), &args)
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool, Arguments: args})
		if err != nil {
			t.Errorf("%s %s: %v", tt.tool, tt.args, err)
			continue
		}
		got := ""
		if len(res.Content) > 0 {
			got = res.Content[0].(*mcp.TextContent).Text
		}
		if !res.IsError || !strings.Contains(got, tt.want) {
			t.Errorf("%s %s: got %q (isError=%v), want error containing %q", tt.tool, tt.args, got, res.IsError, tt.want)
		}
	}
	for _, r := range rec {
		if r.Name != "" && !strings.Contains(r.Query, "currencyCode } }") {
			// Only currency lookups may reach the network for rejected input.
			t.Errorf("rejected call %s still sent a request: %.60s", r.Name, r.Query)
		}
	}
}

func TestMultiStepFlows(t *testing.T) {
	var (
		rec     []captured
		mu      sync.Mutex
		current string
	)
	srv := fakeShopify(t, &rec, &mu, &current)
	defer srv.Close()
	endpointOverride = srv.URL
	t.Cleanup(func() { endpointOverride = "" })
	t.Setenv("SHOPIFY_STORE", "acme")
	t.Setenv("SHOPIFY_ACCESS_TOKEN", "t")
	t.Setenv("SHOPIFY_API_VERSION", "2026-07")
	cs := connect(t)

	call := func(name, args string) {
		t.Helper()
		current = name
		var a map[string]any
		json.Unmarshal([]byte(args), &a)
		if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: a}); err != nil {
			t.Fatal(err)
		}
	}
	last := func() captured { return rec[len(rec)-1] }

	t.Run("publish defaults to Online Store", func(t *testing.T) {
		rec = nil
		call("shopify_publish", `{"id":"1"}`)
		in, _ := last().Variables["input"].([]any)
		pub, _ := in[0].(map[string]any)
		if pub["publicationId"] != "gid://shopify/Publication/11" {
			t.Fatalf("variables = %v", last().Variables)
		}
	})

	t.Run("fulfill only open fulfillment orders", func(t *testing.T) {
		rec = nil
		call("shopify_fulfill", `{"order_id":"7"}`)
		f, _ := last().Variables["fulfillment"].(map[string]any)
		lines, _ := f["lineItemsByFulfillmentOrder"].([]any)
		if len(lines) != 1 || lines[0].(map[string]any)["fulfillmentOrderId"] != "gid://shopify/FulfillmentOrder/30" {
			t.Fatalf("lines = %v", lines)
		}
		if f["notifyCustomer"] != true {
			t.Errorf("notifyCustomer = %v, want default true", f["notifyCustomer"])
		}
	})

	t.Run("shipping rate targets default profile and location group", func(t *testing.T) {
		rec = nil
		call("shopify_shipping_rate_add", `{"zone_name":"EU","country_codes":["de"],"rate_name":"Std","price":"5"}`)
		v := last().Variables
		if v["id"] != "gid://shopify/DeliveryProfile/20" {
			t.Fatalf("profile id = %v", v["id"])
		}
		groups := v["profile"].(map[string]any)["locationGroupsToUpdate"].([]any)
		if groups[0].(map[string]any)["id"] != "gid://shopify/DeliveryLocationGroup/21" {
			t.Fatalf("group = %v", groups[0])
		}
	})

	t.Run("inventory mutations carry an idempotency key on new versions", func(t *testing.T) {
		rec = nil
		call("shopify_inventory_set", `{"inventory_item_id":"4","location_id":"3","quantity":1}`)
		if !strings.Contains(last().Query, "@idempotent(key:") {
			t.Errorf("missing @idempotent: %s", last().Query)
		}
		t.Setenv("SHOPIFY_API_VERSION", "2026-01")
		call("shopify_inventory_adjust", `{"inventory_item_id":"4","location_id":"3","delta":1}`)
		if strings.Contains(last().Query, "@idempotent") {
			t.Errorf("unexpected @idempotent on 2026-01: %s", last().Query)
		}
	})

	t.Run("schema search lists matching mutations", func(t *testing.T) {
		current = "shopify_schema"
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "shopify_schema", Arguments: map[string]any{"search": "fulfil"}})
		if err != nil {
			t.Fatal(err)
		}
		txt := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(txt, "mutation fulfillmentCreate(fulfillment: FulfillmentInput!): FulfillmentCreatePayload") {
			t.Errorf("got %q", txt)
		}
	})
}

func TestUserErrorsFromAnyKey(t *testing.T) {
	_, err := parseResponse([]byte(`{"data":{"orderCancel":{"job":null,"orderCancelUserErrors":[{"field":["orderId"],"message":"already cancelled","code":"INVALID"}]}}}`))
	if err == nil || !strings.Contains(err.Error(), "orderId: already cancelled") {
		t.Fatalf("err = %v", err)
	}
}
