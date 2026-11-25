// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package http

// Initialize creates the factory of clients and dialers for outbound calls. With
// allowPrivateAddresses they may reach loopback, link-local and private addresses; without it every
// client and dialer it builds refuses them.
func Initialize(allowPrivateAddresses bool) OutboundClientFactoryInterface {
	return newOutboundClientFactory(allowPrivateAddresses)
}
