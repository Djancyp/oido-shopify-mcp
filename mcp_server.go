package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type handler struct{}

// ---- args ----

type NoArgs struct{}

type ListArgs struct {
	Query string `json:"query,omitempty" jsonschema:"Optional Shopify search syntax, e.g. status:active vendor:Acme, or financial_status:paid fulfillment_status:unfulfilled, or email:jo@acme.com"`
	First int    `json:"first,omitempty" jsonschema:"Page size, 1-100 (default 20)"`
	After string `json:"after,omitempty" jsonschema:"Cursor from a previous page's pageInfo.endCursor"`
}

type IDArgs struct {
	ID string `json:"id" jsonschema:"Numeric id or full gid://shopify/... id (required)"`
}

type ProductCreateArgs struct {
	Title       string   `json:"title" jsonschema:"Product title (required)"`
	Description string   `json:"description_html,omitempty" jsonschema:"Optional HTML description"`
	Vendor      string   `json:"vendor,omitempty" jsonschema:"Optional vendor"`
	ProductType string   `json:"product_type,omitempty" jsonschema:"Optional product type"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Optional tags"`
	Status      string   `json:"status,omitempty" jsonschema:"ACTIVE, DRAFT or ARCHIVED (default DRAFT, so nothing goes live by accident)"`
}

type ProductUpdateArgs struct {
	ID          string   `json:"id" jsonschema:"Product id (required)"`
	Title       string   `json:"title,omitempty" jsonschema:"New title (omit to keep)"`
	Description string   `json:"description_html,omitempty" jsonschema:"New HTML description (omit to keep)"`
	Vendor      string   `json:"vendor,omitempty" jsonschema:"New vendor (omit to keep)"`
	ProductType string   `json:"product_type,omitempty" jsonschema:"New product type (omit to keep)"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Replacement tag list (omit to keep)"`
	Status      string   `json:"status,omitempty" jsonschema:"ACTIVE, DRAFT or ARCHIVED (omit to keep)"`
}

type OrderUpdateArgs struct {
	ID    string   `json:"id" jsonschema:"Order id (required)"`
	Note  string   `json:"note,omitempty" jsonschema:"New internal note (omit to keep)"`
	Tags  []string `json:"tags,omitempty" jsonschema:"Replacement tag list (omit to keep)"`
	Email string   `json:"email,omitempty" jsonschema:"New customer email on the order (omit to keep)"`
}

type InventoryAdjustArgs struct {
	InventoryItemID string `json:"inventory_item_id" jsonschema:"Inventory item id, from a variant's inventoryItem.id (required)"`
	LocationID      string `json:"location_id" jsonschema:"Location id, from shopify_locations (required)"`
	Delta           int    `json:"delta" jsonschema:"Change in available quantity, e.g. -3 or 10 (required, non-zero)"`
	Reason          string `json:"reason,omitempty" jsonschema:"correction (default), received, damaged, shrinkage, restock, cycle_count_available, movement_created"`
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
	}
	m[key] = val
}

func validStatus(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "", "ACTIVE", "DRAFT", "ARCHIVED":
		return s, nil
	}
	return "", fmt.Errorf("status must be ACTIVE, DRAFT or ARCHIVED")
}

const moneyFields = `shopMoney { amount currencyCode }`

// ---- shop ----

func (h *handler) Shop(ctx context.Context, _ *mcp.CallToolRequest, _ NoArgs) (*mcp.CallToolResult, any, error) {
	return run(ctx, `query { shop { name email myshopifyDomain primaryDomain { url } currencyCode ianaTimezone plan { displayName } } }`, nil)
}

func (h *handler) Locations(ctx context.Context, _ *mcp.CallToolRequest, _ NoArgs) (*mcp.CallToolResult, any, error) {
	return run(ctx, `query { locations(first: 50) { nodes { id name isActive } } }`, nil)
}

// ---- products ----

func (h *handler) Products(ctx context.Context, _ *mcp.CallToolRequest, a ListArgs) (*mcp.CallToolResult, any, error) {
	return run(ctx, `query($first: Int!, $after: String, $query: String) {
  products(first: $first, after: $after, query: $query) {
    nodes {
      id title handle status vendor productType tags totalInventory updatedAt
      variants(first: 5) { nodes { id title sku price inventoryQuantity inventoryItem { id } } }
    }
    pageInfo { hasNextPage endCursor }
  }
}`, listVars(a))
}

