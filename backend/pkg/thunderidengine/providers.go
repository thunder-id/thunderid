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

import "context"

// ClientProvider resolves OAuth clients, entity groups, and applications for the engine runtime.
type ClientProvider interface {
	GetOAuthClientByClientID(ctx context.Context, clientID string) (*OAuthClient, error)
	GetTransitiveEntityGroups(ctx context.Context, entityID string) ([]EntityGroup, error)
	GetApplicationByID(ctx context.Context, appID string) (*Application, error)
}

// AuthnProvider authenticates users and resolves attributes for flow and token issuance.
type AuthnProvider interface {
	AuthenticateUser(ctx context.Context, credentials Credentials) (*AuthnResult, error)
	GetUserAttributes(ctx context.Context, userID string, attrs []string) (map[string]interface{}, error)
}

// AuthzProvider answers authorization decisions for executors and grant handlers.
type AuthzProvider interface {
	IsAuthorized(ctx context.Context, subjectID, action, resourceID string) (bool, error)
}

// ResourceProvider resolves authorization resources by URI.
type ResourceProvider interface {
	GetResource(ctx context.Context, resourceURI string) (*Resource, error)
}

// OUProvider resolves organization units and their ancestor chain.
type OUProvider interface {
	GetOU(ctx context.Context, ouID string) (*OrganizationUnit, error)
	GetOUAncestors(ctx context.Context, ouID string) ([]OrganizationUnit, error)
}

// IDPProvider resolves identity provider configuration.
type IDPProvider interface {
	GetIDPByID(ctx context.Context, id string) (*IdentityProvider, error)
	GetIDPByName(ctx context.Context, name string) (*IdentityProvider, error)
}

// FlowDefinitionProvider supplies flow definitions used by flow execution and metadata.
type FlowDefinitionProvider interface {
	GetFlowByID(ctx context.Context, id string) (*FlowDefinition, error)
	GetFlowByHandle(ctx context.Context, appID, handle string) (*FlowDefinition, error)
}

// ObservabilityProvider publishes runtime events when observability is enabled.
type ObservabilityProvider interface {
	IsEnabled() bool
	PublishEvent(evt *Event)
}

// RuntimeStore persists OAuth PAR, authorization requests/codes, and flow execution context.
type RuntimeStore interface {
	Store(ctx context.Context, parRequest PushedAuthorizationRequest, expirySeconds int64) (string, error)
	Consume(ctx context.Context, requestURI string) (PushedAuthorizationRequest, bool, error)
	AddRequest(ctx context.Context, authRequestContext AuthRequestContext) (string, error)
	GetRequest(ctx context.Context, key string) (bool, AuthRequestContext, error)
	ClearRequest(ctx context.Context, key string) error
	InsertAuthorizationCode(ctx context.Context, code AuthorizationCode) error
	ConsumeAuthorizationCode(ctx context.Context, authCodeString string) (bool, error)
	GetAuthorizationCode(ctx context.Context, authCodeString string) (*AuthorizationCode, error)
	StoreFlowContext(ctx context.Context, flowContext FlowContext, expirySeconds int64) error
	GetFlowContext(ctx context.Context, executionID string) (*FlowContext, error)
	UpdateFlowContext(ctx context.Context, flowContext FlowContext) error
	DeleteFlowContext(ctx context.Context, executionID string) error
}
