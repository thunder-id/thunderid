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

// Package integration serves the read-only catalog of integrations (identity
// providers, notification senders, ...) that ThunderID can connect to. Domain
// packages describe their available integrations as Descriptor values; this package
// aggregates and exposes them so the console can render the integrations catalog and
// configuration forms from metadata rather than hardcoded UI.
package integration

// Integration categories used to group integrations in the catalog.
const (
	CategorySocialLogin = "SOCIAL_LOGIN"
	CategoryEnterprise  = "ENTERPRISE"
	CategorySMS         = "SMS"
)

// Field describes a single configurable property of an integration.
type Field struct {
	// Name is the property key as stored on the integration instance (e.g. "client_id").
	Name string `json:"name"`
	// Required indicates the property must be provided for the integration to work.
	Required bool `json:"required"`
	// Secret indicates the value is sensitive: masked on read, write-only on write.
	Secret bool `json:"secret,omitempty"`
	// ReadOnly indicates the value is managed by ThunderID and cannot be edited.
	ReadOnly bool `json:"readOnly,omitempty"`
	// Default is the value pre-filled when the integration is configured.
	Default string `json:"default,omitempty"`
}

// Descriptor describes an available integration and its configuration shape.
type Descriptor struct {
	// Type is the stable integration type identifier (e.g. "GOOGLE", "twilio").
	Type string `json:"type"`
	// DisplayName is the default human-readable name (the console may localize it).
	DisplayName string `json:"displayName"`
	// Category groups the integration in the catalog (see Category* constants).
	Category string `json:"category"`
	// HostedCredentials reports whether ThunderID ships default test credentials for
	// this integration (hosted/cloud deployments). False for self-hosted.
	HostedCredentials bool `json:"hostedCredentials"`
	// Fields lists the configurable properties of the integration.
	Fields []Field `json:"fields"`
}

// ListResponse is the payload returned by GET /integrations.
type ListResponse struct {
	Integrations []Descriptor `json:"integrations"`
}
