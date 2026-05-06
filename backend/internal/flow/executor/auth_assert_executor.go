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
	"slices"
	"strconv"
	"strings"

	"github.com/asgardeo/thunder/internal/attributecache"
	"github.com/asgardeo/thunder/internal/authn/assert"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	oauth2const "github.com/asgardeo/thunder/internal/oauth/oauth2/constants"
	"github.com/asgardeo/thunder/internal/ou"
	"github.com/asgardeo/thunder/internal/role"
	"github.com/asgardeo/thunder/internal/system/config"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/internal/system/jose/jwt"
	"github.com/asgardeo/thunder/internal/system/log"
)

const (
	authAssertLoggerComponentName = "AuthAssertExecutor"
)

// authAssertExecutor is an executor that handles authentication assertions in the flow.
type authAssertExecutor struct {
	core.ExecutorInterface
	jwtService          jwt.JWTServiceInterface
	ouService           ou.OrganizationUnitServiceInterface
	authAssertGenerator assert.AuthAssertGeneratorInterface
	authnProvider       authnprovidermgr.AuthnProviderManagerInterface
	entityProvider      entityprovider.EntityProviderInterface
	attributeCacheSvc   attributecache.AttributeCacheServiceInterface
	roleService         role.RoleServiceInterface
	logger              *log.Logger
}

var _ core.ExecutorInterface = (*authAssertExecutor)(nil)

// newAuthAssertExecutor creates a new instance of AuthAssertExecutor.
func newAuthAssertExecutor(
	flowFactory core.FlowFactoryInterface,
	jwtService jwt.JWTServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	assertGenerator assert.AuthAssertGeneratorInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	entityProvider entityprovider.EntityProviderInterface,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	roleService role.RoleServiceInterface,
) *authAssertExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, authAssertLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAuthAssert))

	base := flowFactory.CreateExecutor(ExecutorNameAuthAssert, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{})

	return &authAssertExecutor{
		ExecutorInterface:   base,
		jwtService:          jwtService,
		ouService:           ouService,
		authAssertGenerator: assertGenerator,
		authnProvider:       authnProvider,
		entityProvider:      entityProvider,
		attributeCacheSvc:   attributeCacheSvc,
		roleService:         roleService,
		logger:              logger,
	}
}

// Execute executes the authentication assertion logic.
func (a *authAssertExecutor) Execute(ctx *core.NodeContext) (*common.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug("Executing authentication assertion executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if ctx.AuthUser.IsAuthenticated() {
		token, err := a.generateAuthAssertion(ctx, logger)
		if err != nil {
			return nil, err
		}

		logger.Debug("Generated JWT token for authentication assertion")

		execResp.Status = common.ExecComplete
		execResp.Assertion = token
	} else {
		execResp.Status = common.ExecFailure
		execResp.FailureReason = failureReasonUserNotAuthenticated
	}

	logger.Debug("Authentication assertion executor execution completed",
		log.String("status", string(execResp.Status)))

	return execResp, nil
}

