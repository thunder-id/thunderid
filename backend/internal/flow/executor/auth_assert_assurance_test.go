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

package executor

import (
	"context"
	"strconv"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

func assuranceCtx(runtimeData map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: "flow-assurance",
		RuntimeData: runtimeData,
	}
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_NoRequirements() {
	svcErr := suite.executor.checkAssurance(assuranceCtx(map[string]string{}), suite.executor.logger)
	assert.Nil(suite.T(), svcErr)
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_AcrMet() {
	ctx := assuranceCtx(map[string]string{
		common.RuntimeKeyRequestedAuthClasses: "urn:acr:pwd urn:acr:mfa",
		common.RuntimeKeySelectedAuthClass:    "urn:acr:mfa",
	})
	assert.Nil(suite.T(), suite.executor.checkAssurance(ctx, suite.executor.logger))
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_AcrNotMet() {
	ctx := assuranceCtx(map[string]string{
		common.RuntimeKeyRequestedAuthClasses: "urn:acr:mfa",
		common.RuntimeKeySelectedAuthClass:    "urn:acr:pwd",
	})
	svcErr := suite.executor.checkAssurance(ctx, suite.executor.logger)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrInteractionRequired.Code, svcErr.Code)
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_AcrRequestedButNoneSelected() {
	ctx := assuranceCtx(map[string]string{
		common.RuntimeKeyRequestedAuthClasses: "urn:acr:mfa",
	})
	svcErr := suite.executor.checkAssurance(ctx, suite.executor.logger)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrInteractionRequired.Code, svcErr.Code)
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_MaxAgeWithinLimit() {
	ctx := assuranceCtx(map[string]string{
		common.RuntimeKeyMaxAge:   "3600",
		common.RuntimeKeyAuthTime: strconv.FormatInt(time.Now().UTC().Unix()-100, 10),
	})
	assert.Nil(suite.T(), suite.executor.checkAssurance(ctx, suite.executor.logger))
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_MaxAgeExceeded() {
	ctx := assuranceCtx(map[string]string{
		common.RuntimeKeyMaxAge:   "60",
		common.RuntimeKeyAuthTime: strconv.FormatInt(time.Now().UTC().Unix()-3600, 10),
	})
	svcErr := suite.executor.checkAssurance(ctx, suite.executor.logger)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrInteractionRequired.Code, svcErr.Code)
}

// TestCheckAssurance_MaxAgeFreshAuth covers the fresh-auth path where no auth_time is recorded:
// the subject authenticated in this execution, so max_age is trivially satisfied.
func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_MaxAgeFreshAuth() {
	ctx := assuranceCtx(map[string]string{common.RuntimeKeyMaxAge: "60"})
	assert.Nil(suite.T(), suite.executor.checkAssurance(ctx, suite.executor.logger))
}

func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_MaxAgeMalformedIgnored() {
	ctx := assuranceCtx(map[string]string{common.RuntimeKeyMaxAge: "not-a-number"})
	assert.Nil(suite.T(), suite.executor.checkAssurance(ctx, suite.executor.logger))
}

// TestExecute_BelowAssurance_InteractionRequired verifies the executor fails with
// interaction_required (rather than issuing an assertion) when the requested acr_values is
// not satisfied.
func (suite *AuthAssertExecutorTestSuite) TestExecute_BelowAssurance_InteractionRequired() {
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: "flow-assurance",
		FlowType:    providers.FlowTypeAuthentication,
		AuthUser:    newCredentialsAuthAuthenticatedUser(),
		RuntimeData: map[string]string{
			common.RuntimeKeyRequestedAuthClasses: "urn:acr:mfa",
			common.RuntimeKeySelectedAuthClass:    "urn:acr:pwd",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrInteractionRequired.Code, resp.Error.Code)
	assert.Empty(suite.T(), resp.Assertion)
}
