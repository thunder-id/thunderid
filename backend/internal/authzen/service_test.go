/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package authzen

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/tests/mocks/authzmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	testSubjectType       = "user"
	testSubjectID         = "user1"
	testResourceServerID  = "rs1"
	testBookingResourceID = "booking1"
	testBookingReadAction = "booking:read"
)

type ServiceTestSuite struct {
	suite.Suite
	authzMock          *authzmock.AuthorizationProviderMock
	entityProviderMock *entityprovidermock.EntityProviderInterfaceMock
	resourceMock       *resourcemock.ResourceServiceInterfaceMock
	service            AuthZENServiceInterface
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (s *ServiceTestSuite) SetupTest() {
	s.authzMock = authzmock.NewAuthorizationProviderMock(s.T())
	s.entityProviderMock = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.resourceMock = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.service = newService(s.authzMock, s.entityProviderMock, s.resourceMock)
}

func (s *ServiceTestSuite) mockValidSubject() {
	s.entityProviderMock.On("GetEntity", testSubjectID).Return(&providers.Entity{
		ID:       testSubjectID,
		Category: providers.EntityCategory(testSubjectType),
	}, nil)
}

func (s *ServiceTestSuite) mockValidAction(permission string) {
	s.resourceMock.On("ValidatePermissions", mock.Anything, testResourceServerID, []string{permission}).
		Return([]string{}, nil)
}

func (s *ServiceTestSuite) mockResourceServerIdentifier(identifier string) {
	s.resourceMock.On("GetResourceServerByIdentifier", mock.Anything, identifier).
		Return(&providers.ResourceServer{ID: testResourceServerID, Identifier: identifier}, nil).Once()
}

func (s *ServiceTestSuite) TestEvaluateAccessAllowed() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
		Action:   Action{Name: testBookingReadAction},
	}

	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{
		{ID: "group1"},
		{ID: "group1"},
		{ID: "group2"},
	}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, providers.AccessEvaluationRequest{
		Subject:        providers.Subject{Type: "user", ID: "user1", GroupIDs: []string{"group1", "group2"}},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: testResourceServerID},
		Permission:     providers.Permission{Name: testBookingReadAction},
	}).Return(&providers.AccessEvaluationResponse{Decision: true}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.True(resp.Decision)
	s.Nil(resp.Context)
}

func (s *ServiceTestSuite) TestEvaluateAccessPassesPropertiesToAuthz() {
	subjectProperties := map[string]interface{}{"department": "Sales"}
	resourceProperties := map[string]interface{}{"owner": "user1"}
	actionProperties := map[string]interface{}{"method": "GET"}
	req := AccessEvaluationRequest{
		Subject: Subject{
			Type:       "user",
			ID:         "user1",
			Properties: subjectProperties,
		},
		Resource: Resource{
			Type:       "booking",
			ID:         testBookingResourceID,
			Properties: resourceProperties,
		},
		Action: Action{
			Name:       testBookingReadAction,
			Properties: actionProperties,
		},
	}

	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, providers.AccessEvaluationRequest{
		Subject: providers.Subject{
			Type:       "user",
			ID:         "user1",
			GroupIDs:   []string{},
			Properties: subjectProperties,
		},
		ResourceServer: providers.AccessEvaluationResourceServer{
			ID:         testResourceServerID,
			Properties: resourceProperties,
		},
		Permission: providers.Permission{
			Name:       testBookingReadAction,
			Properties: actionProperties,
		},
	}).Return(&providers.AccessEvaluationResponse{Decision: true}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.True(resp.Decision)
}

func (s *ServiceTestSuite) TestEvaluateAccessDenied() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
		Action:   Action{Name: "booking:delete"},
	}

	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction("booking:delete")
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, providers.AccessEvaluationRequest{
		Subject:        providers.Subject{Type: "user", ID: "user1", GroupIDs: []string{}},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: testResourceServerID},
		Permission:     providers.Permission{Name: "booking:delete"},
	}).Return(&providers.AccessEvaluationResponse{Decision: false}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.False(resp.Decision)
	s.assertDecisionContext(resp.Context)
}

