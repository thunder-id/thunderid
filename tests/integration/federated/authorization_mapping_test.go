// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/*
AuthorizationMapping integration tests.

These exercise the connection's authorizationMappings section end to end: a mock IdP returns claims,
a federated authentication flow provisions (or resolves) a local entity, an AuthorizationExecutor node
evaluates the requested permissions, and the resulting AuthAssertExecutor assertion's
authorized_permissions claim is read back to observe what the mapping produced. This is the flow-based
counterpart to the token-exchange AuthorizationMapping scenarios in
tests/integration/oauth/token/token_exchange_authorization_mapping_test.go: together the two cover the
"consistent behavior across entry points" requirement.

Fixtures live in suite_test.go's setupAuthzFixtures: a resource server with read/write/delete actions,
a second resource server (used only to prove a mapped permission target stays scoped to the first), a
role with no DB assignees (the LEFT JOIN fix), and a group whose own role assignment grants access to
whoever a mapping places in the group.
*/
package federated

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// roleTarget builds a role AuthorizationTarget.
func roleTarget(id string) testutils.AuthorizationTarget {
	return testutils.AuthorizationTarget{Type: testutils.AuthorizationTargetRole, ID: id}
}

// groupTarget builds a group AuthorizationTarget.
func groupTarget(id string) testutils.AuthorizationTarget {
	return testutils.AuthorizationTarget{Type: testutils.AuthorizationTargetGroup, ID: id}
}

// permissionTarget builds a permission AuthorizationTarget.
func permissionTarget(resourceServerID, permission string) testutils.AuthorizationTarget {
	return testutils.AuthorizationTarget{
		Type: testutils.AuthorizationTargetPermission, ResourceServerID: resourceServerID, Permission: permission,
	}
}

// authzMapping builds a single-claim AuthorizationMapping configuration, alongside the attribute
// mappings fedPersonType needs to provision (email supplies both the required username and email
// attributes), since these scenarios are about authorization, not attribute mapping. values is a
// claim-value-to-targets map, converted into the equivalent equals rules (sorted by key, so the
// resulting configuration is deterministic) since every scenario in this file is an exact-match case.
func authzMapping(
	claim, delimiter string, values map[string][]testutils.AuthorizationTarget,
) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMappings = []testutils.AuthorizationMapping{
		{Claim: claim, Delimiter: delimiter, Values: equalsRules(values)},
	}
	return config
}

// authzMappingWithRules builds a single-claim AuthorizationMapping configuration from explicit rules,
// for scenarios that need an operator other than equals (authzMapping only ever builds equals rules).
func authzMappingWithRules(
	claim, valueType string, rules []testutils.AuthorizationRule,
) *testutils.AttributeConfiguration {
	return authzMappingWithRulesAndDelimiter(claim, valueType, "", rules)
}

// authzMappingWithRulesAndDelimiter is authzMappingWithRules with an explicit delimiter, for a
// multi-valued string claim (a delimiter is only meaningful for the string value type).
func authzMappingWithRulesAndDelimiter(
	claim, valueType, delimiter string, rules []testutils.AuthorizationRule,
) *testutils.AttributeConfiguration {
	config := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	config.AuthorizationMappings = []testutils.AuthorizationMapping{
		{Claim: claim, ValueType: valueType, Delimiter: delimiter, Values: rules},
	}
	return config
}

// equalsRules converts a claim-value-to-targets map into the equivalent equals rules, one per key,
// sorted by key for a deterministic result.
func equalsRules(values map[string][]testutils.AuthorizationTarget) []testutils.AuthorizationRule {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rules := make([]testutils.AuthorizationRule, 0, len(keys))
	for _, key := range keys {
		rules = append(rules, testutils.AuthorizationRule{
			Operator: testutils.AuthorizationOperatorEquals,
			Value:    key,
			Targets:  values[key],
		})
	}
	return rules
}

