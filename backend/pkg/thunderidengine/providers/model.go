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

// Package providers provides models for the providers module.
//
//nolint:lll
package providers

import (
	"encoding/json"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// LanguageTranslationsResponse represents all translations for a language, organized by namespace.
type LanguageTranslationsResponse struct {
	Language     string                       `json:"language"`
	TotalResults int                          `json:"totalResults,omitempty"`
	Translations map[string]map[string]string `json:"translations"`
}

// DesignResponse represents the response body for design resolve operations.
type DesignResponse struct {
	Theme  json.RawMessage `json:"theme,omitempty"`
	Layout json.RawMessage `json:"layout,omitempty"`
}

// OrganizationUnit represents an organization unit.
type OrganizationUnit struct {
	ID              string    `json:"id"                        yaml:"id"`
	Handle          string    `json:"handle"                    yaml:"handle"`
	Name            string    `json:"name"                      yaml:"name"`
	Description     string    `json:"description,omitempty"     yaml:"description,omitempty"`
	Parent          *string   `json:"parent"                    yaml:"parent"`
	ThemeID         string    `json:"themeId,omitempty"         yaml:"themeId,omitempty"`
	LayoutID        string    `json:"layoutId,omitempty"        yaml:"layoutId,omitempty"`
	LogoURL         string    `json:"logoUrl,omitempty"         yaml:"logoUrl,omitempty"`
	TosURI          string    `json:"tosUri,omitempty"          yaml:"tosUri,omitempty"`
	PolicyURI       string    `json:"policyUri,omitempty"       yaml:"policyUri,omitempty"`
	CookiePolicyURI string    `json:"cookiePolicyUri,omitempty" yaml:"cookiePolicyUri,omitempty"`
	CreatedAt       time.Time `json:"createdAt"                 yaml:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"                 yaml:"updatedAt"`
}

// OrganizationUnitRequestWithID represents the request body for creating an organization unit
// in import/declarative paths where preserving IDs is required.
type OrganizationUnitRequestWithID struct {
	ID              string  `json:"id"                        yaml:"id"                        native:"required"`
	Handle          string  `json:"handle"                    yaml:"handle"                    native:"required,min=3,max=50"`
	Name            string  `json:"name"                      yaml:"name"                      native:"required,min=2,max=100"`
	Description     string  `json:"description,omitempty"     yaml:"description,omitempty"`
	Parent          *string `json:"parent"                    yaml:"parent"`
	ThemeID         string  `json:"themeId,omitempty"         yaml:"themeId,omitempty"`
	LayoutID        string  `json:"layoutId,omitempty"        yaml:"layoutId,omitempty"`
	LogoURL         string  `json:"logoUrl,omitempty"         yaml:"logoUrl,omitempty"         native:"omitempty,url,max=2048"`
	TosURI          string  `json:"tosUri,omitempty"          yaml:"tosUri,omitempty"          native:"omitempty,url,max=2048"`
	PolicyURI       string  `json:"policyUri,omitempty"       yaml:"policyUri,omitempty"       native:"omitempty,url,max=2048"`
	CookiePolicyURI string  `json:"cookiePolicyUri,omitempty" yaml:"cookiePolicyUri,omitempty" native:"url,max=2048"`
}

// OrganizationUnitListResponse represents the response for listing organization units with pagination.
type OrganizationUnitListResponse struct {
	TotalResults      int                     `json:"totalResults"`
	StartIndex        int                     `json:"startIndex"`
	Count             int                     `json:"count"`
	OrganizationUnits []OrganizationUnitBasic `json:"organizationUnits"`
	Links             []utils.Link            `json:"links"`
}

// OrganizationUnitBasic represents the basic information of an organization unit.
type OrganizationUnitBasic struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	LogoURL     string    `json:"logoUrl,omitempty"`
	IsReadOnly  bool      `json:"isReadOnly"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ResourceServerType represents the type of a resource server.
type ResourceServerType string

const (
	// ResourceServerTypeAPI represents an API resource server.
	ResourceServerTypeAPI ResourceServerType = "API"
	// ResourceServerTypeMCP represents an MCP resource server.
	ResourceServerTypeMCP ResourceServerType = "MCP"
	// ResourceServerTypeCustom represents a custom resource server.
	ResourceServerTypeCustom ResourceServerType = "CUSTOM"
)

// supportedResourceServerTypes lists all the supported resource server types.
var supportedResourceServerTypes = []ResourceServerType{
	ResourceServerTypeAPI,
	ResourceServerTypeMCP,
	ResourceServerTypeCustom,
}

// IsValid reports whether the resource server type is one of the supported values.
func (t ResourceServerType) IsValid() bool {
	for _, supported := range supportedResourceServerTypes {
		if t == supported {
			return true
		}
	}
	return false
}

// ActionKind discriminates MCP primitives stored as actions.
type ActionKind string

const (
	// ActionKindTool represents an MCP tool.
	ActionKindTool ActionKind = "tool"
	// ActionKindResource represents an MCP resource.
	ActionKindResource ActionKind = "resource"
)

// supportedActionKinds lists all the supported action kinds.
var supportedActionKinds = []ActionKind{
	ActionKindTool,
	ActionKindResource,
}

// IsValid reports whether the action kind is one of the supported values.
func (k ActionKind) IsValid() bool {
	for _, supported := range supportedActionKinds {
		if k == supported {
			return true
		}
	}
	return false
}

// Consolidated resource models for YAML parsing, processing, and service layer
// These models use:
// - yaml tags for YAML parsing (serialize/deserialize)
// - json tags for many fields (e.g., in Action, Resource, ResourceServer) for service/API use
// - Computed/internal fields marked with json:"-" and yaml:"-" as appropriate

// Action represents an action in both declarative resources and service layer.
type Action struct {
	ID          string `yaml:"-"                     json:"-"` // Set when retrieved from database
	Name        string `yaml:"name"                  json:"name"`
	Handle      string `yaml:"handle"                json:"handle"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Permission  string `yaml:"-"                     json:"-"` // Computed permission string, not serialized to YAML
	// Kind is empty ("") for API/CUSTOM actions; "tool"|"resource" for MCP actions.
	Kind ActionKind `yaml:"kind,omitempty" json:"-"`
}