func (h *handler) Product(ctx context.Context, _ *mcp.CallToolRequest, a IDArgs) (*mcp.CallToolResult, any, error) {
	id, err := gid("Product", a.ID)
	if err != nil {
		return errResult(err), nil, nil
	}
	return run(ctx, `query($id: ID!) {
  product(id: $id) {
    id title handle status descriptionHtml vendor productType tags totalInventory createdAt updatedAt
    variants(first: 100) { nodes { id title sku price compareAtPrice inventoryQuantity inventoryItem { id } } }
  }
}`, map[string]any{"id": id})
}

func (h *handler) ProductCreate(ctx context.Context, _ *mcp.CallToolRequest, a ProductCreateArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(a.Title) == "" {
		return errResult(fmt.Errorf("title is required")), nil, nil
	}
	status, err := validStatus(a.Status)
	if err != nil {
		return errResult(err), nil, nil
	}
	if status == "" {
		status = "DRAFT"
	}
	input := map[string]any{"title": a.Title, "status": status}
	setIf(input, "descriptionHtml", a.Description)
	setIf(input, "vendor", a.Vendor)
	setIf(input, "productType", a.ProductType)
	setIf(input, "tags", a.Tags)
	return run(ctx, `mutation($product: ProductCreateInput!) {
  productCreate(product: $product) {
    product { id title handle status }
    userErrors { field message }
  }
}`, map[string]any{"product": input})
}

func (h *handler) ProductUpdate(ctx context.Context, _ *mcp.CallToolRequest, a ProductUpdateArgs) (*mcp.CallToolResult, any, error) {
	id, err := gid("Product", a.ID)
	if err != nil {
		return errResult(err), nil, nil
	}
	status, err := validStatus(a.Status)
	if err != nil {
		return errResult(err), nil, nil
	}
	input := map[string]any{"id": id}
	setIf(input, "title", a.Title)
	setIf(input, "descriptionHtml", a.Description)
	setIf(input, "vendor", a.Vendor)
	setIf(input, "productType", a.ProductType)
	setIf(input, "tags", a.Tags)
	setIf(input, "status", status)
	if len(input) == 1 {
		return errResult(fmt.Errorf("nothing to update: pass at least one field")), nil, nil
	}
	return run(ctx, `mutation($product: ProductUpdateInput!) {
  productUpdate(product: $product) {
    product { id title handle status vendor productType tags }
    userErrors { field message }
  }
}`, map[string]any{"product": input})
}

// ---- orders ----

func (h *handler) Orders(ctx context.Context, _ *mcp.CallToolRequest, a ListArgs) (*mcp.CallToolResult, any, error) {
	return run(ctx, `query($first: Int!, $after: String, $query: String) {
  orders(first: $first, after: $after, query: $query, sortKey: CREATED_AT, reverse: true) {
    nodes {
      id name createdAt cancelledAt tags
      displayFinancialStatus displayFulfillmentStatus
      totalPriceSet { `+moneyFields+` }
      customer { id displayName email }
    }
    pageInfo { hasNextPage endCursor }
  }
}`, listVars(a))
}

func (h *handler) Order(ctx context.Context, _ *mcp.CallToolRequest, a IDArgs) (*mcp.CallToolResult, any, error) {
	id, err := gid("Order", a.ID)
	if err != nil {
		return errResult(err), nil, nil
	}
	return run(ctx, `query($id: ID!) {
  order(id: $id) {
    id name createdAt cancelledAt note tags email
    displayFinancialStatus displayFulfillmentStatus
    currentTotalPriceSet { `+moneyFields+` }
    customer { id displayName email phone }
    shippingAddress { name address1 address2 city province country zip phone }
    lineItems(first: 50) { nodes { title quantity sku originalUnitPriceSet { `+moneyFields+` } } }
    fulfillments { id status trackingInfo { number url company } }
  }
}`, map[string]any{"id": id})
}

func (h *handler) OrderUpdate(ctx context.Context, _ *mcp.CallToolRequest, a OrderUpdateArgs) (*mcp.CallToolResult, any, error) {
	id, err := gid("Order", a.ID)
	if err != nil {
		return errResult(err), nil, nil
	}
	input := map[string]any{"id": id}
	setIf(input, "note", a.Note)
	setIf(input, "tags", a.Tags)
	setIf(input, "email", a.Email)
	if len(input) == 1 {
		return errResult(fmt.Errorf("nothing to update: pass at least one field")), nil, nil
	}
	return run(ctx, `mutation($input: OrderInput!) {
  orderUpdate(input: $input) {
    order { id name note tags email }
    userErrors { field message }
  }
}`, map[string]any{"input": input})
}

