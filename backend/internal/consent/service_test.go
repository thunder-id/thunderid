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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type ConsentServiceTestSuite struct {
	suite.Suite
}

func TestConsentServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentServiceTestSuite))
}

// initConsentRuntime initializes a minimal server runtime for service tests.
func initConsentRuntime(t *testing.T, enabled bool, baseURL string) {
	t.Helper()
	cfg := &config.Config{
		Consent: config.ConsentConfig{
			Enabled: enabled,
			BaseURL: baseURL,
		},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", cfg))
	t.Cleanup(config.ResetServerRuntime)
}

// newServiceWithMockClient creates a consentService with the provided mock client and config.
func newServiceWithMockClient(t *testing.T, enabled bool, client consentClientInterface) *consentService {
	t.Helper()
	initConsentRuntime(t, enabled, "http://consent.example.com")
	return &consentService{
		enabled: enabled,
		client:  client,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentService")),
	}
}

// ----- newConsentService -----

func (s *ConsentServiceTestSuite) TestNewConsentService_EnabledTrue() {
	initConsentRuntime(s.T(), true, "http://example.com")
	clientMock := newConsentClientInterfaceMock(s.T())

	svc := newConsentService(clientMock)

	s.True(svc.IsEnabled())
}

func (s *ConsentServiceTestSuite) TestNewConsentService_EnabledFalse() {
	initConsentRuntime(s.T(), false, "")
	clientMock := newConsentClientInterfaceMock(s.T())

	svc := newConsentService(clientMock)

	s.False(svc.IsEnabled())
}

// ----- IsEnabled -----

func (s *ConsentServiceTestSuite) TestIsEnabled_ReturnsTrue() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	s.True(svc.IsEnabled())
}

func (s *ConsentServiceTestSuite) TestIsEnabled_ReturnsFalse() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), false, clientMock)

	s.False(svc.IsEnabled())
}

// ----- CreateConsentElements -----

func (s *ConsentServiceTestSuite) TestCreateConsentElements_EmptyInputReturnsNil() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	result, svcErr := svc.CreateConsentElements(context.Background(), "ou1", []ConsentElementInput{})

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentServiceTestSuite) TestCreateConsentElements_DelegatesToClient() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	inputs := []ConsentElementInput{
		{Name: "email", Namespace: NamespaceAttribute},
	}
	expected := []ConsentElement{
		{ID: "elem-1", Name: "email"},
	}

	clientMock.EXPECT().createConsentElements(mock.Anything, "ou1", inputs).Return(expected, nil)

	result, svcErr := svc.CreateConsentElements(context.Background(), "ou1", inputs)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestCreateConsentElements_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	inputs := []ConsentElementInput{{Name: "attr1"}}
	clientErr := &ErrorInvalidConsentElementRequest

	clientMock.EXPECT().createConsentElements(mock.Anything, "ou1", inputs).Return(nil, clientErr)

	result, svcErr := svc.CreateConsentElements(context.Background(), "ou1", inputs)

	s.Nil(result)
	s.Equal(clientErr, svcErr)
}

// ----- ListConsentElements -----

func (s *ConsentServiceTestSuite) TestListConsentElements_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	expected := []ConsentElement{{ID: "elem-1", Name: "email"}}
	clientMock.EXPECT().listConsentElements(mock.Anything, "ou1", NamespaceAttribute, "email").Return(expected, nil)

	result, svcErr := svc.ListConsentElements(context.Background(), "ou1", NamespaceAttribute, "email")

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestListConsentElements_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().listConsentElements(mock.Anything, "ou1", NamespaceAttribute, "").
		Return(nil, &serviceerror.InternalServerError)

	result, svcErr := svc.ListConsentElements(context.Background(), "ou1", NamespaceAttribute, "")

	s.Nil(result)
	s.NotNil(svcErr)
}

// ----- UpdateConsentElement -----

func (s *ConsentServiceTestSuite) TestUpdateConsentElement_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	input := &ConsentElementInput{Name: "email-updated"}
	expected := &ConsentElement{ID: "elem-1", Name: "email-updated"}
	clientMock.EXPECT().updateConsentElement(mock.Anything, "ou1", "elem-1", input).Return(expected, nil)

	result, svcErr := svc.UpdateConsentElement(context.Background(), "ou1", "elem-1", input)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestUpdateConsentElement_NotFound() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	input := &ConsentElementInput{Name: "missing"}
	clientMock.EXPECT().updateConsentElement(mock.Anything, "ou1", "elem-99", input).
		Return(nil, &ErrorConsentElementNotFound)

	result, svcErr := svc.UpdateConsentElement(context.Background(), "ou1", "elem-99", input)

	s.Nil(result)
	s.Equal(&ErrorConsentElementNotFound, svcErr)
}

