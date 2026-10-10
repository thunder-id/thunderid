// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"testing"
	"time"

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

// An exported organization unit is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedOrganizationUnitAsItsRead(t *testing.T) {
	parent := "ou-root"
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	exported, err := yaml.Marshal(&OrganizationUnit{
		ID: "ou-1", Handle: "sales", Name: "Sales", Parent: &parent,
		LogoURL: "var:ORGANIZATION_UNIT_SALES_LOGO_URL", IsRegistrationFlowEnabled: true,
		CreatedAt: createdAt,
	})
	require.NoError(t, err)

	view, err := newOUExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: organization_unit\n"+string(exported)))

	require.NoError(t, err)
	ou, ok := view.(OrganizationUnit)
	require.True(t, ok)
	assert.Equal(t, "ou-1", ou.ID)
	assert.Equal(t, "sales", ou.Handle)
	assert.Equal(t, "Sales", ou.Name)
	require.NotNil(t, ou.Parent)
	assert.Equal(t, "ou-root", *ou.Parent)
	assert.Equal(t, "var:ORGANIZATION_UNIT_SALES_LOGO_URL", ou.LogoURL)
	assert.True(t, ou.IsRegistrationFlowEnabled)
	assert.True(t, createdAt.Equal(ou.CreatedAt))
}

// A document that is not an organization unit is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newOUExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)
}
