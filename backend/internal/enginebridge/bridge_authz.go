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

package enginebridge

import (
	"context"

	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type authzBridge struct {
	provider thunderidengine.AuthzProvider
}

func newAuthzBridge(provider thunderidengine.AuthzProvider) *authzBridge {
	return &authzBridge{provider: provider}
}

func (b *authzBridge) GetAuthorizedPermissions(
	ctx context.Context, request authz.GetAuthorizedPermissionsRequest,
) (*authz.GetAuthorizedPermissionsResponse, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	if len(request.RequestedPermissions) == 0 {
		return &authz.GetAuthorizedPermissionsResponse{AuthorizedPermissions: []string{}}, nil
	}
	if request.GroupIDs == nil {
		request.GroupIDs = []string{}
	}
	authorized := make([]string, 0, len(request.RequestedPermissions))
	for _, permission := range request.RequestedPermissions {
		ok, err := b.isPermissionGranted(ctx, request.EntityID, request.GroupIDs, permission)
		if err != nil {
			return nil, providerError(err)
		}
		if ok {
			authorized = append(authorized, permission)
		}
	}
	return &authz.GetAuthorizedPermissionsResponse{AuthorizedPermissions: authorized}, nil
}

func (b *authzBridge) isPermissionGranted(
	ctx context.Context, entityID string, groupIDs []string, permission string,
) (bool, error) {
	if entityID != "" {
		granted, err := b.provider.IsAuthorized(ctx, entityID, permission, "")
		if err != nil {
			return false, err
		}
		if granted {
			return true, nil
		}
	}
	for _, groupID := range groupIDs {
		if groupID == "" {
			continue
		}
		granted, err := b.provider.IsAuthorized(ctx, groupID, permission, "")
		if err != nil {
			return false, err
		}
		if granted {
			return true, nil
		}
	}
	return false, nil
}

var _ authz.AuthorizationServiceInterface = (*authzBridge)(nil)
