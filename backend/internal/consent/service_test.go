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

package consent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ConsentServiceTestSuite struct {
	suite.Suite
	mockStore          *consentStoreInterfaceMock
	mockInboundClients *InboundClientProviderMock
	service            *consentService
}

func TestConsentServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentServiceTestSuite))
}

func (s *ConsentServiceTestSuite) SetupTest() {
	s.mockStore = newConsentStoreInterfaceMock(s.T())
	s.mockInboundClients = NewInboundClientProviderMock(s.T())
	s.service = &consentService{
		consentStore:          s.mockStore,
		transactioner:         transaction.NewNoOpTransactioner(),
		inboundClientProvider: s.mockInboundClients,
		logger:                log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentService")),
	}
}

// ListPurposes tests

func (s *ConsentServiceTestSuite) TestListPurposes_GroupScoped() {
	s.mockInboundClients.On("GetInboundClientAttributes", mock.Anything, "app1").
		Return(&inboundmodel.InboundClientAttributes{
			InboundClientID: "app1", Attributes: []string{"phone", "email"},
		}, nil)

	purposes, svcErr := s.service.ListPurposes(context.Background(), PurposeFilter{GroupID: "app1"})

	s.Nil(svcErr)
	s.Len(purposes, 1)
	s.Equal(AttributePurposeName("app1"), purposes[0].Name)
	s.Equal("app1", purposes[0].GroupID)
	// Elements are sorted by name.
	s.Len(purposes[0].Elements, 2)
	s.Equal("email", purposes[0].Elements[0].Name)
	s.Equal("phone", purposes[0].Elements[1].Name)
	s.Equal(NamespaceAttribute, purposes[0].Elements[0].Namespace)
}

func (s *ConsentServiceTestSuite) TestListPurposes_GroupScoped_NoAttributes() {
	s.mockInboundClients.On("GetInboundClientAttributes", mock.Anything, "app1").
		Return(&inboundmodel.InboundClientAttributes{InboundClientID: "app1", Attributes: nil}, nil)

	purposes, svcErr := s.service.ListPurposes(context.Background(), PurposeFilter{GroupID: "app1"})

	s.Nil(svcErr)
	s.Empty(purposes)
}

