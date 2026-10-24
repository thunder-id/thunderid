// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/system/config"
)

func viewConnectionDocument(t *testing.T, model connectionExportModel) *yaml.Node {
	t.Helper()
	exported, err := yaml.Marshal(&model)
	require.NoError(t, err)
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("resource_type: connection\n"+string(exported)), &root))
	return root.Content[0]
}

func newViewTestExporter(t *testing.T) *connectionExporter {
	t.Helper()
	initConfigWithTestCryptoKey(t)
	t.Cleanup(config.ResetServerRuntime)
	return newConnectionExporter(nil, nil, nil)
}

// An exported identity-provider connection is shown as its vendor's read returns it, with its
// references as they are and its secret masked.
func TestViewResourceShowsAnIDPConnectionAsItsVendorRead(t *testing.T) {
	exporter := newViewTestExporter(t)

	view, err := exporter.ViewResource(context.Background(), viewConnectionDocument(t, connectionExportModel{
		ID: "conn-1", Type: "oidc", Name: "Corp OIDC",
		ClientID:     "var:CONNECTION_CORP_OIDC_CLIENT_ID",
		ClientSecret: "sec:CONNECTION_CORP_OIDC_CLIENT_SECRET",
		Issuer:       "https://issuer.example", Scopes: []string{"openid", "email"},
	}))

	require.NoError(t, err)
	resp, ok := view.(oidcConnectionResponse)
	require.True(t, ok)
	assert.Equal(t, "conn-1", resp.ID)
	assert.Equal(t, "oidc", resp.Type)
	assert.Equal(t, "var:CONNECTION_CORP_OIDC_CLIENT_ID", resp.ClientID)
	assert.Equal(t, maskedSecretValue, resp.ClientSecret)
	assert.Equal(t, "https://issuer.example", resp.Issuer)
	assert.Equal(t, []string{"openid", "email"}, resp.Scopes)

	view, err = exporter.ViewResource(context.Background(), viewConnectionDocument(t, connectionExportModel{
		ID: "conn-2", Type: "google", Name: "Corp Google", ClientID: "abc",
	}))
	require.NoError(t, err)
	google, ok := view.(googleConnectionResponse)
	require.True(t, ok)
	assert.Equal(t, "google", google.Type)
	assert.Empty(t, google.ClientSecret)
}

// An exported message connection is shown as its vendor's read returns it, secret masked.
func TestViewResourceShowsAnSMSConnectionAsItsVendorRead(t *testing.T) {
	exporter := newViewTestExporter(t)

	view, err := exporter.ViewResource(context.Background(), viewConnectionDocument(t, connectionExportModel{
		ID: "sms-1", Type: "twilio", Name: "Twilio", AccountSID: "var:CONNECTION_TWILIO_ACCOUNT_SID",
		AuthToken: "sec:CONNECTION_TWILIO_AUTH_TOKEN", SenderID: "+100",
	}))

	require.NoError(t, err)
	resp, ok := view.(twilioConnectionResponse)
	require.True(t, ok)
	assert.Equal(t, "sms-1", resp.ID)
	assert.Equal(t, "twilio", resp.Type)
	assert.Equal(t, "var:CONNECTION_TWILIO_ACCOUNT_SID", resp.AccountSID)
	assert.Equal(t, maskedSecretValue, resp.AuthToken)
	assert.Equal(t, "+100", resp.SenderID)

	view, err = exporter.ViewResource(context.Background(), viewConnectionDocument(t, connectionExportModel{
		ID: "sms-2", Type: smsGatewayVendorName, Name: "Gateway", URL: "https://sms.example",
	}))
	require.NoError(t, err)
	gateway, ok := view.(smsGatewayConnectionResponse)
	require.True(t, ok)
	assert.Equal(t, smsGatewayVendorName, gateway.Type)
	assert.Equal(t, "https://sms.example", gateway.URL)
}

// An exported AuthZEN PDP connection is shown as its read returns it.
func TestViewResourceShowsAnAuthZENPDPConnectionAsItsRead(t *testing.T) {
	retries := 2
	view, err := newViewTestExporter(t).ViewResource(context.Background(),
		viewConnectionDocument(t, connectionExportModel{
			ID: "pdp-1", Type: "authzen-pdp", Name: "PDP", AuthZENPDPEndpoint: "var:CONNECTION_PDP_ENDPOINT",
			AuthZENPDPTimeoutMS: 500, AuthZENPDPRetryCount: &retries,
		}))

	require.NoError(t, err)
	resp, ok := view.(authzenpdp.ConnectionResponse)
	require.True(t, ok)
	assert.Equal(t, "pdp-1", resp.ID)
	assert.Equal(t, "authzen-pdp", resp.Type)
	assert.Equal(t, "var:CONNECTION_PDP_ENDPOINT", resp.Endpoint)
	assert.Equal(t, 500, resp.TimeoutMS)
	assert.Equal(t, 2, resp.RetryCount)
}

// A document that is not a connection is refused.
func TestViewResourceRefusesAnUnreadableConnection(t *testing.T) {
	exporter := newViewTestExporter(t)

	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("name: [not, a, name]"), &root))
	_, err := exporter.ViewResource(context.Background(), root.Content[0])
	assert.Error(t, err)

	_, err = exporter.ViewResource(context.Background(),
		viewConnectionDocument(t, connectionExportModel{ID: "conn-9", Type: "unknown", Name: "U"}))
	assert.Error(t, err)
}
