// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// revocationPlan is the trusted intent a flow's pre-processing node produces and the executors that
// follow act on. It is never built from request input, so the criteria, breadth and reason recorded
// here are authoritative for the rest of the flow.
type revocationPlan struct {
	Criteria []revocation.Criterion `json:"criteria"`
	Mode     revocation.Mode        `json:"mode"`
	Cutoff   time.Time              `json:"cutoff,omitempty"`
	Reason   revocation.Reason      `json:"reason"`
}

// encodeRevocationPlan serializes the plan for carriage on the engine context's cross-frame store.
func encodeRevocationPlan(plan revocationPlan) (string, error) {
	encoded, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("failed to encode revocation plan: %w", err)
	}
	return string(encoded), nil
}

// decodeRevocationPlan reads the plan an earlier node published. A missing or empty plan is an error
// rather than a no-op: an executor that acts on revocation must never proceed without one.
func decodeRevocationPlan(data map[string]string) (revocationPlan, error) {
	encoded := data[common.RuntimeKeyRevocationPlan]
	if encoded == "" {
		return revocationPlan{}, errors.New("trusted revocation plan is missing")
	}
	var plan revocationPlan
	if err := json.Unmarshal([]byte(encoded), &plan); err != nil {
		return revocationPlan{}, fmt.Errorf("failed to decode trusted revocation plan: %w", err)
	}
	if len(plan.Criteria) == 0 {
		return revocationPlan{}, errors.New("trusted revocation plan has no criteria")
	}
	return plan, nil
}

// getAuthnServiceName returns the authn service name for an executor.
// Returns empty string if executor doesn't map to an authn service.
func getAuthnServiceName(executorName string) string {
	executorToAuthnServiceMap := map[string]string{
		ExecutorNameCredentialsAuth: authncm.AuthenticatorCredentials,
		ExecutorNameOTPExecutor:     authncm.AuthenticatorOTP,
		ExecutorNameOAuth:           authncm.AuthenticatorOAuth,
		ExecutorNameOIDCAuth:        authncm.AuthenticatorOIDC,
		ExecutorNameGitHubAuth:      authncm.AuthenticatorGithub,
		ExecutorNameGoogleAuth:      authncm.AuthenticatorGoogle,
		ExecutorNameMagicLink:       authncm.AuthenticatorMagicLink,
	}
	return executorToAuthnServiceMap[executorName]
}

// GetUserAttribute extracts a specific attribute value from a user entity's JSON attributes.
func GetUserAttribute(user *providers.Entity, attributeKey string) (string, error) {
	if user == nil || len(user.Attributes) == 0 {
		return "", errors.New("user entity or attributes are empty")
	}

	var attrs map[string]interface{}
	if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
		return "", errors.New("failed to parse user attributes")
	}

	if val, ok := attrs[attributeKey]; ok {
		if strVal, isString := val.(string); isString && strVal != "" {
			return strVal, nil
		}
	}

	return "", fmt.Errorf("attribute '%s' not found, empty, or not a string", attributeKey)
}

// resolveInputIdentifierByType returns the identifier of the first input in ctx.NodeInputs matching inputType,
// or fallback if none is found.
func resolveInputIdentifierByType(ctx *providers.NodeContext, inputType string, fallback string) string {
	if input, ok := findInputByType(ctx.NodeInputs, inputType); ok {
		return input.Identifier
	}
	return fallback
}

// findInputByType returns the first input in the given slice whose Type matches inputType.
func findInputByType(inputs []providers.Input, inputType string) (providers.Input, bool) {
	for _, input := range inputs {
		if input.Type == inputType {
			return input, true
		}
	}
	return providers.Input{}, false
}

// inputTypeForSchemaType maps an entity type schema attribute type to the flow input type used to
// prompt for it. Types without a dedicated input type are prompted as text.
func inputTypeForSchemaType(schemaType string) string {
	switch schemaType {
	case entitytypemodel.TypeBoolean:
		return providers.InputTypeBoolean
	case entitytypemodel.TypeNumber:
		return providers.InputTypeNumber
	default:
		return providers.InputTypeText
	}
}

// schemaTypeForInputType maps a flow input type back to the schema type its collected value should
// be converted to, for callers whose inputs come from the flow definition rather than from a schema.
// An empty result means the value needs no conversion.
func schemaTypeForInputType(inputType string) string {
	switch inputType {
	case providers.InputTypeBoolean:
		return entitytypemodel.TypeBoolean
	case providers.InputTypeNumber:
		return entitytypemodel.TypeNumber
	default:
		return ""
	}
}

