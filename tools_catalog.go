package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---- args ----

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
	Handle      string   `json:"handle,omitempty" jsonschema:"New URL handle (omit to keep)"`
	Vendor      string   `json:"vendor,omitempty" jsonschema:"New vendor (omit to keep)"`
	ProductType string   `json:"product_type,omitempty" jsonschema:"New product type (omit to keep)"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Replacement tag list (omit to keep)"`
	Status      string   `json:"status,omitempty" jsonschema:"ACTIVE, DRAFT or ARCHIVED (omit to keep)"`
	SEOTitle    string   `json:"seo_title,omitempty" jsonschema:"Search engine page title (omit to keep)"`
	SEODesc     string   `json:"seo_description,omitempty" jsonschema:"Search engine meta description (omit to keep)"`
}

type OptionDef struct {
	Name   string   `json:"name" jsonschema:"Option name, e.g. Size or Color"`
	Values []string `json:"values" jsonschema:"Option values, e.g. S, M, L"`
}

type VariantDef struct {
	OptionValues      []string `json:"option_values" jsonschema:"One value per option, in option order, e.g. M, Red. Use a single Default Title for products without options."`
	Price             string   `json:"price,omitempty" jsonschema:"Price as a decimal string, e.g. 29.90"`
	CompareAtPrice    string   `json:"compare_at_price,omitempty" jsonschema:"Original price shown struck through"`
	SKU               string   `json:"sku,omitempty" jsonschema:"Stock keeping unit"`
	Barcode           string   `json:"barcode,omitempty" jsonschema:"Barcode (ISBN, UPC, GTIN)"`
	InventoryQuantity *int     `json:"inventory_quantity,omitempty" jsonschema:"Available stock at location_id"`
	LocationID        string   `json:"location_id,omitempty" jsonschema:"Location for inventory_quantity (required when inventory_quantity is set)"`
	Tracked           *bool    `json:"track_inventory,omitempty" jsonschema:"Track stock for this variant (default true when inventory_quantity is set)"`
	RequiresShipping  *bool    `json:"requires_shipping,omitempty" jsonschema:"False for digital goods"`
	WeightGrams       float64  `json:"weight_grams,omitempty" jsonschema:"Weight in grams, used by weight-based shipping rates"`
	ImageURL          string   `json:"image_url,omitempty" jsonschema:"Public image URL for this variant; should also appear in images"`
	Taxable           *bool    `json:"taxable,omitempty" jsonschema:"Charge tax (default true)"`
}

type ImageDef struct {
	URL string `json:"url" jsonschema:"Public image URL Shopify will download"`
	Alt string `json:"alt,omitempty" jsonschema:"Alt text for accessibility and SEO"`
}

type ProductSetArgs struct {
	ID            string       `json:"id,omitempty" jsonschema:"Existing product id to replace. Omit to create. When set, options, variants and images you pass REPLACE the current ones."`
	Title         string       `json:"title" jsonschema:"Product title (required)"`
	Description   string       `json:"description_html,omitempty" jsonschema:"HTML description"`
	Handle        string       `json:"handle,omitempty" jsonschema:"URL handle"`
	Vendor        string       `json:"vendor,omitempty" jsonschema:"Vendor"`
	ProductType   string       `json:"product_type,omitempty" jsonschema:"Product type"`
	Tags          []string     `json:"tags,omitempty" jsonschema:"Tags"`
	Status        string       `json:"status,omitempty" jsonschema:"ACTIVE, DRAFT or ARCHIVED (default DRAFT)"`
	SEOTitle      string       `json:"seo_title,omitempty" jsonschema:"Search engine page title"`
	SEODesc       string       `json:"seo_description,omitempty" jsonschema:"Search engine meta description"`
	CategoryID    string       `json:"category_id,omitempty" jsonschema:"Optional Shopify taxonomy category gid, e.g. gid://shopify/TaxonomyCategory/aa-1-1"`
	Options       []OptionDef  `json:"options,omitempty" jsonschema:"Options such as Size and Color. Omit for a single-variant product."`
	Variants      []VariantDef `json:"variants,omitempty" jsonschema:"Variants, one per option combination. Omit for a single default variant."`
	Images        []ImageDef   `json:"images,omitempty" jsonschema:"Product images, first one is the main image"`
	CollectionIDs []string     `json:"collection_ids,omitempty" jsonschema:"Manual collections to add the product to"`
}

type ProductDeleteArgs struct {
	ID string `json:"id" jsonschema:"Product id (required). Deletion is permanent."`
}

