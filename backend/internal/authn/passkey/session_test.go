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

package passkey

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type SessionUtilsTestSuite struct {
	suite.Suite
}

func TestSessionUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(SessionUtilsTestSuite))
}

func (suite *SessionUtilsTestSuite) TestGenerateSessionKey() {
	key1, err1 := generateSessionKey()
	key2, err2 := generateSessionKey()

	suite.Nil(err1)
	suite.Nil(err2)
	suite.NotEmpty(key1)
	suite.NotEmpty(key2)
	suite.NotEqual(key1, key2, "Session keys should be unique")
	suite.Greater(len(key1), 32, "Session key should be base64 encoded and longer than 32 chars")
}

func (suite *SessionUtilsTestSuite) TestGenerateSessionKey_Success() {
	// Generate multiple keys to verify uniqueness
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := generateSessionKey()
		suite.NoError(err)
		suite.NotEmpty(key)

		// Verify uniqueness
		suite.False(keys[key], "Generated duplicate key")
		keys[key] = true

		// Verify base64 encoding (should be 44 chars for 32 bytes)
		suite.Equal(44, len(key), "Base64 encoded 32 bytes should be 44 chars")
	}
}

type SessionServiceTestSuite struct {
	suite.Suite
	mockSessionStore *sessionStoreInterfaceMock
	service          *passkeyService
}

func TestSessionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(SessionServiceTestSuite))
}