// convertToSchemaType converts a collected input value, which the engine always carries as a string,
// to the type declared by its schema attribute. Values that fail to parse are returned unchanged so
// that schema validation reports them instead of a zero value being silently substituted.
func convertToSchemaType(value string, schemaType string) interface{} {
	switch schemaType {
	case entitytypemodel.TypeBoolean:
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	case entitytypemodel.TypeNumber:
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return value
}

// isAuthenticationWithoutLocalUserAllowed returns the value of the AllowAuthenticationWithoutLocalUser
// node property, defaulting to false if absent or not a bool.
// This is used to determine if authentication flow can proceed without a local user account.
// Idea is to use this in authentication flows which has a ProvisioningExecutor attached at the end
// to provision the user account and auto login without throwing an error for user not found.
func isAuthenticationWithoutLocalUserAllowed(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowAuthenticationWithoutLocalUser]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isRegistrationWithExistingUserAllowed returns the value of the AllowRegistrationWithExistingUser
// node property, defaulting to false if absent or not a bool.
// This is used to determine if registration flow can proceed when an existing user account is found.
// Idea is to use this in registration flows which can continue with the existing user account
// instead of throwing an error for user already exists and allow the flow to complete successfully.
func isRegistrationWithExistingUserAllowed(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowRegistrationWithExistingUser]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// isCrossOUProvisioningAllowed returns the value of the AllowCrossOUProvisioning node property,
// defaulting to false if absent or not a bool.
// This is used to determine if provisioning can proceed across organizational units (OUs).
// Idea is to use this in registration flows which can continue even if an existing user account
// is found, but the provisioning executor is trying to provision the user to a different OU than
// the one in the existing account.
func isCrossOUProvisioningAllowed(ctx *providers.NodeContext) bool {
	if val, ok := ctx.NodeProperties[common.NodePropertyAllowCrossOUProvisioning]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return false
}

// setFederatedEntityState records whether federated authentication resolved a concrete local user
// (via account linking) into the entityState runtime key, and returns that user's ID, or "" when none.
func setFederatedEntityState(ctx context.Context, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager) string {
	execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateNotExists
	authUser, entityRef, svcErr := authnProvider.GetEntityReference(ctx, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr == nil && entityRef != nil {
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
		return entityRef.EntityID
	}
	return ""
}

// reservedAuthorizationRuntimeKeys must never be overwritten by an external claim of the same name.
var reservedAuthorizationRuntimeKeys = map[string]bool{
	common.RuntimeKeyMappedRoleIDs:          true,
	common.RuntimeKeyMappedGroupIDs:         true,
	common.RuntimeKeyMappedPermissions:      true,
	common.RuntimeKeyGroupsAndRolesSyncMode: true,
	common.RuntimeKeyAttributesSyncMode:     true,
}

// copyFederatedAttributesToRuntimeData copies federatedAttributes into execResp.RuntimeData, skipping
// reservedAuthorizationRuntimeKeys.
func copyFederatedAttributesToRuntimeData(
	execResp *providers.ExecutorResponse, federatedAttributes map[string]interface{},
) {
	if len(federatedAttributes) == 0 {
		return
	}
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	for key, value := range federatedAttributes {
		if reservedAuthorizationRuntimeKeys[key] {
			continue
		}
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
}

// resolveAndSetMappedAuthorizationTargets resolves the IDP's AuthorizationRuleMapping (explicit value
// rules) and AuthorizationDirectMapping (direct name-based lookup) against federatedAttributes,
// unions their targets, and stores the result, with the IDP's sync modes, as runtime data for later
// executors. Returns the IDP and whether every target resolved. Returns an error when the IDP can't be
// resolved, since later executors would otherwise store attributes the connection keeps to the session.
// Logs and continues without the direct targets when they can't be resolved, rather than failing the
// federated login over best-effort enrichment.
func resolveAndSetMappedAuthorizationTargets(
	ctx context.Context, execResp *providers.ExecutorResponse,
	idpService idp.IDPServiceInterface, idpID string, federatedAttributes map[string]interface{},
	logger *log.Logger,
) (*providers.IDPDTO, bool, error) {
	idpDTO, svcErr := idpService.GetIdentityProvider(ctx, idpID)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve IDP for sync modes and authorization mapping",
			log.String("idpId", idpID), log.String("error", svcErr.Error.DefaultValue))
		return nil, false, errors.New("error while retrieving identity provider")
	}
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	execResp.RuntimeData[common.RuntimeKeyGroupsAndRolesSyncMode] = string(idp.GetGroupsAndRolesSyncMode(idpDTO))
	execResp.RuntimeData[common.RuntimeKeyAttributesSyncMode] = string(idp.GetAttributesSyncMode(idpDTO))
	targets := idp.GetRuleAuthorizationTargets(idpDTO, federatedAttributes)
	directTargets, svcErr := idpService.GetDirectAuthorizationTargets(ctx, idpDTO, federatedAttributes)
	complete := svcErr == nil
	if svcErr != nil {
		logger.Warn(ctx, "Failed to resolve direct authorization targets, continuing with rule-based targets only",
			log.String("idpId", idpID), log.String("error", svcErr.Error.DefaultValue))
	} else {
		targets = append(targets, directTargets...)
	}
	setMappedAuthorizationTargets(execResp, targets)
	return idpDTO, complete, nil
}

// expandMappedGroupAncestors adds each mapped group's ancestor groups. Best-effort, like the rest of
// authorization mapping at login.
func expandMappedGroupAncestors(
	ctx context.Context, entityProvider entityprovider.EntityProviderInterface, logger *log.Logger,
	mappedGroupIDs []string,
) []string {
	if entityProvider == nil || len(mappedGroupIDs) == 0 {
		return mappedGroupIDs
	}
	expanded := mappedGroupIDs
	for _, groupID := range mappedGroupIDs {
		ancestors, err := entityProvider.GetTransitiveGroupAncestors(groupID)
		if err != nil {
			logger.Warn(ctx, "Failed to resolve ancestors of a mapped group, continuing without them",
				log.String("groupId", groupID), log.Error(err))
			continue
		}
		expanded = systemutils.MergeUniqueStrings(expanded, ancestors)
	}
	return expanded
}

// setMappedAuthorizationTargets splits resolved authorization mapping targets (rule-based or direct)
// by kind and stores them as runtime data for later executors. Always writes all three keys, even when
// empty.
func setMappedAuthorizationTargets(execResp *providers.ExecutorResponse, targets []providers.AuthorizationTarget) {
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	roleIDs, groupIDs, permissions := idp.SplitAuthorizationTargets(targets)
	execResp.RuntimeData[common.RuntimeKeyMappedRoleIDs] = systemutils.StringifyStringArray(roleIDs, " ")
	execResp.RuntimeData[common.RuntimeKeyMappedGroupIDs] = systemutils.StringifyStringArray(groupIDs, " ")
	encoded, err := json.Marshal(permissions)
	if err != nil {
		encoded = []byte("[]")
	}
	execResp.RuntimeData[common.RuntimeKeyMappedPermissions] = string(encoded)
}

// isAllowAuthenticationWithoutLocalUserRuntimeFlagSet checks if the runtime flag for allowing authentication without
// a local user is set in the context.
func isAllowRegistrationWithExistingUserRuntimeFlagSet(ctx *providers.NodeContext) bool {
	val, ok := ctx.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser]
	return ok && val == dataValueTrue
}