func (s *ServiceTestSuite) TestEvaluateAccessProviderNotImplementedUsesEmptyGroups() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "app", ID: "app1"},
		Resource: Resource{Type: "report", ID: "report1"},
		Action:   Action{Name: "report:read"},
	}

	s.entityProviderMock.On("GetEntity", "app1").Return(
		(*providers.Entity)(nil),
		entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeNotImplemented, "not implemented", "not implemented"),
	)
	s.mockResourceServerIdentifier("report")
	s.mockValidAction("report:read")
	s.entityProviderMock.On("GetTransitiveEntityGroups", "app1").Return(
		[]providers.EntityGroup(nil),
		entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeNotImplemented, "not implemented", "not implemented"),
	)
	s.authzMock.On("EvaluateAccess", mock.Anything, providers.AccessEvaluationRequest{
		Subject:        providers.Subject{Type: "app", ID: "app1", GroupIDs: []string{}},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: testResourceServerID},
		Permission:     providers.Permission{Name: "report:read"},
	}).Return(&providers.AccessEvaluationResponse{Decision: true}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.True(resp.Decision)
}

func (s *ServiceTestSuite) TestEvaluateAccessSkipsSubjectValidationWhenTypeEmpty() {
	req := AccessEvaluationRequest{
		Subject:  Subject{ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
		Action:   Action{Name: testBookingReadAction},
	}

	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, providers.AccessEvaluationRequest{
		Subject:        providers.Subject{ID: "user1", GroupIDs: []string{}},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: testResourceServerID},
		Permission:     providers.Permission{Name: testBookingReadAction},
	}).Return(&providers.AccessEvaluationResponse{Decision: true}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.True(resp.Decision)
	s.entityProviderMock.AssertNotCalled(s.T(), "GetEntity", mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessGroupResolutionFailure() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
		Action:   Action{Name: testBookingReadAction},
	}

	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return(
		[]providers.EntityGroup(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "failed", "failed"),
	)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
	s.authzMock.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessAuthorizationFailure() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
		Action:   Action{Name: testBookingReadAction},
	}

	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.authzMock.On("EvaluateAccess", mock.Anything, providers.AccessEvaluationRequest{
		Subject:        providers.Subject{Type: "user", ID: "user1", GroupIDs: []string{}},
		ResourceServer: providers.AccessEvaluationResourceServer{ID: testResourceServerID},
		Permission:     providers.Permission{Name: testBookingReadAction},
	}).Return((*providers.AccessEvaluationResponse)(nil), &tidcommon.InternalServerError)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestEvaluateAccessValidationErrors() {
	tests := []struct {
		name string
		req  AccessEvaluationRequest
		code string
	}{
		{
			name: "missing subject",
			req: AccessEvaluationRequest{
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: "read"},
			},
			code: ErrorMissingSubject.Code,
		},
		{
			name: "missing resource",
			req: AccessEvaluationRequest{
				Subject: Subject{ID: "user1"},
				Action:  Action{Name: "read"},
			},
			code: ErrorMissingResource.Code,
		},
		{
			name: "missing resource type",
			req: AccessEvaluationRequest{
				Subject:  Subject{ID: "user1"},
				Resource: Resource{ID: testBookingResourceID},
				Action:   Action{Name: "read"},
			},
			code: ErrorMissingResource.Code,
		},
		{
			name: "missing action",
			req: AccessEvaluationRequest{
				Subject:  Subject{ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
			},
			code: ErrorMissingAction.Code,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, svcErr := s.service.EvaluateAccess(context.Background(), tc.req)

			s.Nil(resp)
			s.NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestEvaluateAccessInvalidSubjectType() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "agent", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testResourceServerID},
		Action:   Action{Name: testBookingReadAction},
	}

	s.entityProviderMock.On("GetEntity", "user1").Return(&providers.Entity{
		ID:       "user1",
		Category: providers.EntityCategoryUser,
	}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidSubject.Code, svcErr.Code)
	s.resourceMock.AssertNotCalled(s.T(), "ValidatePermissions", mock.Anything, mock.Anything, mock.Anything)
	s.authzMock.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessInvalidAction() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
		Action:   Action{Name: "booking:write"},
	}

	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.resourceMock.On("ValidatePermissions", mock.Anything, testResourceServerID, []string{"booking:write"}).
		Return([]string{"booking:write"}, nil)

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.False(resp.Decision)
	s.assertErrorContext(resp.Context)
	s.entityProviderMock.AssertNotCalled(s.T(), "GetTransitiveEntityGroups", mock.Anything)
	s.authzMock.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessUnknownResourceReturnsErrorContext() {
	req := AccessEvaluationRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "unknown", ID: testBookingResourceID},
		Action:   Action{Name: "unknown:read"},
	}

	s.mockValidSubject()
	s.resourceMock.On("GetResourceServerByIdentifier", mock.Anything, "unknown").
		Return((*providers.ResourceServer)(nil), &resource.ErrorResourceServerNotFound).Once()

	resp, svcErr := s.service.EvaluateAccess(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.False(resp.Decision)
	s.assertErrorContextWithMessage(resp.Context, "Resource not found")
	s.resourceMock.AssertNotCalled(s.T(), "ValidatePermissions", mock.Anything, mock.Anything, mock.Anything)
	s.entityProviderMock.AssertNotCalled(s.T(), "GetTransitiveEntityGroups", mock.Anything)
	s.authzMock.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchPreservesOrder() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "user", ID: testSubjectID},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
			{
				Subject:  Subject{Type: "user", ID: testSubjectID},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: "booking:delete"},
			},
		},
	}

	s.entityProviderMock.On("GetEntity", testSubjectID).Return(&providers.Entity{
		ID:       testSubjectID,
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.mockValidAction("booking:delete")
	s.entityProviderMock.On("GetTransitiveEntityGroups", testSubjectID).
		Return([]providers.EntityGroup{}, nil).Once()
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 2 &&
				req.Evaluations[0].Subject.ID == testSubjectID &&
				req.Evaluations[0].ResourceServer.ID == testResourceServerID &&
				req.Evaluations[0].Permission.Name == testBookingReadAction &&
				req.Evaluations[1].Permission.Name == "booking:delete"
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
			},
		}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 2)
	s.True(resp.Evaluations[0].Decision)
	s.False(resp.Evaluations[1].Decision)
	s.Nil(resp.Evaluations[0].Context)
	s.assertDecisionContext(resp.Evaluations[1].Context)
	s.entityProviderMock.AssertNumberOfCalls(s.T(), "GetTransitiveEntityGroups", 1)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchInvalidActionReturnsFalse() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: "booking:archive"},
			},
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: "booking:delete"},
			},
		},
	}

	s.entityProviderMock.On("GetEntity", "user1").Return(&providers.Entity{
		ID:       "user1",
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.resourceMock.On("ValidatePermissions", mock.Anything, testResourceServerID, []string{"booking:archive"}).
		Return([]string{"booking:archive"}, nil)
	s.mockValidAction("booking:delete")
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil).Once()
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 2 &&
				req.Evaluations[0].Permission.Name == testBookingReadAction &&
				req.Evaluations[1].Permission.Name == "booking:delete"
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{
				{Decision: true},
				{Decision: true},
			},
		}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 3)
	s.True(resp.Evaluations[0].Decision)
	s.False(resp.Evaluations[1].Decision)
	s.True(resp.Evaluations[2].Decision)
	s.Nil(resp.Evaluations[0].Context)
	s.assertErrorContext(resp.Evaluations[1].Context)
	s.Nil(resp.Evaluations[2].Context)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchInvalidSubjectReturnsItemError() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "agent", ID: "agent1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
		},
	}

	s.entityProviderMock.On("GetEntity", "agent1").Return(&providers.Entity{
		ID:       "agent1",
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.mockValidSubject()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil).Once()
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 1 &&
				req.Evaluations[0].Subject.ID == "user1" &&
				req.Evaluations[0].Permission.Name == testBookingReadAction
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{{Decision: true}},
		}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 2)
	s.False(resp.Evaluations[0].Decision)
	s.assertErrorContextWithMessage(resp.Evaluations[0].Context, ErrorInvalidSubject.Error.DefaultValue)
	s.True(resp.Evaluations[1].Decision)
	s.Nil(resp.Evaluations[1].Context)
	s.entityProviderMock.AssertNotCalled(s.T(), "GetTransitiveEntityGroups", "agent1")
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchGroupResolutionFailureReturnsItemError() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
			{
				Subject:  Subject{Type: "user", ID: "user2"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
		},
	}

	s.entityProviderMock.On("GetEntity", "user1").Return(&providers.Entity{
		ID:       "user1",
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.entityProviderMock.On("GetEntity", "user2").Return(&providers.Entity{
		ID:       "user2",
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return(
		[]providers.EntityGroup(nil),
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "failed", "failed"),
	).Once()
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user2").Return([]providers.EntityGroup{
		{ID: "group1"},
	}, nil).Once()
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 1 &&
				req.Evaluations[0].Subject.ID == "user2" &&
				len(req.Evaluations[0].Subject.GroupIDs) == 1 &&
				req.Evaluations[0].Subject.GroupIDs[0] == "group1" &&
				req.Evaluations[0].Permission.Name == testBookingReadAction
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{{Decision: true}},
		}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 2)
	s.False(resp.Evaluations[0].Decision)
	s.assertErrorContextWithMessage(resp.Evaluations[0].Context, tidcommon.InternalServerError.Error.DefaultValue)
	s.True(resp.Evaluations[1].Decision)
	s.Nil(resp.Evaluations[1].Context)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchMissingActionReturnsItemError() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: testBookingReadAction},
			},
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
			},
		},
	}

	s.entityProviderMock.On("GetEntity", "user1").Return(&providers.Entity{
		ID:       "user1",
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.mockResourceServerIdentifier("booking")
	s.mockValidAction(testBookingReadAction)
	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil).Once()
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 1 &&
				req.Evaluations[0].Permission.Name == testBookingReadAction
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{
				{Decision: true},
			},
		}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 2)
	s.True(resp.Evaluations[0].Decision)
	s.False(resp.Evaluations[1].Decision)
	s.Nil(resp.Evaluations[0].Context)
	s.assertErrorContext(resp.Evaluations[1].Context)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchAllInvalidActionsReturnsFalseWithoutAuthorizationCall() {
	req := AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: "booking:archive"},
			},
			{
				Subject:  Subject{Type: "user", ID: "user1"},
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
				Action:   Action{Name: "booking:export"},
			},
		},
	}

	s.entityProviderMock.On("GetEntity", "user1").Return(&providers.Entity{
		ID:       "user1",
		Category: providers.EntityCategoryUser,
	}, nil).Once()
	s.mockResourceServerIdentifier("booking")
	s.resourceMock.On("ValidatePermissions", mock.Anything, testResourceServerID, []string{"booking:archive"}).
		Return([]string{"booking:archive"}, nil)
	s.resourceMock.On("ValidatePermissions", mock.Anything, testResourceServerID, []string{"booking:export"}).
		Return([]string{"booking:export"}, nil)

	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Evaluations, 2)
	s.False(resp.Evaluations[0].Decision)
	s.False(resp.Evaluations[1].Decision)
	s.assertErrorContext(resp.Evaluations[0].Context)
	s.assertErrorContext(resp.Evaluations[1].Context)
	s.entityProviderMock.AssertNotCalled(s.T(), "GetTransitiveEntityGroups", mock.Anything)
	s.authzMock.AssertNotCalled(s.T(), "EvaluateAccessBatch", mock.Anything, mock.Anything)
}

