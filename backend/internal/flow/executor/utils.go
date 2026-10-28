// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"time"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// setExternalIdentity stores what an external party (a federated identity provider or a
// credential issuer) asserted as one RuntimeData entry, replacing any earlier one whole so the claims of
// two sign-ins never mix. Claims never reach RuntimeData under their own names, so none can stand in
// for flow control state. Token metadata is dropped, and so is the sub claim when it is set as
// the subject.
func setExternalIdentity(execResp *providers.ExecutorResponse, idpID, sub string,
	claims map[string]interface{}) error {
	claims = maps.Clone(claims)
	for _, key := range tokenMetadataClaims {
		delete(claims, key)
	}
	if sub != "" {
		delete(claims, userAttributeSub)
	}
	encoded, err := json.Marshal(core.ExternalIdentity{IdpID: idpID, Sub: sub, Claims: claims})
	if err != nil {
		return fmt.Errorf("failed to encode the external identity: %w", err)
	}
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	execResp.RuntimeData[common.RuntimeKeyExternalIdentity] = string(encoded)
	return nil
}

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
	// AssigneeID is the principal losing a role or leaving a group, when the action names one.
	AssigneeID   string `json:"assigneeId,omitempty"`
	AssigneeType string `json:"assigneeType,omitempty"`
	// ActionArgs carries further acting-node arguments, keyed by the flow input they came from.
	ActionArgs map[string]string `json:"actionArgs,omitempty"`
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

// decodeRevocationPlan reads the plan an earlier node published. A missing plan, or one with no criteria
// that does not declare NothingToRevoke, is an error rather than a no-op.
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

// revocationPlanFor returns the trusted plan an acting node acts on. The reason check makes a mispaired
// graph fail cleanly, since flow validation does not stop one action's preparatory node being wired to
// another's acting node.
func revocationPlanFor(data map[string]string, want revocation.Reason) (revocationPlan, error) {
	plan, err := decodeRevocationPlan(data)
	if err != nil {
		return revocationPlan{}, err
	}
	if plan.Reason != want {
		return revocationPlan{}, fmt.Errorf("trusted revocation plan was produced for %q, not %q",
			plan.Reason, want)
	}
	if plan.TargetID == "" {
		return revocationPlan{}, errors.New("trusted revocation plan has no target")
	}
	return plan, nil
}

