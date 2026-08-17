// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Registration-flow inference is off by default, so it needs the deployment flag enabled and a
// restart before any of it runs. It lives in its own suite rather than the main flow management
// suite so the restart is paid once and cannot disturb the other tests.
type FlowInferenceTestSuite struct {
	suite.Suite
	createdFlowIDs []string
}

func TestFlowInferenceTestSuite(t *testing.T) {
	suite.Run(t, new(FlowInferenceTestSuite))
}

// PatchDeploymentConfig merges at the top level only, so a patch for one key inside "flow" replaces
// the whole block. Both patches therefore restate max_version_history exactly as the integration
// deployment sets it (tests/integration/resources/deployment.yaml); dropping it silently reverts the
// limit to the product default and breaks the version-history tests in this same package.
var inferenceEnablePatch = map[string]interface{}{
	"flow": map[string]interface{}{
		"max_version_history":     3,
		"auto_infer_registration": true,
	},
}

var inferenceDisablePatch = map[string]interface{}{
	"flow": map[string]interface{}{
		"max_version_history":     3,
		"auto_infer_registration": false,
	},
}

func (suite *FlowInferenceTestSuite) SetupSuite() {
	suite.Require().NoError(testutils.PatchDeploymentConfig(inferenceEnablePatch),
		"failed to enable registration flow inference")
	suite.Require().NoError(testutils.RestartServer(),
		"failed to restart server with inference enabled")
	suite.Require().NoError(testutils.ObtainAdminAccessToken(),
		"failed to re-obtain admin token after restart")
}

func (suite *FlowInferenceTestSuite) TearDownSuite() {
	for _, flowID := range suite.createdFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			suite.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}

	// These three restore global state. A silent failure here leaves inference enabled for every
	// suite that runs afterwards, so report it rather than logging it. Assert rather than Require,
	// so one failure does not skip the restoration steps that follow it.
	suite.Assert().NoError(testutils.PatchDeploymentConfig(inferenceDisablePatch),
		"teardown: failed to restore inference config")
	suite.Assert().NoError(testutils.RestartServer(),
		"teardown: server did not restart cleanly after config restore")
	suite.Assert().NoError(testutils.ObtainAdminAccessToken(),
		"teardown: failed to re-obtain admin token after restore")
}

// listFlowsByType returns every flow of the given type, walking pages so a newly inferred flow is
// found regardless of how many already exist.
func (suite *FlowInferenceTestSuite) listFlowsByType(flowType string) []BasicFlowDefinition {
	var all []BasicFlowDefinition

	for offset := 0; ; offset += 100 {
		url := testServerURL + flowsEndpoint + "?flowType=" + flowType +
			"&limit=100&offset=" + strconv.Itoa(offset)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		suite.Require().NoError(err)

		resp, err := testutils.GetHTTPClient().Do(req)
		suite.Require().NoError(err)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		suite.Require().NoError(err)
		suite.Require().Equal(http.StatusOK, resp.StatusCode, "failed to list flows: %s", string(body))

		var listed FlowListResponse
		suite.Require().NoError(json.Unmarshal(body, &listed))

		all = append(all, listed.Flows...)
		if len(listed.Flows) < 100 {
			return all
		}
	}
}

// createFlow posts a flow definition and returns the created flow.
func (suite *FlowInferenceTestSuite) createFlow(flowDef FlowDefinition) *CompleteFlowDefinition {
	body, err := json.Marshal(flowDef)
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+flowsEndpoint, bytes.NewBuffer(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "failed to create flow: %s", string(bodyBytes))

	var response CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(bodyBytes, &response))
	return &response
}

// getFlow reads a flow by id.
func (suite *FlowInferenceTestSuite) getFlow(flowID string) *CompleteFlowDefinition {
	req, err := http.NewRequest(http.MethodGet, testServerURL+flowsEndpoint+"/"+flowID, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "failed to read flow: %s", string(bodyBytes))

	var response CompleteFlowDefinition
	suite.Require().NoError(json.Unmarshal(bodyBytes, &response))
	return &response
}

// Creating an authentication flow with inference enabled also creates a registration flow derived
// from it, named by substituting the authentication term. This is what lets an application offer
// sign-up without an operator authoring a second flow by hand.
func (suite *FlowInferenceTestSuite) TestCreateAuthFlow_InfersRegistrationFlow() {
	before := len(suite.listFlowsByType("REGISTRATION"))

	authFlow := cloneFlowWithUniqueHandle(testAuthFlow)
	authFlow.Name = "Inference Probe Authentication Flow"
	created := suite.createFlow(authFlow)
	suite.createdFlowIDs = append(suite.createdFlowIDs, created.ID)

	after := suite.listFlowsByType("REGISTRATION")
	suite.Greater(len(after), before, "creating an authentication flow should infer a registration flow")

	found := false
	for _, flow := range after {
		if strings.Contains(flow.Name, "Inference Probe") {
			found = true
			suite.Equal("REGISTRATION", flow.FlowType)
			suite.Contains(flow.Name, "Registration",
				"the inferred flow should be renamed from Authentication to Registration")
			suite.createdFlowIDs = append(suite.createdFlowIDs, flow.ID)
		}
	}
	suite.True(found, "expected a registration flow inferred from the probe authentication flow")
}

