// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
)

func TestNewGoogleOIDCAuthExecutor_Success(t *testing.T) {
	mockFlowFactory, mockIDPService, mockAuthnProvider := setupSocialAuthExecutorMock(t, ExecutorNameGoogleAuth)
	mockGoogleSvc := googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(t)

	executor := newGoogleOIDCAuthExecutor(mockFlowFactory, mockIDPService, mockGoogleSvc, mockAuthnProvider,
		entityprovidermock.NewEntityProviderInterfaceMock(t))

	assert.NotNil(t, executor)
	result, ok := executor.(*googleOIDCAuthExecutor)
	assert.True(t, ok)
	assert.NotNil(t, result.oidcAuthExecutorInterface)
	assert.Equal(t, mockGoogleSvc, result.googleAuthService)
}
