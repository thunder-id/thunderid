/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package cert

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

type ServiceTestSuite struct {
	suite.Suite
	mockStore         *certificateStoreInterfaceMock
	mockTransactioner *MockTransactioner
	service           CertificateServiceInterface
}

type MockTransactioner struct {
	mock.Mock
}

func (m *MockTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	args := m.Called(ctx, txFunc)
	if txFunc != nil {
		return txFunc(ctx)
	}
	return args.Error(0)
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.mockStore = newCertificateStoreInterfaceMock(suite.T())
	suite.mockTransactioner = new(MockTransactioner)
	// Default behavior for Transact: execute the function
	suite.mockTransactioner.On("Transact", mock.Anything, mock.Anything).Return(nil)
	suite.service = newCertificateService(suite.mockStore, suite.mockTransactioner)
}

// Helper function to create a valid certificate for testing
func (suite *ServiceTestSuite) createValidCertificate() *Certificate {
	return &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}
}

// ============================================================================
// GetCertificateByID Tests
// ============================================================================

func (suite *ServiceTestSuite) TestGetCertificateByID_Success() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").Return(cert, nil)

	result, err := suite.service.GetCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), cert.ID, result.ID)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetCertificateByID_EmptyID() {
	result, err := suite.service.GetCertificateByID(context.Background(), "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestGetCertificateByID_NotFound() {
	suite.mockStore.On("GetCertificateByID", mock.Anything, "non-existent-id").
		Return(nil, ErrCertificateNotFound)

	result, err := suite.service.GetCertificateByID(context.Background(), "non-existent-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetCertificateByID_NilResult() {
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").Return(nil, nil)

	result, err := suite.service.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetCertificateByID_StoreError() {
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").
		Return(nil, errors.New("database error"))

	result, err := suite.service.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// GetCertificateByReference Tests
// ============================================================================

func (suite *ServiceTestSuite) TestGetCertificateByReference_Success() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").Return(cert, nil)

	result, err := suite.service.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), cert.ID, result.ID)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetCertificateByReference_InvalidReferenceType() {
	result, err := suite.service.GetCertificateByReference(context.Background(),
		CertificateReferenceType("INVALID"), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceType.Code, err.Code)
}

func (suite *ServiceTestSuite) TestGetCertificateByReference_EmptyReferenceID() {
	result, err := suite.service.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestGetCertificateByReference_NotFound() {
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeIDP, "non-existent").
		Return(nil, ErrCertificateNotFound)

	result, err := suite.service.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "non-existent")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetCertificateByReference_NilResult() {
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-id").Return(nil, nil)

	result, err := suite.service.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestGetCertificateByReference_StoreError() {
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-id").
		Return(nil, errors.New("database error"))

	result, err := suite.service.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// CreateCertificate Tests
// ============================================================================

func (suite *ServiceTestSuite) TestCreateCertificate_Success() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		cert.RefType, cert.RefID).Return(nil, ErrCertificateNotFound)
	suite.mockStore.On("CreateCertificate", mock.Anything, cert).Return(nil)

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotEmpty(suite.T(), result.ID)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestCreateCertificate_NilCertificate() {
	result, err := suite.service.CreateCertificate(context.Background(), nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateValue.Code, err.Code)
}

func (suite *ServiceTestSuite) TestCreateCertificate_EmptyReferenceID() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestCreateCertificate_InvalidReferenceType() {
	cert := &Certificate{
		RefType: CertificateReferenceType("INVALID"),
		RefID:   "test-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceType.Code, err.Code)
}

func (suite *ServiceTestSuite) TestCreateCertificate_InvalidCertificateType() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-id",
		Type:    CertificateType("INVALID"),
		Value:   "valid-certificate-value-string",
	}

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateType.Code, err.Code)
}

func (suite *ServiceTestSuite) TestCreateCertificate_ValueTooShort() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-id",
		Type:    CertificateTypeJWKS,
		Value:   "short",
	}

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateValue.Code, err.Code)
}

func (suite *ServiceTestSuite) TestCreateCertificate_ValueTooLong() {
	longValue := ""
	for i := 0; i < 5000; i++ {
		longValue += "a"
	}
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-id",
		Type:    CertificateTypeJWKS,
		Value:   longValue,
	}

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateValue.Code, err.Code)
}

func (suite *ServiceTestSuite) TestCreateCertificate_AlreadyExists() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}
	existingCert := suite.createValidCertificate()

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		cert.RefType, cert.RefID).Return(existingCert, nil)

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateAlreadyExists.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestCreateCertificate_CheckExistingError() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		cert.RefType, cert.RefID).Return(nil, errors.New("database error"))

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestCreateCertificate_StoreError() {
	cert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		cert.RefType, cert.RefID).Return(nil, ErrCertificateNotFound)
	suite.mockStore.On("CreateCertificate", mock.Anything, cert).
		Return(errors.New("database error"))

	result, err := suite.service.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// UpdateCertificateByID Tests
