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
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
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
	logger.Debug(ctx.Context, "Executing authentication assertion executor")

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	if ctx.AuthenticatedUser.IsAuthenticated {
		token, err := a.generateAuthAssertion(ctx, logger)
		if err != nil {
			return nil, err
		}

		logger.Debug(ctx.Context, "Generated JWT token for authentication assertion")

		execResp.Status = common.ExecComplete
		execResp.Assertion = token
		if callbackType, ok := ctx.NodeProperties[propertyKeyCallbackType].(string); ok && callbackType != "" {
			execResp.AdditionalData[propertyKeyCallbackType] = callbackType
		}
	} else {
		execResp.Status = common.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
	}

	logger.Debug(ctx.Context, "Authentication assertion executor execution completed",
		log.String("status", string(execResp.Status)))

	return execResp, nil
}

// generateAuthAssertion generates the authentication assertion token.
func (a *authAssertExecutor) generateAuthAssertion(ctx *core.NodeContext, logger *log.Logger) (string, error) {
	tokenSub := ""
	if ctx.AuthenticatedUser.UserID != "" {
		tokenSub = ctx.AuthenticatedUser.UserID
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

	authenticatorRefs := a.extractAuthenticatorReferences(ctx.ExecutionHistory)

	// Generate assertion from engaged authenticators
	if len(authenticatorRefs) > 0 {
		assertionResult, svcErr := a.authAssertGenerator.GenerateAssertion(ctx.Context, authenticatorRefs)
		if svcErr != nil {
			if svcErr.Type == serviceerror.ServerErrorType {
				logger.Error(ctx.Context, "Failed to generate auth assertion",
					log.String("error", svcErr.Error.DefaultValue))
				return "", errors.New("something went wrong while generating auth assertion")
			}
			return "", errors.New("failed to generate auth assertion: " + svcErr.Error.DefaultValue)
		}

		jwtClaims["assurance"] = assertionResult.Context
	}

	// Include permissions in the JWT (see resolvePermissionsForClaim for the precedence chain).
	if permissions := a.resolvePermissionsForClaim(ctx); permissions != "" {
		jwtClaims["authorized_permissions"] = permissions
	}

	if completedACR, exists := ctx.RuntimeData[common.RuntimeKeySelectedAuthClass]; exists && completedACR != "" {
		jwtClaims[oauth2const.ClaimCompletedAuthClass] = completedACR
	}

	// Bind the assertion to the originating CIBA request when present, so the CIBA callback can
	// verify that this assertion authorizes the specific auth_req_id it accompanies. This key is
	// only set for CIBA-initiated flows, leaving the interactive authorization_code assertion unchanged.
	if cibaAuthReqID, exists := ctx.RuntimeData[common.RuntimeKeyCIBAAuthReqID]; exists && cibaAuthReqID != "" {
		jwtClaims[oauth2const.ClaimCIBAAuthReqID] = cibaAuthReqID
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
				logger.Error(ctx.Context, "Failed to parse TTL seconds from runtime data",
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
				logger.Error(ctx.Context, "Failed to create attribute cache",
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

	jwtClaims["aud"] = ctx.EntityID
	token, _, err := a.jwtService.GenerateJWT(
		ctx.Context, tokenSub, iss, validityPeriod, jwtClaims, jwt.TokenTypeJWT, "")
	if err != nil {
		logger.Error(ctx.Context, "Failed to generate JWT token",
			log.String("error", err.Error.DefaultValue))
		return "", errors.New("failed to generate JWT token: " + err.Error.DefaultValue)
	}

	return token, nil
}

// extractAuthenticatorReferences extracts authenticator references from execution history.
func (a *authAssertExecutor) extractAuthenticatorReferences(
	history map[string]*common.NodeExecutionRecord) []authncm.AuthenticatorReference {
	refs := make([]authncm.AuthenticatorReference, 0)
	seenAuthenticators := make(map[string]bool)

	for _, record := range history {
		if record.ExecutorType != common.ExecutorTypeAuthentication {
			continue
		}
		if record.Status != common.FlowStatusComplete {
			continue
		}

		// Map executor name to the authn service name
		authnServiceName := getAuthnServiceName(record.ExecutorName)
		if authnServiceName == "" {
			continue
		}

		// Skip if we've already seen this authenticator
		if seenAuthenticators[authnServiceName] {
			continue
		}
		seenAuthenticators[authnServiceName] = true

		refs = append(refs, authncm.AuthenticatorReference{
			Authenticator: authnServiceName,
			Step:          record.Step,
			Timestamp:     record.EndTime,
		})
	}

	// Sort by step field
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Step < refs[j].Step
	})

	// Renumber Step field to be auth step
	for i := range refs {
		refs[i].Step = i + 1
	}

	return refs
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
			logger.Debug(ctx.Context, "Consent recorded with approved attributes")
			return strings.Fields(consentedAttrsStr)
		}

		// If consent was recorded but no attributes were approved, attributes are empty
		logger.Debug(ctx.Context, "Consent recorded but no attributes approved")
		return []string{}
	}

	// If consent was not recorded in this flow, check for essential/ optional attributes in runtime data.
	// If present, we should apply attribute filtering based on these attributes
	_, essentialExists := ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes]
	_, optionalExists := ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes]

	if essentialExists || optionalExists {
		userAttributes = []string{}
		logger.Debug(ctx.Context,
			"Essential/ optional attributes exists in runtime data. Applying attribute filtering")

		if essentialExists {
			logger.Debug(ctx.Context, "Adding required essential attributes to user attributes list")
			userAttributes = append(userAttributes,
				strings.Fields(ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes])...)
		}
		if optionalExists {
			logger.Debug(ctx.Context, "Adding required optional attributes to user attributes list")
			userAttributes = append(userAttributes,
				strings.Fields(ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes])...)
		}

		return userAttributes
	}

	// If consent was not recorded and no essential/ optional attributes specified, fallback to
	// application token config
	if ctx.Application.Assertion != nil {
		logger.Debug(ctx.Context, "Adding application token attributes to user attributes list")
		return ctx.Application.Assertion.UserAttributes
	}

	logger.Debug(ctx.Context, "No user attributes configured for inclusion in assertion")
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

		// Check for the attribute in authenticated user attributes
		if val, ok := ctx.AuthenticatedUser.Attributes[attr]; ok {
			attributes[attr] = val
			continue
		}

		// Check runtime data as fallback
		if val, exists := ctx.RuntimeData[attr]; exists && val != "" {
			attributes[attr] = val
			continue
		}

		// Fetch from user/authentication provider
		if ctx.AuthenticatedUser.UserID != "" && fetchedAttributes == nil {
			var err error
			if ctx.AuthUser.IsAuthenticated() {
				metadata := a.buildGetAttributesMetadata(ctx)
				fetchedAttributes, err = a.getUserAttributesFromAuthnProvider(ctx.Context,
					requestedAttributes, metadata, ctx.AuthUser)
			} else {
				fetchedAttributes, err = a.getUserAttributesFromUserProvider(ctx.Context, ctx.AuthenticatedUser.UserID)
			}
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
	if (groupsRequested || rolesRequested) && ctx.AuthenticatedUser.UserID != "" {
		allGroups, err := a.fetchAllUserGroups(ctx.Context, ctx.AuthenticatedUser.UserID)
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
	if slices.Contains(requestedAttributes, oauth2const.ClaimUserType) && ctx.AuthenticatedUser.UserType != "" {
		attributes[oauth2const.ClaimUserType] = ctx.AuthenticatedUser.UserType
	}

	// Add OU details to the claims
	ouAttributesConfigured := slices.Contains(requestedAttributes, oauth2const.ClaimOUID) ||
		slices.Contains(requestedAttributes, oauth2const.ClaimOUName) ||
		slices.Contains(requestedAttributes, oauth2const.ClaimOUHandle)
	if ouAttributesConfigured && ctx.AuthenticatedUser.OUID != "" {
		if err := a.appendOUDetailsToClaims(
			ctx.Context, ctx.AuthenticatedUser.OUID, attributes, requestedAttributes); err != nil {
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

// getUserAttributesFromUserProvider retrieves user attributes from the user provider.
func (a *authAssertExecutor) getUserAttributesFromUserProvider(ctx context.Context, userID string) (
	map[string]interface{}, error) {
	logger := a.logger.With(log.MaskedString(log.LoggerKeyUserID, userID))

	var jsonAttrs json.RawMessage
	res, err := a.entityProvider.GetEntity(userID)
	if err != nil {
		logger.Error(ctx, "Failed to fetch user attributes",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Any("error", err))
		return nil, errors.New("something went wrong while fetching user attributes: " + err.Error())
	}
	jsonAttrs = res.Attributes

	if len(jsonAttrs) == 0 {
		logger.Error(ctx, "No user attributes returned")
		return nil, errors.New("no user attributes returned")
	}

	var attrs map[string]interface{}
	if err := json.Unmarshal(jsonAttrs, &attrs); err != nil {
		logger.Error(ctx, "Failed to unmarshal user attributes",
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.Error(err))
		return nil, errors.New("something went wrong while unmarshalling user attributes: " + err.Error())
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
		logger.Error(ctx, "Failed to fetch organization unit details",
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
func (a *authAssertExecutor) fetchAllUserGroups(
	ctx context.Context, userID string) ([]entityprovider.EntityGroup, error) {
	if a.entityProvider == nil || userID == "" {
		return nil, nil
	}

	groups, err := a.entityProvider.GetTransitiveEntityGroups(userID)
	if err != nil {
		a.logger.Error(ctx, "Failed to fetch transitive user groups",
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
	logger := a.logger.With(log.MaskedString(log.LoggerKeyUserID, ctx.AuthenticatedUser.UserID))

	groupIDs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}

	roles, svcErr := a.roleService.GetUserRoles(ctx.Context, ctx.AuthenticatedUser.UserID, groupIDs)
	if svcErr != nil {
		logger.Error(ctx.Context, "Failed to fetch user roles",
			log.MaskedString(log.LoggerKeyUserID, ctx.AuthenticatedUser.UserID),
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
		if inboundConfig.OAuthConfig != nil && inboundConfig.OAuthConfig.ClientID != "" {
			clientIDs = append(clientIDs, inboundConfig.OAuthConfig.ClientID)
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

// resolvePermissionsForClaim returns the permission set to embed in the assertion. When the
// consent step ran, the result is the consented set intersected with the currently authorized
// set (defense against stale consent records containing permissions the user no longer holds).
// If consent ran in a flow without an authz step, the consented set is returned as-is (no
// authz decision is available to intersect against). Otherwise the authorized set is returned
// directly. Raw requested permissions are intentionally not used — the authz executor must
// validate them first.
func (a *authAssertExecutor) resolvePermissionsForClaim(ctx *core.NodeContext) string {
	if v, ok := ctx.RuntimeData[common.RuntimeKeyConsentedPermissions]; ok {
		if authorized, hasAuthorized := ctx.RuntimeData["authorized_permissions"]; hasAuthorized {
			return intersectPermissionSpaceList(v, authorized)
		}
		return v
	}
	return ctx.RuntimeData["authorized_permissions"]
}

// intersectPermissionSpaceList returns the space-separated set of permissions present in both
// inputs, preserving the order of `a`. Empty inputs are handled as empty sets.
func intersectPermissionSpaceList(a, b string) string {
	if a == "" || b == "" {
		return ""
	}
	allowed := make(map[string]bool)
	for _, p := range strings.Fields(b) {
		allowed[p] = true
	}
	out := make([]string, 0, len(allowed))
	for _, p := range strings.Fields(a) {
		if allowed[p] {
			out = append(out, p)
			delete(allowed, p)
		}
	}
	return strings.Join(out, " ")
}
