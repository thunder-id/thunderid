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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package executor

import (
	"encoding/json"
	"errors"
	"fmt"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
)

var _ core.ExecutorInterface = (*federatedAuthResolverExecutor)(nil)

const (
	federatedAuthResolverLoggerComponentName = "FederatedAuthResolverExecutor"
)

// federatedAuthResolverExecutor resolves an ambiguous user after federated authentication.
// It reads stored candidate users from RuntimeData (set by the IdentifyingExecutor during
// disambiguation) and authenticates the user matching the selected organization handle.
//
// This executor is registered as ExecutorTypeAuthentication so the flow engine allows it
// to set AuthenticatedUser. It should only be used after a federated auth step (e.g., Google,
// GitHub) has already verified the user's identity.
type federatedAuthResolverExecutor struct {
	core.ExecutorInterface
	logger *log.Logger
}

// newFederatedAuthResolverExecutor creates a new instance of FederatedAuthResolverExecutor.
func newFederatedAuthResolverExecutor(
	flowFactory core.FlowFactoryInterface,
) *federatedAuthResolverExecutor {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, federatedAuthResolverLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameFederatedAuthResolver))

	base := flowFactory.CreateExecutor(ExecutorNameFederatedAuthResolver,
		common.ExecutorTypeAuthentication, nil, nil)

	return &federatedAuthResolverExecutor{
		ExecutorInterface: base,
		logger:            logger,
	}
}

// Execute resolves the disambiguated user from stored candidates using the provided user inputs.
// It filters candidates generically against all user inputs (e.g., ouHandle, userType, or any
// attribute), matching the same pattern used by the IdentifyingExecutor's filterUsersByAttributes.
func (f *federatedAuthResolverExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := f.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing federated auth resolver")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !f.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Required inputs not provided")
		execResp.Status = common.ExecUserInputRequired
		return execResp, nil
	}

	storedCandidates, hasCandidates := ctx.RuntimeData[common.RuntimeKeyCandidateUsers]
	if !hasCandidates || storedCandidates == "" {
		return nil, errors.New("no stored candidates found in runtime data")
	}

	var candidates []*entityprovider.Entity
	if err := json.Unmarshal([]byte(storedCandidates), &candidates); err != nil {
		return nil, fmt.Errorf("failed to deserialize candidate users: %w", err)
	}

	// Build filter from user inputs, restricted to the identifiers defined in the node's
	// required inputs. This prevents malicious clients from injecting arbitrary filter keys
	// (e.g., userID) to force-match a specific candidate.
	allowedInputs := make(map[string]bool)
	for _, input := range f.GetRequiredInputs(ctx) {
		allowedInputs[input.Identifier] = true
	}

	filters := make(map[string]interface{})
	for key, value := range ctx.UserInputs {
		if value != "" && allowedInputs[key] {
			filters[key] = value
		}
	}

	// Filter candidates using the same logic as the IdentifyingExecutor
	matched := filterUsersByAttributes(candidates, filters)

	if len(matched) == 0 {
		logger.Debug("No user matched the provided selection")
		execResp.Status = common.ExecUserInputRequired
		execResp.Inputs = f.GetRequiredInputs(ctx)
		execResp.FailureReason = failureReasonUserNotFound
		return execResp, nil
	}

	if len(matched) > 1 {
		// Still ambiguous — extract remaining disambiguation options and request more input
		options := extractDisambiguationOptions(matched)
		if len(options) == 0 {
			logger.Debug("Candidates are indistinguishable, no further disambiguation possible")
			execResp.Status = common.ExecFailure
			execResp.FailureReason = failureReasonFailedToIdentifyUser
			return execResp, nil
		}

		candidatesJSON, err := json.Marshal(matched)
		if err != nil {
			return nil, errors.New("failed to serialize remaining candidates")
		}
		execResp.RuntimeData[common.RuntimeKeyCandidateUsers] = string(candidatesJSON)
		execResp.Status = common.ExecUserInputRequired
		execResp.ForwardedData = map[string]interface{}{
			common.ForwardedDataKeyInputs: options,
		}

		logger.Debug("Multiple users still match, requesting additional attributes",
			log.Int("candidateCount", len(matched)))
		return execResp, nil
	}

	resolvedUser := matched[0]

	// Require a verified federated subject. The "sub" claim is set by the OAuthExecutor
	// after a successful token exchange with the federated IdP. Without it, there is no
	// proof of federated authentication, so we must fail closed.
	sub, hasSub := ctx.RuntimeData[userAttributeSub]
	if !hasSub || sub == "" {
		logger.Debug("No federated sub claim found, cannot authenticate")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = failureReasonUserNotAuthenticated
		return execResp, nil
	}

	execResp.Status = common.ExecComplete
	execResp.RuntimeData[userAttributeSub] = sub
	execResp.AuthenticatedUser = authncm.AuthenticatedUser{
		IsAuthenticated: true,
		UserID:          resolvedUser.ID,
		OUID:            resolvedUser.OUID,
		UserType:        resolvedUser.Type,
	}

	logger.Debug("Federated auth resolver completed successfully",
		log.MaskedString("userID", resolvedUser.ID))

	return execResp, nil
}
