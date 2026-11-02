// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package usermgtprovider implements the runtime-to-user-management boundary for user operations.
package usermgtprovider

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type defaultUserMgtProvider struct {
	userSvc user.UserServiceInterface
}

// newDefaultUserMgtProvider creates a new default user management provider.
func newDefaultUserMgtProvider(userSvc user.UserServiceInterface) providers.UserMgtProvider {
	return &defaultUserMgtProvider{
		userSvc: userSvc,
	}
}

// CreateUser provisions a user through the user service.
func (p *defaultUserMgtProvider) CreateUser(
	ctx context.Context, u *providers.User,
) (*providers.User, *tidcommon.ServiceError) {
	if u == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	return p.userSvc.CreateUser(security.WithRuntimeContext(ctx), u)
}
