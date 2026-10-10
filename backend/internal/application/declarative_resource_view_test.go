// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func viewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

// An exported application is shown as a read returns it, with its references as they are.
func TestViewResourceShowsAnExportedApplicationAsItsRead(t *testing.T) {
	exported, err := yaml.Marshal(&providers.Application{
		ID: "app-1", Name: "Orders", OUID: "ou-1",
		InboundAuthProfile: providers.InboundAuthProfile{AuthFlowID: "flow-1"},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:     "var:APPLICATION_ORDERS_CLIENT_ID",
				RedirectURIs: []string{"var:APPLICATION_ORDERS_REDIRECT_URIS"},
			},
		}},
	})
	require.NoError(t, err)

	view, err := newApplicationExporter(nil).ViewResource(context.Background(),
		viewDocument(t, "resource_type: application\n"+string(exported)))

	require.NoError(t, err)
	app, ok := view.(*model.ApplicationGetResponse)
	require.True(t, ok)
	assert.Equal(t, "app-1", app.ID)
	assert.Equal(t, "Orders", app.Name)
	assert.Equal(t, "flow-1", app.InboundAuthProfileReq.AuthFlowID)
	assert.Equal(t, "var:APPLICATION_ORDERS_CLIENT_ID", app.ClientID)
	require.Len(t, app.InboundAuthConfig, 1)
	assert.Equal(t, []string{"var:APPLICATION_ORDERS_REDIRECT_URIS"}, app.InboundAuthConfig[0].OAuthConfig.RedirectURIs)
	assert.Equal(t, []providers.GrantType{}, app.InboundAuthConfig[0].OAuthConfig.GrantTypes)
}

// A document that is not an application is refused.
func TestViewResourceRefusesAnUnreadableDocument(t *testing.T) {
	exporter := newApplicationExporter(nil)

	_, err := exporter.ViewResource(context.Background(), viewDocument(t, "name: [not, a, name]"))
	assert.Error(t, err)

	_, err = exporter.ViewResource(context.Background(), viewDocument(t,
		"name: Orders\ninboundAuthConfig:\n  - type: saml"))
	assert.Error(t, err)
}
