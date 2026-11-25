// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mgt

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// flowProvGroupNotFoundCode is the flow save error for a provisioning step that assigns a missing group.
	flowProvGroupNotFoundCode = "FLM-1049"
	// flowProvRoleNotFoundCode is the flow save error for a provisioning step that assigns a missing role.
	flowProvRoleNotFoundCode = "FLM-1050"
	// flowProvNeverExistedID is an ID no group or role has.
	flowProvNeverExistedID = "00000000-0000-0000-0000-00000000c0de"
)

// flowProvOU owns every group and role the provisioning validation suite creates.
var flowProvOU = testutils.OrganizationUnit{Handle: "flow-prov-validation-ou", Name: "Flow Provisioning Validation OU"}

// FlowProvisioningValidationTestSuite covers what saving a flow refuses when its provisioning step assigns a group
// or role that does not exist.
type FlowProvisioningValidationTestSuite struct {
	flowSaveSuite
	groupID string
	roleID  string
}

func TestFlowProvisioningValidationTestSuite(t *testing.T) {
	suite.Run(t, new(FlowProvisioningValidationTestSuite))
}

func (s *FlowProvisioningValidationTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(flowProvOU)
	s.Require().NoError(err, "Failed to create the organization unit")
	s.ouID = ouID

	s.groupID, err = testutils.CreateGroup(testutils.Group{Name: "flow-prov-validation-group", OUID: s.ouID})
	s.Require().NoError(err, "Failed to create the group")

	s.roleID, err = testutils.CreateRole(testutils.Role{Name: "flow-prov-validation-role", OUID: s.ouID})
	s.Require().NoError(err, "Failed to create the role")
}

