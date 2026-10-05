# Oido Shopify

Run a Shopify store through the Admin GraphQL API: products, orders,
customers and inventory.

## Setup

Shopify no longer lets you create admin-created custom apps, so use a
**Dev Dashboard** app. This works for your own store: the app and the store must
be in the same Shopify organization.

1. Go to the [Dev Dashboard](https://dev.shopify.com), **Create app**.
2. In a new app version, set the Admin API **scopes**: `read_products`,
   `write_products`, `read_orders`, `write_orders`, `read_customers`,
   `read_inventory`, `write_inventory`, `read_locations`. Drop the `write_*`
   ones for read-only. Add `read_all_orders` to see orders older than 60 days.
   Release the version.
3. **Install** the app on your store from the Dev Dashboard.
4. App **Settings** → copy the **Client ID** and **Client secret**.
5. Fill the extension settings:
   - **SHOPIFY_STORE** — `acme.myshopify.com`
   - **SHOPIFY_CLIENT_ID** / **SHOPIFY_CLIENT_SECRET** — from step 4
   - **SHOPIFY_API_VERSION** — optional, defaults to `2026-07`
6. Save. Verify with `shopify_shop`.

The plugin exchanges the client credentials for an Admin API token (valid 24 h)
and renews it automatically.

Already have a legacy custom app? Leave the client fields empty and set
**SHOPIFY_ACCESS_TOKEN** to its `shpat_…` token instead.

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
