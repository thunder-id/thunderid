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

	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/idp"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
)

// AuthenticationResponse represents the response after successful authentication.
type AuthenticationResponse struct {
	ID        string
	Type      string
	OUID      string
	Assertion string
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