func (s *FlowProvisioningValidationTestSuite) TearDownSuite() {
	if s.roleID != "" {
		if err := testutils.DeleteRole(s.roleID); err != nil {
			s.T().Logf("Failed to delete role during teardown: %v", err)
		}
	}
	if s.groupID != "" {
		if err := testutils.DeleteGroup(s.groupID); err != nil {
			s.T().Logf("Failed to delete group during teardown: %v", err)
		}
	}
	// The OU goes last: an OU that owns groups or roles cannot be deleted.
	if s.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(s.ouID); err != nil {
			s.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

// provisioningFlow returns a registration flow whose provisioning step has the given properties.
func (s *FlowProvisioningValidationTestSuite) provisioningFlow(
	handle string, properties map[string]interface{},
) testutils.Flow {
	return testutils.Flow{
		Name:     "Flow Provisioning Validation " + handle,
		FlowType: "REGISTRATION",
		Handle:   handle,
		OUID:     s.ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
			{
				"id":        "user_type_resolver",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess": "provisioning",
			},
			{
				"id":         "provisioning",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "ProvisioningExecutor"},
				"properties": properties,
				"onSuccess":  "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// provisioningFlowWithVariables is provisioningFlow with a VARIABLES node declaring the groupId and roleId variables
// before the provisioning step, so the step's properties can refer to them with {{var(...)}}. A VARIABLES node
// resolves its values for the organization unit an OUResolverExecutor step picks, so one comes before it.
func (s *FlowProvisioningValidationTestSuite) provisioningFlowWithVariables(
	handle string, properties map[string]interface{},
) testutils.Flow {
	flow := s.provisioningFlow(handle, properties)
	flow.Nodes = []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
		{
			"id":        "user_type_resolver",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "UserTypeResolver"},
			"onSuccess": "resolve_ou",
		},
		{
			"id":         "resolve_ou",
			"type":       "TASK_EXECUTION",
			"executor":   map[string]interface{}{"name": "OUResolverExecutor"},
			"properties": map[string]interface{}{"resolveFrom": "caller"},
			"onSuccess":  "variables",
		},
		{
			"id":        "variables",
			"type":      "VARIABLES",
			"onSuccess": "provisioning",
			"inputs": []map[string]interface{}{
				{"identifier": "groupId", "type": "TEXT_INPUT", "defaultValue": ""},
				{"identifier": "roleId", "type": "TEXT_INPUT", "defaultValue": ""},
			},
		},
		{
			"id":         "provisioning",
			"type":       "TASK_EXECUTION",
			"executor":   map[string]interface{}{"name": "ProvisioningExecutor"},
			"properties": properties,
			"onSuccess":  "end",
		},
		{"id": "end", "type": "END"},
	}
	return flow
}

// assertRefused checks that status and body are the given error, naming the step and the missing IDs under
// idsParam.
func (s *FlowProvisioningValidationTestSuite) assertRefused(status int, body []byte, code, idsParam, ids string) {
	s.T().Helper()
	s.Equal(http.StatusBadRequest, status, "The flow should be refused at save: %s", string(body))
	var e struct {
		Code        string `json:"code"`
		Description struct {
			Params map[string]string `json:"params"`
		} `json:"description"`
	}
	s.Require().NoError(json.Unmarshal(body, &e), "The error should decode: %s", string(body))
	s.Equal(code, e.Code, "Unexpected error: %s", string(body))
	s.Equal("provisioning", e.Description.Params["nodeID"], "The error should name the step")
	s.Equal(ids, e.Description.Params[idsParam], "The error should name the missing IDs")
}

func (s *FlowProvisioningValidationTestSuite) TestExistingGroupAndRoleAreAccepted() {
	status, body, _ := s.createFlow(s.provisioningFlow("flow_prov_existing", map[string]interface{}{
		"assignGroup": s.groupID,
		"assignRole":  s.roleID,
	}))

	s.Equal(http.StatusCreated, status, "The flow should be saved: %s", string(body))
}

func (s *FlowProvisioningValidationTestSuite) TestMissingGroupIsRefused() {
	goneID, err := testutils.CreateGroup(testutils.Group{Name: "flow-prov-validation-gone-group", OUID: s.ouID})
	s.Require().NoError(err, "Failed to create the group to delete")
	s.Require().NoError(testutils.DeleteGroup(goneID), "Failed to delete the group")

	for _, tc := range []struct{ name, missingID string }{
		{"Deleted", goneID},
		{"NeverExisted", flowProvNeverExistedID},
	} {
		s.Run(tc.name, func() {
			handle := "flow_prov_group_" + strings.ToLower(tc.name)
			status, body, _ := s.createFlow(s.provisioningFlow(handle, map[string]interface{}{
				"assignGroup": s.groupID + ", " + tc.missingID,
			}))

			s.assertRefused(status, body, flowProvGroupNotFoundCode, "groupIDs", tc.missingID)
		})
	}
}

func (s *FlowProvisioningValidationTestSuite) TestMissingRoleIsRefused() {
	goneID, err := testutils.CreateRole(testutils.Role{Name: "flow-prov-validation-gone-role", OUID: s.ouID})
	s.Require().NoError(err, "Failed to create the role to delete")
	s.Require().NoError(testutils.DeleteRole(goneID), "Failed to delete the role")

	for _, tc := range []struct{ name, missingID string }{
		{"Deleted", goneID},
		{"NeverExisted", flowProvNeverExistedID},
	} {
		s.Run(tc.name, func() {
			handle := "flow_prov_role_" + strings.ToLower(tc.name)
			status, body, _ := s.createFlow(s.provisioningFlow(handle, map[string]interface{}{
				"assignRole": tc.missingID + "," + s.roleID,
			}))

			s.assertRefused(status, body, flowProvRoleNotFoundCode, "roleIDs", tc.missingID)
		})
	}
}

// An ID with a placeholder is resolved when the step runs, so it is not checked, but a plain ID beside it is: it
// is accepted when it exists.
func (s *FlowProvisioningValidationTestSuite) TestPlaceholderBesideAnExistingIDIsAccepted() {
	status, body, _ := s.createFlow(s.provisioningFlowWithVariables("flow_prov_placeholder_existing",
		map[string]interface{}{
			"assignGroup": "{{var(groupId)}}, " + s.groupID,
			"assignRole":  "{{var(roleId)}}, " + s.roleID,
		}))

	s.Equal(http.StatusCreated, status, "The flow should be saved: %s", string(body))
}

// A plain ID beside a placeholder is refused when it names nothing, and the error names only that ID.
func (s *FlowProvisioningValidationTestSuite) TestPlaceholderBesideAMissingIDIsRefused() {
	for _, tc := range []struct{ name, property, placeholder, code, idsParam string }{
		{"Group", "assignGroup", "{{var(groupId)}}", flowProvGroupNotFoundCode, "groupIDs"},
		{"Role", "assignRole", "{{var(roleId)}}", flowProvRoleNotFoundCode, "roleIDs"},
	} {
		s.Run(tc.name, func() {
			handle := "flow_prov_placeholder_" + strings.ToLower(tc.name)
			status, body, _ := s.createFlow(s.provisioningFlowWithVariables(handle, map[string]interface{}{
				tc.property: tc.placeholder + ", " + flowProvNeverExistedID,
			}))

			s.assertRefused(status, body, tc.code, tc.idsParam, flowProvNeverExistedID)
		})
	}
}

// A flow saved while its group existed is refused on its next save once the group is deleted.
func (s *FlowProvisioningValidationTestSuite) TestUpdateIsRefusedOnceTheGroupIsDeleted() {
	groupID, err := testutils.CreateGroup(testutils.Group{Name: "flow-prov-validation-later-gone", OUID: s.ouID})
	s.Require().NoError(err, "Failed to create the group")
	flow := s.provisioningFlow("flow_prov_update", map[string]interface{}{"assignGroup": groupID})
	status, body, flowID := s.createFlow(flow)
	s.Require().Equal(http.StatusCreated, status, "The flow should be saved: %s", string(body))
	s.Require().NoError(testutils.DeleteGroup(groupID), "Failed to delete the group")

	status, body = s.request(http.MethodPut, flowsEndpoint+"/"+flowID, flow)

	s.assertRefused(status, body, flowProvGroupNotFoundCode, "groupIDs", groupID)
}
