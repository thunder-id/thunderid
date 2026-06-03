/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package idp

import "github.com/thunder-id/thunderid/internal/system/cmodels"

// AttributeMapping defines how a single external IDP attribute maps to a local user attribute.
// ExternalAttribute is the source attribute name (may be a dot-notation path into a nested claim);
// LocalAttribute is the target user-type attribute.
type AttributeMapping struct {
	ExternalAttribute string `json:"externalAttribute" yaml:"external_attribute"`
	LocalAttribute    string `json:"localAttribute" yaml:"local_attribute"`
}

// UserTypeResolution resolves which local user type an incoming identity maps to. This iteration
// supports only Default (a fixed user type); claim-driven resolution is added later as additional
// fields without a breaking change.
type UserTypeResolution struct {
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

// UserTypeAttributeMapping holds the external-to-local attribute mappings for a single local user type.
type UserTypeAttributeMapping struct {
	UserType   string             `json:"userType,omitempty" yaml:"user_type,omitempty"`
	Attributes []AttributeMapping `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}

// AttributeConfiguration holds the user-type resolution and per-user-type attribute mappings for an
// identity provider.
type AttributeConfiguration struct {
	UserTypeResolution        *UserTypeResolution        `json:"userTypeResolution,omitempty" yaml:"user_type_resolution,omitempty"`                //nolint:lll
	UserTypeAttributeMappings []UserTypeAttributeMapping `json:"userTypeAttributeMappings,omitempty" yaml:"user_type_attribute_mappings,omitempty"` //nolint:lll
}

// IDPDTO represents the data transfer object for an identity provider.
type IDPDTO struct {
	ID                     string                  `yaml:"id"`
	Name                   string                  `yaml:"name"`
	Description            string                  `yaml:"description,omitempty"`
	Type                   IDPType                 `yaml:"type"`
	Properties             []cmodels.Property      `yaml:"properties,omitempty"`
	AttributeConfiguration *AttributeConfiguration `yaml:"attribute_configuration,omitempty"`
}

// BasicIDPDTO represents a basic data transfer object for an identity provider.
type BasicIDPDTO struct {
	ID          string
	Name        string
	Description string
	Type        IDPType
	IsReadOnly  bool
}

// idpRequest represents the request payload for creating or updating an identity provider.
type idpRequest struct {
	Name                   string                  `json:"name"`
	Description            string                  `json:"description,omitempty"`
	Type                   string                  `json:"type"`
	Properties             []cmodels.PropertyDTO   `json:"properties,omitempty"`
	AttributeConfiguration *AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// idpResponse represents the response payload for an identity provider.
type idpResponse struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description,omitempty"`
	Type                   string                  `json:"type"`
	Properties             []cmodels.PropertyDTO   `json:"properties,omitempty"`
	AttributeConfiguration *AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// basicIDPResponse represents a basic response payload for an identity provider.
type basicIDPResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// idpRequestWithID represents the request payload for creating an identity provider from file-based config.
type idpRequestWithID struct {
	ID                     string                  `yaml:"id"`
	Name                   string                  `yaml:"name"`
	Description            string                  `yaml:"description,omitempty"`
	Type                   string                  `yaml:"type"`
	Properties             []cmodels.PropertyDTO   `yaml:"properties,omitempty"`
	AttributeConfiguration *AttributeConfiguration `yaml:"attribute_configuration,omitempty"`
}
