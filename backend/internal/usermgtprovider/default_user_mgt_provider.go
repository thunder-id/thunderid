// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package usermgtprovider implements the runtime-to-user-management boundary for user operations.
package usermgtprovider

import (
	"context"
	"encoding/json"

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
	return p.userSvc.CreateUser(ctx, u)
}

// UpdateUserCredentials sets the user's schema-defined credentials through the user service.
func (p *defaultUserMgtProvider) UpdateUserCredentials(
	ctx context.Context, userID string, credentials json.RawMessage,
) *tidcommon.ServiceError {
	return p.userSvc.UpdateUserCredentials(ctx, userID, credentials)
}

// ValidateRemoveUserCredential checks the invalidation through the user service, changing nothing.
func (p *defaultUserMgtProvider) ValidateRemoveUserCredential(
	ctx context.Context, userID, credentialType string,
) *tidcommon.ServiceError {
	return p.userSvc.ValidateRemoveUserCredential(ctx, userID, credentialType)
}

// RemoveUserCredential removes the user's credential through the user service.
func (p *defaultUserMgtProvider) RemoveUserCredential(
	ctx context.Context, userID, credentialType string,
) (string, *tidcommon.ServiceError) {
	return p.userSvc.RemoveUserCredential(ctx, userID, credentialType)
}