func (s *ServiceTestSuite) TestEvaluateAccessBatchMissingEvaluations() {
	resp, svcErr := s.service.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{})

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(ErrorMissingEvaluations.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSearchActionsReturnsAuthorizedActions() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{
		{ID: "group1"},
	}, nil)
	s.mockResourceServerIdentifier("booking")
	bookingResourceID := testBookingResourceID
	invoiceResourceID := "invoice1"
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, (*string)(nil),
		providers.ActionKind(""), mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []providers.Action{
				{Handle: "read", Permission: "booking:booking:read"},
				{Handle: "read-duplicate", Permission: "booking:booking:read"},
			},
		}, nil)
	s.resourceMock.On("GetResourceList", mock.Anything, testResourceServerID, (*string)(nil), mock.Anything, 0).
		Return(&resource.ResourceList{
			Resources: []providers.Resource{
				{ID: bookingResourceID},
				{ID: invoiceResourceID},
			},
		}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, &bookingResourceID,
		providers.ActionKind(""), mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []providers.Action{
				{Handle: "delete", Permission: "booking:booking:delete"},
			},
		}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, &invoiceResourceID,
		providers.ActionKind(""), mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []providers.Action{
				{Handle: "approve", Permission: "invoice:invoice:approve"},
			},
		}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 3 &&
				req.Evaluations[0].Subject.ID == "user1" &&
				req.Evaluations[0].Subject.GroupIDs[0] == "group1" &&
				req.Evaluations[0].ResourceServer.ID == testResourceServerID &&
				req.Evaluations[0].Permission.Name == "booking:booking:read" &&
				req.Evaluations[1].Permission.Name == "booking:booking:delete" &&
				req.Evaluations[2].Permission.Name == "invoice:invoice:approve"
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{
				{Decision: true},
				{Decision: false},
				{Decision: true},
			},
		}, nil)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Results, 2)
	s.Equal("booking:booking:read", resp.Results[0].Name)
	s.Equal("invoice:invoice:approve", resp.Results[1].Name)
}