// validateFederatedIdentifierConsistency checks if the federated identifiers from the authentication result
// are consistent with any existing identifiers in the context (runtime data, user inputs, authenticated
// user attributes).
func validateFederatedIdentifierConsistency(ctx *providers.NodeContext,
	federatedIdentifiers, existingIdentifiers map[string]interface{}) bool {
	if len(federatedIdentifiers) == 0 {
		return true
	}

	// TODO: Refine this well-known-key comparison when IDP-to-local attribute mapping is supported
	fedIdfConsistencyKeys := []string{userAttributeEmail, userAttributeSub}
	for _, key := range fedIdfConsistencyKeys {
		federatedValue := ""
		if value, ok := federatedIdentifiers[key]; ok {
			federatedValue = systemutils.ConvertInterfaceValueToString(value)
		}

		if federatedValue == "" {
			continue
		}

		if value, ok := ctx.RuntimeData[key]; ok && value != "" && value != federatedValue {
			return false
		}
		if value, ok := ctx.UserInputs[key]; ok && value != "" && value != federatedValue {
			return false
		}
		if value := existingIdentifiers[key]; value != nil &&
			systemutils.ConvertInterfaceValueToString(value) != "" &&
			systemutils.ConvertInterfaceValueToString(value) != federatedValue {
			return false
		}
	}

	return true
}
