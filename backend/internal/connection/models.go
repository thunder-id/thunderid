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

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// idpBackedVendor maps a connection path segment to an underlying identity-provider type.
type idpBackedVendor struct {
	name    string
	idpType providers.IDPType
}

// idpBackedVendors is the set of connection types backed by the identity-provider service.
// A single generic "oidc" connection (shown as "Custom OIDC" in the console) covers custom
// providers; a dedicated generic OAuth type can be added later if a non-OIDC provider needs it.
var idpBackedVendors = []idpBackedVendor{
	{name: "google", idpType: providers.IDPTypeGoogle},
	{name: "github", idpType: providers.IDPTypeGitHub},
	{name: "oidc", idpType: providers.IDPTypeOIDC},
}

// connectionTypeSummary is a single entry in the GET /connections listing. It carries only
// the structural data the listing page needs; presentation metadata (logo, display name,
// categories) lives in the frontend.
type connectionTypeSummary struct {
	Type          string `json:"type"`
	Configured    bool   `json:"configured"`
	InstanceCount int    `json:"instanceCount"`
}

// connectionListResponse is the payload for GET /connections.
type connectionListResponse struct {
	Connections []connectionTypeSummary `json:"connections"`
}

// connectionInstanceSummary is a single configured instance returned by
// GET /connections/{type} (the per-type listing). Full configuration is fetched via
// GET /connections/{type}/{id}.
type connectionInstanceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