// generateAuthAssertion generates the authentication assertion token.
func (a *authAssertExecutor) generateAuthAssertion(ctx *core.NodeContext, logger *log.Logger) (string, error) {
	tokenSub := ""
	if ctx.AuthUser.GetUserID() != "" {
		tokenSub = ctx.AuthUser.GetUserID()
	}

	jwtClaims := make(map[string]interface{})
	jwtConfig := config.GetServerRuntime().Config.JWT
	iss := jwtConfig.Issuer
	validityPeriod := int64(0)

	if ctx.Application.Assertion != nil {
		validityPeriod = ctx.Application.Assertion.ValidityPeriod
	}
	if validityPeriod == 0 {
		validityPeriod = jwtConfig.ValidityPeriod
	}

	authenticatorRefs := ctx.AuthUser.GetAuthenticatorReference()

	// Generate assertion from engaged authenticators
	if len(authenticatorRefs) > 0 {
		assertionResult, svcErr := a.authAssertGenerator.GenerateAssertion(authenticatorRefs)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				logger.Error("Failed to generate auth assertion",
					log.String("error", svcErr.Error.DefaultValue))
				return "", errors.New("something went wrong while generating auth assertion")
			}
			return "", errors.New("failed to generate auth assertion: " + svcErr.Error.DefaultValue)
		}

		jwtClaims["assurance"] = assertionResult.Context
	}

	// Include authorized permissions in JWT if present in runtime data
	// The "authorized_permissions" claim contains space-separated permission strings.
	// This claim will be present only if the authorization executor has run before this executor in the flow
	// and has set the authorized permissions in the runtime data.
	if permissions, exists := ctx.RuntimeData["authorized_permissions"]; exists && permissions != "" {
		jwtClaims["authorized_permissions"] = permissions
	}

	requiredAttributes := a.getRequiredUserAttributes(ctx)

	resolvedAttributes, attrErr := a.resolveUserAttributes(ctx, requiredAttributes)
	if attrErr != nil {
		return "", attrErr
	}

	if ttlSecondsStr, exists := ctx.RuntimeData[common.RuntimeKeyUserAttributesCacheTTLSeconds]; exists {
		// We are not in an App Native flow, so we need to cache the user attributes
		if len(resolvedAttributes) > 0 {
			ttlSeconds, err := strconv.Atoi(ttlSecondsStr)
			if err != nil {
				logger.Error("Failed to parse TTL seconds from runtime data",
					log.String("key", common.RuntimeKeyUserAttributesCacheTTLSeconds),
					log.String("ttlValue", ttlSecondsStr),
					log.String("error", err.Error()))
				return "", errors.New("something went wrong while processing attribute cache configuration")
			}
			attributeCache := &attributecache.AttributeCache{
				Attributes: resolvedAttributes,
				TTLSeconds: ttlSeconds,
			}
			result, creationErr := a.attributeCacheSvc.CreateAttributeCache(ctx.Context, attributeCache)
			if creationErr != nil {
				logger.Error("Failed to create attribute cache",
					log.String("error", creationErr.ErrorDescription.DefaultValue))
				return "", errors.New("failed to create attribute cache")
			}
			jwtClaims["aci"] = result.ID
		}
	} else {
		// We are in an App Native flow, so we need to add user attributes to the assertion
		for attrKey, attrVal := range resolvedAttributes {
			jwtClaims[attrKey] = attrVal
		}
	}

	jwtClaims["aud"] = ctx.AppID
	token, _, err := a.jwtService.GenerateJWT(tokenSub, iss, validityPeriod, jwtClaims, jwt.TokenTypeJWT, "")
	if err != nil {
		logger.Error("Failed to generate JWT token", log.String("error", err.Error.DefaultValue))
		return "", errors.New("failed to generate JWT token: " + err.Error.DefaultValue)
	}

	return token, nil
}

// getRequiredUserAttributes determines the list of user attribute keys that should be included in the
// assertion based on runtime and application configuration.
func (a *authAssertExecutor) getRequiredUserAttributes(ctx *core.NodeContext) (userAttributes []string) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	// Check if consent was recorded in this flow. If recorded, we should only include consented attributes
	// We check RuntimeKeyConsentID to determine if the consent executor ran and recorded a consent.
	if _, consentRecorded := ctx.RuntimeData[common.RuntimeKeyConsentID]; consentRecorded {
		// Get user attributes from consented attributes if consent was collected in this flow
		if consentedAttrsStr, exists := ctx.RuntimeData[common.RuntimeKeyConsentedAttributes]; exists {
			logger.Debug("Consent recorded with approved attributes")
			return strings.Fields(consentedAttrsStr)
		}

		// If consent was recorded but no attributes were approved, attributes are empty
		logger.Debug("Consent recorded but no attributes approved")
		return []string{}
	}

	// If consent was not recorded in this flow, check for essential/ optional attributes in runtime data.
	// If present, we should apply attribute filtering based on these attributes
	_, essentialExists := ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes]
	_, optionalExists := ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes]

	if essentialExists || optionalExists {
		userAttributes = []string{}
		logger.Debug("Essential/ optional attributes exists in runtime data. Applying attribute filtering")

		if essentialExists {
			logger.Debug("Adding required essential attributes to user attributes list")
			userAttributes = append(userAttributes,
				strings.Fields(ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes])...)
		}
		if optionalExists {
			logger.Debug("Adding required optional attributes to user attributes list")
			userAttributes = append(userAttributes,
				strings.Fields(ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes])...)
		}

		return userAttributes
	}

	// If consent was not recorded and no essential/ optional attributes specified, fallback to
	// application token config
	if ctx.Application.Assertion != nil {
		logger.Debug("Adding application token attributes to user attributes list")
		return ctx.Application.Assertion.UserAttributes
	}

	logger.Debug("No user attributes configured for inclusion in assertion")
	return []string{}
}

