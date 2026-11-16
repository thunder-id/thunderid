// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package client

import syshttp "github.com/thunder-id/thunderid/internal/system/http"

// Initialize initializes and returns the client factory interface. Every provider is reached through
// a client or dialer the given provider builds, which refuses private addresses unless the deployment
// allows them.
func Initialize(clients syshttp.OutboundClientProviderInterface) ClientFactoryInterface {
	return newClientFactory(clients)
}
