// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	systemutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// setExternalIdentity stores what an external party (a federated identity provider or a
// credential issuer) asserted as one RuntimeData entry, replacing any earlier one whole so the claims of
// two sign-ins never mix. Claims never reach RuntimeData under their own names, so none can stand in
// for flow control state. Token metadata is dropped, and so is the sub claim when it is set as
// the subject. connection is nil for a credential issuer.
func setExternalIdentity(execResp *providers.ExecutorResponse, connection *providers.IDPDTO, sub string,
	claims map[string]interface{}) error {
	claims = maps.Clone(claims)
	for _, key := range oauth2const.TokenMetadataClaims {
		delete(claims, key)
	}
	if sub != "" {
		delete(claims, userAttributeSub)
	}
	identity := core.ExternalIdentity{Sub: sub, Claims: claims}
	if connection != nil {
		var modes *providers.SyncConfiguration
		if connection.AttributeConfiguration != nil {
			modes = connection.AttributeConfiguration.Sync
		}
		identity.IdpID = connection.ID
		identity.AttributesSync = modes.AttributesMode()
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("failed to encode the external identity: %w", err)
	}
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	execResp.RuntimeData[common.RuntimeKeyExternalIdentity] = string(encoded)
	if identity.IdpID != "" && sub != "" {
		execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyFederatedLoginCompleted: dataValueTrue}
	}
	return nil
}

// revocationPlan is the trusted intent a flow's pre-processing node produces and the executors that
// follow act on. It is never built from request input, so the criteria, breadth and reason recorded
// here are authoritative for the rest of the flow. A system post flow reads the plan its pre flow
// produced, which the service hands back unchanged.
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
	// HolderScopes are the scopes every holder of the target loses. A plan records them when holders can
	// be added between the plan and the change, so the post flow can extend the plan to the holders the
	// change removed that the plan did not see.
	HolderScopes []revocation.AudienceScope `json:"holderScopes,omitempty"`
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

