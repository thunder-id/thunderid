// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// allowAllAuthz returns an authorization mock that allows every action and every list, so tests not
// about authorization reach the logic they cover.
func allowAllAuthz(t interface {
	mock.TestingT
	Cleanup(func())
}) *sysauthzmock.SystemAuthorizationServiceInterfaceMock {
	m := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(t)
	m.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	m.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return(&sysauthz.AccessibleResources{AllAllowed: true}, nil).Maybe()
	return m
}