// ---- customers ----

func (h *handler) Customers(ctx context.Context, _ *mcp.CallToolRequest, a ListArgs) (*mcp.CallToolResult, any, error) {
	return run(ctx, `query($first: Int!, $after: String, $query: String) {
  customers(first: $first, after: $after, query: $query) {
    nodes {
      id displayName email phone tags createdAt numberOfOrders
      amountSpent { amount currencyCode }
    }
    pageInfo { hasNextPage endCursor }
  }
}`, listVars(a))
}

func (h *handler) Customer(ctx context.Context, _ *mcp.CallToolRequest, a IDArgs) (*mcp.CallToolResult, any, error) {
	id, err := gid("Customer", a.ID)
	if err != nil {
		return errResult(err), nil, nil
	}
	return run(ctx, `query($id: ID!) {
  customer(id: $id) {
    id displayName firstName lastName email phone note tags createdAt numberOfOrders
    amountSpent { amount currencyCode }
    defaultAddress { address1 address2 city province country zip phone }
  }
}`, map[string]any{"id": id})
}

// ---- inventory ----

func (h *handler) InventoryAdjust(ctx context.Context, _ *mcp.CallToolRequest, a InventoryAdjustArgs) (*mcp.CallToolResult, any, error) {
	item, err := gid("InventoryItem", a.InventoryItemID)
	if err != nil {
		return errResult(err), nil, nil
	}
	loc, err := gid("Location", a.LocationID)
	if err != nil {
		return errResult(err), nil, nil
	}
	if a.Delta == 0 {
		return errResult(fmt.Errorf("delta must be non-zero")), nil, nil
	}
	reason := strings.TrimSpace(a.Reason)
	if reason == "" {
		reason = "correction"
	}
	c, err := newClient(ctx)
	if err != nil {
		return errResult(err), nil, nil
	}
	directive := ""
	if needsIdempotency(c.version) {
		directive = fmt.Sprintf(` @idempotent(key: %q)`, idempotencyKey())
	}
	return runWith(ctx, c, `mutation($input: InventoryAdjustQuantitiesInput!) {
  inventoryAdjustQuantities(input: $input)`+directive+` {
    inventoryAdjustmentGroup { createdAt reason changes { name delta item { id } location { id } } }
    userErrors { field message }
  }
}`, map[string]any{"input": map[string]any{
		"reason": reason,
		"name":   "available",
		"changes": []map[string]any{{
			"inventoryItemId": item,
			"locationId":      loc,
			"delta":           a.Delta,
		}},
	}})
}

// RunMCPServer registers tools and serves over stdio.
func RunMCPServer() {
	h := &handler{}
	server := mcp.NewServer(&mcp.Implementation{Name: "oido-shopify", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_shop",
		Description: "Show the connected store's name, domain, currency, timezone and plan. Use it to verify the connection.",
	}, h.Shop)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_locations",
		Description: "List inventory locations. Needed for shopify_inventory_adjust.",
	}, h.Locations)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_products",
		Description: "List products with their first variants and stock, filtered by Shopify search syntax. Paginates with after.",
	}, h.Products)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_product",
		Description: "Get one product with its description and up to 100 variants (price, SKU, stock, inventory item id).",
	}, h.Product)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_product_create",
		Description: "Create a product. Defaults to DRAFT so it is not published until status is set to ACTIVE.",
	}, h.ProductCreate)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_product_update",
		Description: "Update a product's title, description, vendor, type, tags or status. Omitted fields stay unchanged; tags replace the whole list.",
	}, h.ProductUpdate)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_orders",
		Description: "List orders newest first, filtered by Shopify search syntax. Without read_all_orders scope Shopify only returns the last 60 days.",
	}, h.Orders)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_order",
		Description: "Get one order with line items, customer, shipping address and fulfillment tracking.",
	}, h.Order)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_order_update",
		Description: "Update an order's internal note, tags or email. Tags replace the whole list. Does not cancel, refund or fulfill.",
	}, h.OrderUpdate)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_customers",
		Description: "List customers with order count and total spent, filtered by Shopify search syntax.",
	}, h.Customers)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_customer",
		Description: "Get one customer with contact details, default address and spend.",
	}, h.Customer)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "shopify_inventory_adjust",
		Description: "Change available stock of one inventory item at one location by a relative delta (negative to remove).",
	}, h.InventoryAdjust)

	log.Println("Oido Shopify MCP Server starting on stdio...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