type BulkVariant struct {
	ID             string        `json:"id,omitempty" jsonschema:"Variant id (required for update, omit for create)"`
	OptionValues   []OptionValue `json:"option_values,omitempty" jsonschema:"Option values for create, e.g. option_name Size name L"`
	Price          string        `json:"price,omitempty" jsonschema:"Price as decimal string"`
	CompareAtPrice string        `json:"compare_at_price,omitempty" jsonschema:"Compare-at price"`
	SKU            string        `json:"sku,omitempty" jsonschema:"SKU"`
	Barcode        string        `json:"barcode,omitempty" jsonschema:"Barcode"`
	Quantity       *int          `json:"inventory_quantity,omitempty" jsonschema:"Available stock at location_id (create only)"`
	LocationID     string        `json:"location_id,omitempty" jsonschema:"Location for inventory_quantity"`
	ImageURL       string        `json:"image_url,omitempty" jsonschema:"Public image URL"`
}

type OptionValue struct {
	OptionName string `json:"option_name" jsonschema:"Option name, e.g. Size"`
	Name       string `json:"name" jsonschema:"Value, e.g. L"`
}

type VariantsBulkArgs struct {
	ProductID string        `json:"product_id" jsonschema:"Product id (required)"`
	Variants  []BulkVariant `json:"variants" jsonschema:"Variants to create or update (required)"`
}

type VariantsDeleteArgs struct {
	ProductID  string   `json:"product_id" jsonschema:"Product id (required)"`
	VariantIDs []string `json:"variant_ids" jsonschema:"Variant ids to delete (required)"`
}

type PublishArgs struct {
	ID            string `json:"id" jsonschema:"Product or collection id (required)"`
	Kind          string `json:"kind,omitempty" jsonschema:"product (default) or collection, used when id is numeric"`
	PublicationID string `json:"publication_id,omitempty" jsonschema:"Sales channel id from shopify_publications. Defaults to the Online Store."`
}

type CollectionRule struct {
	Field    string `json:"field" jsonschema:"tag, type, vendor or title"`
	Relation string `json:"relation,omitempty" jsonschema:"For tag: TAGGED_WITH (default) or NOT_TAGGED_WITH. For others: EQUALS (default), NOT_EQUALS, CONTAINS, DOES_NOT_CONTAIN, STARTS_WITH, ENDS_WITH"`
	Value    string `json:"value" jsonschema:"Value to match"`
}

type CollectionCreateArgs struct {
	Title       string           `json:"title" jsonschema:"Collection title (required)"`
	Description string           `json:"description_html,omitempty" jsonschema:"HTML description"`
	Handle      string           `json:"handle,omitempty" jsonschema:"URL handle"`
	SortOrder   string           `json:"sort_order,omitempty" jsonschema:"MANUAL, BEST_SELLING, ALPHA_ASC, ALPHA_DESC, PRICE_ASC, PRICE_DESC, CREATED, CREATED_DESC"`
	Rules       []CollectionRule `json:"rules,omitempty" jsonschema:"Rules for an automated (smart) collection. Omit for a manual collection you fill with shopify_collection_products."`
	MatchAny    bool             `json:"match_any,omitempty" jsonschema:"Rules: product matches ANY rule instead of ALL"`
	ImageURL    string           `json:"image_url,omitempty" jsonschema:"Optional collection image URL"`
	ImageAlt    string           `json:"image_alt,omitempty" jsonschema:"Alt text for the image"`
}

type CollectionUpdateArgs struct {
	ID          string `json:"id" jsonschema:"Collection id (required)"`
	Title       string `json:"title,omitempty" jsonschema:"New title (omit to keep)"`
	Description string `json:"description_html,omitempty" jsonschema:"New HTML description (omit to keep)"`
	Handle      string `json:"handle,omitempty" jsonschema:"New handle (omit to keep)"`
	SortOrder   string `json:"sort_order,omitempty" jsonschema:"New sort order (omit to keep)"`
	ImageURL    string `json:"image_url,omitempty" jsonschema:"New image URL (omit to keep)"`
	ImageAlt    string `json:"image_alt,omitempty" jsonschema:"Alt text for the image"`
}

type CollectionProductsArgs struct {
	ID               string   `json:"id" jsonschema:"Manual collection id (required)"`
	AddProductIDs    []string `json:"add_product_ids,omitempty" jsonschema:"Products to add"`
	RemoveProductIDs []string `json:"remove_product_ids,omitempty" jsonschema:"Products to remove"`
}

type InventorySetArgs struct {
	InventoryItemID string `json:"inventory_item_id" jsonschema:"Inventory item id from a variant's inventoryItem.id (required)"`
	LocationID      string `json:"location_id" jsonschema:"Location id (required)"`
	Quantity        int    `json:"quantity" jsonschema:"Absolute available quantity to set (required, 0 or more)"`
}