func (s *ServiceTestSuite) TestSearchActionsPaginatesResourceServerActions() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.mockResourceServerIdentifier("booking")
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, (*string)(nil),
		providers.ActionKind(""), serverconst.MaxPageSize, 0).
		Return(&resource.ActionList{
			TotalResults: serverconst.MaxPageSize + 1,
			Count:        serverconst.MaxPageSize,
			Actions: []providers.Action{
				{Handle: "read", Permission: "booking:booking:read"},
			},
		}, nil)
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, (*string)(nil),
		providers.ActionKind(""), serverconst.MaxPageSize, serverconst.MaxPageSize).
		Return(&resource.ActionList{
			TotalResults: serverconst.MaxPageSize + 1,
			Count:        1,
			Actions: []providers.Action{
				{Handle: "write", Permission: "booking:booking:write"},
			},
		}, nil)
	s.resourceMock.On("GetResourceList", mock.Anything, testResourceServerID,
		(*string)(nil), serverconst.MaxPageSize, 0).
		Return(&resource.ResourceList{}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything,
		mock.MatchedBy(func(req providers.AccessEvaluationsRequest) bool {
			return len(req.Evaluations) == 2 &&
				req.Evaluations[0].Permission.Name == "booking:booking:read" &&
				req.Evaluations[1].Permission.Name == "booking:booking:write"
		})).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{
				{Decision: true},
				{Decision: true},
			},
		}, nil)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Len(resp.Results, 2)
	s.Equal("booking:booking:read", resp.Results[0].Name)
	s.Equal("booking:booking:write", resp.Results[1].Name)
}

