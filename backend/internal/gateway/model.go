// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package gateway registers the data planes this control plane administers and applies the current
// configuration to them.
package gateway

import "time"

// Gateway is a data plane registered with this control plane.
//
// It is reached over its own API using the management token that data plane is configured with, so
// the control plane needs no inbound path to it and the data plane needs none to the control plane.
type Gateway struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// DataPlaneID is the identifier the data plane knows itself by: its server.identifier, and the
	// value it scopes every stored resource under. It is what makes two registrations the same data
	// plane, so it is unique within this deployment.
	//
	// Identity rather than address. A data plane moves between hostnames, ports and ingresses, and
	// keying on BaseURL instead would let one be registered twice under its internal and external
	// names with nothing noticing.
	//
	// The control plane takes this on trust at registration; nothing here can confirm it without
	// asking the data plane. A wrong value is found when configuration is applied, in the same way a
	// wrong key or an unreachable URL is.
	DataPlaneID string `json:"dataPlaneId"`
	// BaseURL is where the data plane answers.
	BaseURL string `json:"baseUrl"`
	// Key is the data plane's management token. It is never returned by the API: it is held
	// encrypted, and presented as a bearer token by whatever calls that data plane.
	//
	// Whoever holds it can import configuration into that data plane and read and write its variable
	// store, so a read that returned it would hand every operator of this control plane the keys to
	// every data plane it administers.
	Key string `json:"-"`
	// CACertificate is a PEM certificate to trust when calling this data plane, in addition to the
	// system roots. It is for a data plane serving a certificate no public authority signed, which is
	// what a local or on-premise deployment usually has.
	//
	// Empty is the common case and the right default: a data plane behind an ingress with a
	// publicly-issued certificate verifies against the system roots with nothing configured here.
	// Naming the one certificate keeps verification on for that gateway, rather than a switch that
	// turns it off.
	CACertificate string    `json:"caCertificate,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// KeyConfigured reports whether a key is held, which is all a read of one discloses.
func (g *Gateway) KeyConfigured() bool { return g.Key != "" }

// RegisterRequest is the body of a registration.
type RegisterRequest struct {
	Name        string `json:"name"`
	DataPlaneID string `json:"dataPlaneId"`
	BaseURL     string `json:"baseUrl"`
	// Key is the management token configured on that data plane, as
	// server.security.management_token.
	Key           string `json:"key"`
	CACertificate string `json:"caCertificate,omitempty"`
}
