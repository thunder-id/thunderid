// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package connection exposes the /connections API: a thin HTTP layer in front of the
// existing identity-provider (and, later, notification-sender) services. It owns no
// storage; each request is translated to/from the underlying model and delegated, so a
// configured connection remains a real identity provider.
package connection

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// idpBackedVendor maps a connection path segment to an underlying identity-provider type.
type idpBackedVendor struct {
	name    string
	idpType providers.IDPType
}

// idpBackedVendors is the set of connection types backed by the identity-provider service.
// The generic "oidc" connection covers custom OIDC providers;
// "oauth" covers OAuth 2.0 providers that don't implement OIDC discovery and have no id_token,
// taking user attributes from the provider's own profile API instead.
var idpBackedVendors = []idpBackedVendor{
	{name: "google", idpType: providers.IDPTypeGoogle},
	{name: "github", idpType: providers.IDPTypeGitHub},
	{name: "oidc", idpType: providers.IDPTypeOIDC},
	{name: "oauth", idpType: providers.IDPTypeOAuth},
}

// smsGatewayVendorName is the connection vendor name for the generic HTTP SMS gateway. The
// stored message provider stays MessageProviderTypeCustom; this name is presentation-only,
// surfaced in the /connections/{vendor} path and the flat-list type.
const smsGatewayVendorName = "sms-gateway"

// smsBackedVendor maps a connection path segment to an underlying message provider.
type smsBackedVendor struct {
	name     string
	provider ncommon.MessageProviderType
}

// smsBackedVendors is the set of connection types backed by the notification-sender service.
var smsBackedVendors = []smsBackedVendor{
	{name: "twilio", provider: ncommon.MessageProviderTypeTwilio},
	{name: "vonage", provider: ncommon.MessageProviderTypeVonage},
	{name: smsGatewayVendorName, provider: ncommon.MessageProviderTypeCustom},
}

// connectionCategory is the functional category of a connection instance, used as the
// value of the category query parameter on GET /connections.
type connectionCategory string

const (
	categoryIdentityProvider connectionCategory = "identity-provider"
	categorySMSProvider      connectionCategory = "sms-provider"
)

// parseConnectionCategory validates the raw category query value. Empty means "no filter";
// any other unrecognized value returns false.
func parseConnectionCategory(raw string) (connectionCategory, bool) {
	switch connectionCategory(raw) {
	case "", categoryIdentityProvider, categorySMSProvider:
		return connectionCategory(raw), true
	default:
		return "", false
	}
}

// connectionInstance is a single configured connection instance in the flat GET /connections
// listing, spanning IdP- and sender-backed connections.
type connectionInstance struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	Type         string               `json:"type"`
	Categories   []connectionCategory `json:"categories"`
	IDJagEnabled *bool                `json:"idJagEnabled,omitempty"`
	IsReadOnly   bool                 `json:"isReadOnly"`
}

// connectionListResponse is the paginated payload for GET /connections (the flat instance list).
type connectionListResponse struct {
	TotalResults int                  `json:"totalResults"`
	StartIndex   int                  `json:"startIndex"`
	Count        int                  `json:"count"`
	Connections  []connectionInstance `json:"connections"`
	Links        []sysutils.Link      `json:"links"`
}

// connectionInstanceSummary is a single configured instance returned by
// GET /connections/{type} (the per-type listing). Full configuration is fetched via
// GET /connections/{type}/{id}.
type connectionInstanceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
