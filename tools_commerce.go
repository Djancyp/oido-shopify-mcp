package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type DiscountArgs struct {
	Title           string   `json:"title" jsonschema:"Internal discount name (required)"`
	Code            string   `json:"code,omitempty" jsonschema:"Code customers type at checkout (required for code discounts, ignored for automatic)"`
	Percentage      float64  `json:"percentage,omitempty" jsonschema:"Percent off, 0-100 (use this or amount)"`
	Amount          string   `json:"amount,omitempty" jsonschema:"Fixed amount off as a decimal string (use this or percentage)"`
	AppliesTo       string   `json:"applies_to,omitempty" jsonschema:"all (default), products or collections"`
	ProductIDs      []string `json:"product_ids,omitempty" jsonschema:"Products when applies_to is products"`
	CollectionIDs   []string `json:"collection_ids,omitempty" jsonschema:"Collections when applies_to is collections"`
	MinSubtotal     string   `json:"min_subtotal,omitempty" jsonschema:"Minimum cart subtotal required"`
	UsageLimit      int      `json:"usage_limit,omitempty" jsonschema:"Total number of times the code can be used (code discounts only)"`
	OncePerCustomer bool     `json:"once_per_customer,omitempty" jsonschema:"Limit to one use per customer (code discounts only)"`
	StartsAt        string   `json:"starts_at,omitempty" jsonschema:"ISO 8601 start time (default: now)"`
	EndsAt          string   `json:"ends_at,omitempty" jsonschema:"ISO 8601 end time (default: never)"`
	CombinesOrder   bool     `json:"combines_with_order_discounts,omitempty" jsonschema:"Can stack with order discounts"`
	CombinesProduct bool     `json:"combines_with_product_discounts,omitempty" jsonschema:"Can stack with product discounts"`
	CombinesShip    bool     `json:"combines_with_shipping_discounts,omitempty" jsonschema:"Can stack with shipping discounts"`
}

type DiscountDeactivateArgs struct {
	ID   string `json:"id" jsonschema:"Discount id (required)"`
	Kind string `json:"kind,omitempty" jsonschema:"code (default) or automatic"`
}

type GiftCardArgs struct {
	Amount     string `json:"amount" jsonschema:"Initial value as a decimal string (required)"`
	Currency   string `json:"currency,omitempty" jsonschema:"Currency (default: store currency)"`
	Code       string `json:"code,omitempty" jsonschema:"Optional code; Shopify generates one when omitted"`
	CustomerID string `json:"customer_id,omitempty" jsonschema:"Optional customer to assign the card to"`
	Note       string `json:"note,omitempty" jsonschema:"Optional internal note"`
	ExpiresOn  string `json:"expires_on,omitempty" jsonschema:"Optional expiry date YYYY-MM-DD"`
}

type ShippingRateArgs struct {
	ProfileID    string   `json:"profile_id,omitempty" jsonschema:"Delivery profile id from shopify_delivery_profiles. Defaults to the general profile."`
	ZoneName     string   `json:"zone_name" jsonschema:"Shipping zone name, e.g. Europe (required)"`
	CountryCodes []string `json:"country_codes,omitempty" jsonschema:"ISO country codes in the zone, e.g. DE, FR (use this or rest_of_world)"`
	RestOfWorld  bool     `json:"rest_of_world,omitempty" jsonschema:"Zone covers every country not in another zone"`
	RateName     string   `json:"rate_name" jsonschema:"Rate label shown at checkout, e.g. Standard (required)"`
	Price        string   `json:"price" jsonschema:"Flat price as a decimal string, 0 for free (required)"`
	Currency     string   `json:"currency,omitempty" jsonschema:"Currency (default: store currency)"`
	MinSubtotal  string   `json:"min_order_subtotal,omitempty" jsonschema:"Only offer this rate when the order subtotal is at least this amount, e.g. for free shipping over 50"`
}

