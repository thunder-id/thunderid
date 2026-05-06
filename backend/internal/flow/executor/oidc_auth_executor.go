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

	authnoauth "github.com/asgardeo/thunder/internal/authn/oauth"
	authnoidc "github.com/asgardeo/thunder/internal/authn/oidc"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/entitytype"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/idp"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/internal/system/log"
)

const (
	oidcAuthLoggerComponentName = "OIDCAuthExecutor"
)

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
		log.Bool("isAuthenticated", execResp.AuthUser.IsAuthenticated()))

	return execResp, nil
}

// ProcessAuthFlowResponse processes the response from the OIDC authentication flow and authenticates the user.
func (o *oidcAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Processing OIDC authentication response")

	code, ok := ctx.UserInputs[userInputCode]
	if !ok || code == "" {
		logger.Error("Federated authentication failed. Authorization code not found in user inputs")
		return errors.New("federated authentication failed")
	}

	idpID, err := o.GetIdpID(ctx)
	if err != nil {
		return err
	}

	authnData := &authnprovidercm.FederatedAuthnData{
		IDPID:   idpID,
		IDPType: o.idpType,
		OAuthCredential: authnprovidercm.OAuthCredential{
			Code: code,
		},
	}
	authUser, svcErr := o.authnProvider.AuthenticateUser(
		ctx.Context, authnprovidercm.AuthnDataTypeFederated, authnData, nil, nil, ctx.AuthUser)
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

	if !authUser.IsSet() {
		logger.Error("authnProvider.AuthenticateUser returned nil result")
		return errors.New("OIDC authentication failed")
	}

	execResp.AuthUser = authUser

	// Append email to runtime data if present in claims
	if email, ok := authUser.GetRuntimeAttribute(userAttributeEmail).(string); ok && email != "" {
		if execResp.RuntimeData == nil {
			execResp.RuntimeData = make(map[string]string)
		}
		execResp.RuntimeData[userAttributeEmail] = email
	}

	// Validate nonce if configured
	if nonce, ok := ctx.UserInputs[userInputNonce]; ok && nonce != "" {
		claimNonce, ok := authUser.GetRuntimeAttribute(userInputNonce).(string)
		if !ok || claimNonce != nonce {
			execResp.Status = common.ExecFailure
			execResp.FailureReason = "Nonce mismatch in ID token claims."
			return nil
		}
	}

	sub := authUser.GetLastFederatedSub()

	if authUser.IsLocalUserAmbiguous() {
		if execResp.RuntimeData == nil {
			execResp.RuntimeData = make(map[string]string)
		}
		execResp.RuntimeData[common.RuntimeKeyUserAmbiguous] = dataValueTrue
	}

	var internalUser *entityprovider.Entity
	if authUser.IsLocalUserExists() {
		internalUser = &entityprovider.Entity{
			ID:   authUser.GetUserID(),
			OUID: authUser.GetOUID(),
			Type: authUser.GetUserType(),
		}
	}

	err = o.ResolveContextUser(ctx, execResp, sub, internalUser, authUser.IsLocalUserAmbiguous())
	if err != nil {
		return err
	}
	if execResp.Status == common.ExecFailure {
		return nil
	}

	return nil
}
