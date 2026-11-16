// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

// Initialize creates the provider of clients and dialers for destinations an administrator, a client
// or a gateway registration configures. With allowPrivateAddresses they may reach loopback, link-local
// and private addresses; without it every client and dialer it builds refuses them.
func Initialize(allowPrivateAddresses bool) OutboundClientProviderInterface {
	return newOutboundClientProvider(allowPrivateAddresses)
}
