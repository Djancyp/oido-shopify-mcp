package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registeredDocs records every GraphQL document a tool can send, keyed by a
// label. A test dumps them so they can be validated against Shopify's schema.
var registeredDocs = map[string]string{}

func registerDoc(label, doc string) { registeredDocs[label] = doc }

// ---- shared arg types ----

type NoArgs struct{}

type ListArgs struct {
	Query string `json:"query,omitempty" jsonschema:"Optional Shopify search syntax, e.g. status:active vendor:Acme, or financial_status:paid fulfillment_status:unfulfilled, or email:jo@acme.com"`
	First int    `json:"first,omitempty" jsonschema:"Page size, 1-100 (default 20)"`
	After string `json:"after,omitempty" jsonschema:"Cursor from a previous page's pageInfo.endCursor"`
}

type IDArgs struct {
	ID string `json:"id" jsonschema:"Numeric id or full gid://shopify/... id (required)"`
}

// ---- helpers ----

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + err.Error()}}, IsError: true}
}

// run is the shared shape of every tool: build a client, issue one GraphQL
// request, return its JSON.
func run(ctx context.Context, query string, vars map[string]any) (*mcp.CallToolResult, any, error) {
	c, err := newClient(ctx)
	if err != nil {
		return errResult(err), nil, nil
	}
	return runWith(ctx, c, query, vars)
}

func runWith(ctx context.Context, c *client, query string, vars map[string]any) (*mcp.CallToolResult, any, error) {
	out, err := c.gql(ctx, query, vars)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(out), nil, nil
}

// addGQL registers a tool that maps its args to variables for one fixed
// document.
func addGQL[A any](s *mcp.Server, name, desc, doc string, vars func(A) (map[string]any, error)) {
	registerDoc(name, doc)
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, a A) (*mcp.CallToolResult, any, error) {
			v, err := vars(a)
			if err != nil {
				return errResult(err), nil, nil
			}
			return run(ctx, doc, v)
		})
}

// addCustom registers a tool with its own multi-step handler.
func addCustom[A any](s *mcp.Server, name, desc string, fn func(ctx context.Context, c *client, a A) (string, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, a A) (*mcp.CallToolResult, any, error) {
			c, err := newClient(ctx)
			if err != nil {
				return errResult(err), nil, nil
			}
			out, err := fn(ctx, c, a)
			if err != nil {
				return errResult(err), nil, nil
			}
			return textResult(out), nil, nil
		})
}

// call runs a document and returns the decoded data for multi-step tools.
func (c *client) call(ctx context.Context, label, doc string, vars map[string]any) (map[string]any, error) {
	registerDoc(label, doc)
	d, err := c.gqlData(ctx, doc, vars)
	if err != nil {
		return nil, err
	}
	m, _ := d.(map[string]any)
	return m, nil
}

func pretty(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return truncate(string(b))
}

func listVars(a ListArgs) map[string]any {
	v := map[string]any{"first": pageSize(a.First)}
	if a.After != "" {
		v["after"] = a.After
	}
	if q := strings.TrimSpace(a.Query); q != "" {
		v["query"] = q
	}
	return v
}

func idVars(kind string) func(IDArgs) (map[string]any, error) {
	return func(a IDArgs) (map[string]any, error) {
		id, err := gid(kind, a.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id}, nil
	}
}

func listOnly(a ListArgs) (map[string]any, error) { return listVars(a), nil }

func noVars(NoArgs) (map[string]any, error) { return nil, nil }

// setIf copies non-empty values into a GraphQL input object so omitted fields
// stay unchanged on update.
func setIf(m map[string]any, key string, val any) {
	switch v := val.(type) {
	case string:
		if v == "" {
			return
		}
	case []string:
		if len(v) == 0 {
			return
		}
	case nil:
		return
	}
	m[key] = val
}

