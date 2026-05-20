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

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type StoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *certificateStore
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &certificateStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: "test-deployment-id",
	}
}

// Helper function to create test result row
func (suite *StoreTestSuite) createTestResultRow() map[string]interface{} {
	return map[string]interface{}{
		"id":       "test-cert-id",
		"ref_type": "APPLICATION",
		"ref_id":   "test-app-id",
		"type":     "JWKS",
		"value":    "test-certificate-value",
	}
}

// ============================================================================
// GetCertificateByID Tests
// ============================================================================

func (suite *StoreTestSuite) TestGetCertificateByID_Success() {
	row := suite.createTestResultRow()
	results := []map[string]interface{}{row}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCertificateByID, "test-cert-id", mock.Anything).
		Return(results, nil)

	result, err := suite.store.GetCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-cert-id", result.ID)
	assert.Equal(suite.T(), CertificateReferenceTypeApplication, result.RefType)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestGetCertificateByID_DBProviderError() {
	suite.mockDBProvider.On("GetConfigDBClient").
		Return(nil, errors.New("db provider error"))

	result, err := suite.store.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestGetCertificateByID_QueryError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCertificateByID, "test-id", mock.Anything).
		Return(nil, errors.New("query error"))

	result, err := suite.store.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to execute query")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestGetCertificateByID_NotFound() {
	results := []map[string]interface{}{}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCertificateByID, "non-existent", mock.Anything).
		Return(results, nil)

	result, err := suite.store.GetCertificateByID(context.Background(), "non-existent")

	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, ErrCertificateNotFound)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestGetCertificateByID_MultipleResults() {
	row1 := suite.createTestResultRow()
	row2 := suite.createTestResultRow()
	row2["id"] = "test-cert-id-2"
	results := []map[string]interface{}{row1, row2}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCertificateByID, "test-id", mock.Anything).
		Return(results, nil)

	result, err := suite.store.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "multiple certificates found")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

// ============================================================================
// GetCertificateByReference Tests
// ============================================================================

func (suite *StoreTestSuite) TestGetCertificateByReference_Success() {
	row := suite.createTestResultRow()
	results := []map[string]interface{}{row}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCertificateByReference,
		CertificateReferenceTypeApplication, "test-app-id", mock.Anything).
		Return(results, nil)

	result, err := suite.store.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-cert-id", result.ID)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestGetCertificateByReference_NotFound() {
	results := []map[string]interface{}{}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCertificateByReference,
		CertificateReferenceTypeIDP, "non-existent", mock.Anything).
		Return(results, nil)

	result, err := suite.store.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "non-existent")

	assert.Nil(suite.T(), result)
	assert.ErrorIs(suite.T(), err, ErrCertificateNotFound)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

// ============================================================================
// BuildCertificateFromResultRow Tests
// ============================================================================

func (suite *StoreTestSuite) TestBuildCertificateFromResultRow_Success() {
	row := suite.createTestResultRow()

	result, err := suite.store.buildCertificateFromResultRow(row)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "test-cert-id", result.ID)
	assert.Equal(suite.T(), CertificateReferenceTypeApplication, result.RefType)
	assert.Equal(suite.T(), "test-app-id", result.RefID)
	assert.Equal(suite.T(), CertificateTypeJWKS, result.Type)
	assert.Equal(suite.T(), "test-certificate-value", result.Value)
}

func (suite *StoreTestSuite) TestBuildCertificateFromResultRow_InvalidCertID() {
	row := suite.createTestResultRow()
	row["id"] = 123 // Invalid type

	result, err := suite.store.buildCertificateFromResultRow(row)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse id")
}

func (suite *StoreTestSuite) TestBuildCertificateFromResultRow_InvalidRefType() {
	row := suite.createTestResultRow()
	row["ref_type"] = 123 // Invalid type

	result, err := suite.store.buildCertificateFromResultRow(row)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse ref_type")
}

func (suite *StoreTestSuite) TestBuildCertificateFromResultRow_InvalidRefID() {
	row := suite.createTestResultRow()
	row["ref_id"] = 123 // Invalid type

	result, err := suite.store.buildCertificateFromResultRow(row)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse ref_id")
}

func (suite *StoreTestSuite) TestBuildCertificateFromResultRow_InvalidType() {
	row := suite.createTestResultRow()
	row["type"] = 123 // Invalid type

	result, err := suite.store.buildCertificateFromResultRow(row)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse type")
}

func (suite *StoreTestSuite) TestBuildCertificateFromResultRow_InvalidValue() {
	row := suite.createTestResultRow()
	row["value"] = 123 // Invalid type

	result, err := suite.store.buildCertificateFromResultRow(row)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to parse value")
}

// ============================================================================
// CreateCertificate Tests
// ============================================================================

