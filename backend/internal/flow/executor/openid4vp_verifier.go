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
	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// openid4vpVerifier drives an OpenID4VP presentation as a flow step: it
// initiates a request (returning QR / deep-link data) and then polls until the
// wallet's response is verified, surfacing the verified holder as the
// authenticated user.
type openid4vpVerifier struct {
	providers.Executor
	service       openid4vp.OpenID4VPServiceInterface
	authnProvider providers.AuthnProviderManager
	logger        *log.Logger
}

func newOpenID4VPVerifier(
	flowFactory core.FlowFactoryInterface, service openid4vp.OpenID4VPServiceInterface,
	authnProvider providers.AuthnProviderManager,
) providers.Executor {
	base := flowFactory.CreateExecutor(
		ExecutorNameOpenID4VPVerify,
		providers.ExecutorTypeAuthentication,
		[]providers.Input{},
		[]providers.Input{},
		&providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyPresentationDefinitionID, IsRequired: true},
				{Property: common.NodePropertyAllowAuthenticationWithoutLocalUser},
			},
		},
	)
	return &openid4vpVerifier{
		Executor:      base,
		service:       service,
		authnProvider: authnProvider,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyExecutorName, ExecutorNameOpenID4VPVerify)),
	}
}

// Execute initiates the request on first entry and polls for the result on
// subsequent entries.
func (e *openid4vpVerifier) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	state := ctx.RuntimeData[common.RuntimeKeyOpenID4VPState]
	if state == "" {
		return e.initiate(ctx, execResp, logger)
	}
	return e.poll(ctx, state, execResp, logger)
}

// initiate starts a new request and returns the QR / deep-link data as a view.
func (e *openid4vpVerifier) initiate(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	definitionID := presentationDefinitionID(ctx)
	if definitionID == "" {
		logger.Error(ctx.Context, "OpenID4VP node is missing the presentation_definition_id property")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPDefinitionNotConfigured
		return execResp, nil
	}
	init, svcErr := e.service.Initiate(ctx.Context, definitionID)
	if svcErr != nil {
		logger.Error(ctx.Context, "Failed to initiate OpenID4VP request",
			log.String("definitionID", definitionID), log.String("errorCode", svcErr.Code))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPInitiateFailed
		return execResp, nil
	}

	execResp.RuntimeData[common.RuntimeKeyOpenID4VPState] = init.State
	setQRData(execResp, init.ClientID, init.RequestURI, init.WalletURI)
	execResp.Status = providers.ExecUserInputRequired
	return execResp, nil
}

// presentationDefinitionID reads the presentation_definition_id from node
// properties, returning "" when the node has not configured one.
func presentationDefinitionID(ctx *providers.NodeContext) string {
	if ctx.NodeProperties != nil {
		if v, ok := ctx.NodeProperties[propertyKeyPresentationDefinitionID].(string); ok {
			return v
		}
	}
	return ""
}

// setQRData populates the QR / deep-link payload for the client.
func setQRData(execResp *providers.ExecutorResponse, clientID, requestURI, walletURI string) {
	execResp.AdditionalData[common.DataOpenID4VPClientID] = clientID
	execResp.AdditionalData[common.DataOpenID4VPRequestURI] = requestURI
	execResp.AdditionalData[common.DataOpenID4VPWalletURI] = walletURI
}

// poll checks the request result, completing, failing, or continuing to wait.
func (e *openid4vpVerifier) poll(
	ctx *providers.NodeContext, state string, execResp *providers.ExecutorResponse, logger *log.Logger,
) (*providers.ExecutorResponse, error) {
	rs, svcErr := e.service.GetResult(ctx.Context, state)
	if svcErr != nil {
		logger.Debug(ctx.Context, "OpenID4VP request state not found or expired",
			log.String("errorCode", svcErr.Code))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPExpired
		return execResp, nil
	}

	switch rs.Status {
	case openid4vp.StatusExpired:
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPExpired
	case openid4vp.StatusCompleted:
		e.authenticate(ctx, rs, execResp, logger)
		if execResp.Status == providers.ExecFailure {
			return execResp, nil
		}
		delete(ctx.RuntimeData, common.RuntimeKeyOpenID4VPState)
		execResp.Status = providers.ExecComplete
	case openid4vp.StatusFailed:
		logger.Debug(ctx.Context, "OpenID4VP presentation verification failed",
			log.String("reason", rs.FailureReason))
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrOpenID4VPVerificationFailed,
			tidcommon.I18nMessage{
				Key:          ErrOpenID4VPVerificationFailed.ErrorDescription.Key,
				DefaultValue: "OpenID4VP presentation verification failed: {{param(reason)}}",
				Params:       map[string]string{"reason": rs.FailureReason},
			})
	default:
		// Still pending: keep the state, re-emit the QR data so the wait view
		// keeps rendering it across polls, and keep the client polling.
		execResp.RuntimeData[common.RuntimeKeyOpenID4VPState] = state
		setQRData(execResp, rs.ClientID, rs.RequestURI,
			openid4vp.WalletAuthorizationURI(rs.ClientID, rs.RequestURI))
		execResp.Status = providers.ExecUserInputRequired
	}
	return execResp, nil
}

// authenticate passes the verified presentation result through the authn provider
// so that the provider resolves the holder's entity and populates AuthUser via
// the standard chain, just like OAuth/OIDC/Passkey executors.
func (e *openid4vpVerifier) authenticate(
	ctx *providers.NodeContext, rs *openid4vp.RequestState,
	execResp *providers.ExecutorResponse, logger *log.Logger,
) {
	if rs.Result == nil {
		logger.Error(ctx.Context, "OpenID4VP completed state has no result")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPVerificationFailed
		return
	}
	credentials := map[string]interface{}{
		"openid4vp": &authncommon.OpenID4VPCredential{
			Subject: rs.Result.Subject,
			Claims:  rs.Result.Claims,
		},
	}

	authUser, authenticatedClaims, svcErr := e.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, nil, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		logger.Debug(ctx.Context, "OpenID4VP authentication through provider failed",
			log.String("errorCode", svcErr.Code))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPVerificationFailed
		return
	}

	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}

	if ctx.FlowType == providers.FlowTypeAuthentication && isAuthenticationWithoutLocalUserAllowed(ctx) {
		execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
	}
}
