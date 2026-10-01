// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package gateway registers the gateways this control plane administers and applies the current
// configuration to them.
package gateway

import "time"

// Gateway is one deployment this control plane administers.
//
// It is reached over its own API using the management token it is configured with, so the control
// plane needs no inbound path to it and it needs none to the control plane.
type Gateway struct {
	// ID identifies this registration, and is generated here when one is made. It is what this
	// API's routes address, and it means nothing to the gateway itself.
	ID string `json:"id"`
	// Name is what an operator calls this gateway. It is also how a declared gateway is matched, so
	// re-reading the files updates the registration rather than making a second one.
	Name string `json:"name"`
	// BaseURL is where the gateway answers, and it is what identifies one: a gateway registers
	// once, so this is unique within the deployment.
	//
	// One reachable by more than one name can therefore be registered more than once, under each of
	// them. Nothing here can tell that those are the same deployment, so register the name the
	// control plane should call it by and leave the others alone.
	BaseURL string `json:"baseUrl"`
	// Key is the token this control plane presents to the gateway. It is generated here when the
	// gateway is registered, held encrypted, and returned only by the call that generated it.
	//
	// Whoever holds it can import configuration into the gateway and read and write its variable
	// store, so a read that returned it would hand every operator of this control plane the keys to
	// every gateway it administers.
	Key string `json:"-"`
	// CACertificate is a PEM certificate to trust when calling this gateway, in addition to the
	// system roots. It is for a gateway serving a certificate no public authority signed, which is
	// what a local or on-premise deployment usually has.
	//
	// Empty is the common case and the right default: a gateway behind an ingress with a
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
	Name          string `json:"name"`
	BaseURL       string `json:"baseUrl"`
	CACertificate string `json:"caCertificate,omitempty"`
	// Key is the token this control plane will present to that gateway.
	//
	// Optional. A caller that already holds one, because the gateway was configured with it
	// first, supplies it here. A caller that does not leaves it out and the control plane generates
	// one, which is the usual way: a generated key is not typed, mailed or committed on the way in.
	//
	// Either way the registration returns it, and that is the only time it is readable.
	Key string `json:"key,omitempty"`
}

// UpdateRequest changes a registration. Every field is optional: what is omitted is left as it is,
// so a caller can move a gateway's address without restating its certificate.
//
// BaseURL is updatable, but note that it is also what identifies a gateway: changing it is how a
// gateway that moved is followed, and it is refused if another gateway already answers there.
//
// Key follows the same rule as the rest, which is what makes omitting it safe: an edit that did not
// carry one leaves the stored token alone, so an edit never changes a credential a caller did not
// ask it to. Carrying one is how a credential is rotated.
type UpdateRequest struct {
	Name          *string `json:"name,omitempty"`
	BaseURL       *string `json:"baseUrl,omitempty"`
	CACertificate *string `json:"caCertificate,omitempty"`
	Key           *string `json:"key,omitempty"`
}

// Registration is what a registration returns: the gateway, and the key once.
//
// The key appears here and nowhere else. It is stored encrypted and never read back, so a
// registration that generated one gives the only copy there will be. Lose it and the way forward is
// to set a new one with an edit and give the same value to the gateway, not to look this up.
type Registration struct {
	Gateway
	Key string `json:"key"`
}
