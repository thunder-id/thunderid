// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usermgtprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

const (
	testUserID   = "user-id-123"
	testUserOUID = "ou-id-abc"
	testUserType = "customer"
)

type DefaultUserMgtProviderTestSuite struct {
	suite.Suite
	mockService *usermock.UserServiceInterfaceMock
	provider    providers.UserMgtProvider
}

func (suite *DefaultUserMgtProviderTestSuite) SetupTest() {
	suite.mockService = usermock.NewUserServiceInterfaceMock(suite.T())
	suite.provider = newDefaultUserMgtProvider(suite.mockService)
}

func TestDefaultUserMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultUserMgtProviderTestSuite))
}

func newTestUser() *providers.User {
	return &providers.User{
		OUID:       testUserOUID,
		Type:       testUserType,
		Attributes: []byte(`{"email":"a@b.com"}`),
	}
}

// A valid request reaches the user service unmodified and its response is returned to the caller.
func (suite *DefaultUserMgtProviderTestSuite) TestCreateUserDelegatesAndReturnsUser() {
	requested := newTestUser()
	created := &providers.User{
		ID:   testUserID,
		OUID: testUserOUID,
		Type: testUserType,
	}

	suite.mockService.On("CreateUser", mock.Anything, requested).Return(created, nil).Once()

	resp, svcErr := suite.provider.CreateUser(context.Background(), requested)

	suite.Nil(svcErr)
	suite.Equal(testUserID, resp.ID)
	suite.Equal(testUserOUID, resp.OUID)
	suite.Equal(testUserType, resp.Type)
}

func (suite *DefaultUserMgtProviderTestSuite) TestCreateUserWithNilUserIsRejectedBeforeDelegation() {
	resp, svcErr := suite.provider.CreateUser(context.Background(), nil)

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
	suite.mockService.AssertNotCalled(suite.T(), "CreateUser", mock.Anything, mock.Anything)
}

// Failures raised by the user service must reach the caller with their original code. A runtime
// caller distinguishes an attribute clash it can retry with different input from a configuration
// problem it cannot, so collapsing these into one generic error would remove its only signal.
func (suite *DefaultUserMgtProviderTestSuite) TestCreateUserPreservesServiceErrors() {
	tests := []struct {
		name     string
		scenario string
		svcErr   *tidcommon.ServiceError
	}{
		{
			name:     "AttributeConflict",
			scenario: "another user already holds the unique attribute value",
			svcErr:   &user.ErrorAttributeConflict,
		},
		{
			name:     "OrganizationUnitNotFound",
			scenario: "the target organization unit does not exist",
			svcErr:   &user.ErrorOrganizationUnitNotFound,
		},
		{
			name:     "UserTypeNotFound",
			scenario: "the requested user type is not registered",
			svcErr:   &user.ErrorEntityTypeNotFound,
		},
		{
			name:     "SchemaValidationFailed",
			scenario: "the attributes do not satisfy the user type schema",
			svcErr:   &user.ErrorSchemaValidationFailed,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			mockService := usermock.NewUserServiceInterfaceMock(suite.T())
			provider := newDefaultUserMgtProvider(mockService)
			mockService.On("CreateUser", mock.Anything, mock.Anything).
				Return(nil, tt.svcErr).Once()

			resp, svcErr := provider.CreateUser(context.Background(), newTestUser())

			suite.Nil(resp)
			suite.NotNil(svcErr, tt.scenario)
			suite.Equal(tt.svcErr.Code, svcErr.Code, tt.scenario)
			suite.Equal(tidcommon.ClientErrorType, svcErr.Type, tt.scenario)
		})
	}
}

// A system failure must stay a server error. Reporting an outage as a client error would tell the
// caller to change its input when retrying is the correct response.
func (suite *DefaultUserMgtProviderTestSuite) TestCreateUserPreservesServerErrors() {
	suite.mockService.On("CreateUser", mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Once()

	resp, svcErr := suite.provider.CreateUser(context.Background(), newTestUser())

	suite.Nil(resp)
	suite.NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	suite.Equal(tidcommon.ServerErrorType, svcErr.Type)
}

// The user service must see a runtime-marked context that still carries the caller's values, so
// authorization is elevated without losing request-scoped data such as the trace ID.
func (suite *DefaultUserMgtProviderTestSuite) TestCreateUserElevatesCallerContext() {
	type ctxKey string
	const traceKey ctxKey = "trace-id"

	callerCtx := context.WithValue(context.Background(), traceKey, "trace-789")

	var observed context.Context
	suite.mockService.On("CreateUser", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			observed = args.Get(0).(context.Context)
		}).
		Return(&providers.User{ID: testUserID}, nil).Once()

	_, svcErr := suite.provider.CreateUser(callerCtx, newTestUser())

	suite.Nil(svcErr)
	suite.True(security.IsRuntimeContext(observed))
	suite.Equal("trace-789", observed.Value(traceKey))
}

// A service that returns neither a user nor an error must not be turned into a fabricated success.
func (suite *DefaultUserMgtProviderTestSuite) TestCreateUserHandlesEmptyServiceResponse() {
	suite.mockService.On("CreateUser", mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	resp, svcErr := suite.provider.CreateUser(context.Background(), newTestUser())

	suite.Nil(svcErr)
	suite.Nil(resp)
}
