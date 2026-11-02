// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	errorInvalidFlowData     = "FLM-1005"
	errorFlowNotFound        = "FLM-1003"
	errorSharingPolicyExists = "FLM-1031"
	errorUnknownVariable     = "FLM-1035"
	errorInvalidValue        = "FLM-1036"
	errorNotEditable         = "FLM-1037"
	errorMissingOU           = "FLM-1038"
	errorSavedValuesConflict = "FLM-1042"
	errorSharedFlowReadOnly  = "FLM-1039"
)

// FlowVariablesSharingTestSuite covers the variables of a flow and sharing them: the owner declares them in a
// VARIABLES node and shares the flow, a unit beneath it sees them and sets its own values within the owner's terms.
type FlowVariablesSharingTestSuite struct {
	suite.Suite
	parentOU  string
	childOU   string
	siblingOU string
	strangeOU string
	flowIDs   []string
}

func TestFlowVariablesSharingTestSuite(t *testing.T) {
	suite.Run(t, new(FlowVariablesSharingTestSuite))
}

func (suite *FlowVariablesSharingTestSuite) createOU(handle string, parent *string) string {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: generateUniqueHandle(handle), Name: handle, Parent: parent,
	})
	suite.Require().NoError(err)
	return ouID
}

func (suite *FlowVariablesSharingTestSuite) SetupSuite() {
	suite.parentOU = suite.createOU("variables-parent", nil)
	suite.childOU = suite.createOU("variables-child", &suite.parentOU)
	suite.siblingOU = suite.createOU("variables-sibling", &suite.parentOU)
	suite.strangeOU = suite.createOU("variables-stranger", nil)
}

func (suite *FlowVariablesSharingTestSuite) TearDownSuite() {
	for _, id := range suite.flowIDs {
		req, _ := http.NewRequest(http.MethodDelete, testServerURL+flowsEndpoint+"/"+id, nil)
		if resp, err := testutils.GetHTTPClient().Do(req); err == nil {
			resp.Body.Close()
		}
	}
	for _, ouID := range []string{suite.childOU, suite.siblingOU, suite.parentOU, suite.strangeOU} {
		if ouID != "" {
			if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
				suite.T().Logf("teardown: failed to delete organization unit %s: %v", ouID, err)
			}
		}
	}
}

func (suite *FlowVariablesSharingTestSuite) send(method, path string, payload interface{}) (int, []byte) {
	var reader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		suite.Require().NoError(err)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, body
}

func (suite *FlowVariablesSharingTestSuite) expectError(method, path string, payload interface{}, status int, code string) {
	got, body := suite.send(method, path, payload)
	suite.Equal(status, got, string(body))
	var errResp ErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp))
	suite.Equal(code, errResp.Code, string(body))
}

func (suite *FlowVariablesSharingTestSuite) decode(body []byte) map[string]interface{} {
	var out map[string]interface{}
	suite.Require().NoError(json.Unmarshal(body, &out), string(body))
	return out
}

