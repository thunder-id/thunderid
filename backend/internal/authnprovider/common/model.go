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

import "github.com/asgardeo/thunder/internal/idp"

// AuthnMetadata contains metadata for authentication.
type AuthnMetadata struct {
	AppMetadata map[string]interface{} `json:"appMetadata,omitempty"`
}

// AuthnResult represents the result of an authentication attempt.
type AuthnResult struct {
	// Entity-generic fields.
	EntityID       string `json:"entityId"`
	EntityCategory string `json:"entityCategory"`
	EntityType     string `json:"entityType"`
	OUID           string `json:"ouId"`

	// TODO: Remove after refacoring usages
	UserID   string `json:"userId"`
	UserType string `json:"userType"`

	Token                     string              `json:"token"`
	IsAttributeValuesIncluded bool                `json:"isAttributeValuesIncluded"`
	AttributesResponse        *AttributesResponse `json:"attributesResponse,omitempty"`

	// Federated authentication fields. Set when the authentication flow is federated
	// and no internal user was found (IsExistingUser = false).
	ExternalSub     string                 `json:"externalSub,omitempty"`
	ExternalClaims  map[string]interface{} `json:"externalClaims,omitempty"`
	IsExistingUser  bool                   `json:"isExistingUser"`
	IsAmbiguousUser bool                   `json:"isAmbiguousUser"`

	AuthType string `json:"authType,omitempty"`
}

// GetAttributesMetadata contains metadata for fetching attributes.
type GetAttributesMetadata struct {
	AppMetadata map[string]interface{} `json:"appMetadata,omitempty"`
	Locale      string                 `json:"locale"`
}

// GetAttributesResult represents the result of fetching attributes.
type GetAttributesResult struct {
	// Entity-generic fields.
	EntityID       string `json:"entityId"`
	EntityCategory string `json:"entityCategory"`
	EntityType     string `json:"entityType"`
	OUID           string `json:"ouId"`

	// TODO: Remove after refacoring usages
	UserID   string `json:"userId"`
	UserType string `json:"userType"`

	AttributesResponse *AttributesResponse `json:"attributeResponse,omitempty"`
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

// CredentialsAuthnData carries the identifiers and credentials for credential-based authentication.
type CredentialsAuthnData struct {
	Identifiers map[string]interface{}
	Credentials map[string]interface{}
}

// PasskeyAuthnData carries the necessary data for passkey-based authentication.
type PasskeyAuthnData struct {
	CredentialID      string
	CredentialType    string
	ClientDataJSON    string
	AuthenticatorData string
	Signature         string
	UserHandle        string
	SessionToken      string
}

// OTPAuthnData carries the necessary data for OTP-based authentication.
type OTPAuthnData struct {
	SessionToken string
	OTP          string
}

// FederatedAuthnData carries the credential data for federated authentication.
type FederatedAuthnData struct {
	IDPID   string
	IDPType idp.IDPType
	OAuthCredential
}

// OAuthCredential carries the credential data for OAuth-based authentication.
type OAuthCredential struct {
	Code string
}

// AuthenticationFactor represents the type of authentication factor.
type AuthenticationFactor string

// AuthenticatorMeta represents an authenticator's metadata including authentication factors.
type AuthenticatorMeta struct {
	// Name is the unique identifier for the authenticator (used in individual authentication APIs)
	Name string
	// Factors represents the authentication factors this authenticator validates
	Factors []AuthenticationFactor
}