// Resource represents a resource in both declarative resources and service layer.
type Resource struct {
	ID           string   `yaml:"-"                     json:"-"` // Set when retrieved from database
	Name         string   `yaml:"name"                  json:"name"`
	Handle       string   `yaml:"handle"                json:"handle"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	Parent       *string  `yaml:"-"                     json:"-"`                // Resolved parent ID
	ParentHandle string   `yaml:"parent,omitempty"      json:"parent,omitempty"` // Parent handle during YAML parsing only
	Permission   string   `yaml:"-"                     json:"-"`                // Computed permission string
	Actions      []Action `yaml:"actions,omitempty"     json:"actions,omitempty"`
}

// ResourceServer represents a resource server in both declarative resources and service layer.
type ResourceServer struct {
	ID          string             `yaml:"id"                    json:"-"`
	Name        string             `yaml:"name"                  json:"name"`
	Description string             `yaml:"description,omitempty" json:"description,omitempty"`
	Handle      string             `yaml:"handle"                json:"handle"`
	Identifier  string             `yaml:"identifier,omitempty"  json:"identifier,omitempty"`
	Type        ResourceServerType `yaml:"type,omitempty"        json:"type,omitempty"`
	OUID        string             `yaml:"ouId,omitempty"        json:"ouId"`
	OUHandle    string             `yaml:"ouHandle,omitempty"    json:"-"`
	Delimiter   string             `yaml:"delimiter,omitempty"   json:"delimiter,omitempty"   yamlfmt:"quoted"`
	IsReadOnly  bool               `yaml:"-"                     json:"-"`
	Resources   []Resource         `yaml:"resources,omitempty"   json:"resources,omitempty"`
}

// CompleteFlowDefinition represents a complete flow definition with all details.
type CompleteFlowDefinition struct {
	ID            string                  `json:"id"                      yaml:"id"                     jsonschema:"Unique identifier of the flow. UUID format."`
	Handle        string                  `json:"handle"                  yaml:"handle"                 jsonschema:"URL-friendly handle for the flow."`
	Name          string                  `json:"name"                    yaml:"name"                   jsonschema:"Display name of the flow."`
	FlowType      FlowType                `json:"flowType"                yaml:"flowType"               jsonschema:"Type of flow (AUTHENTICATION or REGISTRATION)."`
	ActiveVersion int                     `json:"activeVersion,omitempty" yaml:"activeVersion"          jsonschema:"Current active version number of the flow."`
	Interceptors  []InterceptorDefinition `json:"interceptors,omitempty"  yaml:"interceptors,omitempty" jsonschema:"Interceptor declarations for cross-cutting concerns."`
	Nodes         []NodeDefinition        `json:"nodes,omitempty"         yaml:"nodes"                  jsonschema:"List of nodes defining the flow logic."`
	CreatedAt     string                  `json:"createdAt,omitempty"     yaml:"createdAt"              jsonschema:"Timestamp when the flow was created."`
	UpdatedAt     string                  `json:"updatedAt,omitempty"     yaml:"updatedAt"              jsonschema:"Timestamp when the flow was last updated."`
	IsReadOnly    bool                    `json:"isReadOnly"              yaml:"isReadOnly"             jsonschema:"Whether the flow is immutable (declarative)."`
}

// InterceptorDefinition describes how an interceptor is declared in the flow definition.
type InterceptorDefinition struct {
	Name       string                 `json:"name"`
	Mode       InterceptorMode        `json:"mode"`
	Scope      InterceptorScope       `json:"scope,omitempty"`
	ApplyTo    []string               `json:"applyTo,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// NodeLayout represents the layout information for a node in the flow composer UI.
type NodeLayout struct {
	Size     *NodeSize     `json:"size,omitempty"     yaml:"size,omitempty"     jsonschema:"Dimensions of the node."`
	Position *NodePosition `json:"position,omitempty" yaml:"position,omitempty" jsonschema:"Coordinates of the node on the canvas."`
}

// NodeSize represents the dimensions of a node.
type NodeSize struct {
	Width  float64 `json:"width"  yaml:"width"  jsonschema:"Width of the node in pixels."`
	Height float64 `json:"height" yaml:"height" jsonschema:"Height of the node in pixels."`
}

// NodePosition represents the position of a node on the canvas.
type NodePosition struct {
	X float64 `json:"x" yaml:"x" jsonschema:"X-coordinate of the node."`
	Y float64 `json:"y" yaml:"y" jsonschema:"Y-coordinate of the node."`
}

// NodeDefinition represents a single node in a flow definition.
type NodeDefinition struct {
	ID           string                 `json:"id"                     yaml:"id"                     jsonschema:"Unique node identifier within the flow. Example: 'start', 'username-password', 'end'"`
	Type         string                 `json:"type"                   yaml:"type"                   jsonschema:"Node type: 'START' (entry point), 'END' (exit point), 'TASK_EXECUTION' (backend logic), or 'PROMPT' (user input)"`
	Layout       *NodeLayout            `json:"layout,omitempty"       yaml:"layout,omitempty"       jsonschema:"Optional UI layout information for flow composer (position and size on canvas)"`
	Meta         interface{}            `json:"meta,omitempty"         yaml:"meta,omitempty"         jsonschema:"Optional metadata. For PROMPT nodes, must include 'components' array for UI rendering. See existing flows for examples."`
	Prompts      []PromptDefinition     `json:"prompts,omitempty"      yaml:"prompts,omitempty"      jsonschema:"For PROMPT nodes: defines user inputs and actions. Each prompt has inputs (form fields) and an action (what happens on submit)."`
	Variant      NodeVariant            `json:"variant,omitempty"      yaml:"variant,omitempty"      jsonschema:"Optional PROMPT node variant. Use 'LOGIN_OPTIONS' to enable login option filtering on this node."`
	Next         string                 `json:"next,omitempty"         yaml:"next,omitempty"         jsonschema:"For display-only PROMPT nodes: ID of the next node. Mutually exclusive with 'prompts'."`
	Message      string                 `json:"message,omitempty"      yaml:"message,omitempty"      jsonschema:"For display-only PROMPT nodes: textual message for non-verbose mode."`
	Properties   map[string]interface{} `json:"properties,omitempty"   yaml:"properties,omitempty"   jsonschema:"Optional node-specific properties for configuration"`
	Executor     *ExecutorDefinition    `json:"executor,omitempty"     yaml:"executor,omitempty"     jsonschema:"For TASK_EXECUTION nodes: defines which executor to run (e.g., 'UsernamePasswordAuthenticator', 'OTPGenerator')"`
	OnSuccess    string                 `json:"onSuccess,omitempty"    yaml:"onSuccess,omitempty"    jsonschema:"ID of the next node to execute on successful completion"`
	OnFailure    string                 `json:"onFailure,omitempty"    yaml:"onFailure,omitempty"    jsonschema:"ID of the next node to execute on failure"`
	OnIncomplete string                 `json:"onIncomplete,omitempty" yaml:"onIncomplete,omitempty" jsonschema:"For TASK_EXECUTION nodes: ID of the PROMPT node to forward to when user input is required."`
	Condition    *ConditionDefinition   `json:"condition,omitempty"    yaml:"condition,omitempty"    jsonschema:"Optional condition to determine if this node should execute"`
}

type nodeDefinitionAlias NodeDefinition

// MarshalYAML implements custom YAML marshaling for NodeDefinition.
// It converts the Meta interface{} field to a JSON-encoded string for proper serialization.
func (nd *NodeDefinition) MarshalYAML() (interface{}, error) {
	alias := nodeDefinitionAlias(*nd)

	if alias.Meta == nil {
		return alias, nil
	}

	metaJSON, err := json.Marshal(alias.Meta)
	if err != nil {
		return nil, err
	}

	alias.Meta = string(metaJSON)

	return alias, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for NodeDefinition.
// It parses the Meta field from a JSON-encoded string back to interface{}.
func (nd *NodeDefinition) UnmarshalYAML(value *yaml.Node) error {
	var alias nodeDefinitionAlias

	if err := value.Decode(&alias); err != nil {
		return err
	}

	*nd = NodeDefinition(alias)

	if metaStr, ok := nd.Meta.(string); ok && metaStr != "" {
		var metaData interface{}
		if err := json.Unmarshal([]byte(metaStr), &metaData); err != nil {
			return nil
		}
		nd.Meta = metaData
	}

	return nil
}

// InputDefinition represents an input parameter for a node.
type InputDefinition struct {
	Ref        string                     `json:"ref,omitempty"        yaml:"ref,omitempty"        jsonschema:"Reference ID for the input."`
	Type       string                     `json:"type"                 yaml:"type"                 jsonschema:"Input type (e.g., 'text', 'password', 'email')."`
	Identifier string                     `json:"identifier"           yaml:"identifier"           jsonschema:"Field identifier or name."`
	Required   bool                       `json:"required"             yaml:"required"             jsonschema:"Whether this input is mandatory."`
	Validation []ValidationRuleDefinition `json:"validation,omitempty" yaml:"validation,omitempty" jsonschema:"Server-enforced validation rules applied to the submitted value."`
}

// ValidationRuleDefinition represents a single validation constraint on an input.
// Type is one of "regex", "minLength", or "maxLength"; Value holds the constraint
// parameter (string for regex, number for length types); Message is an i18n key or
// literal string returned in fieldErrors when the rule fails.
type ValidationRuleDefinition struct {
	Type    string      `json:"type"              yaml:"type"              jsonschema:"Rule type: 'regex', 'minLength', or 'maxLength'."`
	Value   interface{} `json:"value"             yaml:"value"             jsonschema:"Constraint value: regex pattern (string) or length (number)."`
	Message string      `json:"message,omitempty" yaml:"message,omitempty" jsonschema:"i18n key or literal message returned when the rule fails."`
}

// ActionDefinition represents an action to be executed by a node.
type ActionDefinition struct {
	Ref      string `json:"ref"            yaml:"ref"            jsonschema:"Reference ID for the action."`
	Type     string `json:"type,omitempty" yaml:"type,omitempty" jsonschema:"Action type. Forwarded to next executor to determine the action to take."`
	NextNode string `json:"nextNode"       yaml:"nextNode"       jsonschema:"ID of the node to transition to when this action is taken."`
}

// PromptDefinition groups inputs with an action for prompt nodes.
type PromptDefinition struct {
	Inputs []InputDefinition `json:"inputs,omitempty" yaml:"inputs,omitempty" jsonschema:"List of input fields shown to the user."`
	Action *ActionDefinition `json:"action,omitempty" yaml:"action,omitempty" jsonschema:"Action to take upon submission."`
}

// ExecutorDefinition represents the executor configuration for a node.
type ExecutorDefinition struct {
	Name   string            `json:"name"             yaml:"name"             jsonschema:"Name of the executor (e.g., 'UsernamePasswordAuthenticator')."`
	Mode   string            `json:"mode,omitempty"   yaml:"mode,omitempty"   jsonschema:"Execution mode or configuration."`
	Inputs []InputDefinition `json:"inputs,omitempty" yaml:"inputs,omitempty" jsonschema:"Static inputs or configuration parameters for the executor."`
}

// ConditionDefinition represents a condition for node execution.
type ConditionDefinition struct {
	Key    string `json:"key"    yaml:"key"    jsonschema:"Attribute key to check."`
	Value  string `json:"value"  yaml:"value"  jsonschema:"Value to match."`
	OnSkip string `json:"onSkip" yaml:"onSkip" jsonschema:"Node ID to skip to if condition is not met."`
}

// AuthnMetadata contains metadata for authentication.
type AuthnMetadata struct {
	AppMetadata     map[string]interface{} `json:"appMetadata,omitempty"`
	RuntimeMetadata map[string]string      `json:"runtimeMetadata,omitempty"`
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
	AppMetadata     map[string]interface{} `json:"appMetadata,omitempty"`
	Locale          string                 `json:"locale"`
	RuntimeMetadata map[string]string      `json:"runtimeMetadata,omitempty"`
}

// AuthUser accumulates per-provider authentication state produced during flow execution.
// All fields are unexported; use the manager methods to interact with this type.
type AuthUser struct {
	entityReferenceToken any
	entityReference      *EntityReference
	attributeToken       any
	attributes           *AttributesResponse
}

// IsAuthenticated reports whether this AuthUser has been populated by a successful
// authentication.
func (a AuthUser) IsAuthenticated() bool {
	return (a.entityReference != nil || a.entityReferenceToken != nil) &&
		(a.attributes != nil || a.attributeToken != nil)
}

// EntityReferenceToken returns the opaque entity-reference token, if any.
func (a AuthUser) EntityReferenceToken() any {
	return a.entityReferenceToken
}

// EntityReference returns the resolved entity reference, if any.
func (a AuthUser) EntityReference() *EntityReference {
	return a.entityReference
}

// AttributeToken returns the opaque attribute token, if any.
func (a AuthUser) AttributeToken() any {
	return a.attributeToken
}

// Attributes returns the resolved attributes, if any.
func (a AuthUser) Attributes() *AttributesResponse {
	return a.attributes
}

// SetEntityReferenceToken stores an entity-reference token and clears any resolved reference.
func (a *AuthUser) SetEntityReferenceToken(token any) {
	a.entityReferenceToken = token
	a.entityReference = nil
}

// SetEntityReference stores a resolved entity reference and clears any token.
func (a *AuthUser) SetEntityReference(ref *EntityReference) {
	a.entityReference = ref
	a.entityReferenceToken = nil
}

// SetAttributeToken stores an attribute token and clears any resolved attributes.
func (a *AuthUser) SetAttributeToken(token any) {
	a.attributeToken = token
	a.attributes = nil
}

// SetAttributes stores resolved attributes and clears any attribute token.
func (a *AuthUser) SetAttributes(attrs *AttributesResponse) {
	a.attributes = attrs
	a.attributeToken = nil
}

// authUserJSON is the internal proxy used for JSON serialization of AuthUser.
type authUserJSON struct {
	EntityReferenceToken any                 `json:"entityReferenceToken"`
	EntityReference      *EntityReference    `json:"entityReference,omitempty"`
	AttributeToken       any                 `json:"attributeToken"`
	Attributes           *AttributesResponse `json:"attributes,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (a *AuthUser) MarshalJSON() ([]byte, error) {
	proxy := authUserJSON{
		EntityReferenceToken: a.entityReferenceToken,
		EntityReference:      a.entityReference,
		AttributeToken:       a.attributeToken,
		Attributes:           a.attributes,
	}

	return json.Marshal(proxy)
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *AuthUser) UnmarshalJSON(b []byte) error {
	var proxy authUserJSON
	if err := json.Unmarshal(b, &proxy); err != nil {
		return err
	}

	a.entityReferenceToken = proxy.EntityReferenceToken
	a.entityReference = proxy.EntityReference
	a.attributeToken = proxy.AttributeToken
	a.attributes = proxy.Attributes

	return nil
}

// OAuthClient is the resolved runtime view.
type OAuthClient struct {
	ID                                 string                  `yaml:"id,omitempty"`
	OUID                               string                  `yaml:"ouId,omitempty"`
	ClientID                           string                  `yaml:"clientId,omitempty"`
	RedirectURIs                       []string                `yaml:"redirectUris,omitempty"`
	GrantTypes                         []GrantType             `yaml:"grantTypes,omitempty"`
	ResponseTypes                      []ResponseType          `yaml:"responseTypes,omitempty"`
	TokenEndpointAuthMethod            TokenEndpointAuthMethod `yaml:"tokenEndpointAuthMethod,omitempty"`
	PKCERequired                       bool                    `yaml:"pkceRequired,omitempty"`
	PublicClient                       bool                    `yaml:"publicClient,omitempty"`
	RequirePushedAuthorizationRequests bool                    `yaml:"requirePushedAuthorizationRequests,omitempty"`
	DPoPBoundAccessTokens              bool                    `yaml:"dpopBoundAccessTokens,omitempty"`
	IncludeActClaim                    bool                    `yaml:"includeActClaim,omitempty"`
	EntityCategory                     EntityCategory          `yaml:"entityCategory,omitempty"`
	Token                              *OAuthTokenConfig       `yaml:"token,omitempty"`
	Scopes                             []string                `yaml:"scopes,omitempty"`
	UserInfo                           *UserInfoConfig         `yaml:"userInfo,omitempty"`
	ScopeClaims                        map[string][]string     `yaml:"scopeClaims,omitempty"`
	Certificate                        *Certificate            `yaml:"certificate,omitempty"`
	AcrValues                          []string                `yaml:"acrValues,omitempty"`
}

// OAuthTokenConfig wraps access and ID token configs.
type OAuthTokenConfig struct {
	AccessToken  *AccessTokenConfig  `json:"accessToken,omitempty"  yaml:"accessToken,omitempty"  jsonschema:"Access token configuration."`
	IDToken      *IDTokenConfig      `json:"idToken,omitempty"      yaml:"idToken,omitempty"      jsonschema:"ID token configuration."`
	RefreshToken *RefreshTokenConfig `json:"refreshToken,omitempty" yaml:"refreshToken,omitempty" jsonschema:"Refresh token configuration."`
}

// AccessTokenConfig is the access token configuration.
type AccessTokenConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty" yaml:"validityPeriod,omitempty" jsonschema:"Access token validity period in seconds."`
	UserAttributes []string `json:"userAttributes,omitempty" yaml:"userAttributes,omitempty" jsonschema:"User attributes to embed in the access token."`
}

// IDTokenConfig is the ID token configuration.
type IDTokenConfig struct {
	ValidityPeriod int64               `json:"validityPeriod,omitempty" yaml:"validityPeriod,omitempty" jsonschema:"ID token validity period in seconds."`
	UserAttributes []string            `json:"userAttributes,omitempty" yaml:"userAttributes,omitempty" jsonschema:"User attributes to embed in the ID token."`
	ResponseType   IDTokenResponseType `json:"responseType,omitempty"   yaml:"responseType,omitempty"   jsonschema:"ID token response type (JWT, JWE, NESTED_JWT). Defaults to JWT."`
	EncryptionAlg  string              `json:"encryptionAlg,omitempty"  yaml:"encryptionAlg,omitempty"  jsonschema:"JWE key-management algorithm. Required when responseType is JWE or NESTED_JWT."`
	EncryptionEnc  string              `json:"encryptionEnc,omitempty"  yaml:"encryptionEnc,omitempty"  jsonschema:"JWE content-encryption algorithm. Required when responseType is JWE or NESTED_JWT."`
}

// RefreshTokenConfig is the refresh token configuration.
type RefreshTokenConfig struct {
	ValidityPeriod int64 `json:"validityPeriod,omitempty" yaml:"validityPeriod,omitempty" jsonschema:"Refresh token validity period in seconds."`
}

// UserInfoConfig is the user info endpoint configuration.
type UserInfoConfig struct {
	ResponseType   UserInfoResponseType `json:"responseType,omitempty"   yaml:"responseType,omitempty"   jsonschema:"UserInfo response type (JSON, JWS, JWE, NESTED_JWT). Required algorithm fields must match the selected response type."`
	UserAttributes []string             `json:"userAttributes,omitempty" yaml:"userAttributes,omitempty" jsonschema:"User attributes to include in the userinfo response."`
	SigningAlg     string               `json:"signingAlg,omitempty"     yaml:"signingAlg,omitempty"     jsonschema:"JWS algorithm for signed userinfo responses (e.g. RS256)."`
	EncryptionAlg  string               `json:"encryptionAlg,omitempty"  yaml:"encryptionAlg,omitempty"  jsonschema:"JWE key-management algorithm for encrypted userinfo responses (e.g. RSA-OAEP-256)."`
	EncryptionEnc  string               `json:"encryptionEnc,omitempty"  yaml:"encryptionEnc,omitempty"  jsonschema:"JWE content-encryption algorithm (e.g. A256GCM). Required when encryptionAlg is set."`
}

// Certificate is a user-supplied certificate input.
type Certificate struct {
	Type  CertificateType `json:"type,omitempty"  yaml:"type,omitempty"  jsonschema:"Certificate type (PEM, JWK, etc.)."`
	Value string          `json:"value,omitempty" yaml:"value,omitempty" jsonschema:"Certificate value in the format specified by type."`
}

// OAuthProfile is the persistence shape (OAUTH_PROFILE JSONB column).
type OAuthProfile struct {
	RedirectURIs                       []string            `json:"redirectUris"`
	GrantTypes                         []string            `json:"grantTypes"`
	ResponseTypes                      []string            `json:"responseTypes"`
	TokenEndpointAuthMethod            string              `json:"tokenEndpointAuthMethod"`
	PKCERequired                       bool                `json:"pkceRequired"`
	PublicClient                       bool                `json:"publicClient"`
	RequirePushedAuthorizationRequests bool                `json:"requirePushedAuthorizationRequests"`
	DPoPBoundAccessTokens              bool                `json:"dpopBoundAccessTokens"`
	IncludeActClaim                    bool                `json:"includeActClaim"`
	Token                              *OAuthTokenConfig   `json:"token,omitempty"`
	Scopes                             []string            `json:"scopes,omitempty"`
	UserInfo                           *UserInfoConfig     `json:"userInfo,omitempty"`
	ScopeClaims                        map[string][]string `json:"scopeClaims,omitempty"`
	Certificate                        *Certificate        `json:"certificate,omitempty"`
	AcrValues                          []string            `json:"acrValues,omitempty"`
}

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

// AssertionConfig is the entity-level assertion config; token configs fall back to it.
type AssertionConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty" yaml:"validityPeriod,omitempty" jsonschema:"Assertion validity period in seconds."`
	UserAttributes []string `json:"userAttributes,omitempty" yaml:"userAttributes,omitempty" jsonschema:"User attributes to include in the assertion."`
}

// LoginConsentConfig is the login consent configuration.
type LoginConsentConfig struct {
	ValidityPeriod int64 `json:"validityPeriod" yaml:"validityPeriod" jsonschema:"Consent validity period in seconds. 0 means never expire."`
}

// Entity represents a unified identity principal returned by the entity provider.
type Entity struct {
	ID               string          `json:"id,omitempty"`
	Category         EntityCategory  `json:"category,omitempty"`
	Type             string          `json:"type,omitempty"`
	State            EntityState     `json:"state,omitempty"`
	OUID             string          `json:"ouId,omitempty"`
	OUHandle         string          `json:"ouHandle,omitempty"`
	Attributes       json.RawMessage `json:"attributes,omitempty"`
	SystemAttributes json.RawMessage `json:"systemAttributes,omitempty"`
	IsReadOnly       bool            `json:"isReadOnly"`
}

// EntityGroup represents a group with basic information for entity group membership queries.
type EntityGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	OUID string `json:"ouId"`
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

// AttributeMapping defines how a single external IDP attribute maps to a local user attribute.
// ExternalAttribute is the source attribute name (may be a dot-notation path into a nested claim);
// LocalAttribute is the target user-type attribute.
type AttributeMapping struct {
	ExternalAttribute string `json:"externalAttribute" yaml:"external_attribute"`
	LocalAttribute    string `json:"localAttribute"    yaml:"local_attribute"`
}

// UserTypeResolution resolves which local user type an incoming identity maps to. This iteration
// supports only Default (a fixed user type); claim-driven resolution is added later as additional
// fields without a breaking change.
type UserTypeResolution struct {
	Default string `json:"default,omitempty" yaml:"default,omitempty"`
}

// UserTypeAttributeMapping holds the external-to-local attribute mappings for a single local user type.
type UserTypeAttributeMapping struct {
	UserType   string             `json:"userType,omitempty"   yaml:"user_type,omitempty"`
	Attributes []AttributeMapping `json:"attributes,omitempty" yaml:"attributes,omitempty"`
}

// AttributeConfiguration holds the user-type resolution and per-user-type attribute mappings for an
// identity provider.
type AttributeConfiguration struct {
	UserTypeResolution        *UserTypeResolution        `json:"userTypeResolution,omitempty"        yaml:"user_type_resolution,omitempty"`         //nolint:lll
	UserTypeAttributeMappings []UserTypeAttributeMapping `json:"userTypeAttributeMappings,omitempty" yaml:"user_type_attribute_mappings,omitempty"` //nolint:lll
}