// isConstantNodeInput reports whether the running node holds the input constant.
func isConstantNodeInput(ctx *providers.NodeContext, identifier string) bool {
	for _, input := range ctx.NodeInputs {
		if input.Identifier == identifier {
			return input.Constant
		}
	}
	return false
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
// provisioned or the account holding the federated link that the user picked, onto base. The
// authentication provider runs any deferred sync of a federated sign-in to the entity when it holds the
// sign-in's link, and created marks an entity the flow just created. On success it sets the AuthUser,
// writes what the sync keeps to the session into runtime data, and copies the authenticated claims into
// runtime data. It returns the authentication provider's error as is, and ErrEntityAuthFailed when the
// AuthUser comes back unauthenticated.
func authenticateEntityByID(ctx *providers.NodeContext, authnProvider providers.AuthnProviderManager,
	entityID string, base providers.AuthUser, execResp *providers.ExecutorResponse,
	created bool) *tidcommon.ServiceError {
	credential := map[string]interface{}{authnprovidercm.CredentialTypeProvisionedEntityID: entityID}
	metadata := core.BuildProviderMetadata(ctx)
	metadata.Created = created
	authUser, authenticatedClaims, svcErr := authnProvider.AuthenticateUser(ctx.Context, nil, credential,
		nil, metadata, base)
	if svcErr != nil {
		return svcErr
	}
	if !authUser.IsAuthenticated() {
		return &ErrEntityAuthFailed
	}
	execResp.AuthUser = authUser
	if values := takeSessionValues(authenticatedClaims); values != nil {
		if err := writeSessionValues(execResp, values); err != nil {
			return &tidcommon.InternalServerError
		}
	}
	for key, value := range authenticatedClaims {
		execResp.RuntimeData[key] = systemutils.ConvertInterfaceValueToString(value)
	}
	return nil
}

// federatedSignInMetadata returns the provider metadata for a federated sign-in. Registration and
// onboarding defer the sync to the existing-user rules, so a registration that rejects an existing user
// never syncs one.
func federatedSignInMetadata(ctx *providers.NodeContext) *providers.AuthnMetadata {
	metadata := core.BuildProviderMetadata(ctx)
	metadata.DeferSync = ctx.FlowType == providers.FlowTypeRegistration ||
		ctx.FlowType == providers.FlowTypeUserOnboarding
	return metadata
}

// takeSessionValues removes the values a sign-in keeps to the session from the provider's authenticated
// claims and returns them, or nil when the claims carry none.
func takeSessionValues(claims map[string]interface{}) *authnprovidercm.SessionValues {
	raw, ok := claims[authnprovidercm.ClaimSessionValues]
	if !ok {
		return nil
	}
	delete(claims, authnprovidercm.ClaimSessionValues)
	values, _ := raw.(*authnprovidercm.SessionValues)
	if values == nil {
		values = &authnprovidercm.SessionValues{}
	}
	return values
}

// writeSessionValues writes what a sign-in keeps to the session into runtime data, in the format the
// rest of the flow reads. A nil values clears what an earlier sign-in in the flow kept.
func writeSessionValues(execResp *providers.ExecutorResponse, values *authnprovidercm.SessionValues) error {
	if execResp.RuntimeData == nil {
		execResp.RuntimeData = make(map[string]string)
	}
	runtimeData := execResp.RuntimeData
	if values == nil {
		values = &authnprovidercm.SessionValues{}
	}
	attributes := ""
	if len(values.Attributes) > 0 {
		encoded, err := json.Marshal(values.Attributes)
		if err != nil {
			return fmt.Errorf("failed to encode the session attributes: %w", err)
		}
		attributes = string(encoded)
	}
	permissions, err := json.Marshal(values.Permissions)
	if err != nil {
		return fmt.Errorf("failed to encode the mapped permissions: %w", err)
	}
	if values.Permissions == nil {
		permissions = []byte("[]")
	}
	runtimeData[common.RuntimeKeySessionAttributes] = attributes
	runtimeData[common.RuntimeKeySessionGroupIDs] = systemutils.StringifyStringArray(
		values.GroupIDsWithAncestors, " ")
	runtimeData[common.RuntimeKeySessionDirectGroupIDs] = systemutils.StringifyStringArray(values.GroupIDs, " ")
	runtimeData[common.RuntimeKeySessionRoleIDs] = systemutils.StringifyStringArray(values.RoleIDs, " ")
	runtimeData[common.RuntimeKeyMappedPermissions] = string(permissions)
	return nil
}

// getSessionGroupAndRoleIDs returns the mapped groups, with their ancestors, and the mapped roles a
// federated sign-in granted for the session only.
func getSessionGroupAndRoleIDs(runtimeData map[string]string) (groupIDs, roleIDs []string) {
	return systemutils.ParseStringArray(runtimeData[common.RuntimeKeySessionGroupIDs], " "),
		systemutils.ParseStringArray(runtimeData[common.RuntimeKeySessionRoleIDs], " ")
}

// getSessionAttributes returns the attributes a federated sign-in kept to the session, or nil when
// there are none or they can't be read.
func getSessionAttributes(runtimeData map[string]string) map[string]interface{} {
	encoded := runtimeData[common.RuntimeKeySessionAttributes]
	if encoded == "" {
		return nil
	}
	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(encoded), &attributes); err != nil {
		return nil
	}
	return attributes
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

// statedHold returns the flow error for a hold, or a failed federated sync, the authentication
// provider manager states, or nil when svcErr states neither.
func statedHold(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	switch svcErr.Code {
	case authnprovidermgr.ErrorAccountLocked.Code:
		return &ErrAccountLocked
	case authnprovidermgr.ErrorAccountSuspended.Code:
		return &ErrAccountSuspended
	case authnprovidermgr.ErrorFederatedSyncFailed.Code:
		return &ErrFederatedUserSyncFailed
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

// verificationClaimContacts returns the verification claims the server issues for a user type, mapped to
// their contact attribute. Another category, or no entity type service, has none.
func verificationClaimContacts(ctx context.Context, entityTypeService entitytype.EntityTypeServiceInterface,
	entityCategory, userType string) (map[string]string, error) {
	if entityTypeService == nil || userType == "" ||
		providers.EntityCategory(entityCategory) != providers.EntityCategoryUser {
		return nil, nil
	}

	attributes, svcErr := entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, userType,
		entitytype.AttributeFilter{AllowNonCredential: true})
	if svcErr != nil {
		return nil, fmt.Errorf("failed to fetch attributes of user type %s: code: %s", userType, svcErr.Code)
	}

	return entitytypemodel.VerificationClaims(attributes), nil
}

// dispatchPlaneFor maps the flow a challenge belongs to onto the plane it is checked at. A lock gives
// way at the recovery plane, so recovery, registration and administrator-initiated challenges still go out;
// every other flow is checked at the dispatch plane.
func dispatchPlaneFor(flowType providers.FlowType) providers.AccessPlane {
	switch flowType {
	case providers.FlowTypeRecovery, providers.FlowTypeRegistration, providers.FlowTypeAdminInitiated:
		return providers.AccessPlaneRecovery
	default:
		return providers.AccessPlaneDispatch
	}
}

// requestedAuthorizationDetails returns the requested authorization details, or nil when there are none.
func requestedAuthorizationDetails(ctx *providers.NodeContext) ([]providers.AuthorizationDetail, error) {
	encoded := ctx.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails]
	if encoded == "" {
		return nil, nil
	}
	var details []providers.AuthorizationDetail
	if err := json.Unmarshal([]byte(encoded), &details); err != nil {
		return nil, fmt.Errorf("authorization details are malformed: %w", err)
	}
	return details, nil
}

// runtimeProperty resolves a node property against the runtime data alone, so no user input, external
// claim or request value can set it. An unresolved placeholder is an error.
func runtimeProperty(ctx *providers.NodeContext, key string) (string, error) {
	value, _ := ctx.NodeProperties[key].(string)
	resolved := core.ResolvePlaceholder(&providers.NodeContext{RuntimeData: ctx.RuntimeData},
		value, nil, nil, false, log.GetLogger())
	if strings.Contains(resolved, "{{") {
		return "", fmt.Errorf("property %q did not resolve from the runtime data", key)
	}
	return resolved, nil
}

// isInviter reports whether the caller is the one who started the invitation flow,
// presenting their own token.
func isInviter(ctx *providers.NodeContext) bool {
	subject := security.GetSubject(ctx.Context)
	return !security.IsRuntimeContext(ctx.Context) && !security.IsRecordedCaller(ctx.Context) && subject != "" &&
		subject == ctx.RuntimeData[common.RuntimeKeyInviter]
}

// deliveredRecipients returns the distinct recipients senders sent to since an earlier node cleared
// the list. A list that does not decode is an error.
func deliveredRecipients(ctx *providers.NodeContext) ([]string, error) {
	raw := ctx.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients]
	if raw == "" {
		return nil, nil
	}

	var recipients []string
	if err := json.Unmarshal([]byte(raw), &recipients); err != nil {
		return nil, errors.New("invalid delivered recipients")
	}

	return recipients, nil
}