func (suite *SessionServiceTestSuite) SetupTest() {
	suite.mockSessionStore = newSessionStoreInterfaceMock(suite.T())
	suite.service = &passkeyService{
		sessionStore: suite.mockSessionStore,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

func (suite *SessionServiceTestSuite) TestStoreSessionData_Success() {
	// Tests the happy path of storeSessionData
	sessionData := &sessionData{
		Challenge: "dGVzdC1jaGFsbGVuZ2U", // base64 encoded "test-challenge"
	}

	// Mock successful session storage
	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"), // sessionKey (random)
		sessionData,
		mock.AnythingOfType("int64"), // expirySeconds
	).Return(nil).Once()

	sessionKey, err := suite.service.storeSessionData(sessionData)

	suite.Nil(err)
	suite.NotEmpty(sessionKey)
	suite.Greater(len(sessionKey), 32)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestStoreSessionData_StoreSessionError() {
	sessionData := &sessionData{
		Challenge: "dGVzdC1jaGFsbGVuZ2U",
	}

	// Mock session storage failure
	storeError := errors.New("database error")
	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		sessionData,
		mock.AnythingOfType("int64"),
	).Return(storeError).Once()

	sessionKey, err := suite.service.storeSessionData(sessionData)

	suite.NotNil(err)
	suite.Equal(&serviceerror.InternalServerError, err)
	suite.Empty(sessionKey)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestStoreSessionData_ExpiryCalculation() {
	// Verify that expiry time is calculated correctly
	sessionData := &sessionData{
		Challenge: "dGVzdC1jaGFsbGVuZ2U",
	}

	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		sessionData,
		int64(sessionTTLSeconds),
	).Return(nil).Once()

	_, err := suite.service.storeSessionData(sessionData)

	suite.Nil(err)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestRetrieveSessionData_Success() {
	expectedSessionData := &sessionData{
		Challenge:      "dGVzdC1jaGFsbGVuZ2U",
		RelyingPartyID: testRelyingPartyID,
		UserID:         []byte(testUserID),
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(expectedSessionData, nil).Once()

	session, userID, relyingPartyID, err := suite.service.retrieveSessionData(testSessionToken)

	suite.Nil(err)
	suite.Equal(expectedSessionData, session)
	suite.Equal(testUserID, userID)
	suite.Equal(testRelyingPartyID, relyingPartyID)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestRetrieveSessionData_RetrieveError() {
	sessionKey := "invalid-session-key"
	retrieveError := errors.New("session not found")

	suite.mockSessionStore.On("retrieveSession", sessionKey).
		Return(nil, retrieveError).Once()

	session, userID, relyingPartyID, err := suite.service.retrieveSessionData(sessionKey)

	suite.NotNil(err)
	suite.Equal(&serviceerror.InternalServerError, err)
	suite.Nil(session)
	suite.Empty(userID)
	suite.Empty(relyingPartyID)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestRetrieveSessionData_NilSessionData() {
	// Mock returns nil sessionData even though no error
	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(nil, nil).Once()

	session, userID, relyingPartyID, err := suite.service.retrieveSessionData(testSessionToken)

	suite.NotNil(err)
	suite.Equal(&ErrorSessionExpired, err)
	suite.Nil(session)
	suite.Empty(userID)
	suite.Empty(relyingPartyID)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestRetrieveSessionData_ExpiredSession() {
	// Tests the scenario where session is expired (common case for L98)
	sessionKey := "expired-session-key"
	expiredError := errors.New("session expired")

	suite.mockSessionStore.On("retrieveSession", sessionKey).
		Return(nil, expiredError).Once()

	session, userID, relyingPartyID, err := suite.service.retrieveSessionData(sessionKey)

	suite.NotNil(err)
	suite.Equal(&serviceerror.InternalServerError, err)
	suite.Nil(session)
	suite.Empty(userID)
	suite.Empty(relyingPartyID)
	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestClearSessionData_Success() {
	suite.mockSessionStore.On("deleteSession", testSessionToken).
		Return(nil).Once()

	// Should not panic or return error
	suite.service.clearSessionData(testSessionToken)

	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestClearSessionData_DeleteError() {
	// Tests that errors from deleteSession are ignored (L111: _ = ...)
	deleteError := errors.New("delete failed")

	suite.mockSessionStore.On("deleteSession", testSessionToken).
		Return(deleteError).Once()

	// Should not panic even if delete fails
	suite.service.clearSessionData(testSessionToken)

	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestClearSessionData_EmptyKey() {
	sessionKey := ""

	suite.mockSessionStore.On("deleteSession", sessionKey).
		Return(nil).Once()

	// Should handle empty key gracefully
	suite.service.clearSessionData(sessionKey)

	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionServiceTestSuite) TestSessionRoundTrip() {
	sessionData := &sessionData{
		Challenge:      "dGVzdC1jaGFsbGVuZ2U",
		RelyingPartyID: testRelyingPartyID,
		UserID:         []byte(testUserID),
	}

	var capturedSessionKey string

	// Mock store
	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		sessionData,
		mock.AnythingOfType("int64"),
	).Run(func(args mock.Arguments) {
		capturedSessionKey = args.Get(0).(string)
	}).Return(nil).Once()

	// Mock retrieve with captured key
	suite.mockSessionStore.On("retrieveSession", mock.MatchedBy(func(key string) bool {
		return key == capturedSessionKey
	})).Return(sessionData, nil).Once()

	// Mock delete
	suite.mockSessionStore.On("deleteSession", mock.MatchedBy(func(key string) bool {
		return key == capturedSessionKey
	})).Return(nil).Once()

	// Store
	sessionKey, err := suite.service.storeSessionData(sessionData)
	suite.Nil(err)
	suite.NotEmpty(sessionKey)

	// Retrieve
	retrievedData, retrievedUserID, retrievedRPID, err := suite.service.retrieveSessionData(sessionKey)
	suite.Nil(err)
	suite.Equal(sessionData, retrievedData)
	suite.Equal(testUserID, retrievedUserID)
	suite.Equal(testRelyingPartyID, retrievedRPID)

	// Clear
	suite.service.clearSessionData(sessionKey)

	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *SessionUtilsTestSuite) TestSessionConstants() {
	// Verify session constants are reasonable
	suite.Equal(32, sessionKeyLength, "Session key should be 32 bytes")
	suite.Equal(120, sessionTTLSeconds, "Session TTL should be 120 seconds (2 minutes)")
}
