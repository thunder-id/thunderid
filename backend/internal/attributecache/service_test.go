// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributecache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

// AttributeCacheServiceTestSuite is the test suite for the attribute cache service.
type AttributeCacheServiceTestSuite struct {
	suite.Suite
	service        AttributeCacheServiceInterface
	mockStore      *attributeCacheStoreInterfaceMock
	mockCrypto     *cryptomock.RuntimeCryptoProviderMock
	ctx            context.Context
	testID         string
	testAttributes map[string]interface{}
}

func TestAttributeCacheServiceSuite(t *testing.T) {
	suite.Run(t, new(AttributeCacheServiceTestSuite))
}

func (suite *AttributeCacheServiceTestSuite) SetupTest() {
	suite.mockStore = newAttributeCacheStoreInterfaceMock(suite.T())
	suite.mockCrypto = cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	// Default service has encryption disabled; encryption-enabled cases build their own instance.
	suite.service = newAttributeCacheService(suite.mockStore, suite.mockCrypto, false)
	suite.ctx = context.Background()

	suite.testID = "test-cache-id"
	suite.testAttributes = map[string]interface{}{"key": "value"}
}

// nonEmptyID matches any generated (non-empty) cache ID.
func nonEmptyID() interface{} {
	return mock.MatchedBy(func(id string) bool { return id != "" })
}

// Tests for CreateAttributeCache

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_Success() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"user": "john", "role": "admin"},
		TTLSeconds: 3600,
	}
	expectedData, _ := json.Marshal(cache.Attributes)

	suite.mockStore.On("CreateAttributeCache", suite.ctx, nonEmptyID(), expectedData, int64(3600)).
		Return(nil).Once()

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.NotEmpty(suite.T(), result.ID, "ID should be generated")
	assert.Equal(suite.T(), cache.Attributes, result.Attributes)
	assert.Equal(suite.T(), cache.TTLSeconds, result.TTLSeconds)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_NilCache() {
	result, err := suite.service.CreateAttributeCache(suite.ctx, nil)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidRequestFormat.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_MissingAttributes() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{}, // Empty attributes
		TTLSeconds: 3600,
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingAttributes.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_ZeroTTL() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 0, // Zero TTL
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_NegativeTTL() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: -100,
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

// TestCreateAttributeCache_MarshalError verifies attributes that cannot be JSON-encoded surface an
// internal error before the store is touched.
func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_MarshalError() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"bad": make(chan int)},
		TTLSeconds: 3600,
	}

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_StoreError() {
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	suite.mockStore.On("CreateAttributeCache", suite.ctx, nonEmptyID(), mock.Anything, int64(3600)).
		Return(errors.New("database error")).Once()

	result, err := suite.service.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

// TestCreateAttributeCache_Encrypted verifies that when encryption is enabled the attributes are
// encrypted and the ciphertext (not the plaintext JSON) is what is handed to the store.
func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_Encrypted() {
	encService := newAttributeCacheService(suite.mockStore, suite.mockCrypto, true)
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}
	plaintext, _ := json.Marshal(cache.Attributes)
	ciphertext := []byte("encrypted-blob")

	suite.mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, plaintext).
		Return(ciphertext, nil, nil).Once()
	suite.mockStore.On("CreateAttributeCache", suite.ctx, nonEmptyID(), ciphertext, int64(3600)).
		Return(nil).Once()

	result, err := encService.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
}

// TestCreateAttributeCache_EncryptError verifies an encryption failure surfaces an internal error
// and the store is never written.
func (suite *AttributeCacheServiceTestSuite) TestCreateAttributeCache_EncryptError() {
	encService := newAttributeCacheService(suite.mockStore, suite.mockCrypto, true)
	cache := &AttributeCache{
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600,
	}

	suite.mockCrypto.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("encrypt failed")).Once()

	result, err := encService.CreateAttributeCache(suite.ctx, cache)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

// Tests for GetAttributeCache

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_Success() {
	data, _ := json.Marshal(suite.testAttributes)
	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).
		Return(data, nil).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), suite.testID, result.ID)
	assert.Equal(suite.T(), suite.testAttributes, result.Attributes)
}

