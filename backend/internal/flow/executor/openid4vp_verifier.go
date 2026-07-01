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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// openid4vpVerifierService is the subset of the OpenID4VP verifier service the
// executor depends on. openid4vp.OpenID4VPServiceInterface satisfies it.
type openid4vpVerifierService interface {
	Initiate(ctx context.Context, definitionID string) (*openid4vp.Initiation, *tidcommon.ServiceError)
	GetResult(ctx context.Context, state string) (*openid4vp.RequestState, *tidcommon.ServiceError)
}

// openid4vpVerifier drives an OpenID4VP presentation as a flow step: it
// initiates a request (returning QR / deep-link data) and then polls until the
// wallet's response is verified, surfacing the verified holder as the
// authenticated user.
type openid4vpVerifier struct {
	providers.Executor
	service           openid4vpVerifierService
	entityTypeService entitytype.EntityTypeServiceInterface
	authnProvider     providers.AuthnProviderManager
	logger            *log.Logger
}

// newOpenID4VPVerifier creates the OpenID4VP verifier executor. service
// may be nil when the verifier is disabled; the executor then fails cleanly
// when reached.
func newOpenID4VPVerifier(
	flowFactory core.FlowFactoryInterface, service openid4vpVerifierService,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authnProvider providers.AuthnProviderManager,
) providers.Executor {
	base := flowFactory.CreateExecutor(
		ExecutorNameOpenID4VPVerify, providers.ExecutorTypeAuthentication, []providers.Input{}, []providers.Input{})
	return &openid4vpVerifier{
		Executor:          base,
		service:           service,
		entityTypeService: entityTypeService,
		authnProvider:     authnProvider,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyExecutorName, ExecutorNameOpenID4VPVerify)),
	}
}

// Execute initiates the request on first entry and polls for the result on
// subsequent entries.
func (e *openid4vpVerifier) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if e.service == nil {
		logger.Error(ctx.Context, "OpenID4VP verifier service is not configured")
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrOpenID4VPNotConfigured
		return execResp, nil
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
	setQRData(execResp, init.ClientID, init.RequestURI)
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
func setQRData(execResp *providers.ExecutorResponse, clientID, requestURI string) {
	execResp.AdditionalData[common.DataOpenID4VPClientID] = clientID
	execResp.AdditionalData[common.DataOpenID4VPRequestURI] = requestURI
	execResp.AdditionalData[common.DataOpenID4VPWalletURI] = openid4vp.WalletAuthorizationURI(clientID, requestURI)
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
	case openid4vp.StatusCompleted:
		e.authenticate(ctx, state, execResp, logger)
		if execResp.Status == providers.ExecFailure {
			return execResp, nil
		}
		e.resolveUserType(ctx, execResp, logger)
		if execResp.Status == providers.ExecFailure {
			return execResp, nil
		}
		execResp.Status = providers.ExecComplete
	case openid4vp.StatusFailed:
		logger.Debug(ctx.Context, "OpenID4VP presentation verification failed",
			log.String("reason", rs.FailureReason))
		execResp.Status = providers.ExecFailure
		execResp.Error = tidcommon.CustomServiceError(ErrOpenID4VPVerificationFailed,
			tidcommon.I18nMessage{
				Key:          ErrOpenID4VPVerificationFailed.ErrorDescription.Key,
				DefaultValue: "OpenID4VP presentation verification failed: " + rs.FailureReason,
			})
	default:
		// Still pending: keep the state, re-emit the QR data so the wait view
		// keeps rendering it across polls, and keep the client polling.
		execResp.RuntimeData[common.RuntimeKeyOpenID4VPState] = state
		setQRData(execResp, rs.ClientID, rs.RequestURI)
		execResp.Status = providers.ExecUserInputRequired
	}
	return execResp, nil
}

// resolveUserType finds the single self-registration-enabled user type from the
// application's allowed types and writes it into RuntimeData so the downstream
// ProvisioningExecutor knows which entity type to create.
func (e *openid4vpVerifier) resolveUserType(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse, logger *log.Logger,
) {
	if e.entityTypeService == nil {
		return
	}
	var candidates []entitytype.EntityType
	for _, name := range ctx.Application.AllowedUserTypes {
		et, svcErr := e.entityTypeService.GetEntityTypeByName(ctx.Context, entitytype.TypeCategoryUser, name)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ClientErrorType {
				continue
			}
			logger.Error(ctx.Context, "Failed to retrieve user type", log.String("userType", name))
			return
		}
		if et.AllowSelfRegistration {
			candidates = append(candidates, *et)
		}
	}
	if len(candidates) == 1 {
		execResp.RuntimeData[userTypeKey] = candidates[0].Name
		execResp.RuntimeData[defaultOUIDKey] = candidates[0].OUID
	}
}

// authenticate passes the presentation session state through the authn provider
// so that the provider calls the OpenID4VP service and populates AuthUser via
// the standard chain, just like OAuth/OIDC/Passkey executors.
func (e *openid4vpVerifier) authenticate(
	ctx *providers.NodeContext, state string,
	execResp *providers.ExecutorResponse, logger *log.Logger,
) {
	credentials := map[string]interface{}{
		"openid4vp": &authncommon.OpenID4VPCredential{State: state},
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

	// Flag the holder for JIT provisioning when the app allows login without a
	// local user, mirroring the federated executors.
	if isAuthenticationWithoutLocalUserAllowed(ctx) {
		execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
	}
}
