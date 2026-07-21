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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	authAssertLoggerComponentName = "AuthAssertExecutor"
)

// authAssertExecutor is an executor that handles authentication assertions in the flow.
type authAssertExecutor struct {
	providers.Executor
	jwtService          jwt.JWTServiceInterface
	ouService           ou.OrganizationUnitServiceInterface
	authAssertGenerator assert.AuthAssertGeneratorInterface
	authnProvider       providers.AuthnProviderManager
	entityProvider      entityprovider.EntityProviderInterface
	attributeCacheSvc   attributecache.AttributeCacheServiceInterface
	roleService         role.RoleServiceInterface
	logger              *log.Logger
}

var _ providers.Executor = (*authAssertExecutor)(nil)

// newAuthAssertExecutor creates a new instance of AuthAssertExecutor.
func newAuthAssertExecutor(
	flowFactory core.FlowFactoryInterface,
	jwtService jwt.JWTServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	assertGenerator assert.AuthAssertGeneratorInterface,
	authnProvider providers.AuthnProviderManager,
	entityProvider entityprovider.EntityProviderInterface,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	roleService role.RoleServiceInterface,
) *authAssertExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, authAssertLoggerComponentName),
		log.String(log.LoggerKeyExecutorName, ExecutorNameAuthAssert))

	base := flowFactory.CreateExecutor(ExecutorNameAuthAssert, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: propertyKeyCallbackType},
			},
		})

	return &authAssertExecutor{
		Executor:            base,
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
func (a *authAssertExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := a.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	logger.Debug(ctx.Context, "Executing authentication assertion executor")

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
		AuthUser:       ctx.AuthUser,
	}

	if execResp.AuthUser.IsAuthenticated() {
		// Verify the assurance accumulated in this execution (whether from executed nodes or a
		// loaded session snapshot) satisfies the request's acr_values and max_age before issuing
		// an assertion.
		if svcErr := a.checkAssurance(ctx, logger); svcErr != nil {
			execResp.Status = providers.ExecFailure
			execResp.Error = svcErr
			return execResp, nil
		}

		token, err := a.generateAuthAssertion(ctx, execResp, logger)
		if err != nil {
			return nil, err
		}

		logger.Debug(ctx.Context, "Generated JWT token for authentication assertion")

		execResp.Status = providers.ExecComplete
		execResp.Assertion = token
		if callbackType, ok := ctx.NodeProperties[propertyKeyCallbackType].(string); ok && callbackType != "" {
			execResp.AdditionalData[propertyKeyCallbackType] = callbackType
		}
	} else {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserNotAuthenticated
	}

	logger.Debug(ctx.Context, "Authentication assertion executor execution completed",
		log.String("status", string(execResp.Status)))

	return execResp, nil
}

// checkAssurance verifies that the assurance accumulated in this execution satisfies the
// request's acr_values and max_age. It returns ErrInteractionRequired when interaction
// (step-up or re-authentication) is required, or nil when the requirements are met.
func (a *authAssertExecutor) checkAssurance(ctx *providers.NodeContext,
	logger *log.Logger) *tidcommon.ServiceError {
	// acr_values: the completed authentication class must be one of the requested classes.
	requested := strings.Fields(ctx.RuntimeData[common.RuntimeKeyRequestedAuthClasses])
	if len(requested) > 0 {
		completed := ctx.RuntimeData[common.RuntimeKeySelectedAuthClass]
		if completed == "" || !slices.Contains(requested, completed) {
			logger.Debug(ctx.Context, "Accumulated assurance does not satisfy requested acr_values",
				log.String("completed", completed))
			return &ErrInteractionRequired
		}
	}

	// max_age: the subject must have authenticated within max_age seconds.
	if rawMaxAge, ok := ctx.RuntimeData[common.RuntimeKeyMaxAge]; ok && rawMaxAge != "" {
		maxAge, err := strconv.ParseInt(rawMaxAge, 10, 64)
		if err != nil || maxAge < 0 {
			// A malformed max_age is treated as no constraint.
			logger.Debug(ctx.Context, "Ignoring malformed max_age", log.String("maxAge", rawMaxAge))
			return nil
		}
		if time.Now().UTC().Unix()-a.resolveAuthTime(ctx) > maxAge {
			logger.Debug(ctx.Context, "Authentication is older than max_age; re-authentication required")
			return &ErrInteractionRequired
		}
	}

	return nil
}

