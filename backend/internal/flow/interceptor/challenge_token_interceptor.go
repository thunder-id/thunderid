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

package interceptor

import (
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// challengeTokenInterceptor validates an incoming challenge token against a stored hash on
// PRE_REQUEST and rotates the token on POST_REQUEST. It uses SharedData to persist the hash
// across requests within a flow instance.
type challengeTokenInterceptor struct {
	core.InterceptorInterface
	postRequestStatuses []providers.FlowStatus
	logger              *log.Logger
}

var _ core.InterceptorInterface = (*challengeTokenInterceptor)(nil)

// newChallengeTokenInterceptor creates a new challenge token interceptor.
func newChallengeTokenInterceptor(flowFactory core.FlowFactoryInterface) *challengeTokenInterceptor {
	base := flowFactory.CreateInterceptor(ChallengeTokenInterceptor, true, PriorityDefault)

	return &challengeTokenInterceptor{
		InterceptorInterface: base,
		postRequestStatuses: []providers.FlowStatus{
			providers.FlowStatusIncomplete,
		},
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, ChallengeTokenInterceptor)),
	}
}

// Execute delegates to the appropriate handler based on the interceptor mode.
func (c *challengeTokenInterceptor) Execute(ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	switch ctx.Mode {
	case providers.InterceptorModePreRequest:
		return c.validateChallengeToken(ctx)
	case providers.InterceptorModePostRequest:
		return c.rotateChallengeToken(ctx)
	default:
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
		}, nil
	}
}

// validateChallengeToken checks the incoming challenge token against the stored hash in SharedData.
// If no hash is stored yet (first request of a flow), validation is skipped.
func (c *challengeTokenInterceptor) validateChallengeToken(
	ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	storedHash := ctx.SharedData[sharedDataKeyChallengeTokenHash]
	if storedHash == "" {
		c.logger.Debug(ctx.Context, "No challenge token hash in shared data; skipping validation",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusComplete,
		}, nil
	}
	if c.shouldSkipValidation(ctx) {
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusComplete,
		}, nil
	}

	incomingToken := ctx.SharedData[common.InterceptorDataKeyChallengeTokenIn]
	if incomingToken == "" {
		c.logger.Debug(ctx.Context, "Challenge token is empty in the request",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  &ErrorChallengeTokenInvalid,
		}, nil
	}

	if !cryptolib.ValidateTokenHash(incomingToken, storedHash) {
		c.logger.Debug(ctx.Context, "Invalid challenge token provided in the request",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  &ErrorChallengeTokenInvalid,
		}, nil
	}

	return &common.InterceptorResponse{
		Status: common.InterceptorStatusComplete,
	}, nil
}

// shouldSkipValidation checks whether challenge token validation should be skipped based on
// the current node's execution policy or the segment restart policy.
func (c *challengeTokenInterceptor) shouldSkipValidation(ctx *core.InterceptorContext) bool {
	if ctx.ExecutionPolicy != nil && ctx.ExecutionPolicy.SkipChallengeValidation {
		c.logger.Debug(ctx.Context, "Current node's execution policy set to skip challenge token validation",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return true
	}

	if ctx.AllowSegmentRestart {
		c.logger.Debug(ctx.Context, "Segment restart allowed; skipping challenge token validation",
			log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
		return true
	}

	return false
}

// rotateChallengeToken generates a new challenge token, stores its hash in SharedData, and
// returns the new token via EngineOutputs so the engine can include it in the flow step response.
func (c *challengeTokenInterceptor) rotateChallengeToken(
	ctx *core.InterceptorContext) (*common.InterceptorResponse, error) {
	if !c.shouldApplyForStatus(ctx) {
		return &common.InterceptorResponse{
			Status: common.InterceptorStatusComplete,
		}, nil
	}
	newToken, err := cryptolib.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge token: %w", err)
	}

	ctx.SharedData[sharedDataKeyChallengeTokenHash] = cryptolib.HashToken(newToken)

	// Clear the incoming token from shared data after rotation.
	delete(ctx.SharedData, common.InterceptorDataKeyChallengeTokenIn)

	return &common.InterceptorResponse{
		Status:         common.InterceptorStatusComplete,
		ChallengeToken: newToken,
	}, nil
}

// shouldApplyForStatus checks whether the current flow status matches one of the
// configured postRequestStatuses. Returns false if the status is not in the list.
func (c *challengeTokenInterceptor) shouldApplyForStatus(ctx *core.InterceptorContext) bool {
	return slices.Contains(c.postRequestStatuses, ctx.FlowStatus)
}