func (suite *AttributeCacheServiceTestSuite) TestAttributeCache_SessionGrantsRoundTrip() {
	grants := &SessionGrants{GroupIDs: []string{"g1"}, RoleIDs: []string{"r1"}}
	var stored []byte
	suite.mockStore.On("CreateAttributeCache", suite.ctx, nonEmptyID(), mock.Anything, int64(3600)).
		Run(func(args mock.Arguments) { stored = args.Get(2).([]byte) }).Return(nil).Once()

	_, err := suite.service.CreateAttributeCache(suite.ctx, &AttributeCache{
		Attributes:    map[string]interface{}{"email": "a@example.com", sessionGrantsKey: "forged"},
		TTLSeconds:    3600,
		SessionGrants: grants,
	})
	suite.Require().Nil(err)

	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).Return(stored, nil).Once()
	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testID)

	suite.Require().Nil(err)
	assert.Equal(suite.T(), map[string]interface{}{"email": "a@example.com"}, result.Attributes)
	assert.Equal(suite.T(), grants, result.SessionGrants)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_DropsForgedSessionGrantsAttribute() {
	suite.mockStore.On("CreateAttributeCache", suite.ctx, nonEmptyID(),
		[]byte(`{"email":"a@example.com"}`), int64(3600)).Return(nil).Once()

	_, err := suite.service.CreateAttributeCache(suite.ctx, &AttributeCache{
		Attributes: map[string]interface{}{"email": "a@example.com", sessionGrantsKey: "forged"},
		TTLSeconds: 3600,
	})

	suite.Nil(err)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_InvalidSessionGrants() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).
		Return([]byte(`{"__session_grants":"not an object"}`), nil).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_EmptyID() {
	result, err := suite.service.GetAttributeCache(suite.ctx, "")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_WhitespaceID() {
	result, err := suite.service.GetAttributeCache(suite.ctx, "   ")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_NotFound() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, "non-existent-id").
		Return(nil, errAttributeCacheNotFound).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, "non-existent-id")

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorAttributeCacheNotFound.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_StoreError() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).
		Return(nil, errors.New("database error")).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

// TestGetAttributeCache_UnmarshalError verifies a payload that cannot be deserialized surfaces an
// internal error rather than returning empty attributes.
func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_UnmarshalError() {
	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).
		Return([]byte("not-json"), nil).Once()

	result, err := suite.service.GetAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

// TestGetAttributeCache_Encrypted verifies that when encryption is enabled the stored ciphertext is
// decrypted before being deserialized back into the attribute map.
func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_Encrypted() {
	encService := newAttributeCacheService(suite.mockStore, suite.mockCrypto, true)
	ciphertext := []byte("encrypted-blob")
	plaintext, _ := json.Marshal(suite.testAttributes)

	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).
		Return(ciphertext, nil).Once()
	suite.mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, ciphertext).
		Return(plaintext, nil).Once()

	result, err := encService.GetAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), suite.testAttributes, result.Attributes)
}

// TestGetAttributeCache_DecryptError verifies a decryption failure surfaces an internal error.
func (suite *AttributeCacheServiceTestSuite) TestGetAttributeCache_DecryptError() {
	encService := newAttributeCacheService(suite.mockStore, suite.mockCrypto, true)
	ciphertext := []byte("encrypted-blob")

	suite.mockStore.On("GetAttributeCache", suite.ctx, suite.testID).
		Return(ciphertext, nil).Once()
	suite.mockCrypto.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, ciphertext).
		Return(nil, errors.New("decrypt failed")).Once()

	result, err := encService.GetAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), result)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

// Tests for ExtendAttributeCacheTTL

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_Success() {
	newTTL := 7200 // 2 hours

	suite.mockStore.On("ExtendAttributeCacheTTL", suite.ctx, suite.testID, newTTL).
		Return(nil).Once()

	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testID, newTTL)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_EmptyID() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, "", 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_WhitespaceID() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, "  ", 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_ZeroTTL() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testID, 0)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_NegativeTTL() {
	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testID, -100)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorInvalidExpiryTime.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_NotFound() {
	suite.mockStore.On("ExtendAttributeCacheTTL", suite.ctx, "non-existent-id", 3600).
		Return(providers.ErrRuntimeStoreKeyNotFound).Once()

	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, "non-existent-id", 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorAttributeCacheNotFound.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestExtendAttributeCacheTTL_StoreUpdateError() {
	suite.mockStore.On("ExtendAttributeCacheTTL", suite.ctx, suite.testID, 3600).
		Return(errors.New("database error")).Once()

	err := suite.service.ExtendAttributeCacheTTL(suite.ctx, suite.testID, 3600)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}

// Tests for DeleteAttributeCache

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_Success() {
	suite.mockStore.On("DeleteAttributeCache", suite.ctx, suite.testID).
		Return(nil).Once()

	err := suite.service.DeleteAttributeCache(suite.ctx, suite.testID)

	assert.Nil(suite.T(), err)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_EmptyID() {
	err := suite.service.DeleteAttributeCache(suite.ctx, "")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_WhitespaceID() {
	err := suite.service.DeleteAttributeCache(suite.ctx, "   ")

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorMissingCacheID.Code, err.Code)
}

func (suite *AttributeCacheServiceTestSuite) TestDeleteAttributeCache_StoreError() {
	suite.mockStore.On("DeleteAttributeCache", suite.ctx, suite.testID).
		Return(errors.New("database error")).Once()

	err := suite.service.DeleteAttributeCache(suite.ctx, suite.testID)

	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), tidcommon.InternalServerError.Code, err.Code)
}
