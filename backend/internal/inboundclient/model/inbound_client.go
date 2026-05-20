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

// Package model holds public data types for the inbound client subsystem.
//
//nolint:lll
package model

import (
	"github.com/thunder-id/thunderid/internal/cert"
)

// InboundClient is the persistence shape for protocol-agnostic inbound client record.
type InboundClient struct {
	ID                        string
	AuthFlowID                string
	RegistrationFlowID        string
	IsRegistrationFlowEnabled bool
	RecoveryFlowID            string
	IsRecoveryFlowEnabled     bool
	ThemeID                   string
	LayoutID                  string
	Assertion                 *AssertionConfig
	LoginConsent              *LoginConsentConfig
	AllowedUserTypes          []string
	Properties                map[string]interface{}
	IsReadOnly                bool
}

// InboundAuthProfile is the wire field block embedded in entity DTOs (requests and responses).
type InboundAuthProfile struct {
	AuthFlowID                string              `json:"authFlowId,omitempty"           yaml:"auth_flow_id,omitempty"           jsonschema:"Authentication flow ID. Optional. Specifies which login flow to use (e.g., MFA, passwordless). If omitted, the default authentication flow is used."`
	AuthFlowHandle            string              `json:"authFlowHandle,omitempty"       yaml:"auth_flow_handle,omitempty"       jsonschema:"Authentication flow handle. Optional. Alternative to authFlowId — resolved to an ID at import time."`
	RegistrationFlowID        string              `json:"registrationFlowId,omitempty"   yaml:"registration_flow_id,omitempty"   jsonschema:"Registration flow ID. Optional. Specifies the user registration/signup flow."`
	RegistrationFlowHandle    string              `json:"registrationFlowHandle,omitempty" yaml:"registration_flow_handle,omitempty" jsonschema:"Registration flow handle. Optional. Alternative to registrationFlowId — resolved to an ID at import time."`
	IsRegistrationFlowEnabled bool                `json:"isRegistrationFlowEnabled"      yaml:"is_registration_flow_enabled"     jsonschema:"Enable self-service registration. Set to true to allow users to sign up themselves. Requires registrationFlowId or registrationFlowHandle to be set."`
	RecoveryFlowID            string              `json:"recoveryFlowId,omitempty"        yaml:"recovery_flow_id,omitempty"        jsonschema:"Recovery flow ID. Optional. Specifies the user recovery flow."`
	RecoveryFlowHandle        string              `json:"recoveryFlowHandle,omitempty"   yaml:"recovery_flow_handle,omitempty"   jsonschema:"Recovery flow handle. Optional. Alternative to recoveryFlowId — resolved to an ID at import time."`
	IsRecoveryFlowEnabled     bool                `json:"isRecoveryFlowEnabled"          yaml:"is_recovery_flow_enabled"       jsonschema:"Enable self-service recovery. Set to true to allow users to recover their accounts (e.g., password reset). Requires recoveryFlowId or recoveryFlowHandle to be set."`
	ThemeID                   string              `json:"themeId,omitempty"              yaml:"theme_id,omitempty"               jsonschema:"Theme configuration ID. Optional. Customizes the visual styling of login pages."`
	LayoutID                  string              `json:"layoutId,omitempty"             yaml:"layout_id,omitempty"              jsonschema:"Layout configuration ID. Optional. Customizes the screen structure and component positioning of login pages."`
	Assertion                 *AssertionConfig    `json:"assertion,omitempty"            yaml:"assertion,omitempty"              jsonschema:"Assertion configuration. Optional. Customize assertion validity periods and included user attributes."`
	LoginConsent              *LoginConsentConfig `json:"loginConsent,omitempty"         yaml:"login_consent,omitempty"          jsonschema:"Login consent configuration settings."`
	AllowedUserTypes          []string            `json:"allowedUserTypes,omitempty"     yaml:"allowed_user_types,omitempty"     jsonschema:"Allowed user types. Optional. Restricts which user types can authenticate to and register against this resource."`
	Certificate               *Certificate        `json:"certificate,omitempty"          yaml:"certificate,omitempty"            jsonschema:"Resource-level certificate. Optional. For certificate-based authentication or JWT validation."`
}

// AssertionConfig is the entity-level assertion config; token configs fall back to it.
type AssertionConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty" yaml:"validity_period,omitempty" jsonschema:"Assertion validity period in seconds."`
	UserAttributes []string `json:"userAttributes,omitempty" yaml:"user_attributes,omitempty" jsonschema:"User attributes to include in the assertion."`
}

// LoginConsentConfig is the login consent configuration.
type LoginConsentConfig struct {
	ValidityPeriod int64 `json:"validityPeriod" yaml:"validity_period" jsonschema:"Consent validity period in seconds. 0 means never expire."`
}

// Certificate is a user-supplied certificate input.
type Certificate struct {
	Type  cert.CertificateType `json:"type,omitempty"  yaml:"type,omitempty"  jsonschema:"Certificate type (PEM, JWK, etc.)."`
	Value string               `json:"value,omitempty" yaml:"value,omitempty" jsonschema:"Certificate value in the format specified by type."`
}

// DeclarativeLoaderConfig describes how to load inbound clients from a YAML resource directory.
type DeclarativeLoaderConfig struct {
	ResourceType  string
	DirectoryName string
	Parser        func(data []byte) (*InboundClient, error)
	Validator     func(*InboundClient) error
}