// ============================================================================

func (suite *ServiceTestSuite) TestUpdateCertificateByID_Success() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-certificate-value",
	}

	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").
		Return(existingCert, nil)
	suite.mockStore.On("UpdateCertificateByID", mock.Anything, existingCert, updatedCert).
		Return(nil)

	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-cert-id", updatedCert)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), updatedCert.Type, result.Type)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_EmptyID() {
	cert := suite.createValidCertificate()

	result, err := suite.service.UpdateCertificateByID(context.Background(), "", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_InvalidCertificate() {
	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-id", nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateValue.Code, err.Code)
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_NotFound() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByID", mock.Anything, "non-existent-id").
		Return(nil, ErrCertificateNotFound)

	result, err := suite.service.UpdateCertificateByID(context.Background(), "non-existent-id", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_NilExisting() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").Return(nil, nil)

	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-id", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_ReferenceChanged() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeIDP,
		RefID:   "different-ref-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").
		Return(existingCert, nil)

	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-cert-id", updatedCert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorReferenceUpdateIsNotAllowed.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_GetExistingError() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").
		Return(nil, errors.New("database error"))

	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-id", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_UpdateError() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-certificate-value",
	}

	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").
		Return(existingCert, nil)
	suite.mockStore.On("UpdateCertificateByID", mock.Anything, existingCert, updatedCert).
		Return(errors.New("database error"))

	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-cert-id", updatedCert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByID_UpdateNotFoundError() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-certificate-value",
	}

	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").
		Return(existingCert, nil)
	suite.mockStore.On("UpdateCertificateByID", mock.Anything, existingCert, updatedCert).
		Return(ErrCertificateNotFound)

	result, err := suite.service.UpdateCertificateByID(context.Background(), "test-cert-id", updatedCert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// UpdateCertificateByReference Tests
// ============================================================================

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_Success() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-certificate-value",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").
		Return(existingCert, nil)
	suite.mockStore.On("UpdateCertificateByReference", mock.Anything, existingCert, updatedCert).
		Return(nil)

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id", updatedCert)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), existingCert.ID, result.ID)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_InvalidReferenceType() {
	cert := suite.createValidCertificate()

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceType("INVALID"), "test-id", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceType.Code, err.Code)
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_EmptyReferenceID() {
	cert := suite.createValidCertificate()

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_InvalidCertificate() {
	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-id", nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateValue.Code, err.Code)
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_NotFound() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeIDP, "non-existent").
		Return(nil, ErrCertificateNotFound)

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "non-existent", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_NilExisting() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-id").Return(nil, nil)

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-id", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_ReferenceChanged() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeIDP,
		RefID:   "different-ref-id",
		Type:    CertificateTypeJWKS,
		Value:   "valid-certificate-value-string",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").
		Return(existingCert, nil)

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id", updatedCert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorReferenceUpdateIsNotAllowed.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_GetExistingError() {
	cert := suite.createValidCertificate()
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-id").
		Return(nil, errors.New("database error"))

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-id", cert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_UpdateError() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-certificate-value",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").
		Return(existingCert, nil)
	suite.mockStore.On("UpdateCertificateByReference", mock.Anything, existingCert, updatedCert).
		Return(errors.New("database error"))

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id", updatedCert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestUpdateCertificateByReference_UpdateNotFoundError() {
	existingCert := suite.createValidCertificate()
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-certificate-value",
	}

	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").
		Return(existingCert, nil)
	suite.mockStore.On("UpdateCertificateByReference", mock.Anything, existingCert, updatedCert).
		Return(ErrCertificateNotFound)

	result, err := suite.service.UpdateCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id", updatedCert)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorCertificateNotFound.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// DeleteCertificateByID Tests
// ============================================================================