// applicationTargetFromPlan returns the application the trusted plan acts on.
func applicationTargetFromPlan(data map[string]string, want revocation.Reason) (string, error) {
	plan, err := revocationPlanFor(data, want)
	if err != nil {
		return "", err
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
// (via account linking) into the entityState runtime key. Several users holding the link is recorded
// as ambiguous, for a later node to let the user pick between them.
func setFederatedEntityState(ctx context.Context, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager) {
	execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateNotExists

	// An unauthenticated AuthUser names nobody, and GetEntityReference would log it as an error.
	if !execResp.AuthUser.IsAuthenticated() {
		return
	}

	authUser, entityRef, svcErr := authnProvider.GetEntityReference(ctx, execResp.AuthUser)
	execResp.AuthUser = authUser
	switch {
	case svcErr == nil && entityRef != nil:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateExists
	case svcErr != nil && svcErr.Code == authnprovidermgr.ErrorAmbiguousUser.Code:
		execResp.RuntimeData[common.RuntimeKeyEntityState] = entityStateAmbiguous
	}
}

// authenticateEntityByID signs in an entity whose ID the flow already trusts, such as one it just
// provisioned or the federated link's holder the user picked, onto base. On success it sets the
// AuthUser and copies the authenticated claims into runtime data. It returns the authentication
// provider's error as is, and ErrEntityAuthFailed when the AuthUser comes back unauthenticated.
func authenticateEntityByID(ctx *providers.NodeContext, authnProvider providers.AuthnProviderManager,
	entityID string, base providers.AuthUser, execResp *providers.ExecutorResponse) *tidcommon.ServiceError {
	credential := map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: entityID}
	authUser, authenticatedClaims, svcErr := authnProvider.AuthenticateUser(ctx.Context, nil, credential,
		nil, nil, base)
	if svcErr != nil {
		return svcErr
	}
	if !authUser.IsAuthenticated() {
		return &ErrEntityAuthFailed
	}
	execResp.AuthUser = authUser
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
	return nil
}

// resolveAndSetMappedAuthorizationTargets resolves the IDP's AuthorizationRuleMapping (explicit value
// rules) and AuthorizationDirectMapping (direct name-based lookup) against federatedAttributes,
// unions their targets, and stores the result as runtime data for later executors. Logs and continues
// without the direct targets when they can't be resolved, rather than failing the federated login over
// what is best-effort enrichment of its runtime state.
func resolveAndSetMappedAuthorizationTargets(
	ctx context.Context, execResp *providers.ExecutorResponse,
	idpService providers.IDPProvider, idpDTO *providers.IDPDTO, federatedAttributes map[string]interface{},
	logger *log.Logger,
) {
	targets := idp.GetRuleAuthorizationTargets(idpDTO, federatedAttributes)
	directTargets, svcErr := idpService.GetDirectAuthorizationTargets(ctx, idpDTO, federatedAttributes)
	if svcErr != nil {
		logger.Warn(ctx, "Failed to resolve direct authorization targets, continuing with rule-based targets only",
			log.String(log.LoggerKeyIDPID, idpDTO.ID), log.String("error", svcErr.Error.DefaultValue))
	} else {
		targets = append(targets, directTargets...)
	}
	setMappedAuthorizationTargets(execResp, targets)
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

// isRegistrationFlow reports whether executors should apply their registration behavior. A linking
// verification segment verifies an existing account, so it runs with authentication behavior even
// inside a registration flow.
func isRegistrationFlow(ctx *providers.NodeContext) bool {
	return ctx.FlowType == providers.FlowTypeRegistration &&
		ctx.RuntimeData[common.RuntimeKeyLinkingVerificationRequested] != dataValueTrue
}

// isAllowAuthenticationWithoutLocalUserRuntimeFlagSet checks if the runtime flag for allowing authentication without
// a local user is set in the context.
func isAllowRegistrationWithExistingUserRuntimeFlagSet(ctx *providers.NodeContext) bool {
	val, ok := ctx.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser]
	return ok && val == dataValueTrue
}

// validateFederatedIdentifierConsistency checks if the federated identity from the authentication result
// is consistent with what the flow already holds. An earlier federated sign-in must be the same
// identity: subjects are unique only within a connection, so the connection and subject are compared
// together. The local attributes the connection's account-linking attributes match on are compared with
// runtime data, the earlier external identity's claims, user inputs and the authenticated user's
// attributes. A connection without account-linking attributes is compared on email.
func validateFederatedIdentifierConsistency(ctx *providers.NodeContext, idpDTO *providers.IDPDTO,
	federatedIdentifiers, existingIdentifiers map[string]interface{}) bool {
	if len(federatedIdentifiers) == 0 {
		return true
	}

	sub := systemutils.ConvertInterfaceValueToString(federatedIdentifiers[userAttributeSub])
	earlier := core.GetExternalIdentity(ctx.RuntimeData)
	if earlier != nil && earlier.Sub != "" && (earlier.IdpID != idpDTO.ID || earlier.Sub != sub) {
		return false
	}

	var linking *providers.AccountLinking
	if idpDTO.AttributeConfiguration != nil {
		linking = idpDTO.AttributeConfiguration.AccountLinking
	}
	if linking == nil || len(linking.Attributes) == 0 {
		linking = &providers.AccountLinking{Attributes: []string{userAttributeEmail}}
	}
	mappings := idp.GetAttributeMappings(idpDTO, federatedIdentifiers)
	for _, attr := range idp.GetAccountLinkingLocalAttributes(linking, federatedIdentifiers, mappings) {
		federated := systemutils.ConvertInterfaceValueToString(federatedIdentifiers[attr])
		if value := ctx.RuntimeData[attr]; value != "" && value != federated {
			return false
		}
		if value, _ := earlier.Claim(attr); value != "" && value != federated {
			return false
		}
		if value := ctx.UserInputs[attr]; value != "" && value != federated {
			return false
		}
		if value := systemutils.ConvertInterfaceValueToString(existingIdentifiers[attr]); value != "" &&
			value != federated {
			return false
		}
	}

	return true
}

// consumeFederatedCallbackInputs removes the code and state from UserInputs and returns the state.
func consumeFederatedCallbackInputs(ctx *providers.NodeContext) string {
	returnedState := ctx.UserInputs[userInputState]
	delete(ctx.UserInputs, userInputCode)
	delete(ctx.UserInputs, userInputState)
	return returnedState
}

// federatedIdentityFrom returns nil when no federated connection set an external identity.
func federatedIdentityFrom(ctx *providers.NodeContext) *core.ExternalIdentity {
	extIdentity := core.GetExternalIdentity(ctx.RuntimeData)
	if extIdentity == nil || extIdentity.IdpID == "" || extIdentity.Sub == "" {
		return nil
	}
	return extIdentity
}

// decodeRolePermissions reads the JSON-encoded permission set of a role permission change.
func decodeRolePermissions(encoded string) ([]role.ResourcePermissions, error) {
	permissions := []role.ResourcePermissions{}
	if err := json.Unmarshal([]byte(encoded), &permissions); err != nil {
		return nil, fmt.Errorf("failed to decode role permissions: %w", err)
	}
	if permissions == nil {
		permissions = []role.ResourcePermissions{}
	}
	return permissions, nil
}

// optionalID returns nil for an absent identifier, the form the resource service reads as unscoped.
func optionalID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}

