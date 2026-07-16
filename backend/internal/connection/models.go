/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package connection exposes the /connections API: a thin HTTP layer in front of the
// existing identity-provider (and, later, notification-sender) services. It owns no
// storage; each request is translated to/from the underlying model and delegated, so a
// configured connection remains a real identity provider.
package connection

import (
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
// relying on userInfoEndpoint instead.
var idpBackedVendors = []idpBackedVendor{
	{name: "google", idpType: providers.IDPTypeGoogle},
	{name: "github", idpType: providers.IDPTypeGitHub},
	{name: "oidc", idpType: providers.IDPTypeOIDC},
	{name: "oauth", idpType: providers.IDPTypeOAuth},
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
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Type        string               `json:"type"`
	Categories  []connectionCategory `json:"categories"`
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