// authorizeFederated drives the authorization-mapping auth flow for a mock identity, requesting the
// given permissions against the given resource server, and returns the authorized_permissions carried
// on the resulting assertion (nil when the claim is absent, meaning nothing was authorized).
func (s *FederatedMappingSuite) authorizeFederated(
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo,
	requestedPermissions, resourceServerIdentifier string,
) []string {
	s.T().Helper()
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	inputs := map[string]string{
		"applicationId":              s.authzAppID,
		"requested_permissions":      requestedPermissions,
		"resource_server_identifier": resourceServerIdentifier,
	}
	flowStep, err := common.InitiateAuthenticationFlow(s.authzAppID, false, inputs, "")
	s.Require().NoError(err, "failed to initiate the authorization mapping flow")
	s.Require().Equal("REDIRECTION", flowStep.Type,
		"expected a redirection to the identity provider, got %+v", flowStep)

	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	step, err := common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to complete the authorization mapping flow")
	s.Require().Equal("COMPLETE", step.FlowStatus, "expected the flow to complete, got %+v", step)
	s.Require().NotEmpty(step.Assertion, "a completed flow should carry an assertion")

	provisioned, lookupErr := testutils.FindUserByAttribute("sub", user.Sub)
	if lookupErr == nil && provisioned != nil {
		s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)
	}

	claims, err := testutils.DecodeJWT(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")

	raw, ok := claims.Additional["authorized_permissions"]
	if !ok {
		return nil
	}
	str, ok := raw.(string)
	s.Require().True(ok, "authorized_permissions should be a string claim")
	if strings.TrimSpace(str) == "" {
		return []string{}
	}
	return strings.Fields(str)
}

// A1 (multi-valued claims table, list shape): a list-valued claim resolves every element, not the
// joined string, and the matched element's mapped role grants its permission.
func (s *FederatedMappingSuite) TestAuthzMapping_ListValuedClaimGrantsMappedRole() {
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"engineering", "platform-admins"}

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins": {roleTarget(s.authzMappedRoleID)},
	})

	authorized := s.authorizeFederated(config, user, "read write", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "the mapped role's permission should be authorized")
	s.NotContains(authorized, "write", "only the mapped role's own permission should be authorized")
}

// A list-valued claim combines with a non-equals operator the same way it does with equals: a rule
// contributes if any one of the claim's elements satisfies it. Here "engineering" (not "guest")
// satisfies not_equals "guest", end to end through a real signed ID token carrying a genuine JSON
// array, not a synthetic in-memory shortcut.
func (s *FederatedMappingSuite) TestAuthzMapping_NotEqualsOperatorMatchesElementInListClaim() {
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"guest", "engineering"}

	config := authzMappingWithRules("groups", "", []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorNotEquals,
			Value:    "guest",
			Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
		},
	})

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "not_equals should match on the list element that differs from the excluded value")
}

// not_includes tests absence from the whole set, unlike not_equals above: on the identical claim data
// (the user is a "guest" among other things), not_equals incorrectly granted access meant for
// non-guests, since "engineering" differs from "guest". not_includes correctly withholds it, since
// "guest" is present in the set, end to end through a real signed ID token.
func (s *FederatedMappingSuite) TestAuthzMapping_NotIncludesWithholdsWhenExcludedValuePresentInListClaim() {
	config := authzMappingWithRules("groups", testutils.AuthorizationValueTypeArray, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorNotIncludes,
			Value:    "guest",
			Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
		},
	})

	guestUser := s.baseUser(s.nextSubject())
	guestUser.Custom["groups"] = []interface{}{"guest", "engineering"}
	authorizedForGuest := s.authorizeFederated(config, guestUser, "read", "federated-authz-mapping-api")
	s.NotContains(authorizedForGuest, "read",
		"not_includes must not grant when the excluded value is present in the set, even alongside others")

	nonGuestUser := s.baseUser(s.nextSubject())
	nonGuestUser.Custom["groups"] = []interface{}{"engineering"}
	authorizedForNonGuest := s.authorizeFederated(config, nonGuestUser, "read", "federated-authz-mapping-api")
	s.Contains(authorizedForNonGuest, "read", "not_includes must grant when the excluded value is absent")
}

// includes is a straightforward membership test, end to end through a real signed ID token carrying a
// genuine JSON array.
func (s *FederatedMappingSuite) TestAuthzMapping_IncludesOperatorTestsSetMembershipInListClaim() {
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"engineering", "platform-admins"}

	config := authzMappingWithRules("groups", testutils.AuthorizationValueTypeArray, []testutils.AuthorizationRule{
		{
			Operator: testutils.AuthorizationOperatorIncludes,
			Value:    "platform-admins",
			Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
		},
	})

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "includes should match when the value is a member of the claim's set")
}