// withDeliveredRecipient returns shared runtime data that adds the recipient to the delivered recipients.
func withDeliveredRecipient(ctx *providers.NodeContext, recipient string) (map[string]string, error) {
	recipients, err := deliveredRecipients(ctx)
	if err != nil {
		return nil, err
	}

	if !slices.Contains(recipients, recipient) {
		recipients = append(recipients, recipient)
	}

	encoded, err := json.Marshal(recipients)
	if err != nil {
		return nil, fmt.Errorf("failed to encode delivered recipients: %w", err)
	}

	return map[string]string{common.RuntimeKeyDeliveredRecipients: string(encoded)}, nil
}

// sharedVerificationEvidence returns shared runtime data carrying the evidence. The evidence holds only
// strings and booleans, so it always encodes.
func sharedVerificationEvidence(proof verificationEvidence) (map[string]string, error) {
	encoded, err := json.Marshal(proof)
	if err != nil {
		return nil, fmt.Errorf("failed to encode attribute verification evidence: %w", err)
	}

	return map[string]string{common.RuntimeKeyAttributeVerificationEvidence: string(encoded)}, nil
}

// pinnedVerificationEvidence returns the attribute verification evidence an earlier node recorded on
// shared runtime data, or nil when no contact is pinned. Evidence that does not decode, or names no pin,
// is an error.
func pinnedVerificationEvidence(ctx *providers.NodeContext) (*verificationEvidence, error) {
	raw := ctx.SharedRuntimeData[common.RuntimeKeyAttributeVerificationEvidence]
	if raw == "" {
		return nil, nil
	}

	var proof verificationEvidence
	if err := json.Unmarshal([]byte(raw), &proof); err != nil {
		return nil, errors.New("invalid verification evidence")
	}

	if proof.Pin == nil {
		return nil, errors.New("missing verification pin")
	}

	return &proof, nil
}

// entityScopeCriterion revokes, for one entity, what a scope grants: the scope itself, or, for a scope of
// a hierarchical resource server, the subtree of permissions below it.
func entityScopeCriterion(entityID string, scope revocation.AudienceScope) revocation.Criterion {
	if scope.SubtreeDelimiter != "" {
		return revocation.Criterion{Type: revocation.CriterionTypeEntityScopeSubtree,
			Value: revocation.EntityScopeSubtreeCriterionValue(entityID, scope.Audience, scope.Scope,
				scope.SubtreeDelimiter)}
	}
	return revocation.Criterion{Type: revocation.CriterionTypeEntityScope,
		Value: revocation.EntityScopeCriterionValue(entityID, scope.Audience, scope.Scope)}
}

