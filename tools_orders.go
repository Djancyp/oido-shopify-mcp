package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type OrderUpdateArgs struct {
	ID    string   `json:"id" jsonschema:"Order id (required)"`
	Note  string   `json:"note,omitempty" jsonschema:"New internal note (omit to keep)"`
	Tags  []string `json:"tags,omitempty" jsonschema:"Replacement tag list (omit to keep)"`
	Email string   `json:"email,omitempty" jsonschema:"New customer email on the order (omit to keep)"`
}

type OrderCancelArgs struct {
	ID             string `json:"id" jsonschema:"Order id (required)"`
	Reason         string `json:"reason,omitempty" jsonschema:"CUSTOMER, DECLINED, FRAUD, INVENTORY, STAFF or OTHER (default OTHER)"`
	Restock        *bool  `json:"restock,omitempty" jsonschema:"Return items to inventory (default true)"`
	RefundToOrigin bool   `json:"refund_to_original_payment,omitempty" jsonschema:"Refund the customer to their original payment method"`
	NotifyCustomer bool   `json:"notify_customer,omitempty" jsonschema:"Email the customer about the cancellation"`
	StaffNote      string `json:"staff_note,omitempty" jsonschema:"Internal note"`
}

type FulfillArgs struct {
	OrderID         string `json:"order_id" jsonschema:"Order id (required). Fulfills every open item on the order."`
	TrackingNumber  string `json:"tracking_number,omitempty" jsonschema:"Carrier tracking number"`
	TrackingURL     string `json:"tracking_url,omitempty" jsonschema:"Tracking page URL"`
	TrackingCompany string `json:"tracking_company,omitempty" jsonschema:"Carrier name, e.g. DHL"`
	NotifyCustomer  *bool  `json:"notify_customer,omitempty" jsonschema:"Email the customer the shipping confirmation (default true)"`
}

type DraftLine struct {
	VariantID string `json:"variant_id,omitempty" jsonschema:"Variant id for a catalog item"`
	Title     string `json:"title,omitempty" jsonschema:"Title for a custom item (when no variant_id)"`
	Price     string `json:"price,omitempty" jsonschema:"Unit price for a custom item as a decimal string"`
	Quantity  int    `json:"quantity" jsonschema:"Quantity (required, 1 or more)"`
}

type DraftOrderArgs struct {
	Lines           []DraftLine `json:"line_items" jsonschema:"Items on the order (required)"`
	CustomerID      string      `json:"customer_id,omitempty" jsonschema:"Existing customer id"`
	Email           string      `json:"email,omitempty" jsonschema:"Customer email (when no customer_id)"`
	Note            string      `json:"note,omitempty" jsonschema:"Internal note"`
	Tags            []string    `json:"tags,omitempty" jsonschema:"Tags"`
	DiscountPercent float64     `json:"discount_percent,omitempty" jsonschema:"Optional percent discount on the whole order, 0-100"`
	DiscountTitle   string      `json:"discount_title,omitempty" jsonschema:"Label for the discount"`
	Currency        string      `json:"currency,omitempty" jsonschema:"Currency for custom item prices (default: store currency)"`
}

type DraftInvoiceArgs struct {
	ID      string `json:"id" jsonschema:"Draft order id (required)"`
	To      string `json:"to,omitempty" jsonschema:"Recipient email (default: the draft's customer)"`
	Subject string `json:"subject,omitempty" jsonschema:"Optional subject"`
	Message string `json:"message,omitempty" jsonschema:"Optional message shown in the invoice email"`
}

