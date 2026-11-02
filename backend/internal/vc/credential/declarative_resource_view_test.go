// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

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

// An exported credential configuration is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedConfigurationAsItsRead(t *testing.T) {
	validity := 3600
	dto := &CredentialConfigurationDTO{
		ID: "cc-1", Handle: "employee", OUID: "ou-1", Name: "Employee", Description: "Staff badge",
		Format: "dc+sd-jwt", VCT: "var:CREDENTIAL_CONFIGURATION_EMPLOYEE_VCT",
		Claims:          []ClaimMapping{{Name: "email", DisplayName: "Email"}},
		Display:         &CredentialDisplay{Locale: "en", LogoURI: "https://example.com/logo.png"},
		ValiditySeconds: &validity,
	}
	exported, err := yaml.Marshal(dto)
	require.NoError(t, err)

	view, err := newConfigurationExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: credential_configuration\n"+string(exported)))

	require.NoError(t, err)
	assert.Equal(t, toResponse(*dto), view)
}

// A document that is not a credential configuration is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newConfigurationExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "handle: [not, a, handle]"))
	assert.Error(t, err)
}
