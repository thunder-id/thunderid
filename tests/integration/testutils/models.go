// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"encoding/json"
	"time"
)

// UserType represents a user type definition
type UserType struct {
	ID                    string                 `json:"id,omitempty"`
	Name                  string                 `json:"name"`
	OUID                  string                 `json:"ouId"`
	AllowSelfRegistration bool                   `json:"allowSelfRegistration,omitempty"`
	Schema                map[string]interface{} `json:"schema"`
}

// User represents a user in the system
type User struct {
	ID         string          `json:"id"`
	OUID       string          `json:"ouId"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

// Application represents an application in the system
type Application struct {
	ID                        string                   `json:"id,omitempty"`
	OUID                      string                   `json:"ouId,omitempty"`
	Name                      string                   `json:"name"`
	Description               string                   `json:"description"`
	Type                      string                   `json:"type,omitempty"`
	IsRegistrationFlowEnabled bool                     `json:"isRegistrationFlowEnabled"`
	IsRecoveryFlowEnabled     bool                     `json:"isRecoveryFlowEnabled,omitempty"`
	AuthFlowID                string                   `json:"authFlowId,omitempty"`
	RegistrationFlowID        string                   `json:"registrationFlowId,omitempty"`
	RecoveryFlowID            string                   `json:"recoveryFlowId,omitempty"`
	ClientID                  string                   `json:"clientId,omitempty"`
	ClientSecret              string                   `json:"clientSecret,omitempty"`
	RedirectURIs              []string                 `json:"redirectUris,omitempty"`
	AllowedUserTypes          []string                 `json:"allowedUserTypes,omitempty"`
	SubjectAttribute          map[string]string        `json:"subjectAttribute,omitempty"`
	Certificate               map[string]interface{}   `json:"certificate,omitempty"`
	InboundAuthConfig         []map[string]interface{} `json:"inboundAuthConfig,omitempty"`
	AssertionConfig           map[string]interface{}   `json:"assertion,omitempty"`
	// Attestation is the client-level platform attestation config, set at the top level of the
	// application independent of any OAuth profile.
	Attestation map[string]interface{} `json:"attestation,omitempty"`
	// Embedded creates a native app with no inbound OAuth profile — the canonical flow-native app
	// that authenticates flow initiation with a Flow Secret. When set, no default OAuth config is
	// synthesized.
	Embedded bool `json:"-"`
}

// OrganizationUnit represents an organization unit in the system
type OrganizationUnit struct {
	ID              string  `json:"id,omitempty"`
	Handle          string  `json:"handle"`
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	Parent          *string `json:"parent,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty"`
	TosURI          string  `json:"tosUri,omitempty"`
	PolicyURI       string  `json:"policyUri,omitempty"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty"`
}

// IDPProperty represents a property of an identity provider
type IDPProperty struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
}

// AttributeMapping maps a single external IDP claim to a local user attribute. The external name may
// be a dot-notation path into a nested claim.
type AttributeMapping struct {
	ExternalAttribute string `json:"externalAttribute"`
	LocalAttribute    string `json:"localAttribute"`
}

// UserTypeAttributeMapping holds the external-to-local mappings for one local user type.
type UserTypeAttributeMapping struct {
	UserType   string             `json:"userType,omitempty"`
	Attributes []AttributeMapping `json:"attributes,omitempty"`
}

// UserTypeResolution selects the local user type for an incoming identity. Default applies when
// claim-driven resolution is absent or does not match.
type UserTypeResolution struct {
	Default           string            `json:"default,omitempty"`
	ExternalAttribute string            `json:"externalAttribute,omitempty"`
	ValueMapping      map[string]string `json:"valueMapping,omitempty"`
}

// AccountLinking lists the external claims used to resolve a local user when the subject does not.
type AccountLinking struct {
	Attributes []string `json:"attributes,omitempty"`
}