// refusalResponse fails execResp with the service's refusal, or with fallback when the refusal carries
// no code. A server error is returned as an error prefixed with failure instead of being shown as a refusal.
func refusalResponse(execResp *providers.ExecutorResponse, svcErr *tidcommon.ServiceError,
	fallback tidcommon.ServiceError, failure string) (*providers.ExecutorResponse, error) {
	if svcErr.Type == tidcommon.ServerErrorType {
		return nil, fmt.Errorf("%s: %s", failure, svcErr.Error.DefaultValue)
	}
	refusal := *svcErr
	if refusal.Code == "" {
		refusal = fallback
	}
	execResp.Status = providers.ExecFailure
	execResp.Error = &refusal
	return execResp, nil
}

// statedHold returns the flow error for a hold the authentication provider manager states, or nil
// when svcErr states none.
func statedHold(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	switch svcErr.Code {
	case authnprovidermgr.ErrorAccountLocked.Code:
		return &ErrAccountLocked
	case authnprovidermgr.ErrorAccountSuspended.Code:
		return &ErrAccountSuspended
	default:
		return nil
	}
}

// statedDecision returns the flow error for the hold a refused decision names, or nil when it names
// none.
func statedDecision(decision providers.AccessDecision) *tidcommon.ServiceError {
	switch decision.Reason {
	case providers.AccessRefusalReasonLocked:
		return &ErrAccountLocked
	case providers.AccessRefusalReasonSuspended:
		return &ErrAccountSuspended
	default:
		return nil
	}
}

// governanceFault is the error a gate raises when the identity governance provider could not decide.
func governanceFault(svcErr *tidcommon.ServiceError) error {
	return fmt.Errorf("identity governance check failed: %s", svcErr.Error.DefaultValue)
}

// dispatchPlaneFor maps the flow a challenge belongs to onto the plane it is checked at. A lock gives
// way at the recovery plane, so recovery and registration challenges still go out; every other flow
// is checked at the dispatch plane.
func dispatchPlaneFor(flowType providers.FlowType) providers.AccessPlane {
	switch flowType {
	case providers.FlowTypeRecovery, providers.FlowTypeRegistration:
		return providers.AccessPlaneRecovery
	default:
		return providers.AccessPlaneDispatch
	}
}
