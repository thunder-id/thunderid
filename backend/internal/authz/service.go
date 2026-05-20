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

// Package authz provides authorization service functionality.
package authz

import (
	"context"

	"github.com/thunder-id/thunderid/internal/authz/engine"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "AuthorizationService"

// AuthorizationServiceInterface defines the interface for authorization operations.
// This is the public interface exposed to external consumers.
type AuthorizationServiceInterface interface {
	// GetAuthorizedPermissions returns the subset of requested permissions
	// that the entity (directly or through groups) is authorized for.
	GetAuthorizedPermissions(
		ctx context.Context,
		request GetAuthorizedPermissionsRequest,
	) (*GetAuthorizedPermissionsResponse, *serviceerror.ServiceError)
}

// authorizationService is the default implementation of AuthorizationServiceInterface.
type authorizationService struct {
	engine engine.AuthorizationEngine
}

// newAuthorizationService creates a new instance of authorizationService.
func newAuthorizationService(engine engine.AuthorizationEngine) AuthorizationServiceInterface {
	return &authorizationService{
		engine: engine,
	}
}

// GetAuthorizedPermissions returns the subset of requested permissions that the entity is authorized for.
func (s *authorizationService) GetAuthorizedPermissions(
	ctx context.Context,
	request GetAuthorizedPermissionsRequest,
) (*GetAuthorizedPermissionsResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Evaluating authorization request",
		log.MaskedString(log.LoggerKeyUserID, request.EntityID),
		log.Int("groupCount", len(request.GroupIDs)),
		log.Int("requestedPermissionCount", len(request.RequestedPermissions)))

	// Handle nil group IDs
	if request.GroupIDs == nil {
		request.GroupIDs = []string{}
	}

	// Return empty list if no permissions requested (optimization)
	if len(request.RequestedPermissions) == 0 {
		return &GetAuthorizedPermissionsResponse{
			AuthorizedPermissions: []string{},
		}, nil
	}

	// Delegate to engine (engine/underlying service handles validation)
	authorizedPerms, err := s.engine.GetAuthorizedPermissions(
		ctx,
		request.EntityID,
		request.GroupIDs,
		request.RequestedPermissions,
	)
	if err != nil {
		logger.Error("Authorization evaluation failed",
			log.MaskedString(log.LoggerKeyUserID, request.EntityID),
			log.Int("groupCount", len(request.GroupIDs)),
			log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	logger.Debug("Authorization evaluation completed",
		log.MaskedString(log.LoggerKeyUserID, request.EntityID),
		log.Int("groupCount", len(request.GroupIDs)),
		log.Int("requestedCount", len(request.RequestedPermissions)),
		log.Int("authorizedCount", len(authorizedPerms)))

	return &GetAuthorizedPermissionsResponse{
		AuthorizedPermissions: authorizedPerms,
	}, nil
}