// AttributeConfiguration is the connection's user-type resolution, per-user-type attribute mappings
// and account-linking configuration. Mirrors the /connections wire format; pointers so a nil section
// stays distinguishable from an empty one.
type AttributeConfiguration struct {
	UserTypeResolution        *UserTypeResolution        `json:"userTypeResolution,omitempty"`
	UserTypeAttributeMappings []UserTypeAttributeMapping `json:"userTypeAttributeMappings,omitempty"`
	AccountLinking            *AccountLinking            `json:"accountLinking,omitempty"`
}

// IDP represents an identity provider in the system
type IDP struct {
	ID                     string                  `json:"id,omitempty"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	Type                   string                  `json:"type"`
	Properties             []IDPProperty           `json:"properties"`
	AttributeConfiguration *AttributeConfiguration `json:"attributeConfiguration,omitempty"`
}

// Link represents a pagination link.
type Link struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

// UserListResponse represents the paginated response for user listing
type UserListResponse struct {
	TotalResults int    `json:"totalResults"`
	StartIndex   int    `json:"startIndex"`
	Count        int    `json:"count"`
	Users        []User `json:"users"`
	Links        []Link `json:"links"`
}

type I18nMessage struct {
	Key          string `json:"key,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
}

func (m *I18nMessage) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		m.DefaultValue = s
		return nil
	}
	type alias I18nMessage
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*m = I18nMessage(a)
	return nil
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Code        string      `json:"code"`
	Message     I18nMessage `json:"message"`
	Description I18nMessage `json:"description"`
}

// FlowExecutionError represents a structured error returned by flow execution.
type FlowExecutionError struct {
	Code        string      `json:"code,omitempty"`
	Message     I18nMessage `json:"message,omitempty"`
	Description I18nMessage `json:"description,omitempty"`
}

// AuthenticationResponse represents the response from an authentication request
type AuthenticationResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	OUID      string `json:"ouId"`
	Assertion string `json:"assertion,omitempty"`
}

// GroupMember represents a member of a group
type GroupMember struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Display string `json:"display,omitempty"`
}

// Group represents a group in the system
type Group struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	OUID        string   `json:"ouId,omitempty"`
	Members     []Member `json:"members,omitempty"`
}

// Member represents a member of a group (either user or another group).
type Member struct {
	Id   string `json:"id"`
	Type string `json:"type"` // "user" or "group"
}

// Assignment represents a role assignment
type Assignment struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "user" or "group"
	Display string `json:"display,omitempty"`
}

// Role represents a role in the system
type Role struct {
	ID          string                `json:"id,omitempty"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OUID        string                `json:"ouId"`
	Permissions []ResourcePermissions `json:"permissions,omitempty"`
	Assignments []Assignment          `json:"assignments,omitempty"`
}

// TokenResponse represents the response from token exchange
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    float64   `json:"expires_in"`
	Scope        string    `json:"scope,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"` // Absolute expiry time
}

// TokenHTTPResult captures raw HTTP response details from the token endpoint.
type TokenHTTPResult struct {
	StatusCode int
	Body       []byte
	Token      *TokenResponse
}

// AuthorizationResponse represents the response from authorization completion
type AuthorizationResponse struct {
	RedirectURI string `json:"redirect_uri"`
}

// ResourceServer represents a resource server in the system
type ResourceServer struct {
	ID          string  `json:"id,omitempty"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Identifier  string  `json:"identifier,omitempty"`
	OUID        string  `json:"ouId"`
	Delimiter   *string `json:"delimiter,omitempty"`
}

// Action represents an action in the resource system
type Action struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Handle      string `json:"handle"`
	Description string `json:"description,omitempty"`
	Permission  string `json:"permission,omitempty"`
}

// ResourcePermissions represents permissions grouped by resource server
type ResourcePermissions struct {
	ResourceServerID string   `json:"resourceServerId"`
	Permissions      []string `json:"permissions"`
}