func (s *ConsentServiceTestSuite) TestUpdateConsentElement_NilInput() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	// Client should not be called when input is nil
	result, svcErr := svc.UpdateConsentElement(context.Background(), "ou1", "elem-1", nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(&ErrorInvalidRequestFormat, svcErr)
}

// ----- DeleteConsentElement -----

func (s *ConsentServiceTestSuite) TestDeleteConsentElement_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().deleteConsentElement(mock.Anything, "ou1", "elem-1").Return(
		(*serviceerror.ServiceError)(nil))

	svcErr := svc.DeleteConsentElement(context.Background(), "ou1", "elem-1")

	s.Nil(svcErr)
}

func (s *ConsentServiceTestSuite) TestDeleteConsentElement_NotFoundIsIdempotent() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().deleteConsentElement(mock.Anything, "ou1", "elem-missing").
		Return(&ErrorConsentElementNotFound)

	svcErr := svc.DeleteConsentElement(context.Background(), "ou1", "elem-missing")

	s.Nil(svcErr)
}

func (s *ConsentServiceTestSuite) TestDeleteConsentElement_OtherErrorPropagated() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().deleteConsentElement(mock.Anything, "ou1", "elem-1").
		Return(&ErrorDeletingConsentElementWithAssociatedPurpose)

	svcErr := svc.DeleteConsentElement(context.Background(), "ou1", "elem-1")

	s.NotNil(svcErr)
	s.Equal(&ErrorDeletingConsentElementWithAssociatedPurpose, svcErr)
}

// ----- ValidateConsentElements -----

func (s *ConsentServiceTestSuite) TestValidateConsentElements_EmptyNamesReturnsEmpty() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	result, svcErr := svc.ValidateConsentElements(context.Background(), "ou1", []string{})

	s.Nil(svcErr)
	s.Equal([]string{}, result)
}

func (s *ConsentServiceTestSuite) TestValidateConsentElements_DelegatesToClient() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	names := []string{"email", "phone"}
	expected := []string{"email"}
	clientMock.EXPECT().validateConsentElements(mock.Anything, "ou1", names).Return(expected, nil)

	result, svcErr := svc.ValidateConsentElements(context.Background(), "ou1", names)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestValidateConsentElements_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	names := []string{"attr1"}
	clientMock.EXPECT().validateConsentElements(mock.Anything, "ou1", names).
		Return(nil, &serviceerror.InternalServerError)

	result, svcErr := svc.ValidateConsentElements(context.Background(), "ou1", names)

	s.Nil(result)
	s.NotNil(svcErr)
}

// ----- CreateConsentPurpose -----

func (s *ConsentServiceTestSuite) TestCreateConsentPurpose_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	input := &ConsentPurposeInput{Name: "My Purpose", GroupID: "app-1"}
	expected := &ConsentPurpose{ID: "purpose-1", Name: "My Purpose"}
	clientMock.EXPECT().createConsentPurpose(mock.Anything, "ou1", input).Return(expected, nil)

	result, svcErr := svc.CreateConsentPurpose(context.Background(), "ou1", input)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestCreateConsentPurpose_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	input := &ConsentPurposeInput{Name: "Duplicate"}
	clientMock.EXPECT().createConsentPurpose(mock.Anything, "ou1", input).
		Return(nil, &ErrorConsentPurposeAlreadyExists)

	result, svcErr := svc.CreateConsentPurpose(context.Background(), "ou1", input)

	s.Nil(result)
	s.Equal(&ErrorConsentPurposeAlreadyExists, svcErr)
}

func (s *ConsentServiceTestSuite) TestCreateConsentPurpose_NilInput() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	// Client should not be called when input is nil
	result, svcErr := svc.CreateConsentPurpose(context.Background(), "ou1", nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(&ErrorInvalidRequestFormat, svcErr)
}

// ----- ListConsentPurposes -----

