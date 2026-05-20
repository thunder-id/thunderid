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
	"errors"
	"slices"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnoidc "github.com/thunder-id/thunderid/internal/authn/oidc"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
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
	entityTypeService entitytype.EntityTypeServiceInterface,
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
		flowFactory, idpService, entityTypeService, oauthSvcCast, authnProvider, idpType)

	return &oidcAuthExecutor{
		oAuthExecutorInterface: base,
		authService:            authService,
		authnProvider:          authnProvider,
		idpType:                idpType,
		logger:                 logger,
	}
}

// Execute executes the OIDC authentication logic.
func (o *oidcAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing OIDC authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if !o.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Required inputs for OIDC authentication executor is not provided")
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

	logger.Debug("OIDC authentication executor execution completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthenticatedUser.IsAuthenticated))

	return execResp, nil
}

// ProcessAuthFlowResponse processes the response from the OIDC authentication flow and authenticates the user.
func (o *oidcAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Processing OIDC authentication response")

	code, ok := ctx.UserInputs[userInputCode]
	if !ok || code == "" {
		execResp.AuthenticatedUser = authncm.AuthenticatedUser{
			IsAuthenticated: false,
		}
		return nil
	}

	// Validate the OAuth state parameter to prevent CSRF attacks.
	// State is validated only when the client sends it back. Clients that handle CSRF
	// protection client-side (e.g., via sessionStorage) may omit it.
	if returnedState, ok := ctx.UserInputs[userInputState]; ok && returnedState != "" {
		expectedState := ctx.RuntimeData[common.RuntimeKeyOAuthState]
		if returnedState != expectedState {
			logger.Debug("OAuth state mismatch")
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Invalid OAuth state parameter"
			return nil
		}
		delete(ctx.RuntimeData, common.RuntimeKeyOAuthState)
	}

	idpID, err := o.GetIdpID(ctx)
	if err != nil {
		return err
	}

	credentials := map[string]interface{}{
		"federated": &authncm.FederatedAuthCredential{
			IDPID:   idpID,
			IDPType: o.idpType,
			Code:    code,
		},
	}
	newAuthUser, basicResult, svcErr := o.authnProvider.AuthenticateUser(
		ctx.Context, nil, credentials, nil, nil, ctx.AuthUser)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecFailure
			execResp.FailureReason = svcErr.ErrorDescription.DefaultValue
			return nil
		}

		logger.Error("OIDC authentication failed", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("OIDC authentication failed")
	}

	if basicResult == nil {
		logger.Error("authnProvider.AuthenticateUser returned nil result")
		return errors.New("OIDC authentication failed")
	}

	// Validate nonce if configured
	if nonce, ok := ctx.UserInputs[userInputNonce]; ok && nonce != "" {
		claimNonce := basicResult.ExternalClaims[userInputNonce]
		if claimNonce != nonce {
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Nonce mismatch in ID token claims."
			return nil
		}
	}

	if !validateFederatedIdentifierConsistency(ctx, basicResult) {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "Invalid federated user"
		return nil
	}

	sub := basicResult.ExternalSub

	if basicResult.IsAmbiguousUser {
		if execResp.RuntimeData == nil {
			execResp.RuntimeData = make(map[string]string)
		}
		execResp.RuntimeData[common.RuntimeKeyUserAmbiguous] = dataValueTrue
	}

	var internalUser *entityprovider.Entity
	if basicResult.IsExistingUser {
		internalUser = &entityprovider.Entity{
			ID:   basicResult.UserID,
			OUID: basicResult.OUID,
			Type: basicResult.UserType,
		}
	}

	contextUser, err := o.ResolveContextUser(ctx, execResp, sub, internalUser, basicResult.IsAmbiguousUser)
	if err != nil {
		return err
	}
	if execResp.Status == common.ExecFailure {
		return nil
	}
	if contextUser == nil {
		logger.Error("Failed to resolve context user after OIDC authentication")
		return errors.New("unexpected error occurred while resolving user")
	}

	contextUser.Attributes = o.getContextUserAttributes(execResp, basicResult.ExternalClaims)
	execResp.AuthenticatedUser = *contextUser
	execResp.AuthUser = newAuthUser

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
