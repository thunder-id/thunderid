// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tests/mocks/authn/githubmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
)

func TestNewGithubOAuthExecutor_Success(t *testing.T) {
	mockFlowFactory, mockIDPService, mockAuthnProvider := setupSocialAuthExecutorMock(t, ExecutorNameGitHubAuth)
	mockGithubSvc := githubmock.NewGithubOAuthAuthnServiceInterfaceMock(t)

	executor := newGithubOAuthExecutor(mockFlowFactory, mockIDPService, mockGithubSvc, mockAuthnProvider,
		entityprovidermock.NewEntityProviderInterfaceMock(t))

	assert.NotNil(t, executor)
	result, ok := executor.(*githubOAuthExecutor)
	assert.True(t, ok)
	assert.NotNil(t, result.oAuthExecutorInterface)
	assert.Equal(t, mockGithubSvc, result.githubAuthService)
}