func (s *ConsentServiceTestSuite) TestListConsentPurposes_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	expected := []ConsentPurpose{{ID: "purpose-1", Name: "Login Purpose"}}
	clientMock.EXPECT().listConsentPurposes(mock.Anything, "ou1", "app-1").Return(expected, nil)

	result, svcErr := svc.ListConsentPurposes(context.Background(), "ou1", "app-1")

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestListConsentPurposes_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().listConsentPurposes(mock.Anything, "ou1", "").
		Return(nil, &serviceerror.InternalServerError)

	result, svcErr := svc.ListConsentPurposes(context.Background(), "ou1", "")

	s.Nil(result)
	s.NotNil(svcErr)
}

// ----- UpdateConsentPurpose -----

func (s *ConsentServiceTestSuite) TestUpdateConsentPurpose_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	input := &ConsentPurposeInput{Name: "Updated"}
	expected := &ConsentPurpose{ID: "purpose-1", Name: "Updated"}
	clientMock.EXPECT().updateConsentPurpose(mock.Anything, "ou1", "purpose-1", input).Return(expected, nil)

	result, svcErr := svc.UpdateConsentPurpose(context.Background(), "ou1", "purpose-1", input)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestUpdateConsentPurpose_NotFound() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	input := &ConsentPurposeInput{Name: "X"}
	clientMock.EXPECT().updateConsentPurpose(mock.Anything, "ou1", "missing", input).
		Return(nil, &ErrorConsentPurposeNotFound)

	result, svcErr := svc.UpdateConsentPurpose(context.Background(), "ou1", "missing", input)

	s.Nil(result)
	s.Equal(&ErrorConsentPurposeNotFound, svcErr)
}

func (s *ConsentServiceTestSuite) TestUpdateConsentPurpose_NilInput() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	// Client should not be called when input is nil
	result, svcErr := svc.UpdateConsentPurpose(context.Background(), "ou1", "purpose-1", nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(&ErrorInvalidRequestFormat, svcErr)
}

// ----- DeleteConsentPurpose -----

func (s *ConsentServiceTestSuite) TestDeleteConsentPurpose_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().deleteConsentPurpose(mock.Anything, "ou1", "purpose-1").Return(
		(*serviceerror.ServiceError)(nil))

	svcErr := svc.DeleteConsentPurpose(context.Background(), "ou1", "purpose-1")

	s.Nil(svcErr)
}

func (s *ConsentServiceTestSuite) TestDeleteConsentPurpose_NotFoundIsIdempotent() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().deleteConsentPurpose(mock.Anything, "ou1", "missing").
		Return(&ErrorConsentPurposeNotFound)

	svcErr := svc.DeleteConsentPurpose(context.Background(), "ou1", "missing")

	s.Nil(svcErr)
}

func (s *ConsentServiceTestSuite) TestDeleteConsentPurpose_OtherErrorPropagated() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().deleteConsentPurpose(mock.Anything, "ou1", "purpose-1").
		Return(&ErrorDeletingConsentPurposeWithAssociatedRecords)

	svcErr := svc.DeleteConsentPurpose(context.Background(), "ou1", "purpose-1")

	s.NotNil(svcErr)
	s.Equal(&ErrorDeletingConsentPurposeWithAssociatedRecords, svcErr)
}

// ----- CreateConsent -----

func (s *ConsentServiceTestSuite) TestCreateConsent_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	req := &ConsentRequest{Type: "authentication", GroupID: "app-1"}
	expected := &Consent{ID: "consent-1", Type: "authentication"}
	clientMock.EXPECT().createConsent(mock.Anything, "ou1", req).Return(expected, nil)

	result, svcErr := svc.CreateConsent(context.Background(), "ou1", req)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestCreateConsent_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	req := &ConsentRequest{Type: "authentication"}
	clientMock.EXPECT().createConsent(mock.Anything, "ou1", req).
		Return(nil, &ErrorInvalidConsentRecordRequest)

	result, svcErr := svc.CreateConsent(context.Background(), "ou1", req)

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentRecordRequest, svcErr)
}

func (s *ConsentServiceTestSuite) TestCreateConsent_NilInput() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	// Client should not be called when input is nil
	result, svcErr := svc.CreateConsent(context.Background(), "ou1", nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(&ErrorInvalidRequestFormat, svcErr)
}

// ----- SearchConsents -----

func (s *ConsentServiceTestSuite) TestSearchConsents_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	filter := &ConsentSearchFilter{ConsentTypes: []ConsentType{ConsentTypeAuthentication}}
	expected := []Consent{{ID: "c1", Type: "authentication"}}
	clientMock.EXPECT().searchConsents(mock.Anything, "ou1", filter).Return(expected, nil)

	result, svcErr := svc.SearchConsents(context.Background(), "ou1", filter)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestSearchConsents_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	filter := &ConsentSearchFilter{}
	clientMock.EXPECT().searchConsents(mock.Anything, "ou1", filter).
		Return(nil, &ErrorInvalidConsentSearchFilter)

	result, svcErr := svc.SearchConsents(context.Background(), "ou1", filter)

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentSearchFilter, svcErr)
}