func (suite *ServiceTestSuite) TestDeleteCertificateByID_Success() {
	suite.mockStore.On("DeleteCertificateByID", mock.Anything, "test-cert-id").Return(nil)

	err := suite.service.DeleteCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestDeleteCertificateByID_EmptyID() {
	err := suite.service.DeleteCertificateByID(context.Background(), "")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidCertificateID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestDeleteCertificateByID_StoreError() {
	suite.mockStore.On("DeleteCertificateByID", mock.Anything, "test-id").
		Return(errors.New("database error"))

	err := suite.service.DeleteCertificateByID(context.Background(), "test-id")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// DeleteCertificateByReference Tests
// ============================================================================

func (suite *ServiceTestSuite) TestDeleteCertificateByReference_Success() {
	suite.mockStore.On("DeleteCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").Return(nil)

	err := suite.service.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestDeleteCertificateByReference_InvalidReferenceType() {
	err := suite.service.DeleteCertificateByReference(context.Background(),
		CertificateReferenceType("INVALID"), "test-id")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceType.Code, err.Code)
}

func (suite *ServiceTestSuite) TestDeleteCertificateByReference_EmptyReferenceID() {
	err := suite.service.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidReferenceID.Code, err.Code)
}

func (suite *ServiceTestSuite) TestDeleteCertificateByReference_StoreError() {
	suite.mockStore.On("DeleteCertificateByReference", mock.Anything,
		CertificateReferenceTypeIDP, "test-id").
		Return(errors.New("database error"))

	err := suite.service.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "test-id")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), serviceerror.InternalServerError.Code, err.Code)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func (suite *ServiceTestSuite) TestIsValidReferenceType() {
	testCases := []struct {
		name     string
		refType  CertificateReferenceType
		expected bool
	}{
		{"Application type", CertificateReferenceTypeApplication, true},
		{"IDP type", CertificateReferenceTypeIDP, true},
		{"Invalid type", CertificateReferenceType("INVALID"), false},
		{"Empty type", CertificateReferenceType(""), false},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := isValidReferenceType(tc.refType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *ServiceTestSuite) TestIsValidCertificateType() {
	testCases := []struct {
		name     string
		certType CertificateType
		expected bool
	}{
		{"JWKS type", CertificateTypeJWKS, true},
		{"JWKS URI type", CertificateTypeJWKSURI, true},
		{"Invalid type", CertificateType("INVALID"), false},
		{"Empty type", CertificateType(""), false},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := isValidCertificateType(tc.certType)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func (suite *ServiceTestSuite) TestValidateCertificate() {
	testCases := []struct {
		name          string
		cert          *Certificate
		expectedError *serviceerror.ServiceError
	}{
		{
			name:          "Nil certificate",
			cert:          nil,
			expectedError: &ErrorInvalidCertificateValue,
		},
		{
			name: "Empty ID",
			cert: &Certificate{
				ID:      "",
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-id",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidCertificateID,
		},
		{
			name: "Empty RefID",
			cert: &Certificate{
				ID:      "test-id",
				RefType: CertificateReferenceTypeApplication,
				RefID:   "",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidReferenceID,
		},
		{
			name: "Invalid RefType",
			cert: &Certificate{
				ID:      "test-id",
				RefType: CertificateReferenceType("INVALID"),
				RefID:   "test-ref-id",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidReferenceType,
		},
		{
			name: "Invalid Type",
			cert: &Certificate{
				ID:      "test-id",
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-ref-id",
				Type:    CertificateType("INVALID"),
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidCertificateType,
		},
		{
			name: "Value too short",
			cert: &Certificate{
				ID:      "test-id",
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-ref-id",
				Type:    CertificateTypeJWKS,
				Value:   "short",
			},
			expectedError: &ErrorInvalidCertificateValue,
		},
		{
			name: "Valid certificate",
			cert: &Certificate{
				ID:      "test-id",
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-ref-id",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := validateCertificate(tc.cert)
			if tc.expectedError == nil {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError.Code, err.Code)
			}
		})
	}
}

func (suite *ServiceTestSuite) TestValidateCertificateForCreation() {
	testCases := []struct {
		name          string
		cert          *Certificate
		expectedError *serviceerror.ServiceError
	}{
		{
			name:          "Nil certificate",
			cert:          nil,
			expectedError: &ErrorInvalidCertificateValue,
		},
		{
			name: "Empty RefID",
			cert: &Certificate{
				RefType: CertificateReferenceTypeApplication,
				RefID:   "",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidReferenceID,
		},
		{
			name: "Invalid RefType",
			cert: &Certificate{
				RefType: CertificateReferenceType("INVALID"),
				RefID:   "test-ref-id",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidReferenceType,
		},
		{
			name: "Invalid Type",
			cert: &Certificate{
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-ref-id",
				Type:    CertificateType("INVALID"),
				Value:   "valid-certificate-value",
			},
			expectedError: &ErrorInvalidCertificateType,
		},
		{
			name: "Value too short",
			cert: &Certificate{
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-ref-id",
				Type:    CertificateTypeJWKS,
				Value:   "short",
			},
			expectedError: &ErrorInvalidCertificateValue,
		},
		{
			name: "Valid certificate without ID",
			cert: &Certificate{
				RefType: CertificateReferenceTypeApplication,
				RefID:   "test-ref-id",
				Type:    CertificateTypeJWKS,
				Value:   "valid-certificate-value",
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := validateCertificateForCreation(tc.cert)
			if tc.expectedError == nil {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError.Code, err.Code)
			}
		})
	}
}