func (s *ConsentServiceTestSuite) TestListPurposes_GroupScoped_ProviderError() {
	s.mockInboundClients.On("GetInboundClientAttributes", mock.Anything, "app1").
		Return(nil, errors.New("boom"))

	purposes, svcErr := s.service.ListPurposes(context.Background(), PurposeFilter{GroupID: "app1"})

	s.Nil(purposes)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestListPurposes_ListAll() {
	s.mockInboundClients.On("ListInboundClientAttributes", mock.Anything).Return([]inboundmodel.InboundClientAttributes{
		{InboundClientID: "app1", Attributes: []string{"email"}},
		{InboundClientID: "app2", Attributes: nil}, // no attributes -> no purpose
		{InboundClientID: "app3", Attributes: []string{"name"}},
	}, nil)

	purposes, svcErr := s.service.ListPurposes(context.Background(), PurposeFilter{})

	s.Nil(svcErr)
	s.Len(purposes, 2)
	names := []string{purposes[0].Name, purposes[1].Name}
	s.Contains(names, AttributePurposeName("app1"))
	s.Contains(names, AttributePurposeName("app3"))
}

func (s *ConsentServiceTestSuite) TestPurposeNames() {
	s.Equal("attributes:app1", AttributePurposeName("app1"))
	s.Equal("attributes:", AttributePurposeName(""))
	s.Equal("permissions:app1", PermissionPurposeName("app1"))
	s.Equal("permissions:", PermissionPurposeName(""))
}

func (s *ConsentServiceTestSuite) TestListPurposes_ListAll_ProviderError() {
	s.mockInboundClients.On("ListInboundClientAttributes", mock.Anything).Return(nil, errors.New("boom"))

	purposes, svcErr := s.service.ListPurposes(context.Background(), PurposeFilter{})

	s.Nil(purposes)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// CreateConsent tests

func (s *ConsentServiceTestSuite) validCreateRequest() *ConsentRequest {
	return &ConsentRequest{
		GroupID:      "app1",
		ValidityTime: 0,
		Purposes: []ConsentPurposeItem{
			{Name: "attributes:app1", Elements: []ConsentElementApproval{
				{Name: "email", Namespace: NamespaceAttribute, IsUserApproved: true},
			}},
		},
		Authorizations: []ConsentAuthorizationRequest{
			{UserID: "user1", Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved},
		},
	}
}

func (s *ConsentServiceTestSuite) TestCreateConsent_Success() {
	s.mockStore.On("CreateConsent", mock.Anything, mock.AnythingOfType("*consent.Consent")).Return(nil)

	created, svcErr := s.service.CreateConsent(context.Background(), s.validCreateRequest())

	s.Nil(svcErr)
	s.NotNil(created)
	s.NotEmpty(created.ID)
	s.Equal("app1", created.GroupID)
	s.Equal(ConsentStatusActive, created.Status)
	s.Len(created.Authorizations, 1)
	s.NotEmpty(created.Authorizations[0].ID)
	s.Equal("user1", created.Authorizations[0].UserID)
	s.NotZero(created.Authorizations[0].UpdatedTime)
}

func (s *ConsentServiceTestSuite) TestCreateConsent_StoreError() {
	s.mockStore.On("CreateConsent", mock.Anything, mock.AnythingOfType("*consent.Consent")).
		Return(errors.New("db down"))

	created, svcErr := s.service.CreateConsent(context.Background(), s.validCreateRequest())

	s.Nil(created)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestCreateConsent_ValidationErrors() {
	cases := []struct {
		name    string
		request *ConsentRequest
		code    string
	}{
		{"nil request", nil, ErrorInvalidRequestFormat.Code},
		{"empty group", &ConsentRequest{GroupID: ""}, ErrorInvalidRequestFormat.Code},
		{
			"empty purpose name",
			&ConsentRequest{GroupID: "app1", Purposes: []ConsentPurposeItem{{Name: ""}}},
			ErrorInvalidRequestFormat.Code,
		},
		{
			"invalid element namespace",
			&ConsentRequest{GroupID: "app1", Purposes: []ConsentPurposeItem{
				{Name: "p", Elements: []ConsentElementApproval{{Name: "email", Namespace: "bogus"}}},
			}},
			ErrorInvalidNamespace.Code,
		},
		{
			"empty authorization user",
			&ConsentRequest{GroupID: "app1", Authorizations: []ConsentAuthorizationRequest{
				{UserID: "", Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved},
			}},
			ErrorInvalidRequestFormat.Code,
		},
		{
			"invalid authorization type",
			&ConsentRequest{GroupID: "app1", Authorizations: []ConsentAuthorizationRequest{
				{UserID: "user1", Type: "bogus", Status: AuthorizationStatusApproved},
			}},
			ErrorInvalidAuthorizationType.Code,
		},
		{
			"invalid authorization status",
			&ConsentRequest{GroupID: "app1", Authorizations: []ConsentAuthorizationRequest{
				{UserID: "user1", Type: AuthorizationTypeAuthorization, Status: "bogus"},
			}},
			ErrorInvalidAuthorizationStatus.Code,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			created, svcErr := s.service.CreateConsent(context.Background(), tc.request)
			s.Nil(created)
			s.NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
	// No store call should have happened for validation failures.
	s.mockStore.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
}

// UpdateConsent tests

func (s *ConsentServiceTestSuite) TestUpdateConsent_MissingID() {
	created, svcErr := s.service.UpdateConsent(context.Background(), "", s.validCreateRequest())

	s.Nil(created)
	s.NotNil(svcErr)
	s.Equal(ErrorMissingConsentID.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestUpdateConsent_NotFound() {
	s.mockStore.On("GetConsent", mock.Anything, "c1").Return(nil, errConsentNotFound)

	created, svcErr := s.service.UpdateConsent(context.Background(), "c1", s.validCreateRequest())

	s.Nil(created)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentNotFound.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestUpdateConsent_Success() {
	existing := &Consent{
		ID:      "c1",
		GroupID: "app1",
		Status:  ConsentStatusActive,
	}
	s.mockStore.On("GetConsent", mock.Anything, "c1").Return(existing, nil)
	s.mockStore.On("UpdateConsent", mock.Anything, mock.AnythingOfType("*consent.Consent")).Return(nil)

	req := s.validCreateRequest()
	req.ValidityTime = 999

	updated, svcErr := s.service.UpdateConsent(context.Background(), "c1", req)

	s.Nil(svcErr)
	s.NotNil(updated)
	s.Equal("c1", updated.ID)
	// GroupID and Status are preserved from the existing record.
	s.Equal("app1", updated.GroupID)
	s.Equal(ConsentStatusActive, updated.Status)
	s.Equal(int64(999), updated.ValidityTime)
	s.Len(updated.Authorizations, 1)
	s.NotEmpty(updated.Authorizations[0].ID)
}

func (s *ConsentServiceTestSuite) TestUpdateConsent_StoreError() {
	existing := &Consent{ID: "c1", GroupID: "app1", Status: ConsentStatusActive}
	s.mockStore.On("GetConsent", mock.Anything, "c1").Return(existing, nil)
	s.mockStore.On("UpdateConsent", mock.Anything, mock.AnythingOfType("*consent.Consent")).
		Return(errors.New("db down"))

	updated, svcErr := s.service.UpdateConsent(context.Background(), "c1", s.validCreateRequest())

	s.Nil(updated)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestUpdateConsent_ValidationError() {
	updated, svcErr := s.service.UpdateConsent(context.Background(), "c1", &ConsentRequest{GroupID: ""})

	s.Nil(updated)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidRequestFormat.Code, svcErr.Code)
	s.mockStore.AssertNotCalled(s.T(), "GetConsent", mock.Anything, mock.Anything)
}

// SearchConsents tests

func (s *ConsentServiceTestSuite) TestSearchConsents_InvalidStatusFilter() {
	consents, svcErr := s.service.SearchConsents(context.Background(),
		ConsentFilter{ConsentStatus: "BOGUS"})

	s.Nil(consents)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidConsentStatus.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestSearchConsents_StoreError() {
	s.mockStore.On("SearchConsents", mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	consents, svcErr := s.service.SearchConsents(context.Background(), ConsentFilter{GroupID: "app1"})

	s.Nil(consents)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentServiceTestSuite) TestSearchConsents_ExpiredStatusDerived() {
	// A consent whose validity time has elapsed is reported as EXPIRED even though it is stored
	// as ACTIVE; a consent with no validity time stays ACTIVE.
	stored := []*Consent{
		{ID: "expired", Status: ConsentStatusActive, ValidityTime: 1},
		{ID: "active", Status: ConsentStatusActive, ValidityTime: 0},
	}
	s.mockStore.On("SearchConsents", mock.Anything, mock.Anything).Return(stored, nil)

	consents, svcErr := s.service.SearchConsents(context.Background(), ConsentFilter{GroupID: "app1"})

	s.Nil(svcErr)
	s.Len(consents, 2)
	byID := map[string]ConsentStatus{}
	for _, c := range consents {
		byID[c.ID] = c.Status
	}
	s.Equal(ConsentStatusExpired, byID["expired"])
	s.Equal(ConsentStatusActive, byID["active"])
}

func (s *ConsentServiceTestSuite) TestSearchConsents_StatusFilterAppliesToEffectiveStatus() {
	stored := []*Consent{
		{ID: "expired", Status: ConsentStatusActive, ValidityTime: 1},
		{ID: "active", Status: ConsentStatusActive, ValidityTime: 0},
	}
	s.mockStore.On("SearchConsents", mock.Anything, mock.Anything).Return(stored, nil)

	// Filtering for ACTIVE must exclude the record that is effectively expired.
	consents, svcErr := s.service.SearchConsents(context.Background(),
		ConsentFilter{GroupID: "app1", ConsentStatus: ConsentStatusActive})

	s.Nil(svcErr)
	s.Len(consents, 1)
	s.Equal("active", consents[0].ID)
}

// effectiveStatus tests

func (s *ConsentServiceTestSuite) TestEffectiveStatus() {
	const now = int64(1000)
	cases := []struct {
		name         string
		status       ConsentStatus
		validityTime int64
		expected     ConsentStatus
	}{
		{"never expires", ConsentStatusActive, 0, ConsentStatusActive},
		{"negative validity never expires", ConsentStatusActive, -5, ConsentStatusActive},
		{"not yet expired", ConsentStatusActive, now + 1, ConsentStatusActive},
		{"boundary equal is expired", ConsentStatusActive, now, ConsentStatusExpired},
		{"past validity is expired", ConsentStatusActive, now - 1, ConsentStatusExpired},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.expected, effectiveStatus(tc.status, tc.validityTime, now))
		})
	}
}

func (s *ConsentServiceTestSuite) TestBuildAuthorizations_StampsIDAndTime() {
	before := time.Now().Unix()
	auths, err := buildAuthorizations([]ConsentAuthorizationRequest{
		{UserID: "user1", Type: AuthorizationTypeAuthorization, Status: AuthorizationStatusApproved},
	})

	s.NoError(err)
	s.Len(auths, 1)
	s.NotEmpty(auths[0].ID)
	s.Equal("user1", auths[0].UserID)
	s.GreaterOrEqual(auths[0].UpdatedTime, before)
}
