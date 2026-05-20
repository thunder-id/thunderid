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

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/tests/mocks/cachemock"
)

type CacheBackedStoreTestSuite struct {
	suite.Suite
	mockStore          *certificateStoreInterfaceMock
	mockCertByIDCache  *cachemock.CacheInterfaceMock[*Certificate]
	mockCertByRefCache *cachemock.CacheInterfaceMock[*Certificate]
	cacheBackedStore   *cacheBackedStore
}

func TestCacheBackedStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CacheBackedStoreTestSuite))
}

func (suite *CacheBackedStoreTestSuite) SetupTest() {
	suite.mockStore = newCertificateStoreInterfaceMock(suite.T())
	suite.mockCertByIDCache = cachemock.NewCacheInterfaceMock[*Certificate](suite.T())
	suite.mockCertByRefCache = cachemock.NewCacheInterfaceMock[*Certificate](suite.T())

	suite.cacheBackedStore = &cacheBackedStore{
		certByIDCache:        suite.mockCertByIDCache,
		certByReferenceCache: suite.mockCertByRefCache,
		store:                suite.mockStore,
	}
}

// Helper function to create a test certificate
func (suite *CacheBackedStoreTestSuite) createTestCertificate() *Certificate {
	return &Certificate{
		ID:      "test-cert-id",
		RefType: CertificateReferenceTypeApplication,
		RefID:   "test-app-id",
		Type:    CertificateTypeJWKS,
		Value:   "test-certificate-value",
	}
}

