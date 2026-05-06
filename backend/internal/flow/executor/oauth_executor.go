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
	"context"
	"errors"
	"fmt"

	authnoauth "github.com/asgardeo/thunder/internal/authn/oauth"
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
	oAuthLoggerComponentName            = "OAuthExecutor"
	errCannotProvisionUserAutomatically = "user not found and cannot provision automatically"
	errSelfRegistrationDisabled         = "self registration is disabled for the user type"
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

// oAuthExecutorInterface defines the interface for OAuth authentication executors.
type oAuthExecutorInterface interface {
	core.ExecutorInterface
	BuildAuthorizeFlow(ctx *core.NodeContext, execResp *common.ExecutorResponse) error
	ProcessAuthFlowResponse(ctx *core.NodeContext, execResp *common.ExecutorResponse) error
	ResolveContextUser(ctx *core.NodeContext, execResp *common.ExecutorResponse,
		sub string, internalUser *entityprovider.Entity, isAmbiguous bool) error
	GetIdpID(ctx *core.NodeContext) (string, error)
}

// oAuthExecutor implements the OAuthExecutorInterface for handling generic OAuth authentication flows.
type oAuthExecutor struct {
	core.ExecutorInterface
	authService       authnoauth.OAuthAuthnCoreServiceInterface
	authnProvider     authnprovidermgr.AuthnProviderManagerInterface
	idpType           idp.IDPType
	idpService        idp.IDPServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	logger            *log.Logger
}

var _ core.ExecutorInterface = (*oAuthExecutor)(nil)

// newOAuthExecutor creates a new instance of OAuthExecutor.
func newOAuthExecutor(
	name string,
	defaultInputs, prerequisites []common.Input,
	flowFactory core.FlowFactoryInterface,
	idpService idp.IDPServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
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
		entityTypeService: entityTypeService,
		logger:            logger,
	}
}

// Execute executes the OAuth authentication flow.
func (o *oAuthExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing OAuth authentication executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if ctx.FlowType != common.FlowTypeAuthentication && ctx.FlowType != common.FlowTypeRegistration {
		logger.Warn("Invalid flow type for OAuth executor. Skipping execution")
		execResp.Status = common.ExecComplete
		return execResp, nil
	}

	if !o.HasRequiredInputs(ctx, execResp) {
		logger.Debug("Required inputs for OAuth authentication executor is not provided")
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

	logger.Debug("OAuth authentication executor execution completed",
		log.String("status", string(execResp.Status)),
		log.Bool("isAuthenticated", execResp.AuthUser.IsAuthenticated()))

	return execResp, nil
}

// BuildAuthorizeFlow constructs the redirection to the external OAuth provider for user authentication.
func (o *oAuthExecutor) BuildAuthorizeFlow(ctx *core.NodeContext, execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Initiating OAuth authentication flow")

	idpID, err := o.GetIdpID(ctx)
	if err != nil {
		return err
	}

	authorizeURL, svcErr := o.authService.BuildAuthorizeURL(ctx.Context, idpID)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			execResp.Status = common.ExecFailure
			execResp.FailureReason = svcErr.ErrorDescription.DefaultValue
			return nil
		}

		logger.Error("Failed to build authorize URL", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("failed to build authorize URL")
	}

	// Get the idp name for additional data
	idpName, err := o.getIDPName(ctx.Context, idpID)
	if err != nil {
		return fmt.Errorf("failed to get idp name: %w", err)
	}

	// Set the response to redirect the user to the authorization URL.
	execResp.Status = common.ExecExternalRedirection
	execResp.RedirectURL = authorizeURL
	execResp.AdditionalData = map[string]string{
		common.DataIDPName: idpName,
	}

	return nil
}

// ProcessAuthFlowResponse processes the response from the OAuth authentication flow and authenticates the user.
func (o *oAuthExecutor) ProcessAuthFlowResponse(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Processing OAuth authentication response")

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

		logger.Error("Federated authentication failed", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("federated authentication failed")
	}

	execResp.AuthUser = authUser

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

	// TODO: userAttributeEmail was previously added to runtime data. we need to see if this is needed or not.
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
	logger.Debug("Retrieving IDP name for the given IDP ID")

	idp, svcErr := o.idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ClientErrorType {
			return "", fmt.Errorf("failed to get identity provider: %s", svcErr.ErrorDescription.DefaultValue)
		}

		logger.Error("Error while retrieving identity provider", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return "", errors.New("error while retrieving identity provider")
	}

	return idp.Name, nil
}

// ResolveContextUser resolves the authenticated user in context with the attributes.
func (o *oAuthExecutor) ResolveContextUser(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, sub string, localUser *entityprovider.Entity, isLocalUserAmbiguous bool) error {
	if ctx.FlowType == common.FlowTypeAuthentication {
		return o.getContextUserForAuthentication(ctx, execResp, sub, localUser, isLocalUserAmbiguous)
	}
	return o.getContextUserForRegistration(ctx, execResp, sub, localUser, isLocalUserAmbiguous)
}

