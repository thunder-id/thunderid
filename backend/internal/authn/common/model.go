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

// Package common defines the common models and functions for authentication handling.
package common

import (
	"context"
	"time"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

// AuthenticatedUser represents the user information of an authenticated user.
type AuthenticatedUser struct {
	IsAuthenticated     bool
	UserID              string
	OUID                string
	UserType            string
	Attributes          map[string]interface{}
	AvailableAttributes *authnprovidercm.AttributesResponse
	Token               string
}

// AuthenticationContext represents the context of an authentication session.
type AuthenticationContext struct {
	context.Context
	SessionDataKey     string
	RequestQueryParams map[string]string
	AuthenticatedUser  AuthenticatedUser
	AuthTime           time.Time
}

// AuthenticationResponse represents the response after successful authentication.
type AuthenticationResponse struct {
	ID        string
	Type      string
	OUID      string
	Assertion string
}

// AuthenticatorMeta represents an authenticator's metadata including authentication factors.
type AuthenticatorMeta struct {
	// Name is the unique identifier for the authenticator (used in individual authentication APIs)
	Name string
	// Factors represents the authentication factors this authenticator validates
	Factors []AuthenticationFactor
	// AssociatedIDP is the optional identity provider type this authenticator is associated with.
	AssociatedIDP idp.IDPType
}

// AuthenticatorReference represents an engaged authenticator in the authentication flow.
type AuthenticatorReference struct {
	// Authenticator is the name of the authenticator
	Authenticator string `json:"authenticator"`
	// Step is the step number in the flow where this authenticator was engaged
	Step int `json:"step"`
	// Timestamp is the authenticator engaged time (Unix epoch time in seconds)
	Timestamp int64 `json:"timestamp"`
}

// FederatedAuthCredential carries the credential data for federated authentication.
type FederatedAuthCredential struct {
	IDPID   string
	IDPType idp.IDPType
	Code    string
}

// FederatedAuthResult is the result of a federated authentication attempt.
// InternalEntity is nil when no local user was found or when the user is ambiguous.
type FederatedAuthResult struct {
	Sub             string
	Claims          map[string]interface{}
	InternalEntity  *entityprovider.Entity
	IsAmbiguousUser bool
}

// FederatedAuthenticator defines the interface for federated authentication services.
// Authenticate performs the full flow (code exchange, claims extraction, internal user lookup).
// It returns an error only for actual failures; a missing internal user is NOT an error.
type FederatedAuthenticator interface {
	Authenticate(ctx context.Context, idpID, code string) (*FederatedAuthResult, *serviceerror.ServiceError)
}