func (suite *StoreTestSuite) TestCreateCertificate_Success() {
	cert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "test-certificate-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertCertificate,
		cert.ID, cert.RefType, cert.RefID, cert.Type, cert.Value, mock.Anything).
		Return(int64(1), nil)

	err := suite.store.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestCreateCertificate_DBProviderError() {
	cert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "test-certificate-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").
		Return(nil, errors.New("db provider error"))

	err := suite.store.CreateCertificate(context.Background(), cert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestCreateCertificate_ExecuteError() {
	cert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "test-certificate-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertCertificate, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.CreateCertificate(context.Background(), cert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to insert certificate")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestCreateCertificate_NoRowsAffected() {
	cert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "test-certificate-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertCertificate,
		cert.ID, cert.RefType, cert.RefID, cert.Type, cert.Value, mock.Anything).
		Return(int64(0), nil)

	err := suite.store.CreateCertificate(context.Background(), cert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no rows affected")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

// ============================================================================
// UpdateCertificateByID Tests
// ============================================================================

func (suite *StoreTestSuite) TestUpdateCertificateByID_Success() {
	existingCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "old-value",
	}
	updatedCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "new-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCertificateByID,
		existingCert.ID, updatedCert.Type, updatedCert.Value, mock.Anything).
		Return(int64(1), nil)

	err := suite.store.UpdateCertificateByID(context.Background(), existingCert, updatedCert)

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestUpdateCertificateByID_DBProviderError() {
	existingCert := &Certificate{ID: "test-id"}
	updatedCert := &Certificate{Type: CertificateTypeJWKS, Value: "new-value"}

	suite.mockDBProvider.On("GetConfigDBClient").
		Return(nil, errors.New("db provider error"))

	err := suite.store.UpdateCertificateByID(context.Background(), existingCert, updatedCert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestUpdateCertificateByID_ExecuteError() {
	existingCert := &Certificate{ID: "test-id"}
	updatedCert := &Certificate{Type: CertificateTypeJWKS, Value: "new-value"}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCertificateByID, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.UpdateCertificateByID(context.Background(), existingCert, updatedCert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to update certificate")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestUpdateCertificateByID_NoRowsAffected() {
	existingCert := &Certificate{ID: "test-id"}
	updatedCert := &Certificate{Type: CertificateTypeJWKS, Value: "new-value"}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCertificateByID,
		existingCert.ID, updatedCert.Type, updatedCert.Value, mock.Anything).
		Return(int64(0), nil)

	err := suite.store.UpdateCertificateByID(context.Background(), existingCert, updatedCert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no rows affected")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

// ============================================================================
// UpdateCertificateByReference Tests
// ============================================================================

func (suite *StoreTestSuite) TestUpdateCertificateByReference_Success() {
	existingCert := &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "old-value",
	}
	updatedCert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKSURI,
		Value:   "new-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCertificateByReference,
		existingCert.RefType, existingCert.RefID, updatedCert.Type, updatedCert.Value, mock.Anything).
		Return(int64(1), nil)

	err := suite.store.UpdateCertificateByReference(context.Background(), existingCert, updatedCert)

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestUpdateCertificateByReference_NoRowsAffected() {
	existingCert := &Certificate{
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
	}
	updatedCert := &Certificate{
		Type:  CertificateTypeJWKS,
		Value: "new-value",
	}

	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCertificateByReference,
		existingCert.RefType, existingCert.RefID, updatedCert.Type, updatedCert.Value, mock.Anything).
		Return(int64(0), nil)

	err := suite.store.UpdateCertificateByReference(context.Background(), existingCert, updatedCert)

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no rows affected")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

// ============================================================================
// DeleteCertificateByID Tests
// ============================================================================

func (suite *StoreTestSuite) TestDeleteCertificateByID_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteCertificateByID, "test-cert-id", mock.Anything).
		Return(int64(1), nil)

	err := suite.store.DeleteCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestDeleteCertificateByID_DBProviderError() {
	suite.mockDBProvider.On("GetConfigDBClient").
		Return(nil, errors.New("db provider error"))

	err := suite.store.DeleteCertificateByID(context.Background(), "test-id")

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestDeleteCertificateByID_ExecuteError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteCertificateByID, "test-id", mock.Anything).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.DeleteCertificateByID(context.Background(), "test-id")

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to execute delete query")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestDeleteCertificateByID_NoRowsAffected() {
	// Delete operations should not fail even if no rows are affected
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteCertificateByID, "non-existent", mock.Anything).
		Return(int64(0), nil)

	err := suite.store.DeleteCertificateByID(context.Background(), "non-existent")

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

// ============================================================================
// DeleteCertificateByReference Tests
// ============================================================================

func (suite *StoreTestSuite) TestDeleteCertificateByReference_Success() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteCertificateByReference,
		CertificateReferenceTypeApplication, "test-app-id", mock.Anything).
		Return(int64(1), nil)

	err := suite.store.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestDeleteCertificateByReference_ExecuteError() {
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteCertificateByReference,
		CertificateReferenceTypeIDP, "test-id", mock.Anything).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "test-id")

	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to execute delete query")
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *StoreTestSuite) TestDeleteCertificateByReference_NoRowsAffected() {
	// Delete operations should not fail even if no rows are affected
	suite.mockDBProvider.On("GetConfigDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteCertificateByReference,
		CertificateReferenceTypeApplication, "non-existent", mock.Anything).
		Return(int64(0), nil)

	err := suite.store.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "non-existent")

	assert.Nil(suite.T(), err)
	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}
