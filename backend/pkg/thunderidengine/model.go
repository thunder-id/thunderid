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

package thunderidengine

import (
	"encoding/json"
	"time"
)

// FlowType identifies the kind of flow definition or execution.
type FlowType string

const (
	// FlowTypeAuthentication is an end-user authentication flow.
	FlowTypeAuthentication FlowType = "AUTHENTICATION"
	// FlowTypeRegistration is a self-service registration flow.
	FlowTypeRegistration FlowType = "REGISTRATION"
	// FlowTypeUserOnboarding is a post-registration onboarding flow.
	FlowTypeUserOnboarding FlowType = "USER_ONBOARDING"
	// FlowTypeRecovery is an account recovery flow.
	FlowTypeRecovery FlowType = "RECOVERY"
)

// Credentials carries authentication inputs supplied by the host.
type Credentials map[string]interface{}

// AuthnResult is the outcome of a successful AuthenticateUser call.
type AuthnResult struct {
	EntityID       string
	EntityCategory string
	EntityType     string
	OUID           string
	UserID         string
	Token          string
	Attributes     map[string]interface{}
}

// EntityGroup represents a group membership for authorization.
type EntityGroup struct {
	ID   string
	Name string
	OUID string
}

// OAuthClient is the resolved OAuth/OIDC client configuration used at runtime.
type OAuthClient struct {
	ID                                 string
	OUID                               string
	ClientID                           string
	RedirectURIs                       []string
	GrantTypes                         []string
	ResponseTypes                      []string
	TokenEndpointAuthMethod            string
	PKCERequired                       bool
	PublicClient                       bool
	RequirePushedAuthorizationRequests bool
	Scopes                             []string
	AcrValues                          []string
}

// Application is the host-facing view of an OAuth-enabled application.
type Application struct {
	ID          string
	OUID        string
	Name        string
	Description string
	URL         string
	LogoURL     string
	ClientID    string
	AuthFlowID  string
	ThemeID     string
	LayoutID    string
	Metadata    map[string]interface{}
}

// Resource is a host-facing authorization resource.
type Resource struct {
	ID          string
	Name        string
	Handle      string
	Description string
	ParentID    string
	Permission  string
}

// OrganizationUnit is a host-facing organization unit.
type OrganizationUnit struct {
	ID          string
	Handle      string
	Name        string
	Description string
	ParentID    string
	ThemeID     string
	LayoutID    string
}

// IdentityProvider is a host-facing identity provider definition.
type IdentityProvider struct {
	ID          string
	Name        string
	Description string
	Type        string
	Properties  map[string]string
}

// FlowDefinition is a host-facing flow definition without admin CRUD metadata.
type FlowDefinition struct {
	ID       string
	Handle   string
	Name     string
	FlowType FlowType
	Nodes    json.RawMessage
}

// OAuthParameters holds JSON-encoded OAuth authorization request parameters.
type OAuthParameters json.RawMessage

// PushedAuthorizationRequest is the payload stored for a PAR request.
type PushedAuthorizationRequest struct {
	ClientID        string
	OAuthParameters OAuthParameters
}

// AuthRequestContext holds in-flight OAuth authorization request state.
type AuthRequestContext struct {
	OAuthParameters OAuthParameters
}

// AuthorizationCode is a host-facing authorization code record.
type AuthorizationCode struct {
	CodeID              string
	Code                string
	ClientID            string
	RedirectURI         string
	AuthorizedUserID    string
	AttributeCacheID    string
	TimeCreated         time.Time
	ExpiryTime          time.Time
	Scopes              string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resources           []string
	Nonce               string
	CompletedACR        string
	ClaimsRequestJSON   json.RawMessage
}

// FlowContext is the persisted flow execution state.
type FlowContext struct {
	ExecutionID string
	Context     string
	ExpiryTime  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Event is a host-facing observability event.
type Event struct {
	TraceID   string
	EventID   string
	Type      string
	Timestamp time.Time
	Component string
	Status    string
	Data      map[string]interface{}
}

// DesignResponse holds resolved theme and layout JSON for Gate /flow/meta.
type DesignResponse struct {
	Theme  json.RawMessage
	Layout json.RawMessage
}

// TranslationsResponse holds resolved i18n translations for a language and namespace.
type TranslationsResponse struct {
	Language     string
	TotalResults int
	Translations map[string]map[string]string
}

// Role is a role name assigned to a user for AuthAssert and token claims.
type Role struct {
	Name string
}
