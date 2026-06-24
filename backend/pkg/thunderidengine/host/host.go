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

// Package host defines the public, dependency-free SDK contract an embedding application
// implements to plug its own identity source into the engine.
//
// It is deliberately free of any internal/* types so that any external Go application can
// implement these interfaces. The engine adapts these host interfaces to its internal
// provider interfaces itself (see WithHostActorProvider / WithHostAuthnProvider). Host
// implementations signal "no such record" by returning runtime.ErrNotFound.
package host

import (
	"context"
	"encoding/json"
)

// ActorProvider supplies the actors (users, applications, and OAuth/inbound clients) that the
// engine resolves while running flows and OAuth2/OIDC requests. All methods are read-only.
// Return runtime.ErrNotFound when a requested record does not exist.
type ActorProvider interface {
	// IdentifyEntity resolves a single entity ID from indexed attribute filters
	// (for example {"username": "alice"}). Returns runtime.ErrNotFound when no entity matches.
	IdentifyEntity(filters map[string]any) (*string, error)
	// GetEntity returns the entity (user/application) for the given ID, or nil when the engine
	// should treat it as absent. Credentials must never be returned.
	GetEntity(entityID string) (*Actor, error)
	// SearchEntities returns all entities matching the given filters (possibly empty).
	SearchEntities(filters map[string]any) ([]*Actor, error)
	// GetApplication returns the application registration for the given application ID.
	GetApplication(ctx context.Context, appID string) (*Application, error)
	// GetInboundClientByEntityID returns the inbound (OAuth) client for the given entity ID.
	GetInboundClientByEntityID(ctx context.Context, entityID string) (*InboundClient, error)
	// GetInboundClientByClientID returns the inbound (OAuth) client for the given client_id.
	GetInboundClientByClientID(ctx context.Context, clientID string) (*InboundClient, error)
	// GetEntityType returns metadata for the given entity type, or runtime.ErrNotFound.
	GetEntityType(ctx context.Context, typeID string) (*EntityType, error)
}

// AuthnProvider performs credential authentication and attribute retrieval against the
// embedder's identity source.
type AuthnProvider interface {
	// Authenticate verifies the supplied identifiers/credentials. On success it returns an
	// AuthnResult with Authenticated set true and an AuthToken the engine later passes to
	// GetAttributes.
	Authenticate(
		ctx context.Context,
		identifiers, credentials map[string]any,
		metadata *AuthnMetadata,
	) (*AuthnResult, error)
	// GetAttributes returns the attributes for a previously authenticated subject, identified by
	// the AuthToken from a prior Authenticate call.
	GetAttributes(
		ctx context.Context,
		token string,
		requested *RequestedAttributes,
		metadata *GetAttributesMetadata,
	) (*GetAttributesResult, error)
}

// Actor is a resolved user or application. Attributes is the entity's public attribute set as
// raw JSON; it must never contain credentials.
type Actor struct {
	ID               string
	EntityType       string
	OUID             string
	Attributes       json.RawMessage
	SystemAttributes json.RawMessage
}

// Application is an OAuth application registration.
type Application struct {
	ID       string
	Name     string
	OUID     string
	EntityID string
	// ThemeID and LayoutID optionally reference declarative theme/layout resources used by
	// flow metadata design resolution.
	ThemeID  string
	LayoutID string
}

// RoleProvider supplies role and permission data for authorization and token assertion flows.
// All methods are read-only. External embedders implement this instead of the full in-tree
// RoleService when they manage roles in their own store.
type RoleProvider interface {
	// GetAuthorizedPermissions returns the subset of requestedPermissions the subject is allowed
	// to exercise. entityID is the authenticated user; groups are the subject's group IDs.
	GetAuthorizedPermissions(
		ctx context.Context, entityID string, groups []string, requestedPermissions []string,
	) ([]string, error)
	// GetUserRoles returns role names assigned to the entity directly or via group membership.
	GetUserRoles(ctx context.Context, entityID string, groupIDs []string) ([]string, error)
}

// InboundClient is the OAuth/OIDC client configuration the engine needs to process protocol
// requests for an application.
type InboundClient struct {
	Name                      string
	LogoURL                   string
	URL                       string
	TosURI                    string
	PolicyURI                 string
	ClientID                  string
	EntityID                  string
	ApplicationID             string
	OUID                      string
	GrantTypes                []string
	RedirectURIs              []string
	ResponseTypes             []string
	TokenEndpointAuthMethod   string
	PKCERequired              bool
	PublicClient              bool
	Certificate               *Certificate
	AuthFlowID                string
	RegistrationFlowID        string
	IsRegistrationFlowEnabled bool
	RecoveryFlowID            string
	IsRecoveryFlowEnabled     bool
}

// Certificate holds client certificate material (for example a PEM body or a JWKS reference).
type Certificate struct {
	Type  string
	Value string
}

// EntityType describes a type of entity (for example "user" or "application").
type EntityType struct {
	ID   string
	Name string
}

// AuthnMetadata carries contextual data into an Authenticate call.
type AuthnMetadata struct {
	// AppMetadata is application-scoped metadata (for example configured client IDs).
	AppMetadata map[string]interface{}
	// RuntimeMetadata is per-execution data accumulated during the flow.
	RuntimeMetadata map[string]string
}

// AuthnResult is the outcome of an Authenticate call.
type AuthnResult struct {
	// Authenticated reports whether the credentials were valid.
	Authenticated bool
	// UserID is the resolved subject's stable identifier.
	UserID string
	// AuthToken is an opaque handle the engine passes to GetAttributes to fetch attributes for
	// the authenticated subject. It may be the same as UserID.
	AuthToken string
	// Attributes optionally carries the subject's attributes inline (as raw JSON) so the engine
	// can skip a separate GetAttributes call.
	Attributes json.RawMessage
}

// RequestedAttributes selects which attributes the engine wants returned. A nil value means
// "all available attributes".
type RequestedAttributes struct {
	Names []string
}

// GetAttributesMetadata carries contextual data into a GetAttributes call.
type GetAttributesMetadata struct {
	AppMetadata     map[string]interface{}
	Locale          string
	RuntimeMetadata map[string]string
}

// GetAttributesResult carries the attributes returned for an authenticated subject.
type GetAttributesResult struct {
	Attributes json.RawMessage
}