// The inferred flow is a registration flow in its own right, so it must carry the provisioning node
// that turns collected credentials into a user. Without it the inferred flow would complete without
// creating anything.
func (suite *FlowInferenceTestSuite) TestInferredRegistrationFlow_ContainsProvisioningNode() {
	authFlow := cloneFlowWithUniqueHandle(testAuthFlow)
	authFlow.Name = "Provisioning Probe Signin Flow"
	created := suite.createFlow(authFlow)
	suite.createdFlowIDs = append(suite.createdFlowIDs, created.ID)

	var inferredID string
	for _, flow := range suite.listFlowsByType("REGISTRATION") {
		if strings.Contains(flow.Name, "Provisioning Probe") {
			inferredID = flow.ID
			suite.createdFlowIDs = append(suite.createdFlowIDs, flow.ID)
		}
	}
	suite.Require().NotEmpty(inferredID, "expected a registration flow inferred from the probe flow")

	inferred := suite.getFlow(inferredID)

	hasProvisioning := false
	for _, node := range inferred.Nodes {
		if node.Executor != nil && node.Executor.Name == "ProvisioningExecutor" {
			hasProvisioning = true
		}
	}
	suite.True(hasProvisioning, "an inferred registration flow must provision the user it registers")
}

// Inference only applies to authentication flows. Creating a registration flow directly must not
// produce a further flow derived from it.
func (suite *FlowInferenceTestSuite) TestCreateNonAuthFlow_DoesNotInfer() {
	before := len(suite.listFlowsByType("REGISTRATION"))

	regFlow := cloneFlowWithUniqueHandle(testAuthFlow)
	regFlow.Name = "Non Auth Probe Flow"
	regFlow.FlowType = "REGISTRATION"
	// A registration flow must resolve a user type and provision the user it registers, so the cloned
	// authentication nodes need both executors before the flow is accepted.
	regFlow.Nodes = withRegistrationExecutors(regFlow.Nodes)

	created := suite.createFlow(regFlow)
	suite.createdFlowIDs = append(suite.createdFlowIDs, created.ID)

	after := suite.listFlowsByType("REGISTRATION")
	suite.Equal(before+1, len(after),
		"creating a registration flow should add exactly one flow, with nothing inferred from it")
}

// Flow validation requires each flow type to carry the executor that makes it meaningful, so a
// registration flow without a provisioning step is rejected rather than stored and left to fail at
// execution time.
func (suite *FlowInferenceTestSuite) TestCreateRegistrationFlow_RequiresProvisioningExecutor() {
	regFlow := cloneFlowWithUniqueHandle(testAuthFlow)
	regFlow.Name = "Missing Provisioning Probe Flow"
	regFlow.FlowType = "REGISTRATION"

	body, err := json.Marshal(regFlow)
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+flowsEndpoint, bytes.NewBuffer(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Equal(http.StatusBadRequest, resp.StatusCode,
		"a registration flow without a provisioning executor must be rejected: %s", string(bodyBytes))

	var errResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(bodyBytes, &errResp))
	suite.Equal("FLM-1023", errResp.Code)
}

// smsAuthFlowNodes builds an SMS OTP authentication flow whose identify prompt carries authentication
// specific UI: a sign-in heading and a self sign-up link, both of which inference has to rewrite.
func smsAuthFlowNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_identify",
		},
		{
			"id":   "prompt_identify",
			"type": "PROMPT",
			"meta": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"type":    "TEXT",
						"id":      "heading_001",
						"label":   "Sign In to Continue",
						"variant": "HEADING_1",
					},
					{
						"type": "BLOCK",
						"id":   "block_001",
						"components": []map[string]interface{}{
							{
								"type":     "TEXT_INPUT",
								"id":       "input_001",
								"label":    "Username",
								"required": true,
							},
							{
								"type":  "ACTION",
								"id":    "action_001",
								"label": "Continue",
							},
							{
								"type":  "RICH_TEXT",
								"id":    "signup_link_001",
								"label": `<a data-component-ref="self-sign-up-link" href="#">Create an account</a>`,
							},
						},
					},
				},
			},
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_001",
						"nextNode": "generate_otp",
					},
				},
			},
		},
		{
			"id":   "generate_otp",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "OTPExecutor",
				"mode": "generate",
				"inputs": []map[string]interface{}{
					{
						"ref":        "input_mobile",
						"identifier": "mobile_number",
						"type":       "PHONE_INPUT",
						"required":   true,
					},
				},
			},
			"onSuccess": "prompt_otp",
		},
		{
			"id":   "prompt_otp",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_002",
							"identifier": "otp",
							"type":       "OTP_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_002",
						"nextNode": "verify_otp",
					},
				},
			},
		},
		{
			"id":   "verify_otp",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "OTPExecutor",
				"mode": "verify",
			},
			"onSuccess": "auth_assert",
		},
		{
			"id":   "auth_assert",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "AuthAssertExecutor",
			},
			"onSuccess": "end",
		},
		{
			"id":   "end",
			"type": "END",
		},
	}
}

