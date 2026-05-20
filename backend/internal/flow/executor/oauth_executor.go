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
	"slices"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnoauth "github.com/thunder-id/thunderid/internal/authn/oauth"
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

// userInfoSkipAttributes contains the list of user info attributes to skip when mapping to context user.
var userInfoSkipAttributes = []string{"username", "sub", "id"}

// oAuthExecutorInterface defines the interface for OAuth authentication executors.
type oAuthExecutorInterface interface {
	core.ExecutorInterface
	BuildAuthorizeFlow(ctx *core.NodeContext, execResp *common.ExecutorResponse) error
	ProcessAuthFlowResponse(ctx *core.NodeContext, execResp *common.ExecutorResponse) error
	ResolveContextUser(ctx *core.NodeContext, execResp *common.ExecutorResponse,
		sub string, internalUser *entityprovider.Entity, isAmbiguous bool) (*authncm.AuthenticatedUser, error)
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
		log.Bool("isAuthenticated", execResp.AuthenticatedUser.IsAuthenticated))

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
	logger.Debug("Processing OAuth authentication response")

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

		logger.Error("Federated authentication failed", log.String("errorCode", svcErr.Code),
			log.String("errorDescription", svcErr.ErrorDescription.DefaultValue))
		return errors.New("federated authentication failed")
	}

	if basicResult == nil {
		logger.Error("authnProvider.AuthenticateUser returned nil result")
		return errors.New("OAuth authentication failed")
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
		logger.Error("Failed to resolve context user after OAuth authentication")
		return errors.New("unexpected error occurred while resolving user")
	}

	userInfo := systemutils.ConvertInterfaceMapToStringMap(basicResult.ExternalClaims)
	contextUser.Attributes = o.getContextUserAttributes(execResp, userInfo)
	execResp.AuthenticatedUser = *contextUser
	execResp.AuthUser = newAuthUser

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
	execResp *common.ExecutorResponse, sub string, internalUser *entityprovider.Entity, isAmbiguous bool) (
	*authncm.AuthenticatedUser, error) {
	if ctx.FlowType == common.FlowTypeAuthentication {
		return o.getContextUserForAuthentication(ctx, execResp, sub, internalUser, isAmbiguous)
	}
	return o.getContextUserForRegistration(ctx, execResp, sub, internalUser, isAmbiguous)
}

// getContextUserForAuthentication resolves the authenticated user in context for authentication flows.
func (o *oAuthExecutor) getContextUserForAuthentication(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, sub string, internalUser *entityprovider.Entity, isAmbiguous bool) (
	*authncm.AuthenticatedUser, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// If no local user is found, check if authentication without local user is allowed
	if internalUser == nil {
		if isAuthenticationWithoutLocalUserAllowed(ctx) {
			if execResp.RuntimeData == nil {
				execResp.RuntimeData = make(map[string]string)
			}

			if isAmbiguous {
				// Ambiguous user: exists in multiple OUs. Set sub for downstream
				// disambiguation but do NOT mark as eligible for provisioning since
				// the user already exists.
				logger.Debug("Ambiguous user detected, deferring to flow for disambiguation")
				execResp.Status = common.ExecComplete
				execResp.FailureReason = ""
				execResp.RuntimeData[userAttributeSub] = sub

				return &authncm.AuthenticatedUser{
					IsAuthenticated: false,
				}, nil
			}

			// Genuinely new user: no local account exists
			logger.Debug("User not found, but authentication is allowed without a local user")

			err := o.resolveUserTypeForAutoProvisioning(ctx, execResp)
			if err != nil {
				return nil, err
			}
			if execResp.Status == common.ExecFailure {
				return nil, nil
			}

			execResp.Status = common.ExecComplete
			execResp.FailureReason = ""
			execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning] = dataValueTrue
			execResp.RuntimeData[userAttributeSub] = sub

			return &authncm.AuthenticatedUser{
				IsAuthenticated: false,
			}, nil
		}

		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User not found"
		return nil, nil
	}

	// User found, proceed with authentication
	execResp.Status = common.ExecComplete
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	execResp.RuntimeData[userAttributeSub] = sub
	authenticatedUser := authncm.AuthenticatedUser{
		IsAuthenticated: true,
		UserID:          internalUser.ID,
		OUID:            internalUser.OUID,
		UserType:        internalUser.Type,
	}

	return &authenticatedUser, nil
}

// getContextUserForRegistration resolves the authenticated user in context for registration flows.
func (o *oAuthExecutor) getContextUserForRegistration(ctx *core.NodeContext,
	execResp *common.ExecutorResponse, sub string, internalUser *entityprovider.Entity, isAmbiguous bool) (
	*authncm.AuthenticatedUser, error) {
	logger := o.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	if isAmbiguous {
		// An ambiguous user (exists in multiple OUs) can still be provisioned into a new target
		// OU when cross-OU provisioning is explicitly allowed. The ProvisioningExecutor enforces
		// the same-OU duplicate guard, so we don't need to fail here.
		if isRegistrationWithExistingUserAllowed(ctx) && isCrossOUProvisioningAllowed(ctx) {
			logger.Debug("Ambiguous user detected, proceeding with cross-OU provisioning eligibility")
			execResp.Status = common.ExecComplete
			execResp.FailureReason = ""
			execResp.RuntimeData[userAttributeSub] = sub

			return &authncm.AuthenticatedUser{
				IsAuthenticated: false,
			}, nil
		}

		logger.Debug("Ambiguous user detected in registration flow, cannot proceed with registration")
		execResp.Status = common.ExecFailure
		execResp.FailureReason = "User identity is ambiguous and cannot be registered."
		return nil, nil
	}

	// If no local user is found, proceed with registration
	if internalUser == nil {
		logger.Debug("User not found for the provided sub claim. Proceeding with registration flow.")
		execResp.Status = common.ExecComplete
		execResp.FailureReason = ""
		execResp.RuntimeData[userAttributeSub] = sub

		return &authncm.AuthenticatedUser{
			IsAuthenticated: false,
		}, nil
	}

	// If a local user is found, check if registration with existing user is allowed
	if isRegistrationWithExistingUserAllowed(ctx) {
		if isCrossOUProvisioningAllowed(ctx) {
			// Allow the flow to continue so the ProvisioningExecutor can create the user in
			// the target OU. The same-OU duplicate guard is enforced by the ProvisioningExecutor
			// itself, which has access to the target OU context. We intentionally do not set
			// RuntimeKeySkipProvisioning here because we want provisioning to run.
			logger.Debug("User already exists, proceeding with cross-OU provisioning to target OU")
			execResp.Status = common.ExecComplete
			execResp.FailureReason = ""
			execResp.RuntimeData[userAttributeSub] = sub

			return &authncm.AuthenticatedUser{
				IsAuthenticated: false,
			}, nil
		}

		logger.Debug("User already exists, but registration flow is allowed to continue")
		execResp.Status = common.ExecComplete
		execResp.FailureReason = ""
		execResp.RuntimeData[common.RuntimeKeySkipProvisioning] = dataValueTrue

		return &authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          internalUser.ID,
			OUID:            internalUser.OUID,
			UserType:        internalUser.Type,
		}, nil
	}

	// Fail the execution as a unique user is found in the system.
	execResp.Status = common.ExecFailure
	execResp.FailureReason = "User already exists with the provided sub claim."
	return nil, nil
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
