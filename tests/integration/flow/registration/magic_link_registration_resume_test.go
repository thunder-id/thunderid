// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestMagicLinkRegistration_ResumeRetainsDynamicInputs asserts that re-rendering a paused prompt
// reproduces the step, including inputs the flow only learned about at runtime. The schema
// attributes here are not declared on the node: ProvisioningExecutor forwards them, and forwarded
// data lives for a single hop, so a client that resumes rather than replays that step would
// otherwise be shown a prompt with no fields and no way to satisfy the flow.
func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_ResumeRetainsDynamicInputs() {
	ts.mockSMTP.ClearEmails()
	emailAddr := common.GenerateUniqueUsername("resume") + "@example.com"

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": emailAddr},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)

	token := common.ExtractMagicLinkToken(ts.waitForEmail())
	ts.Require().NotEmpty(token)

	// Verifying the token pauses on the dynamic attribute prompt.
	step3, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", step3.FlowStatus)
	ts.Require().True(common.HasInput(step3.Data.Inputs, "given_name"))
	ts.Require().True(common.HasInput(step3.Data.Inputs, "family_name"))

	// Resuming without inputs or an action must render that same prompt, fields included.
	resumed, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "", step3.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", resumed.FlowStatus)
	ts.Require().True(common.HasInput(resumed.Data.Inputs, "given_name"),
		"given_name must survive a resume of the paused prompt")
	ts.Require().True(common.HasInput(resumed.Data.Inputs, "family_name"),
		"family_name must survive a resume of the paused prompt")

	// Resuming repeatedly stays stable.
	resumedAgain, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "", resumed.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(resumedAgain.Data.Inputs, "given_name"))
	ts.Require().True(common.HasInput(resumedAgain.Data.Inputs, "family_name"))

	// The attributes gathered from the resumed prompt still complete the flow.
	inputs := map[string]string{
		"username":    common.GenerateUniqueUsername("username"),
		"given_name":  "Test",
		"family_name": "User",
	}
	final, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_schema_attrs", resumedAgain.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", final.FlowStatus)

	user, err := testutils.FindUserByAttribute("email", emailAddr)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}