// flowBody is an authentication flow that picks an organization unit, then declares variables in a VARIABLES node and
// shows one of them.
func (suite *FlowVariablesSharingTestSuite) flowBody(ouID string, label string) map[string]interface{} {
	return map[string]interface{}{
		"handle":   generateUniqueHandle("variables-flow"),
		"name":     "Variables Flow",
		"flowType": "AUTHENTICATION",
		"ouId":     ouID,
		"nodes": []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "resolve_ou"},
			{
				"id": "resolve_ou", "type": "TASK_EXECUTION", "onSuccess": "vars",
				"executor":   map[string]interface{}{"name": "OUResolverExecutor"},
				"properties": map[string]interface{}{"resolveFrom": "prompt", "parentOuId": ouID},
			},
			{
				"id": "vars", "type": "VARIABLES", "onSuccess": "prompt_credentials",
				"inputs": []map[string]interface{}{
					{"identifier": "supportEmail", "type": "EMAIL_INPUT", "label": "Support email",
						"defaultValue": "support@example.com"},
					{"identifier": "otpValiditySeconds", "type": "NUMBER_INPUT", "defaultValue": 120,
						"min": 30, "max": 600},
					{"identifier": "mfaMode", "type": "SELECT", "options": []string{"NONE", "SMS_OTP", "EMAIL_OTP"},
						"defaultValue": "NONE"},
					{"identifier": "showRegistrationLink", "type": "BOOLEAN_INPUT", "defaultValue": true},
				},
			},
			{
				"id": "prompt_credentials", "type": "PROMPT",
				"properties": map[string]interface{}{"helpText": label},
				"meta": map[string]interface{}{"components": []map[string]interface{}{
					{"type": "TEXT", "id": "help", "label": "Sign in", "variant": "BODY_1"},
				}},
				"prompts": []map[string]interface{}{{
					"inputs": []map[string]interface{}{
						{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_signin", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id": "credentials_auth", "type": "TASK_EXECUTION", "onSuccess": "auth_assert",
				"executor": map[string]interface{}{"name": "CredentialsAuthExecutor"},
			},
			{
				"id": "auth_assert", "type": "TASK_EXECUTION", "onSuccess": "end",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"},
			},
			{"id": "end", "type": "END"},
		},
	}
}

func (suite *FlowVariablesSharingTestSuite) createFlow(ouID string) string {
	status, body := suite.send(http.MethodPost, flowsEndpoint,
		suite.flowBody(ouID, "Trouble signing in? Write to {{var(supportEmail)}}"))
	suite.Require().Equal(http.StatusCreated, status, string(body))
	id, _ := suite.decode(body)["id"].(string)
	suite.Require().NotEmpty(id)
	suite.flowIDs = append(suite.flowIDs, id)
	return id
}

func (suite *FlowVariablesSharingTestSuite) share(flowID string) string {
	status, body := suite.send(http.MethodPost, flowsEndpoint+"/"+flowID+"/sharing-policies", map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode":          "all",
			"excludedOuIds": []string{suite.siblingOU},
			"overlayRules": map[string]interface{}{
				"variables.supportEmail": map[string]interface{}{"editable": true},
				"variables.otpValiditySeconds": map[string]interface{}{
					"editable": true, "allowedValues": []string{"60", "120", "300"}},
				"variables.mfaMode": map[string]interface{}{"editable": false, "value": []string{"SMS_OTP"}},
			},
		},
	})
	suite.Require().Equal(http.StatusCreated, status, string(body))
	id, _ := suite.decode(body)["id"].(string)
	return id
}