func (s *ServiceTestSuite) TestSearchActionsReturnsEmptyResultsWhenDenied() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.mockResourceServerIdentifier("booking")
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, (*string)(nil),
		providers.ActionKind(""), mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []providers.Action{
				{Handle: "read", Permission: "booking:booking:read"},
			},
		}, nil)
	s.resourceMock.On("GetResourceList", mock.Anything, testResourceServerID, (*string)(nil), mock.Anything, 0).
		Return(&resource.ResourceList{}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return(&providers.AccessEvaluationsResponse{
			Evaluations: []providers.AccessEvaluationResponse{{Decision: false}},
		}, nil)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(svcErr)
	s.NotNil(resp)
	s.Empty(resp.Results)
}

func (s *ServiceTestSuite) TestSearchActionsValidationErrors() {
	tests := []struct {
		name string
		req  AccessActionSearchRequest
		code string
	}{
		{
			name: "missing subject",
			req: AccessActionSearchRequest{
				Resource: Resource{Type: "booking", ID: testBookingResourceID},
			},
			code: ErrorMissingSubject.Code,
		},
		{
			name: "missing resource",
			req: AccessActionSearchRequest{
				Subject: Subject{ID: "user1"},
			},
			code: ErrorMissingResource.Code,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			resp, svcErr := s.service.SearchActions(context.Background(), tc.req)

			s.Nil(resp)
			s.NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
}

func (s *ServiceTestSuite) TestSearchActionsUnknownResourceReturnsInvalidResource() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "unknown", ID: testBookingResourceID},
	}

	s.resourceMock.On("GetResourceServerByIdentifier", mock.Anything, "unknown").
		Return((*providers.ResourceServer)(nil), &resource.ErrorResourceServerNotFound).Once()

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidResource.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSearchActionsResourceServiceError() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.mockResourceServerIdentifier("booking")
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, (*string)(nil),
		providers.ActionKind(""), mock.Anything, 0).
		Return((*resource.ActionList)(nil), &tidcommon.InternalServerError)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) TestSearchActionsAuthorizationServiceError() {
	req := AccessActionSearchRequest{
		Subject:  Subject{Type: "user", ID: "user1"},
		Resource: Resource{Type: "booking", ID: testBookingResourceID},
	}

	s.entityProviderMock.On("GetTransitiveEntityGroups", "user1").Return([]providers.EntityGroup{}, nil)
	s.mockResourceServerIdentifier("booking")
	s.resourceMock.On("GetActionList", mock.Anything, testResourceServerID, (*string)(nil),
		providers.ActionKind(""), mock.Anything, 0).
		Return(&resource.ActionList{
			Actions: []providers.Action{
				{Handle: "read", Permission: "booking:booking:read"},
			},
		}, nil)
	s.resourceMock.On("GetResourceList", mock.Anything, testResourceServerID, (*string)(nil), mock.Anything, 0).
		Return(&resource.ResourceList{}, nil)
	s.authzMock.On("EvaluateAccessBatch", mock.Anything, mock.Anything).
		Return((*providers.AccessEvaluationsResponse)(nil), &tidcommon.InternalServerError)

	resp, svcErr := s.service.SearchActions(context.Background(), req)

	s.Nil(resp)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ServiceTestSuite) assertDecisionContext(context map[string]interface{}) {
	s.NotNil(context)
	s.Equal("Subject is not authorized to perform the requested action", context["reason"])
}

func (s *ServiceTestSuite) assertErrorContext(context map[string]interface{}) {
	s.NotNil(context)

	errorContext, ok := context["error"].(map[string]interface{})
	s.True(ok)
	s.NotEmpty(errorContext["message"])
	s.NotContains(errorContext, "status")
}

func (s *ServiceTestSuite) assertErrorContextWithMessage(context map[string]interface{}, message string) {
	s.NotNil(context)

	errorContext, ok := context["error"].(map[string]interface{})
	s.True(ok)
	s.Equal(message, errorContext["message"])
	s.NotContains(errorContext, "status")
}
