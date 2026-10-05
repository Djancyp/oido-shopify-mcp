package main

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CustomerCreateArgs struct {
	Email            string   `json:"email,omitempty" jsonschema:"Email (email or phone required)"`
	Phone            string   `json:"phone,omitempty" jsonschema:"Phone in E.164 format, e.g. +491701234567"`
	FirstName        string   `json:"first_name,omitempty" jsonschema:"First name"`
	LastName         string   `json:"last_name,omitempty" jsonschema:"Last name"`
	Note             string   `json:"note,omitempty" jsonschema:"Internal note"`
	Tags             []string `json:"tags,omitempty" jsonschema:"Tags"`
	AcceptsMarketing bool     `json:"accepts_marketing,omitempty" jsonschema:"Customer agreed to marketing email. Only set when you have their consent."`
}

type CustomerUpdateArgs struct {
	ID        string   `json:"id" jsonschema:"Customer id (required)"`
	Email     string   `json:"email,omitempty" jsonschema:"New email (omit to keep)"`
	Phone     string   `json:"phone,omitempty" jsonschema:"New phone (omit to keep)"`
	FirstName string   `json:"first_name,omitempty" jsonschema:"New first name (omit to keep)"`
	LastName  string   `json:"last_name,omitempty" jsonschema:"New last name (omit to keep)"`
	Note      string   `json:"note,omitempty" jsonschema:"New note (omit to keep)"`
	Tags      []string `json:"tags,omitempty" jsonschema:"Replacement tag list (omit to keep)"`
}

const (
	customersDoc = `query($first: Int!, $after: String, $query: String) {
  customers(first: $first, after: $after, query: $query) {
    nodes {
      id displayName email phone tags createdAt numberOfOrders
      amountSpent { amount currencyCode }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

	customerDoc = `query($id: ID!) {
  customer(id: $id) {
    id displayName firstName lastName email phone note tags createdAt numberOfOrders
    amountSpent { amount currencyCode }
    defaultAddress { address1 address2 city province country zip phone }
  }
}`

	customerCreateDoc = `mutation($input: CustomerInput!) {
  customerCreate(input: $input) { customer { id displayName email phone } userErrors { field message } }
}`

	customerUpdateDoc = `mutation($input: CustomerInput!) {
  customerUpdate(input: $input) { customer { id displayName email phone tags } userErrors { field message } }
}`

	customerDeleteDoc = `mutation($input: CustomerDeleteInput!) {
  customerDelete(input: $input) { deletedCustomerId userErrors { field message } }
}`
)

func customerInput(email, phone, first, last, note string, tags []string) map[string]any {
	in := map[string]any{}
	setIf(in, "email", email)
	setIf(in, "phone", phone)
	setIf(in, "firstName", first)
	setIf(in, "lastName", last)
	setIf(in, "note", note)
	setIf(in, "tags", tags)
	return in
}

func registerCustomers(s *mcp.Server) {
	addGQL(s, "shopify_customers",
		"List customers with order count and total spent, filtered by Shopify search syntax.",
		customersDoc, listOnly)
	addGQL(s, "shopify_customer",
		"Get one customer with contact details, default address and spend.",
		customerDoc, idVars("Customer"))
	addGQL(s, "shopify_customer_create",
		"Create a customer. Needs an email or phone.",
		customerCreateDoc, func(a CustomerCreateArgs) (map[string]any, error) {
			if strings.TrimSpace(a.Email) == "" && strings.TrimSpace(a.Phone) == "" {
				return nil, fmt.Errorf("email or phone is required")
			}
			in := customerInput(a.Email, a.Phone, a.FirstName, a.LastName, a.Note, a.Tags)
			if a.AcceptsMarketing {
				if a.Email == "" {
					return nil, fmt.Errorf("accepts_marketing needs an email")
				}
				in["emailMarketingConsent"] = map[string]any{"marketingState": "SUBSCRIBED", "marketingOptInLevel": "SINGLE_OPT_IN"}
			}
			return map[string]any{"input": in}, nil
		})
	addGQL(s, "shopify_customer_update",
		"Update a customer's details. Omitted fields stay unchanged; tags replace the whole list.",
		customerUpdateDoc, func(a CustomerUpdateArgs) (map[string]any, error) {
			id, err := gid("Customer", a.ID)
			if err != nil {
				return nil, err
			}
			in := customerInput(a.Email, a.Phone, a.FirstName, a.LastName, a.Note, a.Tags)
			if len(in) == 0 {
				return nil, fmt.Errorf("nothing to update: pass at least one field")
			}
			in["id"] = id
			return map[string]any{"input": in}, nil
		})
	addGQL(s, "shopify_customer_delete",
		"Permanently delete a customer. Fails for customers with orders.",
		customerDeleteDoc, func(a IDArgs) (map[string]any, error) {
			id, err := gid("Customer", a.ID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"input": map[string]any{"id": id}}, nil
		})
}
