// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package gateway registers the data planes this control plane administers and applies the current
// configuration to them.
package gateway

import "time"

// Gateway is a data plane registered with this control plane.
//
// It is reached over its own API using a machine to machine client the data plane holds, so the
// control plane needs no inbound path to it and the data plane needs none to the control plane.
type Gateway struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// BaseURL is where the data plane answers.
	BaseURL string `json:"baseUrl"`
	// ClientID names the system application on that data plane which the apply is authorized as.
	ClientID string `json:"clientId"`
	// ClientSecret is never returned by the API. It is held encrypted and used only to obtain a
	// token when applying.
	ClientSecret string `json:"-"`
	// Scope is requested with the token, for a data plane that scopes its import API.
	Scope     string    `json:"scope,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// RegisterRequest is the body of a registration.
type RegisterRequest struct {
	Name         string `json:"name"`
	BaseURL      string `json:"baseUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Scope        string `json:"scope,omitempty"`
}
