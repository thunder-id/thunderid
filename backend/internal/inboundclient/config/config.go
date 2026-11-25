// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package inboundclientconfig holds the configuration the inbound client service reads.
package inboundclientconfig

import "github.com/thunder-id/thunderid/internal/system/config"

// Config is the configuration of the inbound client service.
type Config struct {
	// AllowPrivateAddresses is server.allow_private_addresses, which decides whether a back-channel
	// logout URI may name a private host.
	AllowPrivateAddresses bool
}

// FromServerRuntime reads the configuration from the server runtime.
func FromServerRuntime() Config {
	return Config{AllowPrivateAddresses: config.GetServerRuntime().Config.Server.AllowPrivateAddresses}
}
