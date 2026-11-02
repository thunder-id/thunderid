// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

// exportedEntityType writes an entity type as the export does, with its schema as a JSON string.
func exportedEntityType(t *testing.T, resourceType string, entityType *EntityType, schema string) string {
	t.Helper()
	var node yaml.Node
	require.NoError(t, node.Encode(entityType))
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == "schema" {
			node.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: schema}
		}
	}
	exported, err := yaml.Marshal(&node)
	require.NoError(t, err)
	return "resource_type: " + resourceType + "\n" + string(exported)
}

// An exported user type and agent type are each shown as their read returns them.
func TestViewResourceShowsAnExportedEntityTypeAsItsRead(t *testing.T) {
	for _, tc := range []struct {
		resourceType string
		category     TypeCategory
	}{
		{resourceTypeEntityType, TypeCategoryUser},
		{resourceTypeAgentType, TypeCategoryAgent},
	} {
		t.Run(tc.resourceType, func(t *testing.T) {
			document := exportedEntityType(t, tc.resourceType, &EntityType{
				ID: "type-1", Category: tc.category, Handle: "employee", DisplayName: "var:EMPLOYEE_NAME",
				OUID: "ou-1", AllowSelfRegistration: true,
				SystemAttributes: &SystemAttributes{Display: "email"},
			}, `{"email":{"type":"string"}}`)

			view, err := newEntityTypeExporter(nil, tc.category).ViewResource(context.Background(),
				viewDocument(t, document))

			require.NoError(t, err)
			entityType, ok := view.(*EntityType)
			require.True(t, ok)
			assert.Equal(t, "type-1", entityType.ID)
			assert.Equal(t, tc.category, entityType.Category)
			assert.Equal(t, "employee", entityType.Handle)
			assert.Equal(t, "var:EMPLOYEE_NAME", entityType.DisplayName)
			assert.Equal(t, "ou-1", entityType.OUID)
			assert.True(t, entityType.AllowSelfRegistration)
			assert.Equal(t, "email", entityType.SystemAttributes.Display)
			assert.JSONEq(t, `{"email":{"type":"string"}}`, string(entityType.Schema))
		})
	}
}

// A document that is not an entity type is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newEntityTypeExporter(nil, TypeCategoryUser)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "handle: [not, a, handle]"))
	assert.Error(t, err)

	_, err = exporter.ViewResource(context.Background(), viewDocument(t, "handle: employee\nschema: '{'"))
	assert.Error(t, err)
}
