# Oido Shopify

Build and run a Shopify store through the Admin GraphQL API, from an empty
store to a production site: catalog, collections, pages, navigation, theme,
policies, shipping, discounts, and day-to-day orders, fulfillment and customers.

## Setup

Shopify no longer lets you create admin-created custom apps, so use a
**Dev Dashboard** app. This works for your own store: the app and the store must
be in the same Shopify organization.

1. Go to the [Dev Dashboard](https://dev.shopify.com), **Create app**.
2. In a new app version, set the Admin API **scopes** (see *Scopes* below), then
   release the version.
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

Store and meta

- `shopify_shop`, `shopify_locations`, `shopify_markets`, `shopify_publications`
- `shopify_shop_policies`, `shopify_shop_policy_update` — refund, shipping, privacy, terms.
- `shopify_graphql` — run any query or mutation not covered below.
- `shopify_schema` — look up queries, mutations and input types (`search` or `type`). Use it before `shopify_graphql`.

Theme

- `shopify_themes`, `shopify_theme_files` (list or read), `shopify_theme_file_save`,
  `shopify_theme_file_delete`, `shopify_theme_create` (from a zip URL), `shopify_theme_publish`.

Catalog

- `shopify_products`, `shopify_product`
- `shopify_product_create` (simple), `shopify_product_set` (options, variants, prices, stock, images, SEO, collections in one call), `shopify_product_update`, `shopify_product_delete`
- `shopify_variants_create`, `shopify_variants_update`, `shopify_variants_delete`
- `shopify_collections`, `shopify_collection`, `shopify_collection_create` (manual or rule-based), `shopify_collection_update`, `shopify_collection_products`, `shopify_collection_delete`
- `shopify_publish` — make a product or collection visible on the Online Store.
- `shopify_inventory_levels`, `shopify_inventory_set`, `shopify_inventory_adjust`
- `shopify_metafields_set`

Content and navigation

- `shopify_pages`, `shopify_page_save`, `shopify_page_delete`
- `shopify_blogs`, `shopify_blog_create`, `shopify_articles`, `shopify_article_save`, `shopify_article_delete`
- `shopify_menus`, `shopify_menu_save`, `shopify_menu_delete`
- `shopify_redirects`, `shopify_redirect_create`, `shopify_redirect_delete`
- `shopify_files`, `shopify_file_create`

Sales setup

- `shopify_discounts`, `shopify_discount_code_create`, `shopify_discount_automatic_create`, `shopify_discount_deactivate`
- `shopify_gift_card_create`
- `shopify_delivery_profiles`, `shopify_shipping_rate_add`

Orders and customers

- `shopify_orders`, `shopify_order`, `shopify_order_update`, `shopify_order_cancel`, `shopify_order_close`, `shopify_order_mark_paid`
- `shopify_fulfillment_orders`, `shopify_fulfill`
- `shopify_draft_order_create`, `shopify_draft_order_complete`, `shopify_draft_order_invoice_send`
- `shopify_customers`, `shopify_customer`, `shopify_customer_create`, `shopify_customer_update`, `shopify_customer_delete`

## Scopes

Grant what you will use. Each pair is `read_*` / `write_*`:

| Area | Scopes |
| --- | --- |
| Products, collections, publishing | `products`, `publications` |
| Inventory and locations | `inventory`, `locations` (read only) |
| Files | `files` |
| Pages, blogs, articles | `online_store_pages`, `content` |
| Menus and redirects | `online_store_navigation` |
| Theme | `themes` (and `write_theme_code` for Liquid/JS/CSS files, which Shopify restricts) |
| Policies | `legal_policies` |
| Shipping | `shipping` |
| Markets | `markets` (read) |
| Discounts, gift cards | `discounts`, `gift_cards` |
| Orders | `orders`, plus `read_all_orders` for history past 60 days |
| Draft orders | `draft_orders` |
| Fulfillment | `fulfillments`, `merchant_managed_fulfillment_orders` |
| Customers | `customers` |

If a tool fails with "Access denied", Shopify names the missing scope in the
error: add it to the app version, release it, and re-install.

## From empty store to production

Work in this order. Everything is created unpublished or as a draft first.

1. **Verify**: `shopify_shop`, `shopify_locations`, `shopify_publications`.
2. **Theme**: `shopify_theme_create` from a zip URL (or edit the current one
   with `shopify_theme_file_save` on an unpublished copy), check, then
   `shopify_theme_publish`.
3. **Catalog**: `shopify_collection_create`, then `shopify_product_set` per
   product with `status: ACTIVE`, images, variants and stock (it needs a
   `location_id`). Add products to manual collections with
   `shopify_collection_products`.
4. **Publish**: `shopify_publish` every product and collection. Nothing shows
   on the storefront until it is published to the Online Store.
5. **Pages**: About, Contact, FAQ with `shopify_page_save`.
6. **Navigation**: `shopify_menus` to find `main-menu` and `footer`, then
   `shopify_menu_save` with their ids to link collections and pages.
7. **Policies**: refund, shipping, privacy and terms with
   `shopify_shop_policy_update`. Checkout needs them.
8. **Shipping**: `shopify_delivery_profiles`, then `shopify_shipping_rate_add`
   per zone, e.g. flat 4.90 for DE, and free over 50 with `min_order_subtotal`.
9. **Marketing**: `shopify_discount_code_create`, `shopify_redirect_create`
   for old URLs, metafields and SEO on products.
10. **Operate**: `shopify_orders`, `shopify_fulfill` with tracking,
    `shopify_order_cancel`, `shopify_customers`.

Things the Admin API cannot do, so do them in Shopify admin: connect a
**custom domain**, activate a **payment provider**, configure **tax** details,
and pick a plan. Do these before launch.

## Notes

- IDs may be numeric (`123456`) or full gids (`gid://shopify/Product/123456`).
- Search `query` uses Shopify syntax, e.g. `status:active vendor:Acme`,
  `financial_status:paid fulfillment_status:unfulfilled`, `email:jo@acme.com`.
- Lists paginate: pass the previous `pageInfo.endCursor` as `after`.
- Update tools leave omitted fields unchanged. Tags and menu items **replace**
  the whole list: pass the full set.
- `shopify_product_set` with an `id` replaces options, variants and images:
  send the complete set.
- Destructive tools (`*_delete`, `shopify_order_cancel`,
  `shopify_theme_publish`) act on the live store immediately.
- Smart collection rules cover tag, type, vendor and title. For price or
  inventory rules, use `shopify_schema type=CollectionSourceInclusionConditionInput`
  and `shopify_graphql`.
- Responses are raw Shopify JSON, truncated past ~12 KB.