// isClientDeletionPlan reports whether the plan removes a client, the only case that detaches session
// participation: an inbound client's deletion, or an application deletion an administration flow plans. A secret
// rotation does not: the session is still legitimately the user's.
func isClientDeletionPlan(plan revocationPlan) bool {
	return plan.TargetID != "" &&
		(plan.Reason == revocation.ReasonClientRemoved || plan.Reason == revocation.ReasonApplicationRemoved)
}

// orderByDisplayOrder moves the inputs whose attribute sets a display order to the front, lowest
// first and by attribute name for the same order, and keeps the others after them in their order.
func orderByDisplayOrder(inputs []providers.Input, attrs []entitytype.AttributeInfo) {
	order := make(map[string]int, len(attrs))
	for _, attr := range attrs {
		if attr.DisplayOrder != nil {
			order[attr.Attribute] = *attr.DisplayOrder
		}
	}
	if len(order) == 0 {
		return
	}

	slices.SortStableFunc(inputs, func(a, b providers.Input) int {
		orderA, hasA := order[a.Identifier]
		orderB, hasB := order[b.Identifier]
		switch {
		case hasA && hasB:
			return cmp.Or(cmp.Compare(orderA, orderB), cmp.Compare(a.Identifier, b.Identifier))
		case hasA:
			return -1
		case hasB:
			return 1
		default:
			return 0
		}
	})
}

// isEmptyAttribute reports whether a profile attribute holds no value, so the collector treats it as missing.
func isEmptyAttribute(value any) bool {
	return value == nil || value == ""
}

// resolveKnownRecipient returns the recipient the flow already knows. known is false when it knows none.
func resolveKnownRecipient(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager, entityProvider entityprovider.EntityProviderInterface,
	recipientType, attribute string, logger *log.Logger) (recipient string, known bool, err error) {
	if ctx.RuntimeData[common.RuntimeKeyRecipientType] == recipientType {
		if expected := ctx.RuntimeData[common.RuntimeKeyExpectedRecipient]; expected != "" {
			return expected, true, nil
		}
	}

	if isConstantNodeInput(ctx, attribute) && ctx.UserInputs[attribute] != "" {
		return ctx.UserInputs[attribute], true, nil
	}

	userID := ctx.RuntimeData[userAttributeUserID]
	signedInUserID, signedInResolved := "", false
	if userID == "" {
		if signedInUserID, err = resolveSignedInUserID(ctx, execResp, authnProvider); err != nil {
			return "", true, err
		}
		userID, signedInResolved = signedInUserID, true
	}
	if userID == "" {
		return "", false, nil
	}

	if entityProvider == nil {
		return "", true, errors.New("entity provider is not configured for recipient resolution")
	}
	user, providerErr := entityProvider.GetEntityProfile(ctx.Context, userID)
	if providerErr != nil {
		if providerErr.Code == entityprovider.ErrorCodeEntityNotFound {
			return "", true, nil
		}
		return "", true, fmt.Errorf("failed to fetch user from entity provider: %w", providerErr)
	}
	if recipient, attrErr := GetUserAttribute(user, attribute); attrErr == nil {
		return recipient, true, nil
	}
	if claim, ok := core.GetExternalClaim(ctx.RuntimeData, attribute); ok && claim != "" {
		return claim, true, nil
	}
	if submitted := ctx.UserInputs[attribute]; submitted != "" {
		if !signedInResolved {
			if signedInUserID, err = resolveSignedInUserID(ctx, execResp, authnProvider); err != nil {
				return "", true, err
			}
		}
		if signedInUserID == userID {
			return submitted, true, nil
		}
	}
	logger.Debug(ctx.Context, "Recipient attribute not found on user entity", log.String("attribute", attribute))
	return "", true, nil
}

// resolveSignedInUserID returns the ID of the user signed in to the flow, or an empty ID when no user is signed in
// or the signed in user has no account. Unlike GetUserIDFromContext it skips userID in runtime data, since the
// caller compares the two, and it fails on a server error.
func resolveSignedInUserID(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	authnProvider providers.AuthnProviderManager) (string, error) {
	if authnProvider == nil || !ctx.AuthUser.IsAuthenticated() {
		return "", nil
	}
	authUser, entityRef, svcErr := authnProvider.GetEntityReference(ctx.Context, ctx.AuthUser)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", fmt.Errorf("failed to resolve the authenticated user: %s", svcErr.Code)
		}
		return "", nil
	}
	execResp.AuthUser = authUser
	if entityRef == nil {
		return "", nil
	}
	return entityRef.EntityID, nil
}
