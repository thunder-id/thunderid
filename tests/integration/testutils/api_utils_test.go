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
		Properties: []SenderProperty{{Name: "http_headers", Value: "X-API-Key: secret, X-Tenant: tenant-1"}},
	})
	require.NoError(t, err)
	require.Equal(t, "X-API-Key: secret, X-Tenant: tenant-1", body["httpHeaders"])
	require.NotContains(t, body, "authentication")
}
