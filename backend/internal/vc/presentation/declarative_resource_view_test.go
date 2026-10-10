// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

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

// An exported presentation definition is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedDefinitionAsItsRead(t *testing.T) {
	enforce := true
	dto := &PresentationDefinitionDTO{
		ID: "pd-1", Handle: "employee-check", OUID: "ou-1", Name: "Employee check",
		VCT: "var:PRESENTATION_DEFINITION_EMPLOYEE_CHECK_VCT", Format: DefaultCredentialFormat,
		RequestedClaims: []string{"email"}, MandatoryClaims: []string{"email"}, OptionalClaims: []string{"name"},
		ClaimValues:          map[string][]string{"department": {"engineering"}},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   []string{"corp-ca"},
	}
	exported, err := yaml.Marshal(dto)
	require.NoError(t, err)

	view, err := newDefinitionExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: presentation_definition\n"+string(exported)))

	require.NoError(t, err)
	assert.Equal(t, toResponse(*dto), view)
}

// A document that is not a presentation definition is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newDefinitionExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "handle: [not, a, handle]"))
	assert.Error(t, err)
}