// FlowResponse represents the response from flow execution
type FlowResponse struct {
	ExecutionID string              `json:"executionId"`
	FlowStatus  string              `json:"flowStatus"`
	Type        string              `json:"type"`
	Data        *FlowData           `json:"data,omitempty"`
	Assertion   string              `json:"assertion,omitempty"`
	Error       *FlowExecutionError `json:"error,omitempty"`
}

// FlowData represents the data returned by flow execution
type FlowData struct {
	Inputs         []FlowInput            `json:"inputs,omitempty"`
	Actions        []FlowAction           `json:"actions,omitempty"`
	RedirectURL    string                 `json:"redirectUrl,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	AdditionalData map[string]interface{} `json:"additionalData,omitempty"`
}

// FlowInput represents an input required by the flow
type FlowInput struct {
	Ref        string `json:"ref,omitempty"`
	Identifier string `json:"identifier"`
	Type       string `json:"type"`
	Required   bool   `json:"required"`
}

// FlowAction represents an action available in the flow
type FlowAction struct {
	Ref      string `json:"ref"`
	NextNode string `json:"nextNode"`
}

// FlowStep represents a single step in a flow execution
type FlowStep struct {
	ExecutionID string    `json:"executionId"`
	FlowStatus  string    `json:"flowStatus"`
	Type        string    `json:"type"`
	Data        *FlowData `json:"data,omitempty"`
	Assertion   string    `json:"assertion,omitempty"`
	// ErrorAssertion is the signed error assertion minted when an OAuth-initiated flow terminates in
	// failure. It is relayed to /oauth2/auth/callback in the same field as a success assertion.
	ErrorAssertion string              `json:"errorAssertion,omitempty"`
	Error          *FlowExecutionError `json:"error,omitempty"`
	ChallengeToken string              `json:"challengeToken,omitempty"`
}

// FlowErrorResponse is the body returned when flow execution fails at the engine level (4xx/5xx),
// which has no flow response to carry the assertion. Message and Description are i18n objects, so
// they are left raw; tests assert on the code and the assertion.
type FlowErrorResponse struct {
	Code           string          `json:"code"`
	Message        json.RawMessage `json:"message"`
	Description    json.RawMessage `json:"description"`
	ErrorAssertion string          `json:"errorAssertion,omitempty"`
}

// Flow represents a flow definition
type Flow struct {
	Name     string      `json:"name"`
	FlowType string      `json:"flowType"`
	Handle   string      `json:"handle"`
	Nodes    interface{} `json:"nodes"`
}

// NotificationSender represents a notification sender in the system
type NotificationSender struct {
	ID          string           `json:"id,omitempty"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Provider    string           `json:"provider"`
	Properties  []SenderProperty `json:"properties"`
}

// SenderProperty represents a property of a notification sender
type SenderProperty struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
}