// resolveUserAttributes resolves the user attributes map from the requested attributes.
func (a *authAssertExecutor) resolveUserAttributes(ctx *core.NodeContext, requestedAttributes []string) (
	map[string]interface{}, error) {
	if len(requestedAttributes) == 0 {
		return nil, nil
	}

	attributes := make(map[string]interface{})
	var fetchedAttributes map[string]interface{}

	standardClaims := oauth2const.GetStandardClaims()

	for _, attr := range requestedAttributes {
		// Skip attributes that are handled separately
		if attr == oauth2const.UserAttributeGroups ||
			attr == oauth2const.UserAttributeRoles ||
			attr == oauth2const.ClaimUserType ||
			attr == oauth2const.ClaimOUID ||
			attr == oauth2const.ClaimOUName ||
			attr == oauth2const.ClaimOUHandle {
			continue
		}

		// Skip standard JWT claims if present in the user attributes
		if slices.Contains(standardClaims, attr) {
			continue
		}

		// Check runtime data as fallback
		if val, exists := ctx.RuntimeData[attr]; exists && val != "" {
			attributes[attr] = val
			continue
		}

		// Fetch from authentication provider
		if fetchedAttributes == nil {
			var err error
			metadata := a.buildGetAttributesMetadata(ctx)
			fetchedAttributes, err = a.getUserAttributesFromAuthnProvider(ctx.Context,
				requestedAttributes, metadata, ctx.AuthUser)
			if err != nil {
				return nil, err
			}
		}

		// Check for the attribute in attributes fetched from user/authentication provider
		if fetchedAttributes != nil {
			if val, ok := fetchedAttributes[attr]; ok {
				attributes[attr] = val
				continue
			}
		}
	}

	// Append computed attributes (groups, roles, userType, OU details)
	if err := a.appendComputedAttributes(ctx, requestedAttributes, attributes); err != nil {
		return nil, err
	}

	return attributes, nil
}

// appendComputedAttributes appends computed/derived attributes (groups, roles, userType, OU details) to the claims.
func (a *authAssertExecutor) appendComputedAttributes(
	ctx *core.NodeContext, requestedAttributes []string, attributes map[string]interface{}) error {
	groupsRequested := slices.Contains(requestedAttributes, oauth2const.UserAttributeGroups)
	rolesRequested := slices.Contains(requestedAttributes, oauth2const.UserAttributeRoles)

	// Fetch all user groups once if either groups or roles are requested.
	if (groupsRequested || rolesRequested) && ctx.AuthUser.GetUserID() != "" {
		allGroups, err := a.fetchAllUserGroups(ctx.AuthUser.GetUserID())
		if err != nil {
			return err
		}

		if groupsRequested {
			a.appendGroupsToClaims(allGroups, attributes)
		}

		if rolesRequested {
			if err := a.appendRolesToClaims(ctx, allGroups, attributes); err != nil {
				return err
			}
		}
	}

	// Add user type to the claims
	if slices.Contains(requestedAttributes, oauth2const.ClaimUserType) && ctx.AuthUser.GetUserType() != "" {
		attributes[oauth2const.ClaimUserType] = ctx.AuthUser.GetUserType()
	}

	// Add OU details to the claims
	ouAttributesConfigured := slices.Contains(requestedAttributes, oauth2const.ClaimOUID) ||
		slices.Contains(requestedAttributes, oauth2const.ClaimOUName) ||
		slices.Contains(requestedAttributes, oauth2const.ClaimOUHandle)
	if ouAttributesConfigured && ctx.AuthUser.GetOUID() != "" {
		if err := a.appendOUDetailsToClaims(
			ctx.Context, ctx.AuthUser.GetOUID(), attributes, requestedAttributes); err != nil {
			return err
		}
	}

	return nil
}

// getUserAttributesFromAuthnProvider retrieves user attributes from the authentication provider.
func (a *authAssertExecutor) getUserAttributesFromAuthnProvider(ctx context.Context,
	requestedAttributes []string, metadata *authnprovidercm.GetAttributesMetadata,
	authUser authnprovidermgr.AuthUser) (map[string]interface{}, error) {
	// Convert requested attributes from []string to *RequestedAttributes
	reqAttrs := &authnprovidercm.RequestedAttributes{
		Attributes:    make(map[string]*authnprovidercm.AttributeMetadataRequest),
		Verifications: nil,
	}
	for _, attrName := range requestedAttributes {
		reqAttrs.Attributes[attrName] = nil
	}

	_, res, svcErr := a.authnProvider.GetUserAttributes(ctx, reqAttrs, metadata, authUser)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			return nil, errors.New("something went wrong while fetching user attributes")
		}
		return nil, errors.New("failed to fetch user attributes: " + svcErr.ErrorDescription.DefaultValue)
	}

	// Extract attribute values from AttributesResponse
	attrs := make(map[string]interface{})
	if res != nil && res.Attributes != nil {
		for attrName, attrResp := range res.Attributes {
			if attrResp != nil {
				attrs[attrName] = attrResp.Value
			}
		}
	}
	return attrs, nil
}

