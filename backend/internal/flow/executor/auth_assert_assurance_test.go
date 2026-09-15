// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

// TestCheckAssurance_MaxAgeZeroFreshAuthSatisfied covers max_age=0 answered by an authentication
// that just completed. The SSO-Check node and the authorize endpoint reject max_age=0 outright,
// because both decide whether to reuse an existing authentication and zero admits none. This node
// asks a different question — is the authentication being asserted fresh enough — and a
// just-completed one is, so it must not be rejected. Otherwise max_age=0 would be unsatisfiable
// even immediately after the re-authentication it triggered.
func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_MaxAgeZeroFreshAuthSatisfied() {
	ctx := assuranceCtx(map[string]string{common.RuntimeKeyMaxAge: "0"})
	assert.Nil(suite.T(), suite.executor.checkAssurance(ctx, suite.executor.logger),
		"a freshly completed authentication must satisfy max_age=0")
}

// TestCheckAssurance_MaxAgeZeroStaleSessionRejected is the counterpart: a reused session carrying
// an older auth_time does not satisfy max_age=0, so the assertion is refused.
func (suite *AuthAssertExecutorTestSuite) TestCheckAssurance_MaxAgeZeroStaleSessionRejected() {
	ctx := assuranceCtx(map[string]string{
		common.RuntimeKeyMaxAge:   "0",
		common.RuntimeKeyAuthTime: strconv.FormatInt(time.Now().UTC().Unix()-60, 10),
	})
	svcErr := suite.executor.checkAssurance(ctx, suite.executor.logger)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrInteractionRequired.Code, svcErr.Code)
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
