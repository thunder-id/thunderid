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

// Package common provides common data models and error types shared across authnprovider sub-packages.
package common

// AuthnMetadata contains metadata for authentication.
type AuthnMetadata struct {
	AppMetadata map[string]interface{} `json:"appMetadata,omitempty"`
}

// AuthenticatedClaims holds claims produced by an authentication mechanism.
type AuthenticatedClaims map[string]interface{}

// AuthnResult represents the result of an authentication attempt.
type AuthnResult struct {
	AuthenticatedClaims AuthenticatedClaims `json:"authenticatedClaims,omitempty"`
	// EntityReferenceToken can be nil, iff entity reference is included
	EntityReferenceToken any              `json:"entityReferenceToken"`
	EntityReference      *EntityReference `json:"entityReference,omitempty"`
	// AttributeToken can be nil, iff attribute values are included
	AttributeToken any                 `json:"attributeToken"`
	Attributes     *AttributesResponse `json:"attributes,omitempty"`
}

// AssuranceMetadataResponse contains assurance metadata for an attribute.
type AssuranceMetadataResponse struct {
	IsVerified bool `json:"isVerified"`
	// this should be the key of the corresponding verification response in the verifications map
	VerificationID string `json:"verificationId,omitempty"`
}

// VerificationResponse contains verification details for an attribute.
type VerificationResponse struct {
	TrustFramework      string `json:"trustFramework,omitempty"`
	Time                string `json:"time,omitempty"`
	VerificationProcess string `json:"verificationProcess,omitempty"`
}

// RequestedAttributes contains the requested attributes and verifications.
type RequestedAttributes struct {
	Attributes    map[string]*AttributeMetadataRequest `json:"attributes,omitempty"`
	Verifications map[string]*VerificationRequest      `json:"verifications,omitempty"`
}

// AttributeMetadataRequest contains metadata request details for an attribute.
type AttributeMetadataRequest struct {
	GenericMetadataRequest   *GenericMetadataRequest   `json:"genericMetadataRequest,omitempty"`
	AssuranceMetadataRequest *AssuranceMetadataRequest `json:"assuranceMetadataRequest,omitempty"`
}

// GenericMetadataRequest contains generic metadata request details.
type GenericMetadataRequest struct {
	Essential bool     `json:"essential,omitempty"`
	Value     string   `json:"value,omitempty"`
	Values    []string `json:"values,omitempty"`
}

// GenericTimeMetadataRequest extends GenericMetadataRequest with time-related metadata.
type GenericTimeMetadataRequest struct {
	GenericMetadataRequest
	MaxAge *int `json:"maxAge,omitempty"`
}

// AssuranceMetadataRequest contains assurance metadata request details.
type AssuranceMetadataRequest struct {
	ShouldVerify bool `json:"shouldVerify,omitempty"`
	// this should be the key of the corresponding verification request in the verifications map
	VerificationID string `json:"verificationId,omitempty"`
}

// VerificationRequest contains verification request details.
type VerificationRequest struct {
	TrustFramework      *GenericMetadataRequest     `json:"trustFramework,omitempty"`
	VerificationProcess *GenericMetadataRequest     `json:"verificationProcess,omitempty"`
	Time                *GenericTimeMetadataRequest `json:"time,omitempty"`
}

// AttributesResponse contains the response with attributes and verifications.
type AttributesResponse struct {
	Attributes    map[string]*AttributeResponse    `json:"attributes,omitempty"`
	Verifications map[string]*VerificationResponse `json:"verifications,omitempty"`
}

// AttributeResponse contains the response for an attribute with its value and assurance metadata.
type AttributeResponse struct {
	Value                     interface{}                `json:"value,omitempty"`
	AssuranceMetadataResponse *AssuranceMetadataResponse `json:"assuranceMetadataResponse,omitempty"`
}

// EntityReference contains the reference to an entity.
type EntityReference struct {
	EntityID       string `json:"entityId"`
	EntityCategory string `json:"entityCategory"`
	EntityType     string `json:"entityType"`
	OUID           string `json:"ouId"`
}

// GetAttributesMetadata holds metadata used when retrieving entity attributes.
type GetAttributesMetadata struct {
	AppMetadata map[string]interface{} `json:"appMetadata,omitempty"`
	Locale      string                 `json:"locale"`
}