// appendOUDetailsToClaims appends organization unit details to the JWT claims.
// Only adds attributes that are configured in userAttributes.
func (a *authAssertExecutor) appendOUDetailsToClaims(
	ctx context.Context, ouID string, jwtClaims map[string]interface{}, userAttributes []string) error {
	logger := a.logger.With(log.String(ouIDKey, ouID))

	organizationUnit, svcErr := a.ouService.GetOrganizationUnit(ctx, ouID)
	if svcErr != nil {
		logger.Error("Failed to fetch organization unit details",
			log.String(ouIDKey, ouID), log.Any("error", svcErr))
		return errors.New("something went wrong while fetching organization unit: " +
			svcErr.ErrorDescription.DefaultValue)
	}

	// Only add ouId if configured
	if slices.Contains(userAttributes, oauth2const.ClaimOUID) {
		jwtClaims[oauth2const.ClaimOUID] = organizationUnit.ID
	}

	// Only add ouName if configured
	if slices.Contains(userAttributes, oauth2const.ClaimOUName) && organizationUnit.Name != "" {
		jwtClaims[oauth2const.ClaimOUName] = organizationUnit.Name
	}

	// Only add ouHandle if configured
	if slices.Contains(userAttributes, oauth2const.ClaimOUHandle) && organizationUnit.Handle != "" {
		jwtClaims[oauth2const.ClaimOUHandle] = organizationUnit.Handle
	}

	return nil
}

// fetchAllUserGroups retrieves all groups a user belongs to, including groups inherited through
// nested group membership.
func (a *authAssertExecutor) fetchAllUserGroups(userID string) ([]entityprovider.EntityGroup, error) {
	if a.entityProvider == nil || userID == "" {
		return nil, nil
	}

	groups, err := a.entityProvider.GetTransitiveEntityGroups(userID)
	if err != nil {
		a.logger.Error("Failed to fetch transitive user groups",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Any("error", err))
		return nil, errors.New("something went wrong while fetching user groups: " + err.Error())
	}

	return groups, nil
}

// appendGroupsToClaims appends pre-fetched user groups to the JWT claims.
func (a *authAssertExecutor) appendGroupsToClaims(
	groups []entityprovider.EntityGroup, jwtClaims map[string]interface{}) {
	userGroups := make([]string, 0, len(groups))
	for _, group := range groups {
		userGroups = append(userGroups, group.Name)
	}

	if len(userGroups) > 0 {
		jwtClaims[oauth2const.UserAttributeGroups] = userGroups
	}
}

// appendRolesToClaims appends user roles to the JWT claims using pre-fetched groups for role resolution.
func (a *authAssertExecutor) appendRolesToClaims(
	ctx *core.NodeContext, groups []entityprovider.EntityGroup, jwtClaims map[string]interface{}) error {
	logger := a.logger.With(log.MaskedString(log.LoggerKeyUserID, ctx.AuthUser.GetUserID()))

	groupIDs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}

	roles, svcErr := a.roleService.GetUserRoles(ctx.Context, ctx.AuthUser.GetUserID(), groupIDs)
	if svcErr != nil {
		logger.Error("Failed to fetch user roles",
			log.MaskedString(log.LoggerKeyUserID, ctx.AuthUser.GetUserID()),
			log.Any("error", svcErr))
		return errors.New("something went wrong while fetching user roles: " + svcErr.ErrorDescription.DefaultValue)
	}

	if len(roles) > 0 {
		jwtClaims[oauth2const.UserAttributeRoles] = roles
	}

	return nil
}

// buildGetAttributesMetadata constructs the metadata for fetching user attributes.
func (a *authAssertExecutor) buildGetAttributesMetadata(ctx *core.NodeContext) *authnprovidercm.GetAttributesMetadata {
	metadata := &authnprovidercm.GetAttributesMetadata{
		AppMetadata: make(map[string]interface{}),
	}

	// Copy application metadata if present
	if ctx.Application.Metadata != nil {
		for key, value := range ctx.Application.Metadata {
			metadata.AppMetadata[key] = value
		}
	}

	// Extract client IDs from InboundAuthConfig
	var clientIDs []string
	for _, inboundConfig := range ctx.Application.InboundAuthConfig {
		if inboundConfig.OAuthAppConfig != nil && inboundConfig.OAuthAppConfig.ClientID != "" {
			clientIDs = append(clientIDs, inboundConfig.OAuthAppConfig.ClientID)
		}
	}

	// Add client IDs to metadata if present
	if len(clientIDs) > 0 {
		metadata.AppMetadata["client_ids"] = clientIDs
	}

	// Set locale from runtime data if present
	if locale, exists := ctx.RuntimeData["required_locales"]; exists && locale != "" {
		metadata.Locale = locale
	}

	return metadata
}