// ============================================================================
// GetCertificateByID Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByID_CacheHit() {
	cert := suite.createTestCertificate()
	cacheKey := cache.CacheKey{Key: "test-cert-id"}

	suite.mockCertByIDCache.On("Get", mock.Anything, cacheKey).Return(cert, true)

	result, err := suite.cacheBackedStore.GetCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), cert.ID, result.ID)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	// Store should not be called on cache hit
	suite.mockStore.AssertNotCalled(suite.T(), "GetCertificateByID")
}

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByID_CacheMiss_Success() {
	cert := suite.createTestCertificate()
	cacheKey := cache.CacheKey{Key: "test-cert-id"}
	refCacheKey := getCertByReferenceCacheKey(cert.RefType, cert.RefID)

	suite.mockCertByIDCache.On("Get", mock.Anything, cacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").Return(cert, nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, cacheKey, cert).Return(nil)
	suite.mockCertByRefCache.On("Set", mock.Anything, refCacheKey, cert).Return(nil)

	result, err := suite.cacheBackedStore.GetCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), cert.ID, result.ID)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByID_CacheMiss_StoreError() {
	cacheKey := cache.CacheKey{Key: "test-id"}

	suite.mockCertByIDCache.On("Get", mock.Anything, cacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").
		Return(nil, errors.New("store error"))

	result, err := suite.cacheBackedStore.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByID_CacheMiss_NilResult() {
	cacheKey := cache.CacheKey{Key: "test-id"}

	suite.mockCertByIDCache.On("Get", mock.Anything, cacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").Return(nil, nil)

	result, err := suite.cacheBackedStore.GetCertificateByID(context.Background(), "test-id")

	assert.Nil(suite.T(), result)
	assert.Nil(suite.T(), err)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByID_CacheMiss_CacheSetError() {
	cert := suite.createTestCertificate()
	cacheKey := cache.CacheKey{Key: "test-cert-id"}
	refCacheKey := getCertByReferenceCacheKey(cert.RefType, cert.RefID)

	suite.mockCertByIDCache.On("Get", mock.Anything, cacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").Return(cert, nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, cacheKey, cert).
		Return(errors.New("cache error"))
	suite.mockCertByRefCache.On("Set", mock.Anything, refCacheKey, cert).Return(nil)

	result, err := suite.cacheBackedStore.GetCertificateByID(context.Background(), "test-cert-id")

	// Should still return the certificate even if caching fails
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// GetCertificateByReference Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByReference_CacheHit() {
	cert := suite.createTestCertificate()
	cacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeApplication, "test-app-id")

	suite.mockCertByRefCache.On("Get", mock.Anything, cacheKey).Return(cert, true)

	result, err := suite.cacheBackedStore.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), cert.ID, result.ID)
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockStore.AssertNotCalled(suite.T(), "GetCertificateByReference")
}

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByReference_CacheMiss_Success() {
	cert := suite.createTestCertificate()
	cacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeApplication, "test-app-id")
	idCacheKey := cache.CacheKey{Key: cert.ID}

	suite.mockCertByRefCache.On("Get", mock.Anything, cacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").Return(cert, nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, idCacheKey, cert).Return(nil)
	suite.mockCertByRefCache.On("Set", mock.Anything, cacheKey, cert).Return(nil)

	result, err := suite.cacheBackedStore.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), cert.ID, result.ID)
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestGetCertificateByReference_CacheMiss_StoreError() {
	cacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeIDP, "test-id")

	suite.mockCertByRefCache.On("Get", mock.Anything, cacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeIDP, "test-id").
		Return(nil, errors.New("store error"))

	result, err := suite.cacheBackedStore.GetCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "test-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// CreateCertificate Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestCreateCertificate_Success() {
	cert := suite.createTestCertificate()
	idCacheKey := cache.CacheKey{Key: cert.ID}
	refCacheKey := getCertByReferenceCacheKey(cert.RefType, cert.RefID)

	suite.mockStore.On("CreateCertificate", mock.Anything, cert).Return(nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, idCacheKey, cert).Return(nil)
	suite.mockCertByRefCache.On("Set", mock.Anything, refCacheKey, cert).Return(nil)

	err := suite.cacheBackedStore.CreateCertificate(context.Background(), cert)

	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockCertByRefCache.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestCreateCertificate_StoreError() {
	cert := suite.createTestCertificate()

	suite.mockStore.On("CreateCertificate", mock.Anything, cert).
		Return(errors.New("store error"))

	err := suite.cacheBackedStore.CreateCertificate(context.Background(), cert)

	assert.NotNil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	// Cache should not be called if store fails
	suite.mockCertByIDCache.AssertNotCalled(suite.T(), "Set")
	suite.mockCertByRefCache.AssertNotCalled(suite.T(), "Set")
}

func (suite *CacheBackedStoreTestSuite) TestCreateCertificate_CacheSetError() {
	cert := suite.createTestCertificate()
	idCacheKey := cache.CacheKey{Key: cert.ID}
	refCacheKey := getCertByReferenceCacheKey(cert.RefType, cert.RefID)

	suite.mockStore.On("CreateCertificate", mock.Anything, cert).Return(nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, idCacheKey, cert).
		Return(errors.New("cache error"))
	suite.mockCertByRefCache.On("Set", mock.Anything, refCacheKey, cert).Return(nil)

	err := suite.cacheBackedStore.CreateCertificate(context.Background(), cert)

	// Should succeed even if caching fails
	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// UpdateCertificateByID Tests
// ============================================================================

// Helper function to test successful update operations
func (suite *CacheBackedStoreTestSuite) testSuccessfulUpdate(
	updateFunc func(context.Context, *Certificate, *Certificate) error,
	mockStoreCall func(*Certificate, *Certificate),
) {
	existingCert := suite.createTestCertificate()
	updatedCert := &Certificate{
		ID:      existingCert.ID,
		RefType: existingCert.RefType,
		RefID:   existingCert.RefID,
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-value",
	}
	idCacheKey := cache.CacheKey{Key: existingCert.ID}
	refCacheKey := getCertByReferenceCacheKey(existingCert.RefType, existingCert.RefID)

	mockStoreCall(existingCert, updatedCert)
	suite.mockCertByIDCache.On("Delete", mock.Anything, idCacheKey).Return(nil)
	suite.mockCertByRefCache.On("Delete", mock.Anything, refCacheKey).Return(nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, idCacheKey, updatedCert).Return(nil)
	suite.mockCertByRefCache.On("Set", mock.Anything, refCacheKey, updatedCert).Return(nil)

	err := updateFunc(context.Background(), existingCert, updatedCert)

	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockCertByRefCache.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestUpdateCertificateByID_Success() {
	suite.testSuccessfulUpdate(
		suite.cacheBackedStore.UpdateCertificateByID,
		func(existing, updated *Certificate) {
			suite.mockStore.On("UpdateCertificateByID", mock.Anything, existing, updated).Return(nil)
		},
	)
}

func (suite *CacheBackedStoreTestSuite) TestUpdateCertificateByID_StoreError() {
	existingCert := suite.createTestCertificate()
	updatedCert := &Certificate{
		ID:      existingCert.ID,
		RefType: existingCert.RefType,
		RefID:   existingCert.RefID,
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-value",
	}

	suite.mockStore.On("UpdateCertificateByID", mock.Anything, existingCert, updatedCert).
		Return(errors.New("store error"))

	err := suite.cacheBackedStore.UpdateCertificateByID(context.Background(), existingCert, updatedCert)

	assert.NotNil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	// Cache operations should not be called if store fails
	suite.mockCertByIDCache.AssertNotCalled(suite.T(), "Delete")
	suite.mockCertByRefCache.AssertNotCalled(suite.T(), "Delete")
}

func (suite *CacheBackedStoreTestSuite) TestUpdateCertificateByID_CacheInvalidateError() {
	existingCert := suite.createTestCertificate()
	updatedCert := &Certificate{
		ID:      existingCert.ID,
		RefType: existingCert.RefType,
		RefID:   existingCert.RefID,
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-value",
	}
	idCacheKey := cache.CacheKey{Key: existingCert.ID}
	refCacheKey := getCertByReferenceCacheKey(existingCert.RefType, existingCert.RefID)

	suite.mockStore.On("UpdateCertificateByID", mock.Anything, existingCert, updatedCert).Return(nil)
	suite.mockCertByIDCache.On("Delete", mock.Anything, idCacheKey).
		Return(errors.New("cache error"))
	suite.mockCertByRefCache.On("Delete", mock.Anything, refCacheKey).Return(nil)
	suite.mockCertByIDCache.On("Set", mock.Anything, idCacheKey, updatedCert).Return(nil)
	suite.mockCertByRefCache.On("Set", mock.Anything, refCacheKey, updatedCert).Return(nil)

	err := suite.cacheBackedStore.UpdateCertificateByID(context.Background(), existingCert, updatedCert)

	// Should succeed even if cache operations fail
	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// UpdateCertificateByReference Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestUpdateCertificateByReference_Success() {
	suite.testSuccessfulUpdate(
		suite.cacheBackedStore.UpdateCertificateByReference,
		func(existing, updated *Certificate) {
			suite.mockStore.On("UpdateCertificateByReference", mock.Anything, existing, updated).Return(nil)
		},
	)
}

func (suite *CacheBackedStoreTestSuite) TestUpdateCertificateByReference_StoreError() {
	existingCert := suite.createTestCertificate()
	updatedCert := &Certificate{
		ID:      existingCert.ID,
		RefType: existingCert.RefType,
		RefID:   existingCert.RefID,
		Type:    CertificateTypeJWKSURI,
		Value:   "updated-value",
	}

	suite.mockStore.On("UpdateCertificateByReference", mock.Anything, existingCert, updatedCert).
		Return(errors.New("store error"))

	err := suite.cacheBackedStore.UpdateCertificateByReference(context.Background(), existingCert, updatedCert)

	assert.NotNil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// DeleteCertificateByID Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByID_CacheHit() {
	cert := suite.createTestCertificate()
	idCacheKey := cache.CacheKey{Key: "test-cert-id"}
	refCacheKey := getCertByReferenceCacheKey(cert.RefType, cert.RefID)

	suite.mockCertByIDCache.On("Get", mock.Anything, idCacheKey).Return(cert, true)
	suite.mockStore.On("DeleteCertificateByID", mock.Anything, "test-cert-id").Return(nil)
	suite.mockCertByIDCache.On("Delete", mock.Anything, idCacheKey).Return(nil)
	suite.mockCertByRefCache.On("Delete", mock.Anything, refCacheKey).Return(nil)

	err := suite.cacheBackedStore.DeleteCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByID_CacheMiss_FetchFromStore() {
	cert := suite.createTestCertificate()
	idCacheKey := cache.CacheKey{Key: "test-cert-id"}
	refCacheKey := getCertByReferenceCacheKey(cert.RefType, cert.RefID)

	suite.mockCertByIDCache.On("Get", mock.Anything, idCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-cert-id").Return(cert, nil)
	suite.mockStore.On("DeleteCertificateByID", mock.Anything, "test-cert-id").Return(nil)
	suite.mockCertByIDCache.On("Delete", mock.Anything, idCacheKey).Return(nil)
	suite.mockCertByRefCache.On("Delete", mock.Anything, refCacheKey).Return(nil)

	err := suite.cacheBackedStore.DeleteCertificateByID(context.Background(), "test-cert-id")

	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockCertByRefCache.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByID_CertNotFound() {
	idCacheKey := cache.CacheKey{Key: "non-existent"}

	suite.mockCertByIDCache.On("Get", mock.Anything, idCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "non-existent").
		Return(nil, ErrCertificateNotFound)

	err := suite.cacheBackedStore.DeleteCertificateByID(context.Background(), "non-existent")

	// Should return nil (no error) when certificate not found
	assert.Nil(suite.T(), err)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockStore.AssertNotCalled(suite.T(), "DeleteCertificateByID")
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByID_GetError() {
	idCacheKey := cache.CacheKey{Key: "test-id"}

	suite.mockCertByIDCache.On("Get", mock.Anything, idCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").
		Return(nil, errors.New("store error"))

	err := suite.cacheBackedStore.DeleteCertificateByID(context.Background(), "test-id")

	assert.NotNil(suite.T(), err)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByID_NilCert() {
	idCacheKey := cache.CacheKey{Key: "test-id"}

	suite.mockCertByIDCache.On("Get", mock.Anything, idCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByID", mock.Anything, "test-id").Return(nil, nil)

	err := suite.cacheBackedStore.DeleteCertificateByID(context.Background(), "test-id")

	// Should return nil when certificate is nil
	assert.Nil(suite.T(), err)
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByID_StoreDeleteError() {
	cert := suite.createTestCertificate()
	idCacheKey := cache.CacheKey{Key: "test-cert-id"}

	suite.mockCertByIDCache.On("Get", mock.Anything, idCacheKey).Return(cert, true)
	suite.mockStore.On("DeleteCertificateByID", mock.Anything, "test-cert-id").
		Return(errors.New("delete error"))

	err := suite.cacheBackedStore.DeleteCertificateByID(context.Background(), "test-cert-id")

	assert.NotNil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// DeleteCertificateByReference Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByReference_CacheHit() {
	cert := suite.createTestCertificate()
	refCacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeApplication, "test-app-id")
	idCacheKey := cache.CacheKey{Key: cert.ID}

	suite.mockCertByRefCache.On("Get", mock.Anything, refCacheKey).Return(cert, true)
	suite.mockStore.On("DeleteCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").Return(nil)
	suite.mockCertByIDCache.On("Delete", mock.Anything, idCacheKey).Return(nil)
	suite.mockCertByRefCache.On("Delete", mock.Anything, refCacheKey).Return(nil)

	err := suite.cacheBackedStore.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByReference_CacheMiss_FetchFromStore() {
	cert := suite.createTestCertificate()
	refCacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeApplication, "test-app-id")
	idCacheKey := cache.CacheKey{Key: cert.ID}

	suite.mockCertByRefCache.On("Get", mock.Anything, refCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").Return(cert, nil)
	suite.mockStore.On("DeleteCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-app-id").Return(nil)
	suite.mockCertByIDCache.On("Delete", mock.Anything, idCacheKey).Return(nil)
	suite.mockCertByRefCache.On("Delete", mock.Anything, refCacheKey).Return(nil)

	err := suite.cacheBackedStore.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-app-id")

	assert.Nil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
	suite.mockCertByIDCache.AssertExpectations(suite.T())
	suite.mockCertByRefCache.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByReference_CertNotFound() {
	refCacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeIDP, "non-existent")

	suite.mockCertByRefCache.On("Get", mock.Anything, refCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeIDP, "non-existent").
		Return(nil, ErrCertificateNotFound)

	err := suite.cacheBackedStore.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeIDP, "non-existent")

	// Should return nil when certificate not found
	assert.Nil(suite.T(), err)
	suite.mockCertByRefCache.AssertExpectations(suite.T())
	suite.mockStore.AssertExpectations(suite.T())
}

func (suite *CacheBackedStoreTestSuite) TestDeleteCertificateByReference_GetError() {
	refCacheKey := getCertByReferenceCacheKey(CertificateReferenceTypeApplication, "test-id")

	suite.mockCertByRefCache.On("Get", mock.Anything, refCacheKey).Return(nil, false)
	suite.mockStore.On("GetCertificateByReference", mock.Anything,
		CertificateReferenceTypeApplication, "test-id").
		Return(nil, errors.New("store error"))

	err := suite.cacheBackedStore.DeleteCertificateByReference(context.Background(),
		CertificateReferenceTypeApplication, "test-id")

	assert.NotNil(suite.T(), err)
	suite.mockStore.AssertExpectations(suite.T())
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func (suite *CacheBackedStoreTestSuite) TestGetCertByReferenceCacheKey() {
	testCases := []struct {
		name     string
		refType  CertificateReferenceType
		refID    string
		expected string
	}{
		{
			name:     "Application reference",
			refType:  CertificateReferenceTypeApplication,
			refID:    "app-123",
			expected: "APPLICATION:app-123",
		},
		{
			name:     "IDP reference",
			refType:  CertificateReferenceTypeIDP,
			refID:    "idp-456",
			expected: "IDP:idp-456",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := getCertByReferenceCacheKey(tc.refType, tc.refID)
			assert.Equal(t, tc.expected, result.Key)
		})
	}
}
