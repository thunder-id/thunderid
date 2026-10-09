// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSenderToConnectionBodyMapsLegacyHeaders(t *testing.T) {
	body, err := senderToConnectionBody(NotificationSender{
		Name: "SMS gateway", Provider: "custom",
		Properties: []SenderProperty{{
			Name: "http_headers", Value: `[{"name":"X-API-Key","value":"secret"},` +
				`{"name":"X-Tenant","value":"tenant-1"}]`,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"type": "api_key",
		"properties": map[string]string{
			"X-API-Key": "secret",
			"X-Tenant":  "tenant-1",
		},
	}, body["authentication"])
	require.NotContains(t, body, "apiKeyHeaders")
}

func TestSenderToConnectionBodyRejectsInvalidHeaders(t *testing.T) {
	_, err := senderToConnectionBody(NotificationSender{
		Properties: []SenderProperty{{Name: "http_headers", Value: "not JSON"}},
	})
	require.ErrorContains(t, err, "failed to decode API key headers")
}