func (suite *FlowVariablesSharingTestSuite) TestRemovingAVariableDropsItsRuleAndTheValuesUnitsSavedForIt() {
	flowID := suite.createFlow(suite.parentOU)
	policyID := suite.share(flowID)
	variablesPath := flowsEndpoint + "/" + flowID + "/variables"

	status, body := suite.send(http.MethodPut, variablesPath, map[string]interface{}{
		"ouId": suite.childOU, "values": map[string]interface{}{"otpValiditySeconds": 60}})
	suite.Require().Equal(http.StatusOK, status, string(body))

	status, body = suite.send(http.MethodGet, flowsEndpoint+"/"+flowID, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	flow := suite.decode(body)
	update := map[string]interface{}{
		"handle": flow["handle"], "name": flow["name"], "flowType": flow["flowType"], "ouId": suite.parentOU,
		"nodes": flow["nodes"],
	}
	// The policy gives otpValiditySeconds terms and a unit saved a value for it, so taking the variable out of the
	// flow takes both with it.
	for _, n := range update["nodes"].([]interface{}) {
		node := n.(map[string]interface{})
		if node["id"] != "vars" {
			continue
		}
		var kept []interface{}
		for _, in := range node["inputs"].([]interface{}) {
			if in.(map[string]interface{})["identifier"] != "otpValiditySeconds" {
				kept = append(kept, in)
			}
		}
		node["inputs"] = kept
	}

	status, body = suite.send(http.MethodPut, flowsEndpoint+"/"+flowID, update)
	suite.Require().Equal(http.StatusOK, status, string(body))

	status, body = suite.send(http.MethodGet, flowsEndpoint+"/"+flowID+"/sharing-policies/"+policyID, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	rules, _ := suite.decode(body)["childUnits"].(map[string]interface{})["overlayRules"].(map[string]interface{})
	suite.NotContains(rules, "variables.otpValiditySeconds", "the rule of the removed variable is dropped")
	suite.Contains(rules, "variables.supportEmail", "the rules of the variables it keeps are left alone")

	status, body = suite.send(http.MethodGet, variablesPath+"?ouId="+suite.childOU, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	values, _ := suite.decode(body)["values"].(map[string]interface{})
	suite.NotContains(values, "otpValiditySeconds", "the value a unit saved for the removed variable is dropped")
}

func (suite *FlowVariablesSharingTestSuite) TestAChangeThatInvalidatesAValueAUnitSavedIsRefusedUntilItIsChanged() {
	flowID := suite.createFlow(suite.parentOU)
	suite.share(flowID)
	variablesPath := flowsEndpoint + "/" + flowID + "/variables"

	status, body := suite.send(http.MethodPut, variablesPath, map[string]interface{}{
		"ouId": suite.childOU, "values": map[string]interface{}{"supportEmail": "help@child.example"}})
	suite.Require().Equal(http.StatusOK, status, string(body))

	status, body = suite.send(http.MethodGet, flowsEndpoint+"/"+flowID, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	flow := suite.decode(body)
	update := map[string]interface{}{
		"handle": flow["handle"], "name": flow["name"], "flowType": flow["flowType"], "ouId": suite.parentOU,
		"nodes": flow["nodes"],
	}
	// The unit saved an email, so making the variable a number invalidates it. No policy states a value for it.
	for _, n := range update["nodes"].([]interface{}) {
		node := n.(map[string]interface{})
		if node["id"] != "vars" {
			continue
		}
		for _, in := range node["inputs"].([]interface{}) {
			input := in.(map[string]interface{})
			if input["identifier"] == "supportEmail" {
				input["type"] = "NUMBER_INPUT"
				input["defaultValue"] = 5
				delete(input, "label")
			}
		}
	}

	suite.expectError(http.MethodPut, flowsEndpoint+"/"+flowID, update, http.StatusConflict, errorSavedValuesConflict)

	status, body = suite.send(http.MethodPut, variablesPath, map[string]interface{}{
		"ouId": suite.childOU, "values": map[string]interface{}{}})
	suite.Require().Equal(http.StatusOK, status, string(body))
	status, body = suite.send(http.MethodPut, flowsEndpoint+"/"+flowID, update)
	suite.Equal(http.StatusOK, status, "with the saved value gone, the change is allowed: "+string(body))
}

func (suite *FlowVariablesSharingTestSuite) TestPinningAVariableDropsTheValueAUnitSavedSoTheFlowCanChange() {
	flowID := suite.createFlow(suite.parentOU)
	policyID := suite.share(flowID)
	variablesPath := flowsEndpoint + "/" + flowID + "/variables"
	policyPath := flowsEndpoint + "/" + flowID + "/sharing-policies/" + policyID

	status, body := suite.send(http.MethodPut, variablesPath, map[string]interface{}{
		"ouId": suite.childOU, "values": map[string]interface{}{"otpValiditySeconds": 300}})
	suite.Require().Equal(http.StatusOK, status, string(body))

	// The owner pins the variable, so the unit can no longer change the value it saved.
	status, body = suite.send(http.MethodGet, policyPath, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	version := suite.decode(body)["version"]
	status, body = suite.send(http.MethodPut, policyPath, map[string]interface{}{
		"version": version,
		"childUnits": map[string]interface{}{
			"mode":          "all",
			"excludedOuIds": []string{suite.siblingOU},
			"overlayRules": map[string]interface{}{
				"variables.supportEmail":       map[string]interface{}{"editable": true},
				"variables.otpValiditySeconds": map[string]interface{}{"editable": false, "value": []string{"60"}},
				"variables.mfaMode":            map[string]interface{}{"editable": false, "value": []string{"SMS_OTP"}},
			},
		},
	})
	suite.Require().Equal(http.StatusOK, status, string(body))

	status, body = suite.send(http.MethodGet, variablesPath+ouQuery(suite.childOU), nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	values, _ := suite.decode(body)["values"].(map[string]interface{})
	suite.NotContains(values, "otpValiditySeconds", "the value saved before the pin is dropped with it")

	// Lowering the maximum below what the unit had saved no longer finds a value to refuse.
	status, body = suite.send(http.MethodGet, flowsEndpoint+"/"+flowID, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	flow := suite.decode(body)
	update := map[string]interface{}{
		"handle": flow["handle"], "name": flow["name"], "flowType": flow["flowType"], "ouId": suite.parentOU,
		"nodes": flow["nodes"],
	}
	for _, n := range update["nodes"].([]interface{}) {
		node := n.(map[string]interface{})
		if node["id"] != "vars" {
			continue
		}
		for _, in := range node["inputs"].([]interface{}) {
			input := in.(map[string]interface{})
			if input["identifier"] == "otpValiditySeconds" {
				input["max"] = 200
			}
		}
	}
	status, body = suite.send(http.MethodPut, flowsEndpoint+"/"+flowID, update)
	suite.Equal(http.StatusOK, status, string(body))
}

func (suite *FlowVariablesSharingTestSuite) TestAFlowIsValidatedForItsVariables() {
	cases := map[string]string{
		"a placeholder naming a variable the flow does not declare": "Write to {{var(nobody)}}",
	}
	for name, label := range cases {
		suite.Run(name, func() {
			suite.expectError(http.MethodPost, flowsEndpoint, suite.flowBody(suite.parentOU, label),
				http.StatusBadRequest, errorInvalidFlowData)
		})
	}

	suite.Run("a placeholder in a step no route reaches through the node", func() {
		body := suite.flowBody(suite.parentOU, "Write to {{var(supportEmail)}}")
		nodes := body["nodes"].([]map[string]interface{})
		nodes[1]["onSuccess"] = "prompt_credentials" // the resolver reaches the prompt before the VARIABLES node
		nodes[2]["onSuccess"] = "credentials_auth"
		prompts := nodes[3]["prompts"].([]map[string]interface{})
		prompts[0]["action"].(map[string]interface{})["nextNode"] = "vars"
		suite.expectError(http.MethodPost, flowsEndpoint, body, http.StatusBadRequest, errorInvalidFlowData)
	})

	suite.Run("a VARIABLES node with no organization unit resolver before it", func() {
		body := suite.flowBody(suite.parentOU, "Write to {{var(supportEmail)}}")
		nodes := body["nodes"].([]map[string]interface{})
		nodes[0]["onSuccess"] = "vars"               // START reaches the VARIABLES node first
		nodes[2]["onSuccess"] = "resolve_ou"         // and the resolver only comes after it
		nodes[1]["onSuccess"] = "prompt_credentials" // so a route reaches the node without one
		suite.expectError(http.MethodPost, flowsEndpoint, body, http.StatusBadRequest, errorInvalidFlowData)
	})

	suite.Run("a default that is not valid for its variable", func() {
		body := suite.flowBody(suite.parentOU, "Write to {{var(supportEmail)}}")
		inputs := body["nodes"].([]map[string]interface{})[2]["inputs"].([]map[string]interface{})
		inputs[1]["defaultValue"] = 1000 // above the maximum of 600
		suite.expectError(http.MethodPost, flowsEndpoint, body, http.StatusBadRequest, errorInvalidFlowData)
	})
}

func (suite *FlowVariablesSharingTestSuite) TestTheOrganizationUnitIsRequiredOnTheVariablesAndTheTerms() {
	flowID := suite.createFlow(suite.parentOU)

	suite.expectError(http.MethodGet, flowsEndpoint+"/"+flowID+"/variables", nil, http.StatusBadRequest, errorMissingOU)
	suite.expectError(http.MethodPut, flowsEndpoint+"/"+flowID+"/variables",
		map[string]interface{}{"values": map[string]interface{}{}}, http.StatusBadRequest, errorMissingOU)
	suite.expectError(http.MethodGet, flowsEndpoint+"/"+flowID+"/overlay-rules", nil, http.StatusBadRequest,
		errorMissingOU)
}

func (suite *FlowVariablesSharingTestSuite) TestTheFlowReadsNeedNoOrganizationUnit() {
	flowID := suite.createFlow(suite.parentOU)

	status, body := suite.send(http.MethodGet, flowsEndpoint+"/"+flowID, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	flow := suite.decode(body)
	suite.NotContains(flow, "isOwnedByOU", "the whole definition, as it is read without an organization unit")
	suite.Len(flow["nodes"], 7)

	status, body = suite.send(http.MethodGet, flowsEndpoint+"?limit=100", nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	var found bool
	for _, item := range suite.decode(body)["flows"].([]interface{}) {
		if item.(map[string]interface{})["id"] == flowID {
			found = true
		}
	}
	suite.True(found, "every flow is listed")
}

func (suite *FlowVariablesSharingTestSuite) TestTheOwnerSeesTheWholeDefinition() {
	flowID := suite.createFlow(suite.parentOU)

	status, body := suite.send(http.MethodGet, flowsEndpoint+"/"+flowID+ouQuery(suite.parentOU), nil)

	suite.Equal(http.StatusOK, status, string(body))
	flow := suite.decode(body)
	suite.Equal(true, flow["isOwnedByOU"])
	suite.Equal(false, flow["isReadOnly"])
	nodes, _ := flow["nodes"].([]interface{})
	suite.Len(nodes, 7)
}

// ouFilter is the query that lists the flows of one organization unit.
func ouFilter(ouID string) string {
	return "?filter=" + url.QueryEscape(`ouId eq "`+ouID+`"`)
}

// ouQuery is the query that reads one flow as an organization unit sees it.
func ouQuery(ouID string) string {
	return "?ouId=" + url.QueryEscape(ouID)
}

func (suite *FlowVariablesSharingTestSuite) TestTheFilterListsOwnedFlowsBeforeSharedOnes() {
	flowID := suite.createFlow(suite.parentOU)
	suite.share(flowID)
	ownFlowID := suite.createFlow(suite.childOU)

	status, body := suite.send(http.MethodGet, flowsEndpoint+ouFilter(suite.childOU)+"&limit=100", nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	var order []string
	for _, item := range suite.decode(body)["flows"].([]interface{}) {
		entry := item.(map[string]interface{})
		if id := entry["id"].(string); id == ownFlowID || id == flowID {
			order = append(order, id)
		}
	}
	suite.Equal([]string{ownFlowID, flowID}, order, "the flows the unit owns come first")

	status, body = suite.send(http.MethodGet, flowsEndpoint+ouFilter(suite.childOU)+"&limit=1", nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	links := suite.decode(body)["links"].([]interface{})
	suite.Require().NotEmpty(links)
	suite.Contains(links[0].(map[string]interface{})["href"], "filter=", "a link to another page keeps the filter")

	status, body = suite.send(http.MethodGet, flowsEndpoint+"?limit=100", nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	for _, item := range suite.decode(body)["flows"].([]interface{}) {
		suite.Equal(true, item.(map[string]interface{})["isOwnedByOU"], "an unfiltered list is of owned flows")
	}
}

func (suite *FlowVariablesSharingTestSuite) TestAFilterItCannotApplyIsRefused() {
	for _, expr := range []string{
		"ouId eq " + suite.childOU,
		`ouId eq ""`,
		`ouId sw "` + suite.childOU + `"`,
		`ouId eq "` + suite.childOU + `" and handle eq "x"`,
	} {
		suite.expectError(http.MethodGet, flowsEndpoint+"?filter="+url.QueryEscape(expr), nil,
			http.StatusBadRequest, "FLM-1040")
	}
}

func (suite *FlowVariablesSharingTestSuite) TestAnUnknownUnitHasNoFlowsToList() {
	status, body := suite.send(http.MethodGet, flowsEndpoint+ouFilter("00000000-0000-0000-0000-000000000000"), nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	suite.Empty(suite.decode(body)["flows"])
}

func (suite *FlowVariablesSharingTestSuite) TestAUnitSeesWhatWasSharedWithItAndNothingElse() {
	flowID := suite.createFlow(suite.parentOU)
	suite.share(flowID)

	// Listed as an owned flow is, read only to the unit.
	status, body := suite.send(http.MethodGet, flowsEndpoint+ouFilter(suite.childOU)+"&limit=100", nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	var found map[string]interface{}
	for _, item := range suite.decode(body)["flows"].([]interface{}) {
		if entry := item.(map[string]interface{}); entry["id"] == flowID {
			found = entry
		}
	}
	suite.Require().NotNil(found, "the shared flow is listed for the child")
	suite.Equal(false, found["isOwnedByOU"])
	suite.Equal(true, found["isReadOnly"])
	suite.Equal(suite.parentOU, found["ouId"], "the owner's, not the requested one")
	suite.Contains(found, "activeVersion")
	suite.Contains(found, "createdAt")

	// Read with the variables that apply to it, resolved for it, and no definition.
	status, body = suite.send(http.MethodGet, flowsEndpoint+"/"+flowID+ouQuery(suite.childOU), nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	flow := suite.decode(body)
	suite.Equal(false, flow["isOwnedByOU"])
	suite.NotContains(flow, "nodes")
	byID := map[string]map[string]interface{}{}
	for _, v := range flow["variables"].([]interface{}) {
		variable := v.(map[string]interface{})
		byID[variable["identifier"].(string)] = variable
	}
	suite.Len(byID, 3, "showRegistrationLink is named by no policy")
	suite.Equal("SMS_OTP", byID["mfaMode"]["defaultValue"], "the value the policy pins")
	suite.Equal("support@example.com", byID["supportEmail"]["defaultValue"])

	// A flow is not listed for, nor shown to, a unit it was not shared with.
	suite.expectError(http.MethodGet, flowsEndpoint+"/"+flowID+ouQuery(suite.siblingOU), nil,
		http.StatusNotFound, errorFlowNotFound)
	suite.expectError(http.MethodGet, flowsEndpoint+"/"+flowID+ouQuery(suite.strangeOU), nil,
		http.StatusNotFound, errorFlowNotFound)

	// It is read only to the unit it was shared with, and its history stays with the owner.
	status, body = suite.send(http.MethodGet, flowsEndpoint+"/"+flowID, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	update := suite.flowBody(suite.childOU, "Write to {{var(supportEmail)}}")
	update["handle"] = suite.decode(body)["handle"]
	suite.expectError(http.MethodPut, flowsEndpoint+"/"+flowID, update, http.StatusForbidden, errorSharedFlowReadOnly)
	suite.expectError(http.MethodGet, testutils.WithOU(flowsEndpoint+"/"+flowID+"/versions", suite.childOU), nil,
		http.StatusNotFound, errorFlowNotFound)
	status, _ = suite.send(http.MethodGet, testutils.WithOU(flowsEndpoint+"/"+flowID+"/versions", suite.parentOU), nil)
	suite.Equal(http.StatusOK, status)
}

func (suite *FlowVariablesSharingTestSuite) TestTheTermsAUnitHoldsTheFlowOn() {
	flowID := suite.createFlow(suite.parentOU)
	policyID := suite.share(flowID)

	status, body := suite.send(http.MethodGet,
		testutils.WithOU(flowsEndpoint+"/"+flowID+"/overlay-rules", suite.childOU), nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	overlay := suite.decode(body)
	suite.Equal(true, overlay["visible"])
	suite.Equal(false, overlay["isOwnedByOU"])
	suite.Equal([]interface{}{policyID}, overlay["policyIds"])
	rules := overlay["rules"].(map[string]interface{})
	suite.Len(rules, 3)
	suite.Equal(false, rules["variables.mfaMode"].(map[string]interface{})["editable"])
	suite.Equal(true, rules["variables.supportEmail"].(map[string]interface{})["editable"])

	suite.expectError(http.MethodGet, testutils.WithOU(flowsEndpoint+"/"+flowID+"/overlay-rules", suite.siblingOU), nil,
		http.StatusNotFound, errorFlowNotFound)

	status, body = suite.send(http.MethodGet,
		testutils.WithOU(flowsEndpoint+"/"+flowID+"/overlay-rules", suite.parentOU), nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	suite.Equal(true, suite.decode(body)["isOwnedByOU"])
}

func (suite *FlowVariablesSharingTestSuite) TestAUnitSetsItsOwnValuesWithinTheTerms() {
	flowID := suite.createFlow(suite.parentOU)
	suite.share(flowID)
	variablesPath := flowsEndpoint + "/" + flowID + "/variables"
	variables := testutils.WithOU(variablesPath, suite.childOU)

	status, body := suite.send(http.MethodGet, variables, nil)
	suite.Require().Equal(http.StatusOK, status, string(body))
	suite.Equal(map[string]interface{}{}, suite.decode(body)["values"], "it has set nothing yet")

	status, body = suite.send(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
		"supportEmail": "help@child.example", "otpValiditySeconds": 300,
	}})
	suite.Require().Equal(http.StatusOK, status, string(body))
	suite.Equal(map[string]interface{}{"supportEmail": "help@child.example", "otpValiditySeconds": float64(300)},
		suite.decode(body)["values"])

	// A request replaces what the unit held: the value left out goes back to the default.
	status, body = suite.send(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
		"otpValiditySeconds": 60}})
	suite.Require().Equal(http.StatusOK, status, string(body))
	suite.Equal(map[string]interface{}{"otpValiditySeconds": float64(60)}, suite.decode(body)["values"])

	suite.Run("what it may not set", func() {
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"mfaMode": "EMAIL_OTP"}}, http.StatusForbidden, errorNotEditable)
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"showRegistrationLink": false}}, http.StatusBadRequest, errorUnknownVariable)
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"nobody": "x"}}, http.StatusBadRequest, errorUnknownVariable)
	})
	suite.Run("a value that is not valid, or that the owner does not allow", func() {
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"supportEmail": "not-an-email"}}, http.StatusBadRequest, errorInvalidValue)
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"otpValiditySeconds": 90}}, http.StatusBadRequest, errorInvalidValue)
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"supportEmail": nil}}, http.StatusBadRequest, errorInvalidValue)
	})
	suite.Run("nothing is written unless every value passes", func() {
		suite.expectError(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
			"supportEmail": "other@child.example", "otpValiditySeconds": 90}}, http.StatusBadRequest, errorInvalidValue)
		status, body := suite.send(http.MethodGet, variables, nil)
		suite.Require().Equal(http.StatusOK, status)
		suite.Equal(map[string]interface{}{"otpValiditySeconds": float64(60)}, suite.decode(body)["values"])
	})
	suite.Run("a unit the flow was not shared with", func() {
		suite.expectError(http.MethodPut, flowsEndpoint+"/"+flowID+"/variables",
			map[string]interface{}{"ouId": suite.siblingOU, "values": map[string]interface{}{"supportEmail": "x@y.co"}},
			http.StatusNotFound, errorFlowNotFound)
	})
	suite.Run("the owner holds the defaults and sets nothing of its own", func() {
		suite.expectError(http.MethodPut, flowsEndpoint+"/"+flowID+"/variables",
			map[string]interface{}{"ouId": suite.parentOU, "values": map[string]interface{}{"supportEmail": "x@y.co"}},
			http.StatusBadRequest, errorUnknownVariable)
	})
}