// createRawFlow creates a flow from a raw definition, which the inference fixtures need because they
// use node shapes (executor modes and inputs, prompt meta) that the typed test model does not carry.
func (suite *FlowInferenceTestSuite) createRawFlow(name, handle string,
	nodes []map[string]interface{}) string {
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     name,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes:    nodes,
	})
	suite.Require().NoError(err, "failed to create flow %s", handle)
	suite.createdFlowIDs = append(suite.createdFlowIDs, flowID)
	return flowID
}

// getRawFlow reads a flow as generic JSON so prompt and meta contents can be walked.
func (suite *FlowInferenceTestSuite) getRawFlow(flowID string) map[string]interface{} {
	req, err := http.NewRequest(http.MethodGet, testServerURL+flowsEndpoint+"/"+flowID, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "failed to read flow: %s", string(bodyBytes))

	var flow map[string]interface{}
	suite.Require().NoError(json.Unmarshal(bodyBytes, &flow))
	return flow
}

// inferredFlowIDFor returns the id of the registration flow inferred from the named authentication
// flow, tracking it for cleanup.
func (suite *FlowInferenceTestSuite) inferredFlowIDFor(namePart string) string {
	for _, flow := range suite.listFlowsByType("REGISTRATION") {
		if strings.Contains(flow.Name, namePart) {
			suite.createdFlowIDs = append(suite.createdFlowIDs, flow.ID)
			return flow.ID
		}
	}
	return ""
}

// nodesOf returns the node list of a raw flow.
func nodesOf(flow map[string]interface{}) []interface{} {
	nodes, ok := flow["nodes"].([]interface{})
	if !ok {
		return nil
	}
	return nodes
}

// flowMetaJSON returns the concatenated meta of every node, as JSON text, so label rewrites and
// component removals can be asserted without walking the whole component tree.
func flowMetaJSON(suite *FlowInferenceTestSuite, flow map[string]interface{}) string {
	var builder strings.Builder
	for _, raw := range nodesOf(flow) {
		node, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		meta, ok := node["meta"]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(meta)
		suite.Require().NoError(err)
		builder.Write(encoded)
	}
	return builder.String()
}

// The inferred flow carries the source flow's UI, so its authentication wording and its links back to
// sign-up have to be rewritten: a sign-in heading becomes a sign-up heading, and the self sign-up link
// is dropped because the inferred flow is the sign-up.
func (suite *FlowInferenceTestSuite) TestInferredRegistrationFlow_RewritesPromptMeta() {
	authID := suite.createRawFlow("Meta Probe Authentication Flow",
		"auth_flow_inference_meta_probe", smsAuthFlowNodes())
	suite.Require().NotEmpty(authID)

	inferredID := suite.inferredFlowIDFor("Meta Probe")
	suite.Require().NotEmpty(inferredID, "expected a registration flow inferred from the meta probe flow")

	meta := flowMetaJSON(suite, suite.getRawFlow(inferredID))
	suite.Require().NotEmpty(meta, "the inferred flow should carry the source flow's prompt meta")

	suite.Contains(meta, "Sign Up to Continue",
		"an authentication heading must be rewritten to its registration equivalent")
	suite.NotContains(meta, "Sign In to Continue",
		"the authentication heading must not survive in the inferred flow")
	suite.NotContains(meta, "self-sign-up-link",
		"a self sign-up link is meaningless inside the inferred sign-up flow")
}

// withRegistrationExecutors inserts the executors a registration flow must carry, ahead of the END
// node, producing a definition that satisfies flow-type validation. Registration requires both a
// user type resolver and a provisioning step (see requiredExecutorsByFlowType in the validator).
func withRegistrationExecutors(nodes []NodeDefinition) []NodeDefinition {
	const (
		resolverID  = "resolve_user_type"
		provisionID = "provision_user"
	)

	out := make([]NodeDefinition, 0, len(nodes)+2)
	for _, node := range nodes {
		if node.Type == "END" {
			out = append(out,
				NodeDefinition{
					ID:        resolverID,
					Type:      "TASK_EXECUTION",
					Executor:  &ExecutorDefinition{Name: "UserTypeResolver"},
					OnSuccess: provisionID,
				},
				NodeDefinition{
					ID:        provisionID,
					Type:      "TASK_EXECUTION",
					Executor:  &ExecutorDefinition{Name: "ProvisioningExecutor"},
					OnSuccess: node.ID,
				},
				node,
			)
			continue
		}
		if node.OnSuccess != "" && isEndNode(nodes, node.OnSuccess) {
			node.OnSuccess = resolverID
		}
		out = append(out, node)
	}
	return out
}

func isEndNode(nodes []NodeDefinition, id string) bool {
	for _, node := range nodes {
		if node.ID == id && node.Type == "END" {
			return true
		}
	}
	return false
}