// A2 (multi-valued claims table, space-delimited shape): a space-delimited claim is split on the
// configured delimiter and each token is tried against the mapping's values.
func (s *FederatedMappingSuite) TestAuthzMapping_SpaceDelimitedClaimGrantsMappedRole() {
	user := s.baseUser(s.nextSubject())
	user.Custom["scope"] = "orders.read orders.write"

	config := authzMappingWithRulesAndDelimiter("scope", testutils.AuthorizationValueTypeString, " ",
		[]testutils.AuthorizationRule{
			{
				Operator: testutils.AuthorizationOperatorIncludes,
				Value:    "orders.write",
				Targets:  []testutils.AuthorizationTarget{roleTarget(s.authzMappedRoleID)},
			},
		})

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "a space-delimited token that matches a mapped value should grant its role")
}

// A3 (multi-valued claims table, scalar shape): a single-string claim with no delimiter is one token,
// and here it maps to a group rather than a role, exercising the group target's own path to a
// permission (via the role assigned to that group).
func (s *FederatedMappingSuite) TestAuthzMapping_ScalarClaimGrantsMappedGroup() {
	user := s.baseUser(s.nextSubject())
	user.Custom["department"] = "platform"

	config := authzMapping("department", "", map[string][]testutils.AuthorizationTarget{
		"platform": {groupTarget(s.authzMappedGroupID)},
	})

	authorized := s.authorizeFederated(config, user, "write", "federated-authz-mapping-api")
	s.Contains(authorized, "write", "the group the mapping placed the entity in should grant its role's permission")
}

// An attribute value with no configured mapping grants no permissions, even though a differently
// named claim would have matched.
func (s *FederatedMappingSuite) TestAuthzMapping_UnmappedValueGrantsNothing() {
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"marketing"}

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins": {roleTarget(s.authzMappedRoleID)},
	})

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Empty(authorized, "an unmapped claim value must not grant any permission")
}

// A federated entity receives the union of the permissions every source contributes: a mapped role, a
// mapped group, and a direct permission target together, from three distinct values of one claim.
func (s *FederatedMappingSuite) TestAuthzMapping_UnionOfRoleGroupPermissionTargets() {
	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-admins", "platform-editors", "platform-deleters"}

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":   {roleTarget(s.authzMappedRoleID)},
		"platform-editors":  {groupTarget(s.authzMappedGroupID)},
		"platform-deleters": {permissionTarget(s.authzResourceServerID, "delete")},
	})

	authorized := s.authorizeFederated(config, user, "read write delete", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read", "write", "delete"}, authorized,
		"the union of the mapped role, mapped group, and direct permission target should all be authorized")
}

// The LEFT JOIN fix: a role reachable only by a mapping naming it directly, with zero DB assignment
// rows, must still resolve. Before that fix the INNER JOIN between ROLE_PERMISSION and
// ROLE_ASSIGNMENT dropped such a role before its permissions were ever considered.
func (s *FederatedMappingSuite) TestAuthzMapping_MappedRoleWithNoLocalAssigneesGrants() {
	assignments, err := testutils.GetRoleAssignments(s.authzMappedRoleID)
	s.Require().NoError(err, "failed to read the mapped role's assignments")
	s.Require().Empty(assignments, "this scenario requires a role with no DB assignment rows")

	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-admins"}

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins": {roleTarget(s.authzMappedRoleID)},
	})

	authorized := s.authorizeFederated(config, user, "read", "federated-authz-mapping-api")
	s.Contains(authorized, "read", "a role named only through a mapping, with no assignment rows, must still grant")
}

// Mapped access combines with whatever the entity holds directly: an entity provisioned with no
// mapping, later given a direct role assignment, authenticates again under a mapping granting a
// second role, and both permissions are authorized together.
func (s *FederatedMappingSuite) TestAuthzMapping_DirectAssignmentPlusMappedRoleCombine() {
	user := s.baseUser(s.nextSubject())
	noMappingConfig := mapping(fedPersonType.Name, pair("email", "email"), pair("email", "username"))
	s.register(noMappingConfig, user)

	entity, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to look up the provisioned entity")
	s.Require().NotNil(entity, "the entity should have been provisioned")

	directRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Federated Direct Deleter " + s.nextSubject(),
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"delete"}},
		},
		Assignments: []testutils.Assignment{{ID: entity.ID, Type: "user"}},
	})
	s.Require().NoError(err, "failed to create the direct-assignment role")
	defer func() {
		if err := testutils.DeleteRole(directRoleID); err != nil {
			s.T().Logf("failed to delete the direct-assignment role: %v", err)
		}
	}()

	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins": {roleTarget(s.authzMappedRoleID)},
	})
	user.Custom["groups"] = []interface{}{"platform-admins"}

	authorized := s.authorizeFederated(config, user, "read delete", "federated-authz-mapping-api")
	s.ElementsMatch([]string{"read", "delete"}, authorized,
		"the entity's direct assignment and its mapped role should combine")
}

