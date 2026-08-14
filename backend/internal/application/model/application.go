// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package model defines the data structures for the application module.
//
//nolint:lll
package model

import (
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ApplicationDTO represents the data transfer object for application service operations.
type ApplicationDTO struct {
	ID          string          `json:"id,omitempty" jsonschema:"Application ID. Auto-generated unique identifier."`
	OUID        string          `json:"ouId,omitempty" jsonschema:"Organization unit ID. The OU this application belongs to."`
	OUHandle    string          `json:"ouHandle,omitempty" jsonschema:"Organization unit handle. Resolved to an ID by the service layer."`
	Name        string          `json:"name" jsonschema:"Application name."`
	Description string          `json:"description,omitempty" jsonschema:"Optional description of the application's purpose or functionality."`
	Type        ApplicationType `json:"type,omitempty" jsonschema:"Application type. Canonical platform/client class: browser, fullstack, mobile, m2m, mcp, or custom. Required at creation and immutable."`
	Template    string          `json:"template,omitempty" jsonschema:"Application template. Optional. Display metadata identifying the frontend template used to create the application."`
	FlowSecret  string          `json:"flowSecret,omitempty" jsonschema:"Flow Secret used to authenticate when initiating a flow directly via the Flow Execution API. Issued once on creation only to full-stack, custom, and mcp applications that either have no OAuth 2.0 configuration (embedded) or are configured as a confidential, non-redirect OAuth 2.0 client. Browser, mobile, and m2m applications are never issued one."`

	URL       string   `json:"url,omitempty" jsonschema:"Application home URL. Optional. The main URL where your application is hosted."`
	LogoURL   string   `json:"logoUrl,omitempty" jsonschema:"Logo image URL. Optional. Displayed in login pages and application listings."`
	TosURI    string   `json:"tosUri,omitempty" jsonschema:"Terms of Service URI. Optional. Link to your application's terms of service."`
	PolicyURI string   `json:"policyUri,omitempty" jsonschema:"Privacy Policy URI. Optional. Link to your application's privacy policy."`
	Contacts  []string `json:"contacts,omitempty" jsonschema:"Contact email addresses. Optional. Administrative contact emails for this application."`

	providers.InboundAuthProfile
	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" jsonschema:"OAuth/OIDC authentication configuration. Required for OAuth-enabled applications. Configure OAuth grant types, redirect URIs, and client authentication methods."`
	Metadata          map[string]interface{}                  `json:"metadata,omitempty" jsonschema:"Generic metadata. Optional arbitrary key-value pairs for consumer use."`
}

// BasicApplicationDTO represents a simplified data transfer object for application service operations.
type BasicApplicationDTO struct {
	ID                        string
	Name                      string
	Description               string
	AuthFlowID                string
	RegistrationFlowID        string
	IsRegistrationFlowEnabled bool
	RecoveryFlowID            string
	IsRecoveryFlowEnabled     bool
	SignOutFlowID             string
	ThemeID                   string
	LayoutID                  string
	Type                      ApplicationType
	Template                  string
	ClientID                  string
	LogoURL                   string
	IsReadOnly                bool
}

// ApplicationProcessedDTO represents the processed data transfer object for application service operations.
type ApplicationProcessedDTO struct {
	ID          string          `yaml:"id,omitempty"`
	OUID        string          `yaml:"ouId,omitempty"`
	Name        string          `yaml:"name,omitempty"`
	Description string          `yaml:"description,omitempty"`
	Type        ApplicationType `yaml:"type,omitempty"`
	Template    string          `yaml:"template,omitempty"`

	URL       string `yaml:"url,omitempty"`
	LogoURL   string `yaml:"logoUrl,omitempty"`
	TosURI    string `yaml:"tosUri,omitempty"`
	PolicyURI string `yaml:"policyUri,omitempty"`
	Contacts  []string

	providers.InboundAuthProfile `yaml:",inline"`
	InboundAuthConfig            []inboundmodel.InboundAuthConfigProcessed `yaml:"inboundAuthConfig,omitempty"`
	Metadata                     map[string]interface{}                    `yaml:"metadata,omitempty"`
}

// ApplicationRequest represents the request structure for creating or updating an application.
type ApplicationRequest struct {
	OUID        string          `json:"ouId,omitempty" yaml:"ouId,omitempty"`
	Name        string          `json:"name" yaml:"name" native:"required,min=1,max=100"`
	Description string          `json:"description" yaml:"description"`
	Type        ApplicationType `json:"type,omitempty" yaml:"type,omitempty"`
	Template    string          `json:"template,omitempty" yaml:"template,omitempty"`
	FlowSecret  string          `json:"flowSecret,omitempty" yaml:"flowSecret,omitempty"`
	URL         string          `json:"url,omitempty" yaml:"url,omitempty" native:"omitempty,url,max=2048"`
	LogoURL     string          `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty" native:"omitempty,url,max=2048"`
	TosURI      string          `json:"tosUri,omitempty" yaml:"tosUri,omitempty" native:"omitempty,url,max=2048"`
	PolicyURI   string          `json:"policyUri,omitempty" yaml:"policyUri,omitempty" native:"omitempty,url,max=2048"`
	Contacts    []string        `json:"contacts,omitempty" yaml:"contacts,omitempty"`

	inboundmodel.InboundAuthProfileReq `yaml:",inline"`

	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" yaml:"inboundAuthConfig,omitempty"`
	Metadata          map[string]interface{}                  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ApplicationRequestWithID represents the request structure for importing an application using file based runtime.
type ApplicationRequestWithID struct {
	ID          string          `json:"id" yaml:"id"`
	OUID        string          `json:"ouId,omitempty" yaml:"ouId,omitempty"`
	OUHandle    string          `json:"ouHandle,omitempty" yaml:"ouHandle,omitempty"`
	Name        string          `json:"name" yaml:"name"`
	Description string          `json:"description" yaml:"description"`
	Type        ApplicationType `json:"type,omitempty" yaml:"type,omitempty"`
	Template    string          `json:"template,omitempty" yaml:"template,omitempty"`
	FlowSecret  string          `json:"flowSecret,omitempty" yaml:"flowSecret,omitempty"`
	URL         string          `json:"url,omitempty" yaml:"url,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty" yaml:"logoUrl,omitempty"`
	TosURI      string          `json:"tosUri,omitempty" yaml:"tosUri,omitempty"`
	PolicyURI   string          `json:"policyUri,omitempty" yaml:"policyUri,omitempty"`
	Contacts    []string        `json:"contacts,omitempty" yaml:"contacts,omitempty"`

	inboundmodel.InboundAuthProfileReq `yaml:",inline"`

	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty" yaml:"inboundAuthConfig,omitempty"`
	Metadata          map[string]interface{}                  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ApplicationCompleteResponse represents the complete response structure for an application.
type ApplicationCompleteResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	Type        ApplicationType `json:"type,omitempty"`
	Template    string          `json:"template,omitempty"`
	FlowSecret  string          `json:"flowSecret,omitempty"`
	URL         string          `json:"url,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	TosURI      string          `json:"tosUri,omitempty"`
	PolicyURI   string          `json:"policyUri,omitempty"`
	Contacts    []string        `json:"contacts,omitempty"`

	inboundmodel.InboundAuthProfileReq

	InboundAuthConfig []providers.InboundAuthConfigWithSecret `json:"inboundAuthConfig,omitempty"`
	Metadata          map[string]interface{}                  `json:"metadata,omitempty"`
}

// ApplicationGetResponse represents the response structure for getting an application.
type ApplicationGetResponse struct {
	ID          string          `json:"id,omitempty"`
	OUID        string          `json:"ouId,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	ClientID    string          `json:"clientId,omitempty"`
	Type        ApplicationType `json:"type,omitempty"`
	Template    string          `json:"template,omitempty"`
	URL         string          `json:"url,omitempty"`
	LogoURL     string          `json:"logoUrl,omitempty"`
	TosURI      string          `json:"tosUri,omitempty"`
	PolicyURI   string          `json:"policyUri,omitempty"`
	Contacts    []string        `json:"contacts,omitempty"`

	inboundmodel.InboundAuthProfileReq

	InboundAuthConfig []inboundmodel.InboundAuthConfig `json:"inboundAuthConfig,omitempty"`
	Metadata          map[string]interface{}           `json:"metadata,omitempty"`
}

// BasicApplicationResponse represents a simplified response structure for an application.
// Only carries the subset of inbound-profile fields that make sense in the list view, so it
// does not embed InboundAuthProfile (which carries Assertion/LoginConsent/etc.).
type BasicApplicationResponse struct {
	ID                        string          `json:"id,omitempty" jsonschema:"Application ID."`
	Name                      string          `json:"name" jsonschema:"Application name."`
	Description               string          `json:"description,omitempty" jsonschema:"Application description."`
	ClientID                  string          `json:"clientId,omitempty" jsonschema:"OAuth Client ID."`
	LogoURL                   string          `json:"logoUrl,omitempty" jsonschema:"Logo URL."`
	AuthFlowID                string          `json:"authFlowId,omitempty" jsonschema:"Authentication Flow ID."`
	RegistrationFlowID        string          `json:"registrationFlowId,omitempty" jsonschema:"Registration Flow ID."`
	IsRegistrationFlowEnabled bool            `json:"isRegistrationFlowEnabled" jsonschema:"Registration enabled status."`
	RecoveryFlowID            string          `json:"recoveryFlowId,omitempty" jsonschema:"Recovery Flow ID."`
	IsRecoveryFlowEnabled     bool            `json:"isRecoveryFlowEnabled" jsonschema:"Recovery enabled status."`
	SignOutFlowID             string          `json:"signOutFlowId,omitempty" jsonschema:"Sign-out flow ID."`
	IsSignOutFlowEnabled      bool            `json:"isSignOutFlowEnabled" jsonschema:"Sign-out enabled status."`
	ThemeID                   string          `json:"themeId,omitempty" jsonschema:"Theme ID."`
	LayoutID                  string          `json:"layoutId,omitempty" jsonschema:"Layout ID."`
	Type                      ApplicationType `json:"type,omitempty" jsonschema:"Application type (browser, fullstack, mobile, m2m, mcp, custom)."`
	Template                  string          `json:"template,omitempty" jsonschema:"Application Template."`
	IsReadOnly                bool            `json:"isReadOnly" jsonschema:"Indicates if the application is read-only (declarative/immutable)."`
}

// ApplicationListResponse represents the response structure for listing applications.
type ApplicationListResponse struct {
	TotalResults int                        `json:"totalResults"`
	Count        int                        `json:"count"`
	Applications []BasicApplicationResponse `json:"applications"`
}