// resolveAuthTime returns the Unix time at which the subject authenticated. On the SSO path
// this comes from the loaded session snapshot; otherwise the subject authenticated during this
// execution, so the current time is used.
func (a *authAssertExecutor) resolveAuthTime(ctx *providers.NodeContext) int64 {
	if raw, ok := ctx.RuntimeData[common.RuntimeKeyAuthTime]; ok && raw != "" {
		if ts, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return ts
		}
	}
	return time.Now().UTC().Unix()
}

// generateAuthAssertion generates the authentication assertion token.
func (a *authAssertExecutor) generateAuthAssertion(
	ctx *providers.NodeContext, execResp *providers.ExecutorResponse, logger *log.Logger,
) (string, error) {
	tokenSub := ""

	jwtClaims := make(map[string]interface{})
	validityPeriod := int64(0)

	if ctx.Application.Assertion != nil {
		validityPeriod = ctx.Application.Assertion.ValidityPeriod
	}

	authenticatorRefs := a.extractAuthenticatorReferences(ctx.ExecutionHistory)

	// Generate assertion from engaged authenticators
	if len(authenticatorRefs) > 0 {
		assertionResult, svcErr := a.authAssertGenerator.GenerateAssertion(ctx.Context, authenticatorRefs)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
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

	// Bind the assertion to the originating auth request so the corresponding callback can verify this assertion
	// authorizes the specific request it accompanies.
	if authReqID, exists := ctx.RuntimeData[common.RuntimeKeyAuthorizationRequestID]; exists && authReqID != "" {
		jwtClaims[oauth2const.ClaimAuthorizationRequestID] = authReqID
	}

	requiredAttributes := a.getRequiredUserAttributes(ctx)

	metadata := buildGetAttributesMetadata(ctx)

	// Convert requested attributes from []string to *RequestedAttributes
	reqAttrs := &providers.RequestedAttributes{
		Attributes:    make(map[string]*providers.AttributeMetadataRequest),
		Verifications: nil,
	}
	for _, attrName := range requiredAttributes {
		reqAttrs.Attributes[attrName] = nil
	}

	authUser, entityRef, svcErr := a.authnProvider.GetEntityReference(ctx.Context, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", errors.New("something went wrong while fetching entity references")
		}
		return "", errors.New("failed to fetch entity references: " + svcErr.ErrorDescription.DefaultValue)
	}

	authUser, attrResp, svcErr := a.authnProvider.GetUserAttributes(ctx.Context, reqAttrs, metadata, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", errors.New("something went wrong while fetching user attributes")
		}
		return "", errors.New("failed to fetch user attributes: " + svcErr.ErrorDescription.DefaultValue)
	}

	tokenSub = entityRef.EntityID

	fetchedAttributes := make(map[string]interface{})

	if attrResp != nil && len(attrResp.Attributes) > 0 {
		for attrName, attrResp := range attrResp.Attributes {
			if attrResp != nil {
				fetchedAttributes[attrName] = attrResp.Value
			}
		}
	}

	resolvedAttributes, attrErr := a.resolveUserAttributes(ctx, requiredAttributes, fetchedAttributes,
		entityRef.EntityID, entityRef.EntityType, entityRef.OUID)
	if attrErr != nil {
		return "", attrErr
	}

	if ttlSecondsStr, exists := ctx.RuntimeData[common.RuntimeKeyUserAttributesCacheTTLSeconds]; exists {
		// We are not in an App Native flow, so we need to cache the user attributes
		if len(resolvedAttributes) > 0 {
			ttlSeconds, err := strconv.ParseInt(ttlSecondsStr, 10, 64)
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
	// iss is set to the default issuer configured in the JWT service, which is typically the server's base URL.
	token, _, err := a.jwtService.GenerateJWT(
		ctx.Context, tokenSub, "", validityPeriod, jwtClaims, jwt.TokenTypeJWT, "")
	if err != nil {
		logger.Error(ctx.Context, "Failed to generate JWT token",
			log.String("error", err.Error.DefaultValue))
		return "", errors.New("failed to generate JWT token: " + err.Error.DefaultValue)
	}

	return token, nil
}

// extractAuthenticatorReferences extracts authenticator references from execution history.
func (a *authAssertExecutor) extractAuthenticatorReferences(
	history map[string]*providers.NodeExecutionRecord) []authncm.AuthenticatorReference {
	refs := make([]authncm.AuthenticatorReference, 0)
	seenAuthenticators := make(map[string]bool)

	for _, record := range history {
		if record.ExecutorType != providers.ExecutorTypeAuthentication {
			continue
		}
		if record.Status != providers.FlowStatusComplete {
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
func (a *authAssertExecutor) getRequiredUserAttributes(ctx *providers.NodeContext) (userAttributes []string) {
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
func (a *authAssertExecutor) resolveUserAttributes(
	ctx *providers.NodeContext,
	requestedAttributes []string,
	fetchedAttributes map[string]interface{},
	userID, userType, ouID string,
) (map[string]interface{}, error) {
	if len(requestedAttributes) == 0 {
		return nil, nil
	}

	attributes := make(map[string]interface{})

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

		// Check runtime data
		if val, exists := ctx.RuntimeData[attr]; exists && val != "" {
			attributes[attr] = val
			continue
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
	if err := a.appendComputedAttributes(ctx, requestedAttributes, attributes, userID, userType, ouID); err != nil {
		return nil, err
	}

	return attributes, nil
}

// appendComputedAttributes appends computed/derived attributes (groups, roles, userType, OU details) to the claims.
func (a *authAssertExecutor) appendComputedAttributes(
	ctx *providers.NodeContext,
	requestedAttributes []string,
	attributes map[string]interface{},
	userID, userType, ouID string,
) error {
	groupsRequested := slices.Contains(requestedAttributes, oauth2const.UserAttributeGroups)
	rolesRequested := slices.Contains(requestedAttributes, oauth2const.UserAttributeRoles)

	// Fetch all user groups once if either groups or roles are requested.
	if (groupsRequested || rolesRequested) && userID != "" {
		allGroups, err := a.fetchAllUserGroups(ctx.Context, userID)
		if err != nil {
			return err
		}

		if groupsRequested {
			a.appendGroupsToClaims(allGroups, attributes)
		}

		if rolesRequested {
			if err := a.appendRolesToClaims(ctx, allGroups, attributes, userID); err != nil {
				return err
			}
		}
	}

	// Add user type to the claims
	if slices.Contains(requestedAttributes, oauth2const.ClaimUserType) && userType != "" {
		attributes[oauth2const.ClaimUserType] = userType
	}

	// Add OU details to the claims
	ouAttributesConfigured := slices.Contains(requestedAttributes, oauth2const.ClaimOUID) ||
		slices.Contains(requestedAttributes, oauth2const.ClaimOUName) ||
		slices.Contains(requestedAttributes, oauth2const.ClaimOUHandle)
	if ouAttributesConfigured && ouID != "" {
		if err := a.appendOUDetailsToClaims(
			ctx.Context, ouID, attributes, requestedAttributes); err != nil {
			return err
		}
	}

	return nil
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
	ctx context.Context, userID string) ([]providers.EntityGroup, error) {
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
	groups []providers.EntityGroup, jwtClaims map[string]interface{}) {
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
	ctx *providers.NodeContext, groups []providers.EntityGroup, jwtClaims map[string]interface{}, userID string) error {
	logger := a.logger.With(log.MaskedString(log.LoggerKeyUserID, userID))

	groupIDs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}

	roles, svcErr := a.roleService.GetUserRoles(ctx.Context, userID, groupIDs)
	if svcErr != nil {
		logger.Error(ctx.Context, "Failed to fetch user roles",
			log.MaskedString(log.LoggerKeyUserID, userID),
			log.Any("error", svcErr))
		return errors.New("something went wrong while fetching user roles: " + svcErr.ErrorDescription.DefaultValue)
	}

	if len(roles) > 0 {
		jwtClaims[oauth2const.UserAttributeRoles] = roles
	}

	return nil
}

// resolvePermissionsForClaim returns the permission set to embed in the assertion. When the
// consent step ran, the result is the consented set intersected with the currently authorized
// set (defense against stale consent records containing permissions the user no longer holds).
// If consent ran in a flow without an authz step, the consented set is returned as-is (no
// authz decision is available to intersect against). Otherwise the authorized set is returned
// directly. Raw requested permissions are intentionally not used — the authz executor must
// validate them first.
func (a *authAssertExecutor) resolvePermissionsForClaim(ctx *providers.NodeContext) string {
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
