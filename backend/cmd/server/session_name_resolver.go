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

package main

import (
	"context"

	"github.com/thunder-id/thunderid/internal/application"
	sessionmgt "github.com/thunder-id/thunderid/internal/flow/session/mgt"
	"github.com/thunder-id/thunderid/internal/user"
)

// sessionNameResolver resolves user and application display names for the session listing. It uses
// the server's own service access, so names appear regardless of the caller's list permissions;
// a lookup that fails or finds nothing yields "" and the listing falls back to the id.
type sessionNameResolver struct {
	users user.UserServiceInterface
	apps  application.ApplicationServiceInterface
}

var _ sessionmgt.NameResolver = (*sessionNameResolver)(nil)

// newSessionNameResolver builds the resolver over the user and application services.
func newSessionNameResolver(users user.UserServiceInterface,
	apps application.ApplicationServiceInterface) *sessionNameResolver {
	return &sessionNameResolver{users: users, apps: apps}
}

// UserName returns the subject's display name, or "" when it cannot be resolved.
func (r *sessionNameResolver) UserName(ctx context.Context, userID string) string {
	u, svcErr := r.users.GetUser(ctx, userID, true)
	if svcErr != nil || u == nil {
		return ""
	}
	return u.Display
}

// AppName returns the application's name, or "" when it cannot be resolved.
func (r *sessionNameResolver) AppName(ctx context.Context, appID string) string {
	a, svcErr := r.apps.GetApplication(ctx, appID)
	if svcErr != nil || a == nil {
		return ""
	}
	return a.Name
}
