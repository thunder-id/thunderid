// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package connection exposes the /connections API: a thin HTTP layer in front of the
// existing identity-provider (and, later, notification-sender) services. It owns no
// storage; each request is translated to/from the underlying model and delegated, so a
// configured connection remains a real identity provider.
package connection

import (
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
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
// stored message provider stays NotificationProviderTypeCustom; this name is presentation-only,
// surfaced in the /connections/{vendor} path and the flat-list type.
const smsGatewayVendorName = "sms-gateway"

// smsBackedVendor maps a connection path segment to an underlying message provider. authTypes
// is the set of outbound authentication methods the vendor accepts, served by the metadata
// endpoint; an empty set means the vendor has no configurable choice to offer.
type smsBackedVendor struct {
	name      string
	provider  ncommon.NotificationProviderType
	authTypes []outboundauth.Type
}

// smsBackedVendors is the set of connection types backed by the notification-sender service.
// None of them advertise authentication methods yet: Twilio and Vonage take a fixed vendor
// credential contract, and the HTTP gateway carries its credentials in httpHeaders.
var smsBackedVendors = []smsBackedVendor{
	{name: "twilio", provider: ncommon.NotificationProviderTypeTwilio},
	{name: "vonage", provider: ncommon.NotificationProviderTypeVonage},
	{name: smsGatewayVendorName, provider: ncommon.NotificationProviderTypeCustom},
}

// emailSMTPVendorName is the connection vendor name for email delivered over SMTP. The channel
// is part of the name because the protocol alone does not identify the contract: a carrier
// email-to-SMS gateway would speak the same SMTP to a different channel. The stored email
// provider stays NotificationProviderTypeSMTP; this name is presentation-only, surfaced in the
// /connections/{vendor} path and the flat-list type.
const emailSMTPVendorName = "email-smtp"

// emailBackedVendor maps a connection path segment to an underlying email provider.
// credentialTargetKeys names the transport properties that identify where an outbound
// authentication credential is presented; changing any of them on update requires the
// credential to be supplied again rather than carried over.
type emailBackedVendor struct {
	name                 string
	provider             ncommon.NotificationProviderType
	authTypes            []outboundauth.Type
	credentialTargetKeys []string
}

// emailBackedVendors is the set of email connection types backed by the notification-sender
// service.
var emailBackedVendors = []emailBackedVendor{
	{name: emailSMTPVendorName, provider: ncommon.NotificationProviderTypeSMTP,
		authTypes:            ncommon.SMTPSupportedAuthTypes,
		credentialTargetKeys: []string{ncommon.SMTPPropKeyHost, ncommon.SMTPPropKeyPort}},
}

// connectionCategory is the functional category of a connection instance, used as the
// value of the category query parameter on GET /connections.
type connectionCategory string

const (
	categoryIdentityProvider connectionCategory = "identity-provider"
	categorySMSProvider      connectionCategory = "sms-provider"
	categoryAuthorizationPDP connectionCategory = "authorization-pdp"
	categoryEmailProvider    connectionCategory = "email-provider"
)

// parseConnectionCategory validates the raw category query value. Empty means "no filter";
// any other unrecognized value returns false.
func parseConnectionCategory(raw string) (connectionCategory, bool) {
	switch connectionCategory(raw) {
	case "", categoryIdentityProvider, categorySMSProvider, categoryAuthorizationPDP, categoryEmailProvider:
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