// ----- ValidateConsent -----

func (s *ConsentServiceTestSuite) TestValidateConsent_Valid() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	consent := &Consent{ID: "c1", Status: "ACTIVE"}
	expected := &ConsentValidationResult{IsValid: true, ConsentInformation: consent}
	clientMock.EXPECT().validateConsent(mock.Anything, "ou1", "c1").Return(expected, nil)

	result, svcErr := svc.ValidateConsent(context.Background(), "ou1", "c1")

	s.Nil(svcErr)
	s.True(result.IsValid)
}

func (s *ConsentServiceTestSuite) TestValidateConsent_Invalid() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	expected := &ConsentValidationResult{IsValid: false}
	clientMock.EXPECT().validateConsent(mock.Anything, "ou1", "c1").Return(expected, nil)

	result, svcErr := svc.ValidateConsent(context.Background(), "ou1", "c1")

	s.Nil(svcErr)
	s.False(result.IsValid)
}

func (s *ConsentServiceTestSuite) TestValidateConsent_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	clientMock.EXPECT().validateConsent(mock.Anything, "ou1", "c1").
		Return(nil, &ErrorInvalidConsentValidationRequest)

	result, svcErr := svc.ValidateConsent(context.Background(), "ou1", "c1")

	s.Nil(result)
	s.Equal(&ErrorInvalidConsentValidationRequest, svcErr)
}

// ----- RevokeConsent -----

func (s *ConsentServiceTestSuite) TestRevokeConsent_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	payload := &ConsentRevokeRequest{Reason: "user requested"}
	clientMock.EXPECT().revokeConsent(mock.Anything, "ou1", "c1", payload).Return((*serviceerror.ServiceError)(nil))

	svcErr := svc.RevokeConsent(context.Background(), "ou1", "c1", payload)

	s.Nil(svcErr)
}

func (s *ConsentServiceTestSuite) TestRevokeConsent_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	payload := &ConsentRevokeRequest{}
	clientMock.EXPECT().revokeConsent(mock.Anything, "ou1", "c1", payload).
		Return(&ErrorInvalidConsentRevokeRequest)

	svcErr := svc.RevokeConsent(context.Background(), "ou1", "c1", payload)

	s.Equal(&ErrorInvalidConsentRevokeRequest, svcErr)
}

func (s *ConsentServiceTestSuite) TestRevokeConsent_NilInput() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	// Client should not be called when payload is nil
	svcErr := svc.RevokeConsent(context.Background(), "ou1", "c1", nil)

	s.NotNil(svcErr)
	s.Equal(&ErrorInvalidRequestFormat, svcErr)
}

// ----- UpdateConsent -----

func (s *ConsentServiceTestSuite) TestUpdateConsent_Success() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	req := &ConsentRequest{Type: ConsentTypeAuthentication, GroupID: "app-1"}
	expected := &Consent{ID: "consent-1", Type: ConsentTypeAuthentication, Status: ConsentStatusActive}
	clientMock.EXPECT().updateConsent(mock.Anything, "ou1", "consent-1", req).Return(expected, nil)

	result, svcErr := svc.UpdateConsent(context.Background(), "ou1", "consent-1", req)

	s.Nil(svcErr)
	s.Equal(expected, result)
}

func (s *ConsentServiceTestSuite) TestUpdateConsent_ClientError() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	req := &ConsentRequest{Type: ConsentTypeAuthentication}
	clientMock.EXPECT().updateConsent(mock.Anything, "ou1", "c-missing", req).
		Return(nil, &ErrorConsentRecordNotFound)

	result, svcErr := svc.UpdateConsent(context.Background(), "ou1", "c-missing", req)

	s.Nil(result)
	s.Equal(&ErrorConsentRecordNotFound, svcErr)
}

func (s *ConsentServiceTestSuite) TestUpdateConsent_NilInput() {
	clientMock := newConsentClientInterfaceMock(s.T())
	svc := newServiceWithMockClient(s.T(), true, clientMock)

	// Client should not be called when input is nil
	result, svcErr := svc.UpdateConsent(context.Background(), "ou1", "consent-1", nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(&ErrorInvalidRequestFormat, svcErr)
}
