// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var (
	userMgtProviderDisablePatch = map[string]interface{}{
		"user_mgt_provider": map[string]interface{}{"type": "disabled"},
	}

	userMgtProviderRestorePatch = map[string]interface{}{
		"user_mgt_provider": map[string]interface{}{"type": "default"},
	}

	disabledProviderOU = testutils.OrganizationUnit{
		Handle: "disabled-user-provider-ou",
		Name:   "Disabled User Provider OU",
	}

	disabledProviderUserType = testutils.UserType{
		Name: "disabled-provider-user-type",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string", "required": true},
		},
		AllowSelfRegistration: true,
	}
)

// DisabledUserProviderTestSuite runs against a server configured with
// user_mgt_provider.type=disabled. It covers the deployment switch that turns off provisioning users
// from the runtime, which the default configuration never exercises.
type DisabledUserProviderTestSuite struct {
	suite.Suite
	config             *common.TestSuiteConfig
	ouID               string
	entityTypeID       string
	flowID             string
	isolatedAuthFlowID string
	appID              string
}

func TestDisabledUserProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DisabledUserProviderTestSuite))
}

func (ts *DisabledUserProviderTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Fixtures are created before the switch, while provisioning is still enabled.
	ouID, err := testutils.CreateOrganizationUnit(disabledProviderOU)
	ts.Require().NoError(err, "failed to create the organization unit")
	ts.ouID = ouID

	disabledProviderUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(disabledProviderUserType)
	ts.Require().NoError(err, "failed to create the user type")
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Disabled User Provider Registration Flow",
		Handle:   "disabled-user-provider-flow",
		FlowType: "REGISTRATION",
		Nodes:    testRegFlow.Nodes,
	})
	ts.Require().NoError(err, "failed to create the registration flow")
	ts.flowID = flowID

	isolatedAuthID, err := testutils.CreateIsolatedAuthFlow("disabled-user-provider-isolated-auth")
	ts.Require().NoError(err, "failed to create the isolated auth flow")
	ts.isolatedAuthFlowID = isolatedAuthID

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ouID,
		Name:                      "Disabled User Provider App",
		Description:               "Application for disabled user provider testing",
		IsRegistrationFlowEnabled: true,
		RegistrationFlowID:        flowID,
		AuthFlowID:                isolatedAuthID,
		ClientID:                  "disabled_user_provider_client",
		ClientSecret:              "disabled_user_provider_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{disabledProviderUserType.Name},
	})
	ts.Require().NoError(err, "failed to create the application")
	ts.appID = appID

	ts.Require().NoError(testutils.PatchDeploymentConfig(userMgtProviderDisablePatch),
		"failed to disable the user management provider")
	ts.Require().NoError(testutils.RestartServer(),
		"failed to restart the server with the provider disabled")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(),
		"failed to re-obtain the admin token after restart")
}

func (ts *DisabledUserProviderTestSuite) TearDownSuite() {
	// Restore first: a server left with the provider disabled would fail every later suite.
	if err := testutils.PatchDeploymentConfig(userMgtProviderRestorePatch); err != nil {
		ts.T().Logf("teardown: failed to restore the user management provider config: %v", err)
	}
	if err := testutils.RestartServer(); err != nil {
		ts.T().Logf("teardown: server did not restart cleanly after config restore: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Logf("teardown: failed to re-obtain the admin token after restore: %v", err)
	}

	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("teardown: failed to clean up users: %v", err)
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("teardown: failed to delete the application: %v", err)
		}
	}
	for _, flowID := range []string{ts.flowID, ts.isolatedAuthFlowID} {
		if flowID == "" {
			continue
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete the user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete the organization unit: %v", err)
		}
	}
}

// A registration flow must fail with the provider's own error rather than a generic failure, so an
// operator who switched provisioning off can tell that from a malfunction.
func (ts *DisabledUserProviderTestSuite) TestRegistrationFlowIsRejectedWhenProviderIsDisabled() {
	username := common.GenerateUniqueUsername("disabledprov")

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the registration flow")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": username,
		"password": "disabledProviderPass123",
	}, "action_credentials", flowStep.ChallengeToken)
	ts.Require().NoError(err, "failed to submit credentials")

	// The flow may stop once for the remaining required schema attributes before provisioning runs.
	if flowStep.FlowStatus != "ERROR" && common.HasInput(flowStep.Data.Inputs, "email") {
		flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
			"email": username + "@example.com",
		}, "action_schema_attrs", flowStep.ChallengeToken)
		ts.Require().NoError(err, "failed to submit the schema attributes")
	}

	ts.Require().Equal("ERROR", flowStep.FlowStatus,
		"provisioning must fail while the user management provider is disabled")
	ts.Require().NotNil(flowStep.Error, "the failure must carry an error")
	ts.Equal("UMP-1002", flowStep.Error.Code,
		"the disabled-provider error must reach the flow unchanged")
}

// Disabling the provider must switch off runtime provisioning only. User management through the API
// is a separate path and has to keep working, otherwise the switch is an outage rather than a
// policy control.
func (ts *DisabledUserProviderTestSuite) TestUserManagementAPIStillWorksWhenProviderIsDisabled() {
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ts.ouID,
		Type:       disabledProviderUserType.Name,
		Attributes: []byte(`{"username":"disabled.api.user","email":"disabled.api.user@example.com"}`),
	})
	ts.Require().NoError(err, "the user management API must remain available")
	ts.Require().NotEmpty(userID)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userID)
}
