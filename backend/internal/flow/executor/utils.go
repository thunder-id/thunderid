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
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/revocation"
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
	// TargetID is the resource the acting nodes operate on, when that is not the criterion value. An
	// application revocation is keyed by the OAuth client id while the delete and the session detachment
	// need the application id, so the two travel separately rather than one being re-derived.
	TargetID string `json:"targetId,omitempty"`
	// TTLSeconds is how long the deny-list row must live to outlast the artifacts the criteria match.
	// Zero leaves the revocation service on its configured default.
	TTLSeconds int64 `json:"ttlSeconds,omitempty"`
	// NothingToRevoke records that the preparatory node established there is no artifact to revoke, which
	// is how an empty criteria list is distinguished from a missing one. Only a node that verified the
	// absence sets it; without it an empty list stays an error, so a plan can never be silently skipped.
	NothingToRevoke bool `json:"nothingToRevoke,omitempty"`
}

// encodeRevocationPlan serializes the plan for carriage on the engine context's cross-frame store.
func encodeRevocationPlan(plan revocationPlan) (string, error) {
	encoded, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("failed to encode revocation plan: %w", err)
	}
	return string(encoded), nil
}

// decodeRevocationPlan reads the plan an earlier node published. A missing plan is an error rather than a
// no-op: an executor that acts on revocation must never proceed without one.
//
// A plan with no criteria is likewise an error, unless it declares NothingToRevoke. That flag is the
// difference between a node having established there is nothing to deny and a plan that lost its criteria
// on the way, which would otherwise look identical here.
func decodeRevocationPlan(data map[string]string) (revocationPlan, error) {
	encoded := data[common.RuntimeKeyRevocationPlan]
	if encoded == "" {
		return revocationPlan{}, errors.New("trusted revocation plan is missing")
	}
	var plan revocationPlan
	if err := json.Unmarshal([]byte(encoded), &plan); err != nil {
		return revocationPlan{}, fmt.Errorf("failed to decode trusted revocation plan: %w", err)
	}
	if len(plan.Criteria) == 0 && !plan.NothingToRevoke {
		return revocationPlan{}, errors.New("trusted revocation plan has no criteria")
	}
	return plan, nil
}

// applicationTargetFromPlan returns the application the trusted plan acts on, after checking the plan was
// produced for this action.
//
// The reason check is what makes a mispaired graph fail cleanly. Nothing in flow validation stops a
// preparatory node for one action being wired to the acting node of another, and that pairing would
// otherwise revoke with one breadth and then perform a different mutation.
func applicationTargetFromPlan(data map[string]string, want revocation.Reason) (string, error) {
	plan, err := decodeRevocationPlan(data)
	if err != nil {
		return "", err
	}
	if plan.Reason != want {
		return "", fmt.Errorf("trusted revocation plan was produced for %q, not %q", plan.Reason, want)
	}
	if plan.TargetID == "" {
		return "", errors.New("trusted revocation plan has no target application")
	}
	return plan.TargetID, nil
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
// (via account linking) into the entityState runtime key.
func setFederatedEntityState(ctx context.Context, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager) {
	execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateNotExists
	authUser, entityRef, svcErr := authnProvider.GetEntityReference(ctx, execResp.AuthUser)
	execResp.AuthUser = authUser
	if svcErr == nil && entityRef != nil {
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
	}
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