// A mapped permission target is scoped to the resource server it names: requesting the same
// permission handle against a different resource server must not be authorized by it, and requesting
// it against the resource server it does name must be.
func (s *FederatedMappingSuite) TestAuthzMapping_PermissionTargetScopedToItsOwnResourceServer() {
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins": {permissionTarget(s.authzResourceServerID, "read")},
	})

	other := s.baseUser(s.nextSubject())
	other.Custom["groups"] = []interface{}{"platform-admins"}
	authorizedOnOther := s.authorizeFederated(config, other, "read", "federated-authz-mapping-other-api")
	s.NotContains(authorizedOnOther, "read",
		"a permission target scoped to one resource server must not authorize the same handle on another")

	own := s.baseUser(s.nextSubject())
	own.Custom["groups"] = []interface{}{"platform-admins"}
	authorizedOnOwn := s.authorizeFederated(config, own, "read", "federated-authz-mapping-api")
	s.Contains(authorizedOnOwn, "read",
		"a permission target must authorize its own resource server")
}

// consentPromptElement, consentPromptPurpose, consentElementDecision, consentPurposeDecision, and
// consentDecisions mirror the consent prompt/decision wire shapes used by the flow authentication
// package's own consent suites (tests/integration/flow/authentication). Duplicated locally rather
// than imported, since that package's types are unexported.
type consentPromptElement struct {
	Name   string `json:"name"`
	Parent string `json:"parent,omitempty"`
}

type consentPromptPurpose struct {
	PurposeName string                 `json:"purposeName"`
	Type        string                 `json:"type"`
	Essential   []consentPromptElement `json:"essential"`
	Optional    []consentPromptElement `json:"optional"`
}

type consentElementDecision struct {
	Name     string `json:"name"`
	Approved bool   `json:"approved"`
}

type consentPurposeDecision struct {
	PurposeName string                   `json:"purposeName"`
	Approved    bool                     `json:"approved"`
	Elements    []consentElementDecision `json:"elements"`
}

type consentDecisions struct {
	Approved bool                     `json:"approved"`
	Purposes []consentPurposeDecision `json:"purposes"`
}

