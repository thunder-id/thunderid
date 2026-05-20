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

// IDPDTO represents the data transfer object for an identity provider.
type IDPDTO struct {
	ID          string             `yaml:"id"`
	Name        string             `yaml:"name"`
	Description string             `yaml:"description,omitempty"`
	Type        IDPType            `yaml:"type"`
	Properties  []cmodels.Property `yaml:"properties,omitempty"`
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
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Type        string                `json:"type"`
	Properties  []cmodels.PropertyDTO `json:"properties,omitempty"`
}

// idpResponse represents the response payload for an identity provider.
type idpResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Type        string                `json:"type"`
	Properties  []cmodels.PropertyDTO `json:"properties,omitempty"`
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
	ID          string                `yaml:"id"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Type        string                `yaml:"type"`
	Properties  []cmodels.PropertyDTO `yaml:"properties,omitempty"`
}