const (
	ordersDoc = `query($first: Int!, $after: String, $query: String) {
  orders(first: $first, after: $after, query: $query, sortKey: CREATED_AT, reverse: true) {
    nodes {
      id name createdAt cancelledAt tags
      displayFinancialStatus displayFulfillmentStatus
      totalPriceSet { shopMoney { amount currencyCode } }
      customer { id displayName email }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

	orderDoc = `query($id: ID!) {
  order(id: $id) {
    id name createdAt cancelledAt note tags email
    displayFinancialStatus displayFulfillmentStatus
    currentTotalPriceSet { shopMoney { amount currencyCode } }
    customer { id displayName email phone }
    shippingAddress { name address1 address2 city province country zip phone }
    lineItems(first: 50) { nodes { title quantity sku originalUnitPriceSet { shopMoney { amount currencyCode } } } }
    fulfillments { id status trackingInfo { number url company } }
  }
}`

	orderUpdateDoc = `mutation($input: OrderInput!) {
  orderUpdate(input: $input) { order { id name note tags email } userErrors { field message } }
}`

	orderCancelDoc = `mutation($orderId: ID!, $reason: OrderCancelReason!, $restock: Boolean!, $refundMethod: OrderCancelRefundMethodInput, $notifyCustomer: Boolean, $staffNote: String) {
  orderCancel(orderId: $orderId, reason: $reason, restock: $restock, refundMethod: $refundMethod, notifyCustomer: $notifyCustomer, staffNote: $staffNote) {
    job { id done }
    orderCancelUserErrors { field message code }
  }
}`

	orderCloseDoc = `mutation($input: OrderCloseInput!) {
  orderClose(input: $input) { order { id name closed } userErrors { field message } }
}`

	orderMarkPaidDoc = `mutation($input: OrderMarkAsPaidInput!) {
  orderMarkAsPaid(input: $input) { order { id name displayFinancialStatus } userErrors { field message } }
}`

	fulfillmentOrdersDoc = `query($id: ID!) {
  order(id: $id) {
    id name
    fulfillmentOrders(first: 20) {
      nodes {
        id status assignedLocation { name }
        lineItems(first: 50) { nodes { id remainingQuantity lineItem { title sku } } }
      }
    }
  }
}`

	fulfillmentCreateDoc = `mutation($fulfillment: FulfillmentInput!) {
  fulfillmentCreate(fulfillment: $fulfillment) {
    fulfillment { id status trackingInfo { number url company } }
    userErrors { field message }
  }
}`

	draftOrderCreateDoc = `mutation($input: DraftOrderInput!) {
  draftOrderCreate(input: $input) {
    draftOrder { id name status invoiceUrl totalPriceSet { shopMoney { amount currencyCode } } }
    userErrors { field message }
  }
}`

	draftOrderCompleteDoc = `mutation($id: ID!) {
  draftOrderComplete(id: $id) { draftOrder { id name status order { id name } } userErrors { field message } }
}`

	draftOrderInvoiceDoc = `mutation($id: ID!, $email: EmailInput) {
  draftOrderInvoiceSend(id: $id, email: $email) { draftOrder { id name status } userErrors { field message } }
}`
)

var cancelReasons = []string{"CUSTOMER", "DECLINED", "FRAUD", "INVENTORY", "STAFF", "OTHER"}

func registerOrders(s *mcp.Server) {
	addGQL(s, "shopify_orders",
		"List orders newest first, filtered by Shopify search syntax. Without read_all_orders scope Shopify only returns the last 60 days.",
		ordersDoc, listOnly)
	addGQL(s, "shopify_order",
		"Get one order with line items, customer, shipping address and fulfillment tracking.",
		orderDoc, idVars("Order"))
	addGQL(s, "shopify_order_update",
		"Update an order's internal note, tags or email. Tags replace the whole list. Does not cancel, refund or fulfill.",
		orderUpdateDoc, func(a OrderUpdateArgs) (map[string]any, error) {
			id, err := gid("Order", a.ID)
			if err != nil {
				return nil, err
			}
			in := map[string]any{"id": id}
			setIf(in, "note", a.Note)
			setIf(in, "tags", a.Tags)
			setIf(in, "email", a.Email)
			if len(in) == 1 {
				return nil, fmt.Errorf("nothing to update: pass at least one field")
			}
			return map[string]any{"input": in}, nil
		})

	addGQL(s, "shopify_order_cancel",
		"Cancel an order. Cancelling is irreversible. Optionally restock items, refund to the original payment method and notify the customer.",
		orderCancelDoc, func(a OrderCancelArgs) (map[string]any, error) {
			id, err := gid("Order", a.ID)
			if err != nil {
				return nil, err
			}
			reason := "OTHER"
			if a.Reason != "" {
				if reason, err = oneOf("reason", a.Reason, cancelReasons...); err != nil {
					return nil, err
				}
			}
			restock := true
			if a.Restock != nil {
				restock = *a.Restock
			}
			v := map[string]any{"orderId": id, "reason": reason, "restock": restock, "notifyCustomer": a.NotifyCustomer}
			if a.RefundToOrigin {
				v["refundMethod"] = map[string]any{"originalPaymentMethodsRefund": true}
			}
			setIf(v, "staffNote", a.StaffNote)
			return v, nil
		})
	addGQL(s, "shopify_order_close",
		"Close an order that is complete and needs no further action.",
		orderCloseDoc, func(a IDArgs) (map[string]any, error) {
			id, err := gid("Order", a.ID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"input": map[string]any{"id": id}}, nil
		})
	addGQL(s, "shopify_order_mark_paid",
		"Mark a pending order (bank transfer, cash on delivery) as paid.",
		orderMarkPaidDoc, func(a IDArgs) (map[string]any, error) {
			id, err := gid("Order", a.ID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"input": map[string]any{"id": id}}, nil
		})

	addGQL(s, "shopify_fulfillment_orders",
		"Show an order's fulfillment orders and the remaining quantity to ship per item.",
		fulfillmentOrdersDoc, idVars("Order"))
	registerDoc("fulfillment_orders", fulfillmentOrdersDoc)
	registerDoc("fulfillment_create", fulfillmentCreateDoc)
	addCustom(s, "shopify_fulfill",
		"Mark all open items on an order as shipped, with optional tracking, and email the customer. Needs fulfillment scopes.",
		func(ctx context.Context, c *client, a FulfillArgs) (string, error) {
			id, err := gid("Order", a.OrderID)
			if err != nil {
				return "", err
			}
			d, err := c.call(ctx, "fulfillment_orders", fulfillmentOrdersDoc, map[string]any{"id": id})
			if err != nil {
				return "", err
			}
			foIDs := openFulfillmentOrders(d)
			if len(foIDs) == 0 {
				return "", fmt.Errorf("no open fulfillment orders on this order; it may already be fulfilled or cancelled")
			}
			lines := make([]map[string]any, 0, len(foIDs))
			for _, fo := range foIDs {
				lines = append(lines, map[string]any{"fulfillmentOrderId": fo})
			}
			notify := true
			if a.NotifyCustomer != nil {
				notify = *a.NotifyCustomer
			}
			f := map[string]any{"lineItemsByFulfillmentOrder": lines, "notifyCustomer": notify}
			tracking := map[string]any{}
			setIf(tracking, "number", a.TrackingNumber)
			setIf(tracking, "url", a.TrackingURL)
			setIf(tracking, "company", a.TrackingCompany)
			if len(tracking) > 0 {
				f["trackingInfo"] = tracking
			}
			out, err := c.call(ctx, "fulfillment_create", fulfillmentCreateDoc, map[string]any{"fulfillment": f})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})

	registerDoc("shop_currency", shopCurrencyDoc)
	registerDoc("draft_order_create", draftOrderCreateDoc)
	addCustom(s, "shopify_draft_order_create",
		"Create a draft order (manual order or quote) from catalog variants and/or custom items. Complete it with shopify_draft_order_complete or send an invoice.",
		func(ctx context.Context, c *client, a DraftOrderArgs) (string, error) {
			if len(a.Lines) == 0 {
				return "", fmt.Errorf("line_items is required")
			}
			cur := ""
			lines := make([]map[string]any, 0, len(a.Lines))
			for i, l := range a.Lines {
				if l.Quantity < 1 {
					return "", fmt.Errorf("line %d: quantity must be at least 1", i+1)
				}
				m := map[string]any{"quantity": l.Quantity}
				switch {
				case l.VariantID != "":
					v, err := gid("ProductVariant", l.VariantID)
					if err != nil {
						return "", err
					}
					m["variantId"] = v
				case l.Title != "" && l.Price != "":
					if cur == "" {
						var err error
						if cur, err = shopCurrency(ctx, c, a.Currency); err != nil {
							return "", err
						}
					}
					m["title"] = l.Title
					m["originalUnitPriceWithCurrency"] = money(l.Price, cur)
				default:
					return "", fmt.Errorf("line %d: pass variant_id, or title and price for a custom item", i+1)
				}
				lines = append(lines, m)
			}
			in := map[string]any{"lineItems": lines}
			setIf(in, "email", a.Email)
			setIf(in, "note", a.Note)
			setIf(in, "tags", a.Tags)
			if a.CustomerID != "" {
				cid, err := gid("Customer", a.CustomerID)
				if err != nil {
					return "", err
				}
				in["purchasingEntity"] = map[string]any{"customerId": cid}
			}
			if a.DiscountPercent != 0 {
				if a.DiscountPercent < 0 || a.DiscountPercent > 100 {
					return "", fmt.Errorf("discount_percent must be between 0 and 100")
				}
				d := map[string]any{"value": a.DiscountPercent, "valueType": "PERCENTAGE"}
				setIf(d, "title", a.DiscountTitle)
				in["appliedDiscount"] = d
			}
			out, err := c.call(ctx, "draft_order_create", draftOrderCreateDoc, map[string]any{"input": in})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})
	addGQL(s, "shopify_draft_order_complete",
		"Complete a draft order, turning it into a real order.",
		draftOrderCompleteDoc, idVars("DraftOrder"))
	addGQL(s, "shopify_draft_order_invoice_send",
		"Email a draft order's invoice and payment link to the customer.",
		draftOrderInvoiceDoc, func(a DraftInvoiceArgs) (map[string]any, error) {
			id, err := gid("DraftOrder", a.ID)
			if err != nil {
				return nil, err
			}
			v := map[string]any{"id": id}
			email := map[string]any{}
			setIf(email, "to", a.To)
			setIf(email, "subject", a.Subject)
			setIf(email, "customMessage", a.Message)
			if len(email) > 0 {
				v["email"] = email
			}
			return v, nil
		})
}

// openFulfillmentOrders returns the ids of fulfillment orders that still have
// items to ship.
func openFulfillmentOrders(d map[string]any) []string {
	order, _ := d["order"].(map[string]any)
	fos, _ := order["fulfillmentOrders"].(map[string]any)
	nodes, _ := fos["nodes"].([]any)
	var ids []string
	for _, n := range nodes {
		m, _ := n.(map[string]any)
		status, _ := m["status"].(string)
		if status == "OPEN" || status == "IN_PROGRESS" {
			if id, _ := m["id"].(string); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