func (suite *FlowVariablesSharingTestSuite) TestStoppingToShareTakesTheFlowAwayAndTheValuesWithIt() {
	flowID := suite.createFlow(suite.parentOU)
	policyID := suite.share(flowID)
	variablesPath := flowsEndpoint + "/" + flowID + "/variables"
	variables := testutils.WithOU(variablesPath, suite.childOU)
	status, body := suite.send(http.MethodPut, variablesPath, map[string]interface{}{"ouId": suite.childOU, "values": map[string]interface{}{
		"supportEmail": "help@child.example"}})
	suite.Require().Equal(http.StatusOK, status, string(body))

	// A second policy by the same issuer is refused.
	suite.expectError(http.MethodPost, flowsEndpoint+"/"+flowID+"/sharing-policies", map[string]interface{}{
		"childUnits": map[string]interface{}{"mode": "all"}}, http.StatusConflict, errorSharingPolicyExists)

	status, _ = suite.send(http.MethodDelete, flowsEndpoint+"/"+flowID+"/sharing-policies/"+policyID, nil)
	suite.Equal(http.StatusNoContent, status)
	status, _ = suite.send(http.MethodDelete, flowsEndpoint+"/"+flowID+"/sharing-policies/"+policyID, nil)
	suite.Equal(http.StatusNoContent, status, "deleting a policy that is gone succeeds")

	suite.expectError(http.MethodGet, variables, nil, http.StatusNotFound, errorFlowNotFound)
	suite.expectError(http.MethodGet, flowsEndpoint+"/"+flowID+ouQuery(suite.childOU), nil,
		http.StatusNotFound, errorFlowNotFound)
}