type InventoryAdjustArgs struct {
	InventoryItemID string `json:"inventory_item_id" jsonschema:"Inventory item id, from a variant's inventoryItem.id (required)"`
	LocationID      string `json:"location_id" jsonschema:"Location id, from shopify_locations (required)"`
	Delta           int    `json:"delta" jsonschema:"Change in available quantity, e.g. -3 or 10 (required, non-zero)"`
	Reason          string `json:"reason,omitempty" jsonschema:"correction (default), received, damaged, shrinkage, restock, cycle_count_available, movement_created"`
}

// ---- documents ----

const (
	productsDoc = `query($first: Int!, $after: String, $query: String) {
  products(first: $first, after: $after, query: $query) {
    nodes {
      id title handle status vendor productType tags totalInventory updatedAt
      variants(first: 5) { nodes { id title sku price inventoryQuantity inventoryItem { id } } }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

	productDoc = `query($id: ID!) {
  product(id: $id) {
    id title handle status descriptionHtml vendor productType tags totalInventory createdAt updatedAt
    seo { title description }
    options { id name values }
    media(first: 20) { nodes { id alt mediaContentType ... on MediaImage { image { url } } } }
    collections(first: 20) { nodes { id title } }
    variants(first: 100) { nodes { id title sku barcode price compareAtPrice inventoryQuantity selectedOptions { name value } inventoryItem { id tracked } } }
  }
}`

	productCreateDoc = `mutation($product: ProductCreateInput!) {
  productCreate(product: $product) { product { id title handle status } userErrors { field message } }
}`

	productUpdateDoc = `mutation($product: ProductUpdateInput!) {
  productUpdate(product: $product) { product { id title handle status vendor productType tags } userErrors { field message } }
}`

	productSetDoc = `mutation($input: ProductSetInput!, $identifier: ProductSetIdentifiers) {
  productSet(input: $input, identifier: $identifier, synchronous: true) {
    product {
      id title handle status
      options { name values }
      variants(first: 100) { nodes { id title sku price inventoryQuantity inventoryItem { id } } }
    }
    userErrors { field message code }
  }
}`

	productDeleteDoc = `mutation($input: ProductDeleteInput!) {
  productDelete(input: $input, synchronous: true) { deletedProductId userErrors { field message } }
}`

	variantsCreateDoc = `mutation($productId: ID!, $variants: [ProductVariantsBulkInput!]!) {
  productVariantsBulkCreate(productId: $productId, variants: $variants) {
    productVariants { id title sku price inventoryItem { id } }
    userErrors { field message }
  }
}`

	variantsUpdateDoc = `mutation($productId: ID!, $variants: [ProductVariantsBulkInput!]!) {
  productVariantsBulkUpdate(productId: $productId, variants: $variants) {
    productVariants { id title sku price compareAtPrice }
    userErrors { field message }
  }
}`

	variantsDeleteDoc = `mutation($productId: ID!, $variantsIds: [ID!]!) {
  productVariantsBulkDelete(productId: $productId, variantsIds: $variantsIds) {
    product { id title }
    userErrors { field message }
  }
}`

	publicationsDoc = `query { publications(first: 20) { nodes { id name } } }`

	publishDoc = `mutation($id: ID!, $input: [PublicationInput!]!) {
  publishablePublish(id: $id, input: $input) { publishable { ... on Product { id title } ... on Collection { id title } } userErrors { field message } }
}`

	collectionsDoc = `query($first: Int!, $after: String, $query: String) {
  collections(first: $first, after: $after, query: $query) {
    nodes { id title handle sortOrder productsCount { count } updatedAt }
    pageInfo { hasNextPage endCursor }
  }
}`

	collectionDoc = `query($id: ID!) {
  collection(id: $id) {
    id title handle descriptionHtml sortOrder productsCount { count }
    products(first: 50) { nodes { id title status } }
  }
}`

	collectionCreateDoc = `mutation($collection: CollectionCreateInput!) {
  collectionCreate(collection: $collection) { collection { id title handle sortOrder productsCount { count } } userErrors { field message } }
}`

	collectionUpdateDoc = `mutation($collection: CollectionUpdateInput!) {
  collectionUpdate(collection: $collection) { collection { id title handle sortOrder } userErrors { field message } }
}`

	collectionAddDoc = `mutation($id: ID!, $productIds: [ID!]!) {
  collectionAddProductsV2(id: $id, productIds: $productIds) { job { id done } userErrors { field message } }
}`

	collectionRemoveDoc = `mutation($id: ID!, $productIds: [ID!]!) {
  collectionRemoveProducts(id: $id, productIds: $productIds) { job { id done } userErrors { field message } }
}`

	collectionDeleteDoc = `mutation($input: CollectionDeleteInput!) {
  collectionDelete(input: $input) { deletedCollectionId userErrors { field message } }
}`

	inventoryLevelsDoc = `query($id: ID!) {
  inventoryItem(id: $id) {
    id sku tracked
    inventoryLevels(first: 20) { nodes { location { id name } quantities(names: ["available", "on_hand", "committed"]) { name quantity } } }
  }
}`

	inventorySetDoc = `mutation($input: InventorySetQuantitiesInput!) {
  inventorySetQuantities(input: $input) {
    inventoryAdjustmentGroup { createdAt reason changes { name delta item { id } location { id } } }
    userErrors { field message }
  }
}`

	inventoryAdjustDoc = `mutation($input: InventoryAdjustQuantitiesInput!) {
  inventoryAdjustQuantities(input: $input) {
    inventoryAdjustmentGroup { createdAt reason changes { name delta item { id } location { id } } }
    userErrors { field message }
  }
}`
)

func validStatus(s string) (string, error) {
	s = upper(s)
	switch s {
	case "", "ACTIVE", "DRAFT", "ARCHIVED":
		return s, nil
	}
	return "", fmt.Errorf("status must be ACTIVE, DRAFT or ARCHIVED")
}

func seoInput(title, desc string) map[string]any {
	seo := map[string]any{}
	setIf(seo, "title", title)
	setIf(seo, "description", desc)
	if len(seo) == 0 {
		return nil
	}
	return seo
}

// buildProductSet converts the friendly product args into a ProductSetInput.
func buildProductSet(a ProductSetArgs) (map[string]any, error) {
	if strings.TrimSpace(a.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	status, err := validStatus(a.Status)
	if err != nil {
		return nil, err
	}
	if status == "" {
		status = "DRAFT"
	}
	in := map[string]any{"title": a.Title, "status": status}
	setIf(in, "descriptionHtml", a.Description)
	setIf(in, "handle", a.Handle)
	setIf(in, "vendor", a.Vendor)
	setIf(in, "productType", a.ProductType)
	setIf(in, "tags", a.Tags)
	if seo := seoInput(a.SEOTitle, a.SEODesc); seo != nil {
		in["seo"] = seo
	}
	if a.CategoryID != "" {
		in["category"] = a.CategoryID
	}
	if len(a.CollectionIDs) > 0 {
		ids, err := gids("Collection", a.CollectionIDs)
		if err != nil {
			return nil, err
		}
		in["collections"] = ids
	}

	if len(a.Images) > 0 {
		files := make([]map[string]any, 0, len(a.Images))
		for _, im := range a.Images {
			if strings.TrimSpace(im.URL) == "" {
				return nil, fmt.Errorf("every image needs a url")
			}
			f := map[string]any{"originalSource": im.URL, "contentType": "IMAGE"}
			setIf(f, "alt", im.Alt)
			files = append(files, f)
		}
		in["files"] = files
	}

	if len(a.Options) > 0 {
		opts := make([]map[string]any, 0, len(a.Options))
		for _, o := range a.Options {
			if strings.TrimSpace(o.Name) == "" || len(o.Values) == 0 {
				return nil, fmt.Errorf("every option needs a name and at least one value")
			}
			vals := make([]map[string]any, 0, len(o.Values))
			for _, v := range o.Values {
				vals = append(vals, map[string]any{"name": v})
			}
			opts = append(opts, map[string]any{"name": o.Name, "values": vals})
		}
		in["productOptions"] = opts
	}

	if len(a.Variants) > 0 {
		if len(a.Options) == 0 {
			return nil, fmt.Errorf("variants need options: pass options, or omit variants for a single-variant product")
		}
		vs := make([]map[string]any, 0, len(a.Variants))
		for i, v := range a.Variants {
			if len(v.OptionValues) != len(a.Options) {
				return nil, fmt.Errorf("variant %d has %d option_values, expected %d (one per option)", i+1, len(v.OptionValues), len(a.Options))
			}
			ov := make([]map[string]any, 0, len(v.OptionValues))
			for j, val := range v.OptionValues {
				ov = append(ov, map[string]any{"optionName": a.Options[j].Name, "name": val})
			}
			out := map[string]any{"optionValues": ov}
			setIf(out, "price", v.Price)
			setIf(out, "compareAtPrice", v.CompareAtPrice)
			setIf(out, "sku", v.SKU)
			setIf(out, "barcode", v.Barcode)
			if v.Taxable != nil {
				out["taxable"] = *v.Taxable
			}
			if v.ImageURL != "" {
				out["file"] = map[string]any{"originalSource": v.ImageURL, "contentType": "IMAGE"}
			}
			item := map[string]any{}
			if v.Tracked != nil {
				item["tracked"] = *v.Tracked
			} else if v.InventoryQuantity != nil {
				item["tracked"] = true
			}
			if v.RequiresShipping != nil {
				item["requiresShipping"] = *v.RequiresShipping
			}
			if v.WeightGrams > 0 {
				item["measurement"] = map[string]any{"weight": map[string]any{"value": v.WeightGrams, "unit": "GRAMS"}}
			}
			if len(item) > 0 {
				out["inventoryItem"] = item
			}
			if v.InventoryQuantity != nil {
				if v.LocationID == "" {
					return nil, fmt.Errorf("variant %d: location_id is required with inventory_quantity", i+1)
				}
				loc, err := gid("Location", v.LocationID)
				if err != nil {
					return nil, err
				}
				out["inventoryQuantities"] = []map[string]any{{"locationId": loc, "name": "available", "quantity": *v.InventoryQuantity}}
			}
			vs = append(vs, out)
		}
		in["variants"] = vs
	}
	return in, nil
}

func buildBulkVariants(vs []BulkVariant, forUpdate bool) ([]map[string]any, error) {
	if len(vs) == 0 {
		return nil, fmt.Errorf("variants is required")
	}
	out := make([]map[string]any, 0, len(vs))
	for i, v := range vs {
		m := map[string]any{}
		if forUpdate {
			id, err := gid("ProductVariant", v.ID)
			if err != nil {
				return nil, fmt.Errorf("variant %d: %w", i+1, err)
			}
			m["id"] = id
		} else if len(v.OptionValues) == 0 {
			return nil, fmt.Errorf("variant %d: option_values is required", i+1)
		}
		if len(v.OptionValues) > 0 {
			ov := make([]map[string]any, 0, len(v.OptionValues))
			for _, o := range v.OptionValues {
				ov = append(ov, map[string]any{"optionName": o.OptionName, "name": o.Name})
			}
			m["optionValues"] = ov
		}
		setIf(m, "price", v.Price)
		setIf(m, "compareAtPrice", v.CompareAtPrice)
		setIf(m, "barcode", v.Barcode)
		if v.SKU != "" {
			m["inventoryItem"] = map[string]any{"sku": v.SKU}
		}
		if v.ImageURL != "" {
			m["mediaSrc"] = []string{v.ImageURL}
		}
		if v.Quantity != nil && !forUpdate {
			if v.LocationID == "" {
				return nil, fmt.Errorf("variant %d: location_id is required with inventory_quantity", i+1)
			}
			loc, err := gid("Location", v.LocationID)
			if err != nil {
				return nil, err
			}
			m["inventoryQuantities"] = []map[string]any{{"locationId": loc, "availableQuantity": *v.Quantity}}
		}
		out = append(out, m)
	}
	return out, nil
}

var collectionRelations = map[string]map[string]bool{
	"tag":    {"TAGGED_WITH": true, "NOT_TAGGED_WITH": true},
	"type":   {"EQUALS": true, "NOT_EQUALS": true, "CONTAINS": true, "DOES_NOT_CONTAIN": true, "STARTS_WITH": true, "ENDS_WITH": true},
	"vendor": {"EQUALS": true, "NOT_EQUALS": true, "CONTAINS": true, "DOES_NOT_CONTAIN": true, "STARTS_WITH": true, "ENDS_WITH": true},
	"title":  {"EQUALS": true, "NOT_EQUALS": true, "CONTAINS": true, "DOES_NOT_CONTAIN": true, "STARTS_WITH": true, "ENDS_WITH": true},
}

var collectionConditionKeys = map[string]string{"tag": "productTag", "type": "productType", "vendor": "productVendor", "title": "productTitle"}

func buildCollectionRules(rules []CollectionRule, matchAny bool) (map[string]any, error) {
	conds := make([]map[string]any, 0, len(rules))
	for i, r := range rules {
		field := strings.ToLower(strings.TrimSpace(r.Field))
		allowed, ok := collectionRelations[field]
		if !ok {
			return nil, fmt.Errorf("rule %d: field must be tag, type, vendor or title", i+1)
		}
		if strings.TrimSpace(r.Value) == "" {
			return nil, fmt.Errorf("rule %d: value is required", i+1)
		}
		rel := upper(r.Relation)
		if rel == "" {
			rel = "EQUALS"
			if field == "tag" {
				rel = "TAGGED_WITH"
			}
		}
		if !allowed[rel] {
			return nil, fmt.Errorf("rule %d: relation %s is not valid for %s", i+1, rel, field)
		}
		conds = append(conds, map[string]any{collectionConditionKeys[field]: map[string]any{
			"relation": rel, "values": []string{r.Value}, "matchType": "ANY",
		}})
	}
	match := "ALL"
	if matchAny {
		match = "ANY"
	}
	return map[string]any{"source": map[string]any{
		"title":      "Rules",
		"targetType": "PRODUCTS",
		"inclusion":  map[string]any{"matchType": match, "conditions": conds},
	}}, nil
}

var sortOrders = []string{"MANUAL", "BEST_SELLING", "ALPHA_ASC", "ALPHA_DESC", "PRICE_ASC", "PRICE_DESC", "CREATED", "CREATED_DESC"}

func imageInput(url, alt string) map[string]any {
	img := map[string]any{"src": url}
	setIf(img, "altText", alt)
	return img
}

func registerCatalog(s *mcp.Server) {
	addGQL(s, "shopify_products",
		"List products with their first variants and stock, filtered by Shopify search syntax. Paginates with after.",
		productsDoc, listOnly)
	addGQL(s, "shopify_product",
		"Get one product with description, options, images, collections and up to 100 variants (price, SKU, stock, inventory item id).",
		productDoc, idVars("Product"))

	addGQL(s, "shopify_product_create",
		"Create a simple product (no variants or images). Defaults to DRAFT. For options, variants, images and stock in one call use shopify_product_set.",
		productCreateDoc, func(a ProductCreateArgs) (map[string]any, error) {
			if strings.TrimSpace(a.Title) == "" {
				return nil, fmt.Errorf("title is required")
			}
			status, err := validStatus(a.Status)
			if err != nil {
				return nil, err
			}
			if status == "" {
				status = "DRAFT"
			}
			in := map[string]any{"title": a.Title, "status": status}
			setIf(in, "descriptionHtml", a.Description)
			setIf(in, "vendor", a.Vendor)
			setIf(in, "productType", a.ProductType)
			setIf(in, "tags", a.Tags)
			return map[string]any{"product": in}, nil
		})

	addGQL(s, "shopify_product_update",
		"Update a product's title, description, handle, vendor, type, tags, status or SEO. Omitted fields stay unchanged; tags replace the whole list. Does not touch variants or images: use shopify_product_set or the variants tools.",
		productUpdateDoc, func(a ProductUpdateArgs) (map[string]any, error) {
			id, err := gid("Product", a.ID)
			if err != nil {
				return nil, err
			}
			status, err := validStatus(a.Status)
			if err != nil {
				return nil, err
			}
			in := map[string]any{"id": id}
			setIf(in, "title", a.Title)
			setIf(in, "descriptionHtml", a.Description)
			setIf(in, "handle", a.Handle)
			setIf(in, "vendor", a.Vendor)
			setIf(in, "productType", a.ProductType)
			setIf(in, "tags", a.Tags)
			setIf(in, "status", status)
			if seo := seoInput(a.SEOTitle, a.SEODesc); seo != nil {
				in["seo"] = seo
			}
			if len(in) == 1 {
				return nil, fmt.Errorf("nothing to update: pass at least one field")
			}
			return map[string]any{"product": in}, nil
		})

	addGQL(s, "shopify_product_set",
		"Create or fully replace a product including options, variants (price, SKU, stock), images, SEO and collections in one call. Defaults to DRAFT. With id, options/variants/images you pass replace the existing ones, so send the complete set.",
		productSetDoc, func(a ProductSetArgs) (map[string]any, error) {
			in, err := buildProductSet(a)
			if err != nil {
				return nil, err
			}
			v := map[string]any{"input": in}
			if strings.TrimSpace(a.ID) != "" {
				id, err := gid("Product", a.ID)
				if err != nil {
					return nil, err
				}
				v["identifier"] = map[string]any{"id": id}
			}
			return v, nil
		})

	addGQL(s, "shopify_product_delete",
		"Permanently delete a product and its variants.",
		productDeleteDoc, func(a ProductDeleteArgs) (map[string]any, error) {
			id, err := gid("Product", a.ID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"input": map[string]any{"id": id}}, nil
		})

	addGQL(s, "shopify_variants_create",
		"Add variants to an existing product. The product's options must already exist (see shopify_product).",
		variantsCreateDoc, func(a VariantsBulkArgs) (map[string]any, error) {
			pid, err := gid("Product", a.ProductID)
			if err != nil {
				return nil, err
			}
			vs, err := buildBulkVariants(a.Variants, false)
			if err != nil {
				return nil, err
			}
			return map[string]any{"productId": pid, "variants": vs}, nil
		})
	addGQL(s, "shopify_variants_update",
		"Update variants' price, compare-at price, SKU, barcode or image by variant id. Omitted fields stay unchanged.",
		variantsUpdateDoc, func(a VariantsBulkArgs) (map[string]any, error) {
			pid, err := gid("Product", a.ProductID)
			if err != nil {
				return nil, err
			}
			vs, err := buildBulkVariants(a.Variants, true)
			if err != nil {
				return nil, err
			}
			return map[string]any{"productId": pid, "variants": vs}, nil
		})
	addGQL(s, "shopify_variants_delete",
		"Delete variants from a product by variant id.",
		variantsDeleteDoc, func(a VariantsDeleteArgs) (map[string]any, error) {
			pid, err := gid("Product", a.ProductID)
			if err != nil {
				return nil, err
			}
			ids, err := gids("ProductVariant", a.VariantIDs)
			if err != nil || len(ids) == 0 {
				return nil, fmt.Errorf("variant_ids is required")
			}
			return map[string]any{"productId": pid, "variantsIds": ids}, nil
		})

	registerDoc("publications", publicationsDoc)
	registerDoc("publish", publishDoc)
	addCustom(s, "shopify_publish",
		"Publish a product or collection to a sales channel (default: Online Store) so shoppers can see it. New products are not visible on the storefront until published.",
		func(ctx context.Context, c *client, a PublishArgs) (string, error) {
			kind := "Product"
			if strings.EqualFold(strings.TrimSpace(a.Kind), "collection") {
				kind = "Collection"
			}
			id, err := gid(kind, a.ID)
			if err != nil {
				return "", err
			}
			pub := strings.TrimSpace(a.PublicationID)
			if pub == "" {
				d, err := c.call(ctx, "publications", publicationsDoc, nil)
				if err != nil {
					return "", err
				}
				pub = findOnlineStore(d)
				if pub == "" {
					return "", fmt.Errorf("no Online Store sales channel found; pass publication_id from shopify_publications")
				}
			} else if pub, err = gid("Publication", pub); err != nil {
				return "", err
			}
			d, err := c.call(ctx, "publish", publishDoc, map[string]any{"id": id, "input": []map[string]any{{"publicationId": pub}}})
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})

	addGQL(s, "shopify_collections",
		"List collections with product counts.",
		collectionsDoc, listOnly)
	addGQL(s, "shopify_collection",
		"Get one collection with its first 50 products.",
		collectionDoc, idVars("Collection"))

	addGQL(s, "shopify_collection_create",
		"Create a collection. Without rules it is manual (fill it with shopify_collection_products); with rules it is automated by tag, type, vendor or title. Publish it with shopify_publish.",
		collectionCreateDoc, func(a CollectionCreateArgs) (map[string]any, error) {
			if strings.TrimSpace(a.Title) == "" {
				return nil, fmt.Errorf("title is required")
			}
			in := map[string]any{"title": a.Title}
			setIf(in, "descriptionHtml", a.Description)
			setIf(in, "handle", a.Handle)
			if a.SortOrder != "" {
				so, err := oneOf("sort_order", a.SortOrder, sortOrders...)
				if err != nil {
					return nil, err
				}
				in["sortOrder"] = so
			}
			if a.ImageURL != "" {
				in["image"] = imageInput(a.ImageURL, a.ImageAlt)
			}
			if len(a.Rules) > 0 {
				src, err := buildCollectionRules(a.Rules, a.MatchAny)
				if err != nil {
					return nil, err
				}
				in["sources"] = []map[string]any{src}
			}
			return map[string]any{"collection": in}, nil
		})
	addGQL(s, "shopify_collection_update",
		"Update a collection's title, description, handle, sort order or image. Omitted fields stay unchanged.",
		collectionUpdateDoc, func(a CollectionUpdateArgs) (map[string]any, error) {
			id, err := gid("Collection", a.ID)
			if err != nil {
				return nil, err
			}
			in := map[string]any{"id": id}
			setIf(in, "title", a.Title)
			setIf(in, "descriptionHtml", a.Description)
			setIf(in, "handle", a.Handle)
			if a.SortOrder != "" {
				so, err := oneOf("sort_order", a.SortOrder, sortOrders...)
				if err != nil {
					return nil, err
				}
				in["sortOrder"] = so
			}
			if a.ImageURL != "" {
				in["image"] = imageInput(a.ImageURL, a.ImageAlt)
			}
			if len(in) == 1 {
				return nil, fmt.Errorf("nothing to update: pass at least one field")
			}
			return map[string]any{"collection": in}, nil
		})

	registerDoc("collection_add", collectionAddDoc)
	registerDoc("collection_remove", collectionRemoveDoc)
	addCustom(s, "shopify_collection_products",
		"Add products to and/or remove products from a manual collection.",
		func(ctx context.Context, c *client, a CollectionProductsArgs) (string, error) {
			id, err := gid("Collection", a.ID)
			if err != nil {
				return "", err
			}
			if len(a.AddProductIDs) == 0 && len(a.RemoveProductIDs) == 0 {
				return "", fmt.Errorf("pass add_product_ids and/or remove_product_ids")
			}
			result := map[string]any{}
			if len(a.AddProductIDs) > 0 {
				ids, err := gids("Product", a.AddProductIDs)
				if err != nil {
					return "", err
				}
				d, err := c.call(ctx, "collection_add", collectionAddDoc, map[string]any{"id": id, "productIds": ids})
				if err != nil {
					return "", err
				}
				result["added"] = d
			}
			if len(a.RemoveProductIDs) > 0 {
				ids, err := gids("Product", a.RemoveProductIDs)
				if err != nil {
					return "", err
				}
				d, err := c.call(ctx, "collection_remove", collectionRemoveDoc, map[string]any{"id": id, "productIds": ids})
				if err != nil {
					return "", err
				}
				result["removed"] = d
			}
			return pretty(result), nil
		})
	addGQL(s, "shopify_collection_delete",
		"Delete a collection. Its products are not deleted.",
		collectionDeleteDoc, func(a IDArgs) (map[string]any, error) {
			id, err := gid("Collection", a.ID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"input": map[string]any{"id": id}}, nil
		})

	addGQL(s, "shopify_inventory_levels",
		"Show stock per location for one inventory item (available, on hand, committed).",
		inventoryLevelsDoc, idVars("InventoryItem"))

	registerDoc("inventory_set", inventorySetDoc)
	addCustom(s, "shopify_inventory_set",
		"Set the absolute available stock of one inventory item at one location. Use shopify_inventory_adjust for relative changes.",
		func(ctx context.Context, c *client, a InventorySetArgs) (string, error) {
			item, err := gid("InventoryItem", a.InventoryItemID)
			if err != nil {
				return "", err
			}
			loc, err := gid("Location", a.LocationID)
			if err != nil {
				return "", err
			}
			if a.Quantity < 0 {
				return "", fmt.Errorf("quantity must be 0 or more")
			}
			d, err := c.gqlData(ctx, c.idempotent(inventorySetDoc, "inventorySetQuantities(input: $input)"), map[string]any{"input": map[string]any{
				"reason": "correction",
				"name":   "available",
				"quantities": []map[string]any{{
					"inventoryItemId": item, "locationId": loc, "quantity": a.Quantity,
				}},
			}})
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})
	registerDoc("inventory_adjust", inventoryAdjustDoc)
	addCustom(s, "shopify_inventory_adjust",
		"Change available stock of one inventory item at one location by a relative delta (negative to remove).",
		func(ctx context.Context, c *client, a InventoryAdjustArgs) (string, error) {
			item, err := gid("InventoryItem", a.InventoryItemID)
			if err != nil {
				return "", err
			}
			loc, err := gid("Location", a.LocationID)
			if err != nil {
				return "", err
			}
			if a.Delta == 0 {
				return "", fmt.Errorf("delta must be non-zero")
			}
			reason := strings.TrimSpace(a.Reason)
			if reason == "" {
				reason = "correction"
			}
			d, err := c.gqlData(ctx, c.idempotent(inventoryAdjustDoc, "inventoryAdjustQuantities(input: $input)"), map[string]any{"input": map[string]any{
				"reason": reason,
				"name":   "available",
				"changes": []map[string]any{{
					"inventoryItemId": item, "locationId": loc, "delta": a.Delta,
				}},
			}})
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})
}

// findOnlineStore picks the Online Store publication id from a publications read.
func findOnlineStore(d map[string]any) string {
	pubs, _ := d["publications"].(map[string]any)
	nodes, _ := pubs["nodes"].([]any)
	for _, n := range nodes {
		m, _ := n.(map[string]any)
		if name, _ := m["name"].(string); strings.EqualFold(name, "Online Store") {
			id, _ := m["id"].(string)
			return id
		}
	}
	return ""
}
