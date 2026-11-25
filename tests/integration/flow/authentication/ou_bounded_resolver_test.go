// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// errCodeNoEligibleOU is returned when the signed-in user has no eligible OU to decide.
const errCodeNoEligibleOU = "FET-1122"

// errCodeApplicationNotInOU is returned when the OU picked before sign-in is one the application is not usable from.
const errCodeApplicationNotInOU = "FET-1123"

// ouBoundaryPickFirstFlow returns a sign-in flow that asks for an OU below parentOUID before the credentials.
func ouBoundaryPickFirstFlow(handle, parentOUID string) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "OU Boundary " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "resolve_ou"},
			{
				"id":           "resolve_ou",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "OUResolverExecutor"},
				"properties":   map[string]interface{}{"resolveFrom": "prompt", "parentOuId": parentOUID},
				"onSuccess":    "prompt_credentials",
				"onIncomplete": "prompt_ou",
			},
			{
				"id":   "prompt_ou",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou", "identifier": "ouId", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_ou", "nextNode": "resolve_ou"},
					},
				},
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": credentialInputs,
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
				"onSuccess":    "authorization_check",
				"onIncomplete": "prompt_credentials",
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
}

// sharedAppWithResolver creates an app-native application in the parent, shared with every unit below it,
// whose sign-in decides the OU after the credentials with a resolver of the given properties.
func (ts *OUBoundedSignInTestSuite) sharedAppWithResolver(handle string, resolver map[string]interface{}) string {
	ts.T().Helper()
	flowID := ts.createFlow(ouBoundaryFlow(handle, resolver))
	appID := ts.createNativeApp("OU Boundary "+handle, handle, ts.parentOUID, flowID, ts.allowedTypes,
		map[string]interface{}{"userAttributes": []string{"ouId", "ouHandle"}})
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share application %s", handle)
	return appID
}

// After sign-in, prompt offers the user's eligible OUs at or below parentOuId, at any depth.
func (ts *OUBoundedSignInTestSuite) TestPromptAfterSignInBelowParent_OffersEveryDepth() {
	appID := ts.sharedAppWithResolver("ou_boundary_prompt_any_depth", ouBoundaryPromptAfterSignIn(ts.parentOUID))

	step := ts.signIn(appID, "", ouBoundaryDeepUser, nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "two eligible OUs need a choice: %+v", step.Error)
	input := findDisambiguationInput(step.Data.Inputs, "ouId")
	ts.Require().NotNil(input, "the OU is asked for: %+v", step.Data.Inputs)
	grandchildPath := handlePath(ouBoundaryParentHandle, ouBoundaryChildHandle, ouBoundaryGrandchildHandle)
	ts.Equal([]string{handlePath(ouBoundaryParentHandle, ouBoundaryChildHandle), grandchildPath}, input.Options,
		"the user's own OU, a child, and the grandchild it is shared into")

	chosen, err := common.CompleteFlow(step.ExecutionID, map[string]string{"ouId": grandchildPath}, "action_ou",
		step.ChallengeToken)
	ts.Require().NoError(err)
	ts.requireSignedInto(chosen, ts.grandchildOUID)
}

// With directChildrenOnly, prompt after sign-in offers only the parent's direct children, so a user eligible in a
// child and a grandchild is decided into the child without being asked.
func (ts *OUBoundedSignInTestSuite) TestPromptAfterSignInBelowParent_DirectChildrenOnly() {
	appID := ts.sharedAppWithResolver("ou_boundary_prompt_direct_children", map[string]interface{}{
		"resolveFrom": "prompt", "parentOuId": ts.parentOUID, "directChildrenOnly": true,
	})

	step := ts.signIn(appID, "", ouBoundaryDeepUser, nil)
	ts.False(common.HasInput(step.Data.Inputs, "ouId"), "the grandchild is not offered, leaving one OU")
	ts.requireSignedInto(step, ts.childOUID)
}

// With directChildrenOnly the parent itself is not offered: a user eligible in the parent and a child is decided
// into the child, and a user eligible in the parent only has no OU to sign in to.
func (ts *OUBoundedSignInTestSuite) TestPromptAfterSignInBelowParent_DirectChildrenOnlyExcludesTheParent() {
	appID := ts.sharedAppWithResolver("ou_boundary_prompt_not_parent", map[string]interface{}{
		"resolveFrom": "prompt", "parentOuId": ts.parentOUID, "directChildrenOnly": true,
	})

	step := ts.signIn(appID, "", ouBoundarySharedUser, nil)
	ts.False(common.HasInput(step.Data.Inputs, "ouId"), "the parent is not offered, leaving the child")
	ts.requireSignedInto(step, ts.childOUID)

	refused := ts.signIn(appID, "", ouBoundaryParentOnlyUser, nil)
	ts.NotEqual("COMPLETE", refused.FlowStatus)
	ts.Empty(refused.Assertion, "a user with no OU to sign in to gets no assertion")
	ts.Require().NotNil(refused.Error, "the sign-in is refused: %+v", refused)
	ts.Equal(errCodeNoEligibleOU, refused.Error.Code)
}

// With the OU picked before sign-in, a picked OU the application is shared with bounds the lookup to it, and a
// picked OU it is not shared with is refused at the pick, before any credentials are asked for.
func (ts *OUBoundedSignInTestSuite) TestPickFirst_PickedOUBoundsTheLookup() {
	flowID := ts.createFlow(ouBoundaryPickFirstFlow("ou_boundary_pick_first", ts.parentOUID))
	appID := ts.createNativeApp("OU Boundary Pick First", "ou_boundary_pick_first", ts.parentOUID, flowID,
		ts.allowedTypes, map[string]interface{}{"userAttributes": []string{"ouId"}})
	ts.shareApplicationWith(appID, ts.childOUID)

	pick := func(ouID string) *common.FlowStep {
		status, raw := ts.startSignIn(appID, "", nil)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		var step common.FlowStep
		ts.Require().NoError(json.Unmarshal(raw, &step))
		ts.Require().True(common.HasInput(step.Data.Inputs, "ouId"), "the OU is asked for first: %s", raw)

		next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"ouId": ouID}, "action_ou",
			step.ChallengeToken)
		ts.Require().NoError(err)
		return next
	}
	signInTo := func(ouID, username string) *common.FlowStep {
		credentials := pick(ouID)
		ts.Require().True(common.HasInput(credentials.Data.Inputs, "username"),
			"then the credentials: %+v", credentials.Error)

		next, err := common.CompleteFlow(credentials.ExecutionID,
			map[string]string{"username": username, "password": ouBoundaryPassword}, "action_001",
			credentials.ChallengeToken)
		ts.Require().NoError(err)
		return next
	}

	ts.Run("SharedOU", func() {
		ts.requireSignedInto(signInTo(ts.childOUID, ouBoundarySharedUser), ts.childOUID)
		ts.requireNotFound(signInTo(ts.childOUID, ouBoundaryChildTwoUser))
	})
	ts.Run("UnsharedOU", func() {
		refused := pick(ts.childTwoOUID)
		ts.Equal("INCOMPLETE", refused.FlowStatus, "the pick is refused: %+v", refused)
		ts.Require().NotNil(refused.Error, "the pick is refused with an error: %+v", refused)
		ts.Equal(errCodeApplicationNotInOU, refused.Error.Code)
		ts.True(common.HasInput(refused.Data.Inputs, "ouId"), "the OU is asked for again")
		ts.False(common.HasInput(refused.Data.Inputs, "username"), "no credential prompt was reached")
		ts.Empty(refused.Assertion)
	})
}
