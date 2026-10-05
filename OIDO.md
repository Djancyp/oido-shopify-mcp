# Oido Shopify

Run a Shopify store through the Admin GraphQL API: products, orders,
customers and inventory.

## Setup

1. In Shopify admin: **Settings → Apps and sales channels → Develop apps →
   Create an app**.
2. **Configure Admin API scopes**: `read_products`, `write_products`,
   `read_orders`, `write_orders`, `read_customers`, `read_inventory`,
   `write_inventory`, `read_locations`. Drop the `write_*` ones for a read-only
   setup. Add `read_all_orders` to see orders older than 60 days.
3. **Install app**, then copy the **Admin API access token** (`shpat_…`) —
   Shopify shows it once.
4. Fill the extension settings:
   - **SHOPIFY_STORE** — `acme.myshopify.com`
   - **SHOPIFY_ACCESS_TOKEN** — the token
   - **SHOPIFY_API_VERSION** — optional, defaults to `2026-07`
5. Save. Verify with `shopify_shop`.

## Tools

- `shopify_shop` — store name, domain, currency, plan.
- `shopify_locations` — inventory locations.
- `shopify_products` — list/search products (`query`, `first`, `after`).
- `shopify_product` — one product with variants.
- `shopify_product_create` — create; defaults to `DRAFT`.
- `shopify_product_update` — edit fields; omitted fields stay.
- `shopify_orders` — list/search orders, newest first.
- `shopify_order` — one order with items, address, fulfillments.
- `shopify_order_update` — note, tags, email only.
- `shopify_customers` — list/search customers.
- `shopify_customer` — one customer.
- `shopify_inventory_adjust` — relative stock change at a location.

## Notes

- IDs may be numeric (`123456`) or full gids (`gid://shopify/Product/123456`).
- Search `query` uses Shopify syntax, e.g. `status:active vendor:Acme`,
  `financial_status:paid fulfillment_status:unfulfilled`, `email:jo@acme.com`.
- Lists paginate: pass the previous `pageInfo.endCursor` as `after`.
- Stock: get the variant's `inventoryItem.id` from `shopify_product` and a
  location id from `shopify_locations`, then `shopify_inventory_adjust`.
- Tags on update **replace** the list — pass the full set.
- Cancelling, refunding and fulfilling orders are not exposed.
- Responses are raw Shopify JSON, truncated past ~12 KB.
