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

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tests/mocks/authn/googlemock"
)

func TestNewGoogleOIDCAuthExecutor_Success(t *testing.T) {
	mockFlowFactory, mockIDPService, mockAuthnProvider := setupSocialAuthExecutorMock(t, ExecutorNameGoogleAuth)
	mockGoogleSvc := googlemock.NewGoogleOIDCAuthnServiceInterfaceMock(t)

	executor := newGoogleOIDCAuthExecutor(mockFlowFactory, mockIDPService, mockGoogleSvc, mockAuthnProvider)

	assert.NotNil(t, executor)
	result, ok := executor.(*googleOIDCAuthExecutor)
	assert.True(t, ok)
	assert.NotNil(t, result.oidcAuthExecutorInterface)
	assert.Equal(t, mockGoogleSvc, result.googleAuthService)
}
