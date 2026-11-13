// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usermgtprovider

import (
	"context"
	"encoding/json"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// disabledUserMgtProvider is a user management provider that rejects all operations.
type disabledUserMgtProvider struct{}

// NewDisabledUserMgtProvider creates a user management provider that rejects every operation. It is
// the fallback for deployments and embedders that do not provision users from the runtime.
func NewDisabledUserMgtProvider() providers.UserMgtProvider {
	return &disabledUserMgtProvider{}
}

func (p *disabledUserMgtProvider) CreateUser(
	_ context.Context, _ *providers.User,
) (*providers.User, *tidcommon.ServiceError) {
	return nil, &ErrorUserProvisioningDisabled
}

// UpdateUserCredentials rejects the update, since the provider is disabled.
func (p *disabledUserMgtProvider) UpdateUserCredentials(
	_ context.Context, _ string, _ json.RawMessage,
) *tidcommon.ServiceError {
	return &ErrorUserManagementDisabled
}

// ValidateRemoveUserCredential rejects the invalidation, since the provider is disabled.
func (p *disabledUserMgtProvider) ValidateRemoveUserCredential(
	_ context.Context, _, _ string,
) *tidcommon.ServiceError {
	return &ErrorUserManagementDisabled
}

// RemoveUserCredential rejects the invalidation, since the provider is disabled.
func (p *disabledUserMgtProvider) RemoveUserCredential(
	_ context.Context, _, _ string,
) (string, *tidcommon.ServiceError) {
	return "", &ErrorUserManagementDisabled
}
