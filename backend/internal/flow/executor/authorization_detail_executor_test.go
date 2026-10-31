// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authzdetail "github.com/thunder-id/thunderid/internal/authzdetail/common"
	authzdetailexec "github.com/thunder-id/thunderid/internal/authzdetail/exec"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/authzdetailprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

// requestedDetail is a requested authorization detail whose type declares a processor.
func requestedDetail() providers.AuthorizationDetail {
	value := json.RawMessage(`{"type":"payment_initiation","amount":10}`)
	return providers.AuthorizationDetail{ID: "d-1", Type: "payment_initiation", TypeID: "type-1",
		TypeVersion: 1, Requested: value, Value: value}
}

var requestedDetails = mock.MatchedBy(func(details []providers.AuthorizationDetail) bool {
	return len(details) == 1 && details[0].ID == "d-1" && !details[0].Admitted
})

type AuthorizationDetailExecutorTestSuite struct {
	suite.Suite
	mockAuthzDetails  *authzdetailprovidermock.AuthorizationDetailProviderMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	executor          *authorizationDetailExecutor
}

func TestAuthorizationDetailExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationDetailExecutorTestSuite))
}

func (suite *AuthorizationDetailExecutorTestSuite) SetupTest() {
	suite.mockAuthzDetails = authzdetailprovidermock.NewAuthorizationDetailProviderMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameAuthorizationDetail, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
		}).Return(mockExec)

	suite.executor = newAuthorizationDetailExecutor(suite.mockFlowFactory, suite.mockAuthzDetails,
		suite.mockAuthnProvider)
}

func buildAuthorizationDetailNodeContext() *providers.NodeContext {
	ctx := buildConsentNodeContext()
	encoded, _ := json.Marshal([]providers.AuthorizationDetail{requestedDetail()})
	ctx.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails] = string(encoded)
	ctx.RuntimeData[common.RuntimeKeyResourceServerIdentifier] = "https://bank.example.com"
	return ctx
}

func (suite *AuthorizationDetailExecutorTestSuite) expectEntityReference() {
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), buildConsentEntityRef(), (*tidcommon.ServiceError)(nil)).Once()
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_NoDetailsRequested() {
	ctx := buildConsentNodeContext()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyRequestedAuthorizationDetails)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_MalformedDetails() {
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails] = "not json"
	suite.expectEntityReference()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_UserNotAuthenticated() {
	ctx := buildAuthorizationDetailNodeContext()
	ctx.AuthUser = providers.AuthUser{}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrUserNotAuthenticated, resp.Error)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_EntityReferenceFailure() {
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), (*providers.EntityReference)(nil), &tidcommon.InternalServerError).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrFailedToIdentifyEntity.Code, resp.Error.Code)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_Admitted() {
	suite.expectEntityReference()
	app := buildAuthorizationDetailNodeContext().Application
	client := providers.EntityReference{EntityID: app.ID, EntityCategory: string(app.EntityCategory), OUID: app.OUID}
	suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, client,
		app.OAuthClientID(), *buildConsentEntityRef(), "https://bank.example.com").
		RunAndReturn(func(_ context.Context, details []providers.AuthorizationDetail, _ providers.EntityReference,
			_ string, _ providers.EntityReference, _ string,
		) ([]providers.AuthorizationDetail, *tidcommon.ServiceError) {
			details[0].Admitted = true
			details[0].Value = json.RawMessage(`{"type":"payment_initiation","amount":10,"iban":"DE12"}`)
			return details, nil
		}).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	var admitted []providers.AuthorizationDetail
	suite.Require().NoError(json.Unmarshal(
		[]byte(resp.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails]), &admitted))
	suite.Require().Len(admitted, 1)
	suite.True(admitted[0].Admitted)
	suite.Contains(string(admitted[0].Value), "DE12")
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_Refused() {
	suite.expectEntityReference()
	suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, mock.Anything,
		mock.Anything, *buildConsentEntityRef(), mock.Anything).
		Return(nil, &authzdetailexec.ErrorAuthorizationDetailRefused).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrAuthorizationDetailsRefused, resp.Error)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyRequestedAuthorizationDetails)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_RefusedWithReason() {
	suite.expectEntityReference()
	refused := authzdetailexec.ErrorAuthorizationDetailRefused.WithParams(map[string]string{"reason": "over limit"})
	suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, mock.Anything,
		mock.Anything, *buildConsentEntityRef(), mock.Anything).
		Return(nil, refused).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrAuthorizationDetailsRefused.Code, resp.Error.Code)
	suite.Equal("The requested authorization details were refused: over limit", resp.Error.ErrorDescription.String())
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_RefusedWithAnUnsafeOrLongReason() {
	long := strings.Repeat("x", authzdetail.MaxReasonLength+50)
	cases := map[string]struct {
		reason   string
		expected string
	}{
		"bidirectional formatting": {"pay \u202eevil", "The requested authorization details were refused"},
		"control character":        {"over\x00limit", "The requested authorization details were refused"},
		"too long": {long, "The requested authorization details were refused: " +
			long[:authzdetail.MaxReasonLength]},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.expectEntityReference()
			refused := authzdetailexec.ErrorAuthorizationDetailRefused.WithParams(
				map[string]string{"reason": tc.reason})
			suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, mock.Anything,
				mock.Anything, *buildConsentEntityRef(), mock.Anything).
				Return(nil, refused).Once()

			resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

			suite.NoError(err)
			suite.Equal(ErrAuthorizationDetailsRefused.Code, resp.Error.Code)
			suite.Equal(tc.expected, resp.Error.ErrorDescription.String())
		})
	}
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_ProcessorUnavailable() {
	suite.expectEntityReference()
	unavailable := tidcommon.ServiceError{Type: tidcommon.ServerErrorType,
		Code: authzdetail.ErrorProcessorUnavailable.Code}
	suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, mock.Anything,
		mock.Anything, *buildConsentEntityRef(), mock.Anything).
		Return(nil, &unavailable).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrAuthorizationDetailsUnavailable, resp.Error)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_ServerError() {
	suite.expectEntityReference()
	suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, mock.Anything,
		mock.Anything, *buildConsentEntityRef(), mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.Error(err)
	suite.Nil(resp)
}

func (suite *AuthorizationDetailExecutorTestSuite) TestExecute_TypeChanged() {
	suite.expectEntityReference()
	suite.mockAuthzDetails.EXPECT().ProcessAuthorizationDetails(mock.Anything, requestedDetails, mock.Anything,
		mock.Anything, *buildConsentEntityRef(), mock.Anything).
		Return(nil, &authzdetail.ErrorAuthorizationDetailTypeChanged).Once()

	resp, err := suite.executor.Execute(buildAuthorizationDetailNodeContext())

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrAuthorizationDetailsUnavailable, resp.Error)
}