func upper(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

func oneOf(field, v string, allowed ...string) (string, error) {
	v = upper(v)
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s must be one of %s", field, strings.Join(allowed, ", "))
}

// shopCurrency returns given when set, else the store's currency.
func shopCurrency(ctx context.Context, c *client, given string) (string, error) {
	if g := upper(given); g != "" {
		return g, nil
	}
	d, err := c.call(ctx, "shop_currency", shopCurrencyDoc, nil)
	if err != nil {
		return "", err
	}
	shop, _ := d["shop"].(map[string]any)
	cur, _ := shop["currencyCode"].(string)
	if cur == "" {
		return "", fmt.Errorf("could not read the store currency; pass currency explicitly")
	}
	return cur, nil
}

func money(amount, currency string) map[string]any {
	return map[string]any{"amount": amount, "currencyCode": currency}
}

// ---- core: shop, policies, locations, markets, publications ----

type PolicyUpdateArgs struct {
	Type string `json:"type" jsonschema:"REFUND_POLICY, SHIPPING_POLICY, PRIVACY_POLICY, TERMS_OF_SERVICE, TERMS_OF_SALE, LEGAL_NOTICE, SUBSCRIPTION_POLICY or CONTACT_INFORMATION"`
	Body string `json:"body" jsonschema:"Policy text (HTML or plain text)"`
}

type GraphQLArgs struct {
	Query         string `json:"query" jsonschema:"A GraphQL query or mutation document"`
	VariablesJSON string `json:"variables_json,omitempty" jsonschema:"Optional JSON object of variables"`
}

type SchemaArgs struct {
	Search string `json:"search,omitempty" jsonschema:"Keyword to find root queries and mutations by name, e.g. fulfillment or metaobject"`
	Type   string `json:"type,omitempty" jsonschema:"Exact type name to describe fields, input fields or enum values, e.g. ProductSetInput"`
}

const typeRef = `kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } }`

const schemaSearchDoc = `query {
  __schema {
    queryType { fields { name description args { name type { ` + typeRef + ` } } type { ` + typeRef + ` } } }
    mutationType { fields { name description args { name type { ` + typeRef + ` } } type { ` + typeRef + ` } } }
  }
}`

const schemaTypeDoc = `query($name: String!) {
  __type(name: $name) {
    kind name description
    fields { name description type { ` + typeRef + ` } }
    inputFields { name description type { ` + typeRef + ` } }
    enumValues { name description }
  }
}`

const shopCurrencyDoc = `query { shop { currencyCode } }`

func registerCore(s *mcp.Server) {
	registerDoc("schema_search", schemaSearchDoc)
	registerDoc("schema_type", schemaTypeDoc)
	registerDoc("shop_currency", shopCurrencyDoc)
	addGQL(s, "shopify_shop",
		"Show the connected store's name, domain, currency, timezone and plan. Use it to verify the connection.",
		`query { shop { name email contactEmail myshopifyDomain primaryDomain { url host } currencyCode ianaTimezone weightUnit taxesIncluded plan { displayName } shipsToCountries } }`, noVars)
	addGQL(s, "shopify_shop_policies",
		"Read the store's legal policies (refund, shipping, privacy, terms). A production store needs these filled in.",
		`query { shop { shopPolicies { id type title body url } } }`, noVars)
	addGQL(s, "shopify_shop_policy_update",
		"Create or replace one legal policy: refund, shipping, privacy, terms of service and so on.",
		`mutation($policy: ShopPolicyInput!) {
  shopPolicyUpdate(shopPolicy: $policy) { shopPolicy { id type url } userErrors { field message } }
}`, func(a PolicyUpdateArgs) (map[string]any, error) {
			if strings.TrimSpace(a.Body) == "" {
				return nil, fmt.Errorf("body is required")
			}
			t, err := oneOf("type", a.Type, "REFUND_POLICY", "SHIPPING_POLICY", "PRIVACY_POLICY", "TERMS_OF_SERVICE", "TERMS_OF_SALE", "LEGAL_NOTICE", "SUBSCRIPTION_POLICY", "CONTACT_INFORMATION")
			if err != nil {
				return nil, err
			}
			return map[string]any{"policy": map[string]any{"type": t, "body": a.Body}}, nil
		})
	addGQL(s, "shopify_locations",
		"List inventory locations. Needed for stock and shipping tools.",
		`query { locations(first: 50) { nodes { id name isActive address { city countryCode } } } }`, noVars)
	addGQL(s, "shopify_markets",
		"List markets (the regions the store sells to) with their status.",
		`query { markets(first: 50) { nodes { id name handle status } } }`, noVars)
	addGQL(s, "shopify_publications",
		"List sales channels (publications), e.g. Online Store. Products and collections must be published to a channel to be visible.",
		`query { publications(first: 20) { nodes { id name } } }`, noVars)

	addCustom(s, "shopify_graphql",
		"Run any Admin GraphQL query or mutation. Use it for anything the other tools do not cover. Look up exact names with shopify_schema first. Mutations here change the live store.",
		func(ctx context.Context, c *client, a GraphQLArgs) (string, error) {
			if strings.TrimSpace(a.Query) == "" {
				return "", fmt.Errorf("query is required")
			}
			var vars map[string]any
			if strings.TrimSpace(a.VariablesJSON) != "" {
				if err := json.Unmarshal([]byte(a.VariablesJSON), &vars); err != nil {
					return "", fmt.Errorf("variables_json must be a JSON object: %w", err)
				}
			}
			return c.gql(ctx, a.Query, vars)
		})
	addCustom(s, "shopify_schema",
		"Look up the Admin API schema. With search: find queries and mutations by keyword, with their arguments. With type: list a type's fields, input fields or enum values. Use before shopify_graphql.",
		func(ctx context.Context, c *client, a SchemaArgs) (string, error) {
			if name := strings.TrimSpace(a.Type); name != "" {
				d, err := c.call(ctx, "schema_type", schemaTypeDoc, map[string]any{"name": name})
				if err != nil {
					return "", err
				}
				return describeType(d["__type"])
			}
			if strings.TrimSpace(a.Search) == "" {
				return "", fmt.Errorf("pass search or type")
			}
			d, err := c.call(ctx, "schema_search", schemaSearchDoc, nil)
			if err != nil {
				return "", err
			}
			return searchSchema(d, a.Search), nil
		})
}

// typeString renders an introspection type reference like [ID!]!.
func typeString(v any) string {
	m, _ := v.(map[string]any)
	if m == nil {
		return ""
	}
	kind, _ := m["kind"].(string)
	switch kind {
	case "NON_NULL":
		return typeString(m["ofType"]) + "!"
	case "LIST":
		return "[" + typeString(m["ofType"]) + "]"
	}
	name, _ := m["name"].(string)
	return name
}

func describeType(v any) (string, error) {
	m, _ := v.(map[string]any)
	if m == nil {
		return "", fmt.Errorf("type not found")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%v %v\n", m["kind"], m["name"])
	for _, key := range []string{"fields", "inputFields"} {
		items, _ := m[key].([]any)
		for _, it := range items {
			f, _ := it.(map[string]any)
			fmt.Fprintf(&b, "  %v: %s\n", f["name"], typeString(f["type"]))
		}
	}
	if items, _ := m["enumValues"].([]any); len(items) > 0 {
		names := make([]string, 0, len(items))
		for _, it := range items {
			f, _ := it.(map[string]any)
			names = append(names, fmt.Sprint(f["name"]))
		}
		b.WriteString("  values: " + strings.Join(names, ", ") + "\n")
	}
	return truncate(b.String()), nil
}

func searchSchema(d map[string]any, keyword string) string {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	schema, _ := d["__schema"].(map[string]any)
	var lines []string
	for _, root := range []struct{ key, label string }{{"queryType", "query"}, {"mutationType", "mutation"}} {
		rt, _ := schema[root.key].(map[string]any)
		fields, _ := rt["fields"].([]any)
		for _, it := range fields {
			f, _ := it.(map[string]any)
			name, _ := f["name"].(string)
			if !strings.Contains(strings.ToLower(name), kw) {
				continue
			}
			args, _ := f["args"].([]any)
			parts := make([]string, 0, len(args))
			for _, ai := range args {
				a, _ := ai.(map[string]any)
				parts = append(parts, fmt.Sprintf("%v: %s", a["name"], typeString(a["type"])))
			}
			lines = append(lines, fmt.Sprintf("%s %s(%s): %s", root.label, name, strings.Join(parts, ", "), typeString(f["type"])))
		}
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "no queries or mutations match " + keyword
	}
	return truncate(strings.Join(lines, "\n"))
}