// getContextUserForAuthentication resolves the authenticated user in context for authentication flows.
func (o *oAuthExecutor) getContextUserForAuthentication(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, sub string, localUser *entityprovider.Entity, isLocalUserAmbiguous bool) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if localUser != nil {
		execResp.Status = common.ExecComplete
		execResp.RuntimeData[userAttributeSub] = sub
		return nil
	}

	if !isAuthenticationAllowedWithoutLocalUser(ctx) {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User not found"
		return nil
	}

	if isLocalUserAmbiguous {
		// Ambiguous user: exists in multiple OUs. Set sub for downstream
		// disambiguation but do NOT mark as eligible for provisioning since
		// the user already exists.
		logger.Debug("Ambiguous user detected, deferring to flow for disambiguation")
		execResp.Status = common.ExecComplete
		execResp.RuntimeData[userAttributeSub] = sub
		return nil
	}

	logger.Debug("User not found, but authentication is allowed without a local user")

	if err := o.resolveUserTypeForAutoProvisioning(ctx, execResp); err != nil {
		return err
	}
	if execResp.Status == common.ExecFailure {
		return nil
	}

	execResp.Status = common.ExecComplete
	execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
	execResp.RuntimeData[userAttributeSub] = sub
	return nil
}

func isAuthenticationAllowedWithoutLocalUser(ctx *core.NodeContext) bool {
	allowAuthWithoutLocalUser := false
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser]; ok {
		if boolVal, ok := val.(bool); ok {
			allowAuthWithoutLocalUser = boolVal
		}
	}
	return allowAuthWithoutLocalUser
}

// getContextUserForRegistration resolves the authenticated user in context for registration flows.
func (o *oAuthExecutor) getContextUserForRegistration(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, sub string, localUser *entityprovider.Entity,
	isLocalUserAmbiguous bool) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if isLocalUserAmbiguous {
		logger.Debug("Ambiguous user detected in registration flow, cannot proceed with registration")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User identity is ambiguous and cannot be registered."
		return nil
	}

	if localUser == nil {
		logger.Debug("User not found for the provided sub claim. Proceeding with registration flow.")
		execResp.Status = common.ExecComplete
		execResp.RuntimeData[userAttributeSub] = sub
		return nil
	}

	if isRegistrationWithExistingUserAllowed(ctx) {
		if isCrossOUProvisioningAllowed(ctx) {
			// Allow the flow to continue so the ProvisioningExecutor can create the user in
			// the target OU. The same-OU duplicate guard is enforced by the ProvisioningExecutor
			// itself, which has access to the target OU context. We intentionally do not set
			// RuntimeKeySkipProvisioning here because we want provisioning to run.
			logger.Debug("User already exists, proceeding with cross-OU provisioning to target OU")
			execResp.Status = common.ExecComplete
			execResp.RuntimeData[userAttributeSub] = sub
			return nil
		}
		logger.Debug("User already exists, but registration flow is allowed to continue")
		execResp.Status = common.ExecComplete
		execResp.RuntimeData[common.RuntimeKeySkipProvisioning] = dataValueTrue
		return nil
	}

	execResp.Status = common.ExecFailure
	execResp.FailureReason = "User already exists with the provided sub claim."
	return nil
}

func isCrossOUProvisioningAllowed(ctx *core.NodeContext) bool {
	allowCrossOUProvisioning := false
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowCrossOUProvisioning]; ok {
		if boolVal, ok := val.(bool); ok {
			allowCrossOUProvisioning = boolVal
		}
	}
	return allowCrossOUProvisioning
}

func isRegistrationWithExistingUserAllowed(ctx *core.NodeContext) bool {
	allowRegistrationWithExistingUser := false
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowRegistrationWithExistingUser]; ok {
		if boolVal, ok := val.(bool); ok {
			allowRegistrationWithExistingUser = boolVal
		}
	}
	return allowRegistrationWithExistingUser
}

// resolveUserTypeForAutoProvisioning resolves the user type for auto provisioning in authentication flows.
func (o *oAuthExecutor) resolveUserTypeForAutoProvisioning(ctx *core.NodeContext,
	execResp *common.ExecutorResponse) error {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Resolving user type for automatic provisioning")

	if len(ctx.Application.AllowedUserTypes) == 0 {
		logger.Debug("No allowed user types configured for the application")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = errCannotProvisionUserAutomatically
		return nil
	}

	// Filter allowed user types to only those with self-registration enabled
	selfRegEnabledSchemas := make([]entitytype.EntityType, 0)
	for _, userType := range ctx.Application.AllowedUserTypes {
		entityType, svcErr := o.entityTypeService.GetEntityTypeByName(ctx.Context,
			entitytype.TypeCategoryUser, userType)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ClientErrorType {
				execResp.Status = common.ExecFailure
				execResp.FailureReason = svcErr.ErrorDescription.DefaultValue
				return nil
			}

			logger.Error("Error while retrieving user type", log.String("errorCode", svcErr.Code),
				log.String("description", svcErr.ErrorDescription.DefaultValue))
			return errors.New("error while retrieving user type")
		}
		if entityType.AllowSelfRegistration {
			selfRegEnabledSchemas = append(selfRegEnabledSchemas, *entityType)
		}
	}

	// Fail if no user types have self-registration enabled
	if len(selfRegEnabledSchemas) == 0 {
		logger.Debug("No user types with self-registration enabled, cannot provision automatically")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = errSelfRegistrationDisabled
		return nil
	}

	// Fail if multiple user types have self-registration enabled
	if len(selfRegEnabledSchemas) > 1 {
		logger.Debug("Multiple user types with self-registration enabled, cannot resolve user type automatically")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = errCannotProvisionUserAutomatically
		return nil
	}

	// Proceed with the single resolved user type
	// Add userType and ouID to runtime data
	execResp.RuntimeData[userTypeKey] = selfRegEnabledSchemas[0].Name
	execResp.RuntimeData[defaultOUIDKey] = selfRegEnabledSchemas[0].OUID
	return nil
}