// TestAuthzMapping_ConsentSurfacesAndCanDeclineMappedPermission drives a one-off flow that adds a
// ConsentExecutor after the authorization check, so consent's "shown and can be declined" requirement
// is exercised for a permission that only a mapping (not a direct assignment) authorized. Consent
// reads authorized_permissions from runtime data, which is the same key the authorization node just
// populated from the mapping, so no separate wiring is under test here beyond that the two compose.
func (s *FederatedMappingSuite) TestAuthzMapping_ConsentSurfacesAndCanDeclineMappedPermission() {
	flow := testutils.Flow{
		Name:     "Federated Authorization Mapping Consent Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_federated_authz_mapping_consent",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":   "oidc_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                               s.idpID,
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor":  map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess": "provisioning",
			},
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "authorization_check",
			},
			{
				"id":        "authorization_check",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
				"onSuccess": "consent_check",
			},
			{
				"id":           "consent_check",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "ConsentExecutor"},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_consent",
			},
			{
				"id":   "prompt_consent",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "consent_input", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "consent_action_allow", "nextNode": "consent_check"},
					},
					{
						"inputs": []map[string]interface{}{
							{"ref": "consent_input", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "consent_action_deny", "nextNode": "consent_check"},
					},
				},
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
	appID := s.createScenarioApp(flow, "federated-authz-consent-client")

	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-admins", "platform-deleters"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":   {roleTarget(s.authzMappedRoleID)},
		"platform-deleters": {permissionTarget(s.authzResourceServerID, "delete")},
	})
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	inputs := map[string]string{
		"applicationId":              appID,
		"requested_permissions":      "read delete",
		"resource_server_identifier": "federated-authz-mapping-api",
	}
	flowStep, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	s.Require().NoError(err, "failed to initiate the flow")
	s.Require().Equal("REDIRECTION", flowStep.Type, "expected a redirection, got %+v", flowStep)

	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to advance past the federated login")
	s.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"the flow should pause on the consent prompt, got %+v", flowStep)

	if provisioned, lookupErr := testutils.FindUserByAttribute("sub", user.Sub); lookupErr == nil && provisioned != nil {
		s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)
	}

	promptJSON, ok := flowStep.Data.AdditionalData["consentPrompt"]
	s.Require().True(ok, "the consent prompt must carry the purposes to render, got %+v", flowStep.Data.AdditionalData)

	var purposes []consentPromptPurpose
	s.Require().NoError(json.Unmarshal([]byte(promptJSON), &purposes), "failed to parse the consent prompt")

	var permissionsPurpose *consentPromptPurpose
	for i := range purposes {
		if purposes[i].Type == "permissions" {
			permissionsPurpose = &purposes[i]
			break
		}
	}
	s.Require().NotNil(permissionsPurpose, "the consent prompt must carry a permissions purpose, got %+v", purposes)

	var names []string
	for _, element := range append(
		append([]consentPromptElement{}, permissionsPurpose.Essential...), permissionsPurpose.Optional...) {
		names = append(names, element.Name)
	}
	s.Contains(names, "read", "the mapped role's permission must be offered for consent")
	s.Contains(names, "delete", "the mapped permission target must be offered for consent")

	// Approve read, decline delete: the consented set determines what the assertion carries. Every
	// other purpose (e.g. the scenario app's default attribute release) is approved as-is, since only
	// the permissions purpose is under test here.
	decisions := consentDecisions{Approved: true}
	for _, purpose := range purposes {
		if purpose.Type != "permissions" {
			decision := consentPurposeDecision{PurposeName: purpose.PurposeName, Approved: true}
			for _, element := range append(append([]consentPromptElement{}, purpose.Essential...), purpose.Optional...) {
				decision.Elements = append(decision.Elements, consentElementDecision{Name: element.Name, Approved: true})
			}
			decisions.Purposes = append(decisions.Purposes, decision)
			continue
		}
		decision := consentPurposeDecision{PurposeName: purpose.PurposeName, Approved: true}
		for _, name := range names {
			decision.Elements = append(decision.Elements, consentElementDecision{Name: name, Approved: name == "read"})
		}
		decisions.Purposes = append(decisions.Purposes, decision)
	}
	decisionsJSON, err := json.Marshal(decisions)
	s.Require().NoError(err, "failed to marshal the consent decision")

	completed, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"consent_decisions": string(decisionsJSON)}, "consent_action_allow", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to submit the consent decision")
	s.Require().Equal("COMPLETE", completed.FlowStatus, "expected the flow to complete, got %+v", completed)
	s.Require().NotEmpty(completed.Assertion, "a completed flow should carry an assertion")

	claims, err := testutils.DecodeJWT(completed.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	raw, ok := claims.Additional["authorized_permissions"]
	s.Require().True(ok, "authorized_permissions claim should be present")
	authorized := strings.Fields(raw.(string))

	s.Contains(authorized, "read", "the approved mapped permission should be granted")
	s.NotContains(authorized, "delete", "the declined mapped permission must not be granted")
}