// Agent represents an agent resource in the system.
type Agent struct {
	ID          string      `json:"id,omitempty"`
	OUID        string      `json:"ouId,omitempty"`
	OUHandle    string      `json:"ouHandle,omitempty"`
	Type        string      `json:"type,omitempty"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Owner       string      `json:"owner,omitempty"`
	Attributes  interface{} `json:"attributes,omitempty"`
	IsReadOnly  bool        `json:"isReadOnly"`

	AuthFlowID                string                   `json:"authFlowId,omitempty"`
	RegistrationFlowID        string                   `json:"registrationFlowId,omitempty"`
	IsRegistrationFlowEnabled bool                     `json:"isRegistrationFlowEnabled,omitempty"`
	ThemeID                   string                   `json:"themeId,omitempty"`
	LayoutID                  string                   `json:"layoutId,omitempty"`
	AllowedUserTypes          []string                 `json:"allowedUserTypes,omitempty"`
	InboundAuthConfig         []AgentInboundAuthConfig `json:"inboundAuthConfig,omitempty"`
}

// AgentInboundAuthConfig represents an inbound auth config entry for an agent.
type AgentInboundAuthConfig struct {
	Type   string            `json:"type"`
	Config *AgentOAuthConfig `json:"config,omitempty"`
}

// AgentOAuthConfig holds OAuth client settings for an agent.
type AgentOAuthConfig struct {
	ClientID                string   `json:"clientId,omitempty"`
	ClientSecret            string   `json:"clientSecret,omitempty"`
	GrantTypes              []string `json:"grantTypes,omitempty"`
	ResponseTypes           []string `json:"responseTypes,omitempty"`
	TokenEndpointAuthMethod string   `json:"tokenEndpointAuthMethod,omitempty"`
	PKCERequired            bool     `json:"pkceRequired,omitempty"`
	PublicClient            bool     `json:"publicClient,omitempty"`
}

// AgentListResponse is the paginated list response for agents.
type AgentListResponse struct {
	TotalResults int     `json:"totalResults"`
	StartIndex   int     `json:"startIndex"`
	Count        int     `json:"count"`
	Agents       []Agent `json:"agents"`
}

// ClaimMapping is a single selectively-disclosable claim in a credential
// configuration. Name must match a user profile attribute key.
type ClaimMapping struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
}

// CredentialDisplay is the optional display metadata of a credential configuration.
type CredentialDisplay struct {
	Locale  string `json:"locale,omitempty"`
	LogoURI string `json:"logoUri,omitempty"`
}

// CredentialConfiguration is the request body for the OpenID4VCI credential
// configuration management API. Handle is both the credential_configuration_id
// and the OAuth scope; VCT and an OU (OUID or OUHandle) are required.
type CredentialConfiguration struct {
	Handle          string             `json:"handle"`
	OUID            string             `json:"ouId,omitempty"`
	OUHandle        string             `json:"ouHandle,omitempty"`
	Name            string             `json:"name,omitempty"`
	Description     string             `json:"description,omitempty"`
	Format          string             `json:"format,omitempty"`
	VCT             string             `json:"vct"`
	Claims          []ClaimMapping     `json:"claims,omitempty"`
	Display         *CredentialDisplay `json:"display,omitempty"`
	ValiditySeconds *int               `json:"validitySeconds,omitempty"`
}

// PresentationDefinition is the request body for the OpenID4VP presentation
// definition management API. Handle is the definition_id used on initiate and
// the DCQL credential id; VCT and an OU (OUID or OUHandle) are required.
type PresentationDefinition struct {
	Handle               string              `json:"handle"`
	OUID                 string              `json:"ouId,omitempty"`
	OUHandle             string              `json:"ouHandle,omitempty"`
	Name                 string              `json:"name,omitempty"`
	Description          string              `json:"description,omitempty"`
	VCT                  string              `json:"vct"`
	Format               string              `json:"format,omitempty"`
	RequestedClaims      []string            `json:"requestedClaims,omitempty"`
	MandatoryClaims      []string            `json:"mandatoryClaims,omitempty"`
	OptionalClaims       []string            `json:"optionalClaims,omitempty"`
	ClaimValues          map[string][]string `json:"claimValues,omitempty"`
	EnforceTrustedIssuer *bool               `json:"enforceTrustedIssuer,omitempty"`
	TrustedAuthorities   []string            `json:"trustedAuthorities,omitempty"`
}

// VPInitiateResponse is the body returned by POST /openid4vp/initiate.
type VPInitiateResponse struct {
	TxnID     string `json:"txn_id"`
	WalletURL string `json:"wallet_url"`
	StatusURL string `json:"status_url"`
	ExpiresAt string `json:"expires_at"`
}

// VPStatusResponse is the body returned by GET /openid4vp/status/{txn_id}.
type VPStatusResponse struct {
	Status      string `json:"status"`
	ResultToken string `json:"result_token"`
	Error       string `json:"error"`
}

// VCHTTPResult captures a raw HTTP status and body from a VC endpoint for assertions.
type VCHTTPResult struct {
	StatusCode int
	Body       []byte
}
