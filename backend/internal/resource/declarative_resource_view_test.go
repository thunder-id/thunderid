// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

func exportedResourceServerDocument(t *testing.T) *yaml.Node {
	t.Helper()
	exported, err := yaml.Marshal(&providers.ResourceServer{
		ID: "rs-1", Name: "Orders API", Description: "var:RESOURCE_SERVER_ORDERS_DESCRIPTION",
		Identifier: "https://orders.example.com", Type: providers.ResourceServerTypeCustom, OUID: "ou-1",
		AuthorizationEngine: providers.AuthorizationEngineConfig{Type: providers.AuthorizationEngineTypeRBAC},
		Resources: []providers.Resource{
			{Name: "Orders", Handle: "orders", Actions: []providers.Action{
				{Name: "Read", Handle: "read"},
				{Name: "Write", Handle: "write", Description: "Change orders"},
			}},
			{Name: "Items", Handle: "items", ParentHandle: "orders", Actions: []providers.Action{
				{Name: "Read", Handle: "read"},
			}},
			{Name: "Invoices", Handle: "invoices", Actions: []providers.Action{}},
		},
	})
	require.NoError(t, err)
	return viewDocument(t, "resource_type: resource_server\n"+string(exported))
}

func viewResourceServerParts(t *testing.T) map[string]interface{} {
	t.Helper()
	parts, err := newResourceServerExporterForTest(nil).ViewResourceParts(context.Background(),
		exportedResourceServerDocument(t))
	require.NoError(t, err)
	return parts
}

// An exported resource server is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedResourceServerAsItsRead(t *testing.T) {
	view, err := newResourceServerExporterForTest(nil).ViewResource(context.Background(),
		exportedResourceServerDocument(t))

	require.NoError(t, err)
	assert.Equal(t, &ResourceServerResponse{
		ID: "rs-1", Name: "Orders API", Description: "var:RESOURCE_SERVER_ORDERS_DESCRIPTION",
		Identifier: "https://orders.example.com", Type: providers.ResourceServerTypeCustom, OUID: "ou-1",
		AuthorizationEngine: providers.AuthorizationEngineConfig{Type: providers.AuthorizationEngineTypeRBAC},
	}, view)
}

// A resource server has its resources and its actions as parts, and for each resource the resource
// itself, its children and its actions, each named by the id a version gives it.
func TestViewResourcePartsNamesEveryPartOfAResourceServer(t *testing.T) {
	parts := viewResourceServerParts(t)

	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	assert.ElementsMatch(t, []string{
		"resources", "actions",
		"resources/rs-1_orders", "resources/rs-1_orders/resources", "resources/rs-1_orders/actions",
		"resources/rs-1_items", "resources/rs-1_items/resources", "resources/rs-1_items/actions",
		"resources/rs-1_invoices", "resources/rs-1_invoices/resources", "resources/rs-1_invoices/actions",
	}, names)
}

// The resources part shows the top-level resources, all on one page.
func TestViewResourcePartsShowsTheTopLevelResources(t *testing.T) {
	assert.Equal(t, &ResourceListResponse{
		TotalResults: 2, StartIndex: 1, Count: 2,
		Resources: []ResourceResponse{
			{ID: "rs-1_orders", Name: "Orders", Handle: "orders", Permission: "orders"},
			{ID: "rs-1_invoices", Name: "Invoices", Handle: "invoices", Permission: "invoices"},
		},
		Links: []LinkResponse{},
	}, viewResourceServerParts(t)["resources"])
}

// A resource's children are shown all on one page, as the live API lists them by parent.
func TestViewResourcePartsShowsTheChildrenOfAResource(t *testing.T) {
	parts := viewResourceServerParts(t)
	parent := "rs-1_orders"
	assert.Equal(t, &ResourceListResponse{
		TotalResults: 1, StartIndex: 1, Count: 1,
		Resources: []ResourceResponse{
			{ID: "rs-1_items", Name: "Items", Handle: "items", Parent: &parent, Permission: "orders:items"},
		},
		Links: []LinkResponse{},
	}, parts["resources/rs-1_orders/resources"])
	assert.Equal(t, 0, parts["resources/rs-1_items/resources"].(*ResourceListResponse).TotalResults)
}

// One resource is shown with its parent and its permission.
func TestViewResourcePartsShowsOneResource(t *testing.T) {
	parent := "rs-1_orders"
	assert.Equal(t, &ResourceResponse{
		ID: "rs-1_items", Name: "Items", Handle: "items", Parent: &parent, Permission: "orders:items",
	}, viewResourceServerParts(t)["resources/rs-1_items"])
}

// The actions of a resource are shown all on one page.
func TestViewResourcePartsShowsTheActionsOfAResource(t *testing.T) {
	parts := viewResourceServerParts(t)
	assert.Equal(t, &ActionListResponse{
		TotalResults: 2, StartIndex: 1, Count: 2,
		Actions: []ActionResponse{
			{ID: "rs-1_orders_read", Name: "Read", Handle: "read", Permission: "orders:read"},
			{ID: "rs-1_orders_write", Name: "Write", Handle: "write", Description: "Change orders",
				Permission: "orders:write"},
		},
		Links: []LinkResponse{},
	}, parts["resources/rs-1_orders/actions"])

	list, ok := parts["resources/rs-1_invoices/actions"].(*ActionListResponse)
	require.True(t, ok)
	assert.Equal(t, []ActionResponse{}, list.Actions)
}

// An export carries no actions at the resource server level, so they show as an empty list.
func TestViewResourcePartsShowsNoResourceServerActions(t *testing.T) {
	assert.Equal(t, &ActionListResponse{
		StartIndex: 1, Actions: []ActionResponse{}, Links: []LinkResponse{},
	}, viewResourceServerParts(t)["actions"])
}

// A document that is not a resource server is refused, and so is one whose resources do not resolve.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newResourceServerExporterForTest(nil)
	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)

	_, err = exporter.ViewResourceParts(context.Background(), viewDocument(t, "resources: not-a-list"))
	assert.Error(t, err)

	_, err = exporter.ViewResourceParts(context.Background(), viewDocument(t,
		"id: rs-1\nresources:\n  - handle: items\n    parent: missing"))
	assert.Error(t, err)
}