func (suite *FlowVariablesSharingTestSuite) TestSharingTermsAreCheckedAgainstTheFlow() {
	flowID := suite.createFlow(suite.parentOU)
	policies := flowsEndpoint + "/" + flowID + "/sharing-policies"
	policy := func(rules map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all", "overlayRules": rules}}
	}

	suite.Run("a rule for a variable the flow does not declare", func() {
		suite.expectError(http.MethodPost, policies, policy(map[string]interface{}{
			"variables.nope": map[string]interface{}{"editable": true}}), http.StatusBadRequest, "FLM-1034")
	})
	suite.Run("a value that is not valid for its variable", func() {
		suite.expectError(http.MethodPost, policies, policy(map[string]interface{}{
			"variables.otpValiditySeconds": map[string]interface{}{"editable": false, "value": []string{"9000"}}}),
			http.StatusBadRequest, "FLM-1034")
	})
	suite.Run("two values for one variable", func() {
		suite.expectError(http.MethodPost, policies, policy(map[string]interface{}{
			"variables.mfaMode": map[string]interface{}{"editable": false, "value": []string{"SMS_OTP", "EMAIL_OTP"}}}),
			http.StatusBadRequest, "FLM-1034")
	})
	suite.Run("a reach no unit's position bounds is left to a resource file", func() {
		suite.expectError(http.MethodPost, policies, map[string]interface{}{
			"allUnits": map[string]interface{}{}}, http.StatusBadRequest, "FLM-1043")
	})
}
