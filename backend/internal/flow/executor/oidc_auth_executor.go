/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	"errors"
	"slices"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnoidc "github.com/thunder-id/thunderid/internal/authn/oidc"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	oidcAuthLoggerComponentName = "OIDCAuthExecutor"
)

// idTokenNonUserAttributes contains the list of non-user attributes that are expected in the ID token.
var idTokenNonUserAttributes = []string{"aud", "exp", "iat", "iss", "at_hash", "azp", "nonce", "sub"}

// oidcAuthExecutorInterface defines the interface for OIDC authentication executors.
type oidcAuthExecutorInterface interface {
	oAuthExecutorInterface
}

// oidcAuthExecutor implements the OIDCAuthExecutorInterface for handling generic OIDC authentication flows.
type oidcAuthExecutor struct {
	oAuthExecutorInterface
	authService   authnoidc.OIDCAuthnCoreServiceInterface
	authnProvider authnprovidermgr.AuthnProviderManagerInterface
	idpType       idp.IDPType
	logger        *log.Logger
}

var _ core.ExecutorInterface = (*oidcAuthExecutor)(nil)

// newOIDCAuthExecutor creates a new instance of OIDCAuthExecutor.
func newOIDCAuthExecutor(
	name string,
	defaultInputs, prerequisites []common.Input,
	flowFactory core.FlowFactoryInterface,
	idpService idp.IDPServiceInterface,
	authService authnoidc.OIDCAuthnCoreServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	idpType idp.IDPType,
) oidcAuthExecutorInterface {
	if name == "" {
		name = ExecutorNameOIDCAuth
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, oidcAuthLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, name))

	oauthSvcCast, ok := authService.(authnoauth.OAuthAuthnCoreServiceInterface)
	if !ok {
		panic("failed to cast OIDCAuthnService to OAuthAuthnCoreServiceInterface")
	}

	base := newOAuthExecutor(name, defaultInputs, prerequisites,
		flowFactory, idpService, oauthSvcCast, authnProvider, idpType)

	return &oidcAuthExecutor{
		oAuthExecutorInterface: base,
		authService:            authService,
		authnProvider:          authnProvider,
		idpType:                idpType,
		logger:                 logger,
	}
}

// Execute executes the OIDC authentication logic.
//
//nolint:dupl // OAuth and OIDC executors share the same execute skeleton with type-specific behavior.
func (o *oidcAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing OIDC authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	if !o.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for OIDC authentication executor is not provided")
		err := o.BuildAuthorizeFlow(ctx, execResp)
		if err != nil {
			return nil, err
		}
	} else {
		err := o.ProcessAuthFlowResponse(ctx, execResp)
		if err != nil {
			return nil, err
		}
	}

	logger.Debug(ctx.Context, "OIDC authentication executor execution completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthUser.IsAuthenticated()))

	return execResp, nil
}

// ProcessAuthFlowResponse processes the response from the OIDC authentication flow and authenticates the user.
func (o *oidcAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Processing OIDC authentication response")

	code, ok := ctx.UserInputs[userInputCode]
	if !ok || code == "" {
		execResp.AuthUser = authnprovidermgr.AuthUser{}
		return nil
	}

	// Validate the OAuth state parameter to prevent CSRF attacks.
	// State is validated only when the client sends it back. Clients that handle CSRF
	// protection client-side (e.g., via sessionStorage) may omit it.
	if returnedState, ok := ctx.UserInputs[userInputState]; ok && returnedState != "" {
		expectedState := ctx.RuntimeData[common.RuntimeKeyOAuthState]
		if returnedState != expectedState {
			logger.Debug(ctx.Context, "OAuth state mismatch")
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrInvalidOAuthState
			return nil
		}
		delete(ctx.RuntimeData, common.RuntimeKeyOAuthState)
	}

	idpID, err := o.GetIdpID(ctx)
	if err != nil {
		return err
	}

	existingCtxUserAttributes := make(map[string]interface{})
	if execResp.AuthUser.IsAuthenticated() {
		authUser, attributes, err := o.authnProvider.GetUserAttributes(ctx.Context, nil, nil, execResp.AuthUser)
		execResp.AuthUser = authUser
		if err != nil {
			logger.Warn(ctx.Context,
				"Failed to fetch user attributes for authenticated user, proceeding without attributes")
		} else {
			for key, value := range attributes.Attributes {
				existingCtxUserAttributes[key] = value
			}
		}
	}

	credentials := map[string]interface{}{
		"federated": &authncm.FederatedAuthCredential{
			IDPID:   idpID,
			IDPType: o.idpType,
			Code:    code,
		},
	}
	authUser, federatedAttributes, svcErr := o.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, nil, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecFailure
			execResp.Error = svcErr
			return nil
		}

		logger.Error(ctx.Context, "OIDC authentication failed", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("OIDC authentication failed")
	}

	// Validate nonce if configured
	if claimNonce, ok := federatedAttributes[userInputNonce]; ok && claimNonce != "" {
		expectedNonce := ctx.UserInputs[userInputNonce]
		if expectedNonce != "" && claimNonce != expectedNonce {
			execResp.Status = common.ExecFailure
			execResp.Error = &ErrNonceMismatch
			return nil
		}
	}

	if !validateFederatedIdentifierConsistency(ctx, federatedAttributes, existingCtxUserAttributes) {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrInvalidFederatedUser
		return nil
	}

	if len(federatedAttributes) > 0 {
		if execResp.RuntimeData == nil {
			execResp.RuntimeData = make(map[string]string)
		}
		for key, value := range federatedAttributes {
			execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
		}
	}

	if ctx.FlowType == common.FlowTypeAuthentication {
		if isAuthenticationWithoutLocalUserAllowed(ctx) {
			execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
		}
	} else if ctx.FlowType == common.FlowTypeRegistration {
		if isRegistrationWithExistingUserAllowed(ctx) {
			execResp.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser] = dataValueTrue
		}
	}

	execResp.Status = common.ExecComplete
	return nil
}

// getContextUserAttributes extracts user-facing attributes from the external claims map.
// TODO: Need to convert attributes as per the IDP to local attribute mapping when the support is implemented.
func (o *oidcAuthExecutor) getContextUserAttributes(execResp *common.ExecutorResponse,
	claims map[string]interface{}) map[string]interface{} {
	userClaims := make(map[string]interface{})

	for attr, val := range claims {
		if !slices.Contains(idTokenNonUserAttributes, attr) {
			userClaims[attr] = systemutils.ConvertInterfaceValueToString(val)
		}
	}

	// Append email to runtime data if available.
	if email, ok := userClaims[userAttributeEmail]; ok {
		if emailStr, ok := email.(string); ok && emailStr != "" {
			if execResp.RuntimeData == nil {
				execResp.RuntimeData = make(map[string]string)
			}
			execResp.RuntimeData[userAttributeEmail] = emailStr
		}
	}

	return userClaims
}