// TestAuthzMapping_ProvisioningSeedsRolesAndGroupsFromMapping drives a one-off flow whose provisioning
// step opts into seedGroupsFromMapping and seedRolesFromMapping, so a newly provisioned federated
// entity is given real, persisted role and group assignments derived from whatever its claims mapped
// to at that moment, alongside whatever the AuthorizationExecutor grants dynamically. Seeding is a
// one-time creation-time action, so this is verified by reading the assignments back directly rather
// than only observing the assertion's authorized_permissions claim (which every other test in this
// file already exercises for the dynamic, unseeded path). A dedicated throwaway role and group are used
// rather than the suite's shared mapped fixtures, since seeding persists a real assignment that would
// otherwise leak into TestAuthzMapping_MappedRoleWithNoLocalAssigneesGrants' "zero assignees" premise
// and into other tests sharing those fixtures; there is no test API to remove a group member once
// added, so the group itself is deleted afterward instead.
func (s *FederatedMappingSuite) TestAuthzMapping_ProvisioningSeedsRolesAndGroupsFromMapping() {
	seedRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Federated Seeding Test Role",
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"read"}},
		},
	})
	s.Require().NoError(err, "failed to create the throwaway seeding role")
	defer func() {
		if err := testutils.DeleteRole(seedRoleID); err != nil {
			s.T().Logf("failed to delete the throwaway seeding role: %v", err)
		}
	}()

	seedGroupID, err := testutils.CreateGroup(testutils.Group{Name: "Federated Seeding Test Group", OUID: s.ouID})
	s.Require().NoError(err, "failed to create the throwaway seeding group")
	defer func() {
		if err := testutils.DeleteGroup(seedGroupID); err != nil {
			s.T().Logf("failed to delete the throwaway seeding group: %v", err)
		}
	}()
	seedGroupRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "Federated Seeding Test Group Role",
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: s.authzResourceServerID, Permissions: []string{"write"}},
		},
		Assignments: []testutils.Assignment{{ID: seedGroupID, Type: "group"}},
	})
	s.Require().NoError(err, "failed to create the throwaway seeding group's role")
	defer func() {
		if err := testutils.DeleteRole(seedGroupRoleID); err != nil {
			s.T().Logf("failed to delete the throwaway seeding group's role: %v", err)
		}
	}()

	flow := testutils.Flow{
		Name:     "Federated Authorization Mapping Seeding Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_federated_authz_mapping_seeding",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":   "oidc_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                               s.idpID,
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor":  map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"seedGroupsFromMapping": true,
					"seedRolesFromMapping":  true,
				},
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "authorization_check",
			},
			{
				"id":        "authorization_check",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
	appID := s.createScenarioApp(flow, "federated-authz-seeding-client")

	user := s.baseUser(s.nextSubject())
	user.Custom["groups"] = []interface{}{"platform-admins", "platform-editors"}
	config := authzMapping("groups", "", map[string][]testutils.AuthorizationTarget{
		"platform-admins":  {roleTarget(seedRoleID)},
		"platform-editors": {groupTarget(seedGroupID)},
	})
	s.applyConfig(config)
	s.mockOIDC.AddUser(user)
	s.activeSub = user.Sub

	inputs := map[string]string{
		"applicationId":              appID,
		"requested_permissions":      "read write",
		"resource_server_identifier": "federated-authz-mapping-api",
	}
	flowStep, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	s.Require().NoError(err, "failed to initiate the flow")
	s.Require().Equal("REDIRECTION", flowStep.Type, "expected a redirection, got %+v", flowStep)

	code, state, err := testutils.SimulateFederatedOAuthFlow(flowStep.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")

	completed, err := common.CompleteFlow(
		flowStep.ExecutionID, map[string]string{"code": code, "state": state}, "", flowStep.ChallengeToken)
	s.Require().NoError(err, "failed to complete the flow")
	s.Require().Equal("COMPLETE", completed.FlowStatus, "expected the flow to complete, got %+v", completed)
	s.Require().NotEmpty(completed.Assertion, "a completed flow should carry an assertion")

	provisioned, err := testutils.FindUserByAttribute("sub", user.Sub)
	s.Require().NoError(err, "failed to look up the provisioned entity")
	s.Require().NotNil(provisioned, "the entity should have been provisioned")
	s.config.CreatedUserIDs = append(s.config.CreatedUserIDs, provisioned.ID)

	// The permissions are granted on this first login regardless of seeding (the dynamic mapping alone
	// would produce the same result), so the real signal is the persisted assignment below.
	claims, err := testutils.DecodeJWT(completed.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	authorized := strings.Fields(claims.Additional["authorized_permissions"].(string))
	s.ElementsMatch([]string{"read", "write"}, authorized, "the mapped role and group should both grant on first login")

	roleAssignments, err := testutils.GetRoleAssignments(seedRoleID)
	s.Require().NoError(err, "failed to read the seeding role's assignments")
	var roleAssigneeIDs []string
	for _, a := range roleAssignments {
		roleAssigneeIDs = append(roleAssigneeIDs, a.ID)
	}
	s.Contains(roleAssigneeIDs, provisioned.ID,
		"seedRolesFromMapping should have persisted a real assignment to the mapped role")

	groupMembers, err := testutils.GetGroupMembers(seedGroupID)
	s.Require().NoError(err, "failed to read the seeding group's members")
	var groupMemberIDs []string
	for _, m := range groupMembers {
		groupMemberIDs = append(groupMemberIDs, m.ID)
	}
	s.Contains(groupMemberIDs, provisioned.ID,
		"seedGroupsFromMapping should have persisted real membership in the mapped group")
}
