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
	"context"
	"errors"
	"fmt"
	"slices"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	oAuthLoggerComponentName = "OAuthExecutor"
)

// OAuthTokenResponse represents the response from a OAuth token endpoint.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// userInfoSkipAttributes contains the list of user info attributes to skip when mapping to context user.
var userInfoSkipAttributes = []string{"username", "sub", "id"}

// oAuthExecutorInterface defines the interface for OAuth authentication executors.
type oAuthExecutorInterface interface {
	core.ExecutorInterface
	BuildAuthorizeFlow(ctx *core.NodeContext, execResp *common.ExecutorResponse) error
	ProcessAuthFlowResponse(ctx *core.NodeContext, execResp *common.ExecutorResponse) error
	GetIdpID(ctx *core.NodeContext) (string, error)
}

// oAuthExecutor implements the OAuthExecutorInterface for handling generic OAuth authentication flows.
type oAuthExecutor struct {
	core.ExecutorInterface
	authService   authnoauth.OAuthAuthnCoreServiceInterface
	authnProvider authnprovidermgr.AuthnProviderManagerInterface
	idpType       idp.IDPType
	idpService    idp.IDPServiceInterface
	logger        *log.Logger
}

var _ core.ExecutorInterface = (*oAuthExecutor)(nil)

// newOAuthExecutor creates a new instance of OAuthExecutor.
func newOAuthExecutor(
	name string,
	defaultInputs, prerequisites []common.Input,
	flowFactory core.FlowFactoryInterface,
	idpService idp.IDPServiceInterface,
	authService authnoauth.OAuthAuthnCoreServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	idpType idp.IDPType,
) oAuthExecutorInterface {
	if name == "" {
		name = ExecutorNameOAuth
	}
	if len(defaultInputs) == 0 {
		defaultInputs = []common.Input{
			{
				Identifier: userInputCode,
				Type:       "string",
				Required:   true,
			},
		}
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, oAuthLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, name))

	base := flowFactory.CreateExecutor(name, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites)

	return &oAuthExecutor{
		ExecutorInterface: base,
		authService:       authService,
		authnProvider:     authnProvider,
		idpType:           idpType,
		idpService:        idpService,
		logger:            logger,
	}
}

// Execute executes the OAuth authentication flow.
//
//nolint:dupl // OAuth and OIDC executors share the same execute skeleton with type-specific behavior.
func (o *oAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing OAuth authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	if !o.HasRequiredInputs(ctx, execResp) {
		logger.Debug(ctx.Context, "Required inputs for OAuth authentication executor is not provided")
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

	logger.Debug(ctx.Context, "OAuth authentication executor execution completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthUser.IsAuthenticated()))

	return execResp, nil
}

// BuildAuthorizeFlow constructs the redirection to the external OAuth provider for user authentication.
func (o *oAuthExecutor) BuildAuthorizeFlow(ctx *core.NodeContext, execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Initiating OAuth authentication flow")

	idpID, err := o.GetIdpID(ctx)
	if err != nil {
		return err
	}

	authorizeURL, svcErr := o.authService.BuildAuthorizeURL(ctx.Context, idpID)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecFailure
			execResp.Error = svcErr
			return nil
		}

		logger.Error(ctx.Context, "Failed to build authorize URL", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("failed to build authorize URL")
	}

	// Get the idp name for additional data
	idpName, err := o.getIDPName(ctx.Context, idpID)
	if err != nil {
		return fmt.Errorf("failed to get idp name: %w", err)
	}

	// Generate a random state parameter for CSRF protection and append it to the authorize URL.
	state := systemutils.GenerateUUID()
	authorizeURL = authorizeURL + "&" + "state=" + state

	// Set the response to redirect the user to the authorization URL.
	execResp.Status = common.ExecExternalRedirection
	execResp.RedirectURL = authorizeURL
	execResp.AdditionalData = map[string]string{
		common.DataIDPName: idpName,
	}
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	execResp.RuntimeData[common.RuntimeKeyOAuthState] = state

	return nil
}

// ProcessAuthFlowResponse processes the response from the OAuth authentication flow and authenticates the user.
func (o *oAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Processing OAuth authentication response")

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

		logger.Error(ctx.Context, "Federated authentication failed", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("federated authentication failed")
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

// HasRequiredInputs checks if the required inputs are provided in the context and appends any
// missing inputs to the executor response. Returns true if required inputs are found, otherwise false.
func (o *oAuthExecutor) HasRequiredInputs(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
	if code, ok := ctx.UserInputs[userInputCode]; ok && code != "" {
		return true
	}

	return o.ExecutorInterface.HasRequiredInputs(ctx, execResp)
}

// GetIdpID retrieves the identity provider ID from the node properties.
func (o *oAuthExecutor) GetIdpID(ctx *core.NodeContext) (string, error) {
	if len(ctx.NodeProperties) > 0 {
		if val, ok := ctx.NodeProperties["idpId"]; ok {
			if idpID, valid := val.(string); valid && idpID != "" {
				return idpID, nil
			}
		}
	}
	return "", errors.New("idpId is not configured in node properties")
}

// getIDPName retrieves the name of the identity provider using its ID.
func (o *oAuthExecutor) getIDPName(ctx context.Context, idpID string) (string, error) {
	logger := o.logger
	logger.Debug(ctx, "Retrieving IDP name for the given IDP ID")

	idp, svcErr := o.idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			return "", fmt.Errorf("failed to get identity provider: %s", svcErr.ErrorDescription.DefaultValue)
		}

		logger.Error(ctx, "Error while retrieving identity provider", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return "", errors.New("error while retrieving identity provider")
	}

	return idp.Name, nil
}

// getContextUserAttributes extracts and returns user attributes from the user info map.
// TODO: Need to convert attributes as per the IDP to local attribute mapping when the support is implemented.
func (o *oAuthExecutor) getContextUserAttributes(execResp *common.ExecutorResponse,
	userInfo map[string]string) map[string]interface{} {
	attributes := make(map[string]interface{})
	for key, value := range userInfo {
		if !slices.Contains(userInfoSkipAttributes, key) {
			attributes[key] = value
		}
	}

	// Append email to runtime data if available.
	if email, ok := attributes[userAttributeEmail]; ok {
		if emailStr, ok := email.(string); ok && emailStr != "" {
			if execResp.RuntimeData == nil {
				execResp.RuntimeData = make(map[string]string)
			}
			execResp.RuntimeData[userAttributeEmail] = emailStr
		}
	}

	return attributes
}