const (
	discountsDoc = `query($first: Int!, $after: String, $query: String) {
  discountNodes(first: $first, after: $after, query: $query) {
    nodes {
      id
      discount {
        __typename
        ... on DiscountCodeBasic { title status startsAt endsAt usageLimit asyncUsageCount codes(first: 3) { nodes { code } } }
        ... on DiscountAutomaticBasic { title status startsAt endsAt }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

	discountCodeCreateDoc = `mutation($basicCodeDiscount: DiscountCodeBasicInput!) {
  discountCodeBasicCreate(basicCodeDiscount: $basicCodeDiscount) {
    codeDiscountNode { id codeDiscount { ... on DiscountCodeBasic { title status codes(first: 1) { nodes { code } } } } }
    userErrors { field message code }
  }
}`

	discountAutoCreateDoc = `mutation($automaticBasicDiscount: DiscountAutomaticBasicInput!) {
  discountAutomaticBasicCreate(automaticBasicDiscount: $automaticBasicDiscount) {
    automaticDiscountNode { id automaticDiscount { ... on DiscountAutomaticBasic { title status } } }
    userErrors { field message code }
  }
}`

	discountCodeDeactivateDoc = `mutation($id: ID!) {
  discountCodeDeactivate(id: $id) { codeDiscountNode { id } userErrors { field message } }
}`

	discountAutoDeactivateDoc = `mutation($id: ID!) {
  discountAutomaticDeactivate(id: $id) { automaticDiscountNode { id } userErrors { field message } }
}`

	giftCardCreateDoc = `mutation($input: GiftCardCreateInput!) {
  giftCardCreate(input: $input) { giftCard { id lastCharacters initialValue { amount currencyCode } } giftCardCode userErrors { field message } }
}`

	deliveryProfilesDoc = `query {
  deliveryProfiles(first: 10) {
    nodes {
      id name default
      profileLocationGroups {
        locationGroup { id }
        locationGroupZones(first: 25) {
          nodes {
            zone { id name countries { code { countryCode restOfWorld } } }
            methodDefinitions(first: 25) { nodes { id name active rateProvider { ... on DeliveryRateDefinition { price { amount currencyCode } } } } }
          }
        }
      }
    }
  }
}`

	deliveryProfileUpdateDoc = `mutation($id: ID!, $profile: DeliveryProfileInput!) {
  deliveryProfileUpdate(id: $id, profile: $profile) { profile { id name } userErrors { field message } }
}`
)

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// buildDiscount builds the shared body of code and automatic basic discounts.
func buildDiscount(a DiscountArgs) (map[string]any, error) {
	if strings.TrimSpace(a.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if (a.Percentage > 0) == (a.Amount != "") {
		return nil, fmt.Errorf("pass exactly one of percentage or amount")
	}
	if a.Percentage < 0 || a.Percentage > 100 {
		return nil, fmt.Errorf("percentage must be between 0 and 100")
	}
	var value map[string]any
	if a.Percentage > 0 {
		value = map[string]any{"percentage": a.Percentage / 100}
	} else {
		value = map[string]any{"discountAmount": map[string]any{"amount": a.Amount, "appliesOnEachItem": false}}
	}
	var items map[string]any
	switch strings.ToLower(strings.TrimSpace(a.AppliesTo)) {
	case "", "all":
		items = map[string]any{"all": true}
	case "products":
		ids, err := gids("Product", a.ProductIDs)
		if err != nil || len(ids) == 0 {
			return nil, fmt.Errorf("product_ids is required when applies_to is products")
		}
		items = map[string]any{"products": map[string]any{"productsToAdd": ids}}
	case "collections":
		ids, err := gids("Collection", a.CollectionIDs)
		if err != nil || len(ids) == 0 {
			return nil, fmt.Errorf("collection_ids is required when applies_to is collections")
		}
		items = map[string]any{"collections": map[string]any{"add": ids}}
	default:
		return nil, fmt.Errorf("applies_to must be all, products or collections")
	}
	starts := a.StartsAt
	if starts == "" {
		starts = nowUTC()
	}
	d := map[string]any{
		"title":        a.Title,
		"startsAt":     starts,
		"context":      map[string]any{"all": "ALL"},
		"customerGets": map[string]any{"value": value, "items": items},
		"combinesWith": map[string]any{
			"orderDiscounts":    a.CombinesOrder,
			"productDiscounts":  a.CombinesProduct,
			"shippingDiscounts": a.CombinesShip,
		},
	}
	setIf(d, "endsAt", a.EndsAt)
	if a.MinSubtotal != "" {
		d["minimumRequirement"] = map[string]any{"subtotal": map[string]any{"greaterThanOrEqualToSubtotal": a.MinSubtotal}}
	}
	return d, nil
}

func registerCommerce(s *mcp.Server) {
	addGQL(s, "shopify_discounts", "List discounts (codes and automatic) with status and usage.", discountsDoc, listOnly)

	registerDoc("discount_code_create", discountCodeCreateDoc)
	registerDoc("discount_auto_create", discountAutoCreateDoc)
	addCustom(s, "shopify_discount_code_create",
		"Create a discount code: percent or fixed amount off, for everything or specific products/collections, with optional minimum subtotal, usage limits and dates.",
		func(ctx context.Context, c *client, a DiscountArgs) (string, error) {
			if strings.TrimSpace(a.Code) == "" {
				return "", fmt.Errorf("code is required")
			}
			d, err := buildDiscount(a)
			if err != nil {
				return "", err
			}
			d["code"] = a.Code
			if a.UsageLimit > 0 {
				d["usageLimit"] = a.UsageLimit
			}
			if a.OncePerCustomer {
				d["appliesOncePerCustomer"] = true
			}
			out, err := c.call(ctx, "discount_code_create", discountCodeCreateDoc, map[string]any{"basicCodeDiscount": d})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})
	addCustom(s, "shopify_discount_automatic_create",
		"Create an automatic discount that applies at checkout without a code, e.g. 10 percent off a collection.",
		func(ctx context.Context, c *client, a DiscountArgs) (string, error) {
			d, err := buildDiscount(a)
			if err != nil {
				return "", err
			}
			out, err := c.call(ctx, "discount_auto_create", discountAutoCreateDoc, map[string]any{"automaticBasicDiscount": d})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})
	registerDoc("discount_code_deactivate", discountCodeDeactivateDoc)
	registerDoc("discount_auto_deactivate", discountAutoDeactivateDoc)
	addCustom(s, "shopify_discount_deactivate",
		"Deactivate a discount so it stops applying. Use kind=automatic for automatic discounts.",
		func(ctx context.Context, c *client, a DiscountDeactivateArgs) (string, error) {
			if strings.EqualFold(strings.TrimSpace(a.Kind), "automatic") {
				id, err := gid("DiscountAutomaticNode", a.ID)
				if err != nil {
					return "", err
				}
				out, err := c.call(ctx, "discount_auto_deactivate", discountAutoDeactivateDoc, map[string]any{"id": id})
				if err != nil {
					return "", err
				}
				return pretty(out), nil
			}
			id, err := gid("DiscountCodeNode", a.ID)
			if err != nil {
				return "", err
			}
			out, err := c.call(ctx, "discount_code_deactivate", discountCodeDeactivateDoc, map[string]any{"id": id})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})

	registerDoc("gift_card_create", giftCardCreateDoc)
	addCustom(s, "shopify_gift_card_create",
		"Issue a gift card with an initial value. Needs the write_gift_cards scope.",
		func(ctx context.Context, c *client, a GiftCardArgs) (string, error) {
			if strings.TrimSpace(a.Amount) == "" {
				return "", fmt.Errorf("amount is required")
			}
			cur, err := shopCurrency(ctx, c, a.Currency)
			if err != nil {
				return "", err
			}
			in := map[string]any{"initialAmount": money(a.Amount, cur)}
			setIf(in, "code", a.Code)
			setIf(in, "note", a.Note)
			setIf(in, "expiresOn", a.ExpiresOn)
			if a.CustomerID != "" {
				id, err := gid("Customer", a.CustomerID)
				if err != nil {
					return "", err
				}
				in["customerId"] = id
			}
			out, err := c.call(ctx, "gift_card_create", giftCardCreateDoc, map[string]any{"input": in})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})

	addGQL(s, "shopify_delivery_profiles",
		"List shipping profiles with their zones and rates. Use this to see what shipping is configured.",
		deliveryProfilesDoc, noVars)

	registerDoc("delivery_profile_update", deliveryProfileUpdateDoc)
	addCustom(s, "shopify_shipping_rate_add",
		"Add a shipping zone with one flat rate to a delivery profile, optionally only above a minimum order subtotal (free shipping over X). Ships from the profile's existing location group.",
		func(ctx context.Context, c *client, a ShippingRateArgs) (string, error) {
			if strings.TrimSpace(a.ZoneName) == "" || strings.TrimSpace(a.RateName) == "" || strings.TrimSpace(a.Price) == "" {
				return "", fmt.Errorf("zone_name, rate_name and price are required")
			}
			if len(a.CountryCodes) == 0 && !a.RestOfWorld {
				return "", fmt.Errorf("pass country_codes or rest_of_world")
			}
			cur, err := shopCurrency(ctx, c, a.Currency)
			if err != nil {
				return "", err
			}
			profiles, err := c.call(ctx, "delivery_profiles", deliveryProfilesDoc, nil)
			if err != nil {
				return "", err
			}
			profileID, groupID, err := pickProfileGroup(profiles, a.ProfileID)
			if err != nil {
				return "", err
			}
			countries := make([]map[string]any, 0, len(a.CountryCodes)+1)
			for _, cc := range a.CountryCodes {
				countries = append(countries, map[string]any{"code": upper(cc), "includeAllProvinces": true})
			}
			if a.RestOfWorld {
				countries = append(countries, map[string]any{"restOfWorld": true})
			}
			method := map[string]any{
				"name":           a.RateName,
				"active":         true,
				"rateDefinition": map[string]any{"price": money(a.Price, cur)},
			}
			if a.MinSubtotal != "" {
				method["priceConditionsToCreate"] = []map[string]any{{"operator": "GREATER_THAN_OR_EQUAL_TO", "criteria": money(a.MinSubtotal, cur)}}
			}
			profile := map[string]any{"locationGroupsToUpdate": []map[string]any{{
				"id": groupID,
				"zonesToCreate": []map[string]any{{
					"name":                      a.ZoneName,
					"countries":                 countries,
					"methodDefinitionsToCreate": []map[string]any{method},
				}},
			}}}
			out, err := c.call(ctx, "delivery_profile_update", deliveryProfileUpdateDoc, map[string]any{"id": profileID, "profile": profile})
			if err != nil {
				return "", err
			}
			return pretty(out), nil
		})
}

// pickProfileGroup finds the profile (given id or the default) and its first
// location group.
func pickProfileGroup(d map[string]any, want string) (profileID, groupID string, err error) {
	if want != "" {
		if want, err = gid("DeliveryProfile", want); err != nil {
			return "", "", err
		}
	}
	dp, _ := d["deliveryProfiles"].(map[string]any)
	nodes, _ := dp["nodes"].([]any)
	for _, n := range nodes {
		p, _ := n.(map[string]any)
		id, _ := p["id"].(string)
		isDefault, _ := p["default"].(bool)
		if (want != "" && id != want) || (want == "" && !isDefault) {
			continue
		}
		groups, _ := p["profileLocationGroups"].([]any)
		for _, g := range groups {
			gm, _ := g.(map[string]any)
			lg, _ := gm["locationGroup"].(map[string]any)
			if gid, _ := lg["id"].(string); gid != "" {
				return id, gid, nil
			}
		}
		return "", "", fmt.Errorf("delivery profile %s has no location group; add a shipping origin location in Shopify admin first", id)
	}
	return "", "", fmt.Errorf("delivery profile not found; list them with shopify_delivery_profiles")
}
