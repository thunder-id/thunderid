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
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

const (
	testUserID         = "user123"
	testRelyingPartyID = "example.com"
	testSessionToken   = "session_token_123"
	//nolint:gosec // Token type identifier, not a credential
	testCredentialID = "credential_123"
	testSessionKey   = "test-session-key"
)

type WebAuthnServiceTestSuite struct {
	suite.Suite
	mockEntityService *entitymock.EntityServiceInterfaceMock
	mockSessionStore  *sessionStoreInterfaceMock
	service           *passkeyService
}

func TestWebAuthnServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WebAuthnServiceTestSuite))
}

func (suite *WebAuthnServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
			Audience:       "application",
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *WebAuthnServiceTestSuite) SetupTest() {
	suite.mockEntityService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.mockSessionStore = newSessionStoreInterfaceMock(suite.T())

	suite.service = &passkeyService{
		entityService: suite.mockEntityService,
		sessionStore:  suite.mockSessionStore,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_NilRequest() {
	result, svcErr := suite.service.StartRegistration(context.Background(), nil)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_EmptyUserID() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
	}

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptyUserIdentifier.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_EmptyRelyingPartyID() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "",
	}

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptyRelyingPartyID.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_UserNotFound() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorUserNotFound.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_UserServiceServerError() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(nil, assert.AnError).Once()

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_GetCredentialsError() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, assert.AnError).Once()

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_NilRequest() {
	result, svcErr := suite.service.FinishRegistration(context.Background(), nil)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_EmptySessionToken() {
	req := &PasskeyRegistrationFinishRequest{
		SessionToken:      "",
		CredentialID:      testCredentialID,
		ClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
		AttestationObject: "o2NmbXRkbm9uZQ",
	}

	result, svcErr := suite.service.FinishRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptySessionToken.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_EmptyCredentialID() {
	req := &PasskeyRegistrationFinishRequest{
		SessionToken:      testSessionToken,
		CredentialID:      "",
		ClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
		AttestationObject: "o2NmbXRkbm9uZQ",
	}

	result, svcErr := suite.service.FinishRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_EmptyClientDataJSON() {
	req := &PasskeyRegistrationFinishRequest{
		SessionToken:      testSessionToken,
		CredentialID:      testCredentialID,
		ClientDataJSON:    "",
		AttestationObject: "o2NmbXRkbm9uZQ",
	}

	result, svcErr := suite.service.FinishRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_EmptyAttestationObject() {
	req := &PasskeyRegistrationFinishRequest{
		SessionToken:      testSessionToken,
		CredentialID:      testCredentialID,
		ClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
		AttestationObject: "",
	}

	result, svcErr := suite.service.FinishRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_NilRequest() {
	result, svcErr := suite.service.StartAuthentication(context.Background(), nil)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_EmptyUserID() {
	req := &PasskeyAuthenticationStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
	}

	// Mock session store for usernameless flow (empty userID)
	suite.mockSessionStore.On("storeSession", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	// Usernameless flow should succeed
	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.NotEmpty(result.SessionToken)
	suite.NotEmpty(result.PublicKeyCredentialRequestOptions.Challenge)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_EmptyRelyingPartyID() {
	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "",
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptyRelyingPartyID.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_UserNotFound() {
	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(nil, entity.ErrEntityNotFound).Once()

	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorUserNotFound.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_UserServiceServerError() {
	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(nil, assert.AnError).Once()

	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_GetCredentialsError() {
	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, assert.AnError).Once()

	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_NoCredentialsFound() {
	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
		OUID:     "org123",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, nil).Once()

	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorNoCredentialsFound.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_NilRequest() {
	result, svcErr := suite.service.FinishAuthentication(context.Background(), nil)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidFinishData.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_EmptyCredentialID() {
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      "",
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptyCredentialID.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_EmptyCredentialType() {
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptyCredentialType.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_EmptyClientDataJSON() {
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidAuthenticatorResponse.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_EmptyAuthenticatorData() {
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidAuthenticatorResponse.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_EmptySignature() {
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidAuthenticatorResponse.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_EmptySessionToken() {
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      "",
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorEmptySessionToken.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestGetStoredPasskeyCredentials_Success() {
	mockCredential := map[string]interface{}{
		"id":        []byte("credential123"),
		"publicKey": []byte("publickey123"),
		"aaguid":    []byte("aaguid123"),
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	entries, svcErr := suite.service.getStoredPasskeyEntries(context.Background(), testUserID)
	suite.Nil(svcErr)
	suite.Len(entries, 1)

	credentials := suite.service.decodePasskeyCredentials(testUserID, entries)
	suite.Len(credentials, 1)
}

func (suite *WebAuthnServiceTestSuite) TestGetStoredPasskeyCredentials_ServiceError() {
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, assert.AnError).Once()

	entries, svcErr := suite.service.getStoredPasskeyEntries(context.Background(), testUserID)

	suite.NotNil(svcErr)
	suite.Nil(entries)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestGetStoredPasskeyCredentials_NotFound() {
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, entity.ErrEntityNotFound).Once()

	entries, svcErr := suite.service.getStoredPasskeyEntries(context.Background(), testUserID)

	suite.NotNil(svcErr)
	suite.Nil(entries)
	suite.Equal(ErrorUserNotFound.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestGetStoredPasskeyCredentials_SkipsInvalidEntries() {
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{
			{Value: "{invalid json}"},
			{Value: ""},
		}, nil).Once()

	entries, svcErr := suite.service.getStoredPasskeyEntries(context.Background(), testUserID)
	suite.Nil(svcErr)
	suite.Len(entries, 2) // raw entries are returned as-is

	credentials := suite.service.decodePasskeyCredentials(testUserID, entries)
	suite.Len(credentials, 0) // both decode failures are skipped
}

func (suite *WebAuthnServiceTestSuite) TestGetStoredPasskeyCredentials_NoCredentials() {
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, nil).Once()

	entries, svcErr := suite.service.getStoredPasskeyEntries(context.Background(), testUserID)
	suite.Nil(svcErr)
	suite.Empty(entries)

	credentials := suite.service.decodePasskeyCredentials(testUserID, entries)
	suite.Empty(credentials)
}

func (suite *WebAuthnServiceTestSuite) TestStoreWebAuthnCredentialInDB_Success() {
	mockCredential := &webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			AAGUID:    []byte("aaguid123"),
			SignCount: 0,
		},
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, nil).Once()

	suite.mockEntityService.On("UpdateSystemCredentials", mock.Anything, testUserID, mock.MatchedBy(
		func(credentialsJSON json.RawMessage) bool {
			var credMap map[string][]entity.StoredCredential
			if err := json.Unmarshal(credentialsJSON, &credMap); err != nil {
				return false
			}
			creds, ok := credMap["passkey"]
			return ok && len(creds) == 1
		})).Return(nil).Once()

	err := suite.service.storePasskeyCredential(context.Background(), testUserID, mockCredential)

	suite.NoError(err)
}

func (suite *WebAuthnServiceTestSuite) TestStoreWebAuthnCredentialInDB_GetCredentialsError() {
	mockCredential := &webauthnCredential{
		ID: []byte("credential123"),
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, assert.AnError).Once()

	err := suite.service.storePasskeyCredential(context.Background(), testUserID, mockCredential)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load existing passkey credentials")
}

func (suite *WebAuthnServiceTestSuite) TestStoreWebAuthnCredentialInDB_UpdateCredentialsError() {
	mockCredential := &webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, nil).Once()

	suite.mockEntityService.On("UpdateSystemCredentials", mock.Anything, testUserID, mock.Anything).
		Return(assert.AnError).Once()

	err := suite.service.storePasskeyCredential(context.Background(), testUserID, mockCredential)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to update passkey credentials")
}

func (suite *WebAuthnServiceTestSuite) TestUpdateWebAuthnCredentialInDB_Success() {
	credentialID := []byte("credential123")
	existingCredential := webauthnCredential{
		ID:        credentialID,
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			SignCount: 5,
		},
	}
	existingCredJSON, _ := json.Marshal(existingCredential)

	updatedCredential := &webauthnCredential{
		ID:        credentialID,
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			SignCount: 6,
		},
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(existingCredJSON)}}, nil).Once()

	suite.mockEntityService.On("UpdateSystemCredentials", mock.Anything, testUserID, mock.MatchedBy(
		func(credentialsJSON json.RawMessage) bool {
			var credMap map[string][]entity.StoredCredential
			if err := json.Unmarshal(credentialsJSON, &credMap); err != nil {
				return false
			}
			creds, ok := credMap["passkey"]
			if !ok || len(creds) != 1 {
				return false
			}
			var cred webauthnCredential
			_ = json.Unmarshal([]byte(creds[0].Value), &cred)

			return cred.Authenticator.SignCount == 6
		})).Return(nil).Once()

	err := suite.service.updatePasskeyCredential(context.Background(), testUserID, updatedCredential)

	suite.NoError(err)
}

func (suite *WebAuthnServiceTestSuite) TestUpdateWebAuthnCredentialInDB_CredentialNotFound() {
	credentialID := []byte("credential123")
	updatedCredential := &webauthnCredential{
		ID:        credentialID,
		PublicKey: []byte("publickey123"),
	}

	differentCredential := webauthnCredential{
		ID:        []byte("different_id"),
		PublicKey: []byte("publickey456"),
	}
	existingCredJSON, _ := json.Marshal(differentCredential)

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(existingCredJSON)}}, nil).Once()

	err := suite.service.updatePasskeyCredential(context.Background(), testUserID, updatedCredential)

	suite.Error(err)
	suite.Contains(err.Error(), "credential not found for update")
}

func (suite *WebAuthnServiceTestSuite) TestUpdateWebAuthnCredentialInDB_GetCredentialsError() {
	updatedCredential := &webauthnCredential{
		ID: []byte("credential123"),
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, assert.AnError).Once()

	err := suite.service.updatePasskeyCredential(context.Background(), testUserID, updatedCredential)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to load existing passkey credentials")
}

func (suite *WebAuthnServiceTestSuite) TestUpdateWebAuthnCredentialInDB_UpdateError() {
	credentialID := []byte("credential123")
	existingCredential := webauthnCredential{
		ID:        credentialID,
		PublicKey: []byte("publickey123"),
	}
	existingCredJSON, _ := json.Marshal(existingCredential)

	updatedCredential := &webauthnCredential{
		ID:        credentialID,
		PublicKey: []byte("publickey123"),
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(existingCredJSON)}}, nil).Once()

	suite.mockEntityService.On("UpdateSystemCredentials", mock.Anything, testUserID, mock.Anything).
		Return(assert.AnError).Once()

	err := suite.service.updatePasskeyCredential(context.Background(), testUserID, updatedCredential)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to update passkey credentials")
}

func (suite *WebAuthnServiceTestSuite) TestUpdateWebAuthnCredentialInDB_InvalidExistingCredential() {
	credentialID := []byte("credential123")
	updatedCredential := &webauthnCredential{
		ID:        credentialID,
		PublicKey: []byte("publickey123"),
	}

	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{
			{Value: "{invalid json}"},
			{Value: "{invalid}"},
		}, nil).Once()

	err := suite.service.updatePasskeyCredential(context.Background(), testUserID, updatedCredential)

	suite.Error(err)
	suite.Contains(err.Error(), "credential not found for update")
}

func (suite *WebAuthnServiceTestSuite) TestStoreSessionData_Success() {
	sessionData := &sessionData{
		Challenge:            "challenge123",
		UserID:               []byte(testUserID),
		AllowedCredentialIDs: [][]byte{},
		UserVerification:     "preferred",
	}

	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		sessionData,
		mock.AnythingOfType("int64")).
		Return(nil).Once()

	sessionToken, svcErr := suite.service.storeSessionData(sessionData)

	suite.Nil(svcErr)
	suite.NotEmpty(sessionToken)
}

func (suite *WebAuthnServiceTestSuite) TestStoreSessionData_StoreError() {
	sessionData := &sessionData{
		Challenge: "challenge123",
		UserID:    []byte(testUserID),
	}

	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		sessionData,
		mock.AnythingOfType("int64")).
		Return(assert.AnError).Once()

	sessionToken, svcErr := suite.service.storeSessionData(sessionData)

	suite.Empty(sessionToken)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestRetrieveSessionData_Success() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	retrievedSessionData, userID, rpID, svcErr := suite.service.retrieveSessionData(testSessionToken)

	suite.Nil(svcErr)
	suite.NotNil(retrievedSessionData)
	suite.Equal(testUserID, userID)
	suite.Equal(testRelyingPartyID, rpID)
	suite.Equal(sessionData.Challenge, retrievedSessionData.Challenge)
}

func (suite *WebAuthnServiceTestSuite) TestRetrieveSessionData_SessionNotFound() {
	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(nil, assert.AnError).Once()

	retrievedSessionData, userID, rpID, svcErr := suite.service.retrieveSessionData(testSessionToken)

	suite.NotNil(svcErr)
	suite.Nil(retrievedSessionData)
	suite.Empty(userID)
	suite.Empty(rpID)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestClearSessionData() {
	suite.mockSessionStore.On("deleteSession", testSessionToken).
		Return(nil).Once()

	// This method doesn't return anything, just verify it calls the mock
	suite.service.clearSessionData(testSessionToken)

	suite.mockSessionStore.AssertExpectations(suite.T())
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_StoreSessionError() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
		OUID:     "org123",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, nil).Once()

	// Mock session store to return error
	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int64")).
		Return(assert.AnError).Once()

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_InvalidCredentialType() {
	req := &PasskeyRegistrationFinishRequest{
		CredentialID:      "cred123",
		CredentialType:    "", // Empty will default to "public-key"
		ClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0=",
		AttestationObject: "attestationdata",
		SessionToken:      testSessionToken,
	}

	result, svcErr := suite.service.FinishRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(ErrorInvalidAttestationResponse.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishRegistration_RetrieveSessionError() {
	req := &PasskeyRegistrationFinishRequest{
		CredentialID:      "cred123",
		CredentialType:    "public-key",
		ClientDataJSON:    "clientdata",
		AttestationObject: "attestationdata",
		SessionToken:      testSessionToken,
	}

	result, svcErr := suite.service.FinishRegistration(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)

	suite.Equal(ErrorInvalidAttestationResponse.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestGenerateAssertionWithAttributes() {
	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(nil, assert.AnError).Once()

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, svcErr := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(svcErr)
	suite.Equal(serviceerror.InternalServerError.Code, svcErr.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_GetUserError() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(nil, assert.AnError).Once()

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_GetCredentialsError() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, assert.AnError).Once()

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_NoCredentialsError() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return(nil, nil).Once()

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      testCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorNoCredentialsFound.Code, err.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_InvalidAssertionResponse() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	// Use invalid base64 to trigger parsing error
	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      "!!!invalid-base64!!!",
		CredentialType:    "public-key",
		ClientDataJSON:    "clientDataJSON",
		AuthenticatorData: "authenticatorData",
		Signature:         "signature",
		UserHandle:        "userHandle",
		SessionToken:      testSessionToken,
	}
	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidAuthenticatorResponse.Code, err.Code)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_CredentialsValidation() {
	// Test with a valid credential structure
	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			AAGUID:    []byte("aaguid123"),
			SignCount: 5,
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int64")).
		Return(nil).Once()

	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.NotEmpty(result.SessionToken)
	suite.NotEmpty(result.PublicKeyCredentialRequestOptions.Challenge)
}

func (suite *WebAuthnServiceTestSuite) TestStartAuthentication_CredentialWithZeroSignCount() {
	// Test credential with zero sign count (new credential)
	mockCredential := webauthnCredential{
		ID:        []byte("new-credential"),
		PublicKey: []byte("publickey"),
		Authenticator: authenticator{
			AAGUID:    []byte("aaguid"),
			SignCount: 0, // Zero sign count
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int64")).
		Return(nil).Once()

	req := &PasskeyAuthenticationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}
	result, svcErr := suite.service.StartAuthentication(context.Background(), req)

	suite.Nil(svcErr)
	suite.NotNil(result)
}

func (suite *WebAuthnServiceTestSuite) TestStartRegistration_WithExistingValidCredential() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	// Create a properly structured credential
	mockCredential := webauthnCredential{
		ID:        []byte("existing-credential-id"),
		PublicKey: []byte("existing-publickey"),
		Authenticator: authenticator{
			AAGUID:       []byte("existing-aaguid"),
			SignCount:    10,
			CloneWarning: false,
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	suite.mockSessionStore.On("storeSession",
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int64")).
		Return(nil).Once()

	result, svcErr := suite.service.StartRegistration(context.Background(), req)

	suite.Nil(svcErr)
	suite.NotNil(result)
	suite.NotEmpty(result.SessionToken)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_UpdateCredentialError() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	// Use valid base64url encoded credential ID
	validCredentialID := base64.RawURLEncoding.EncodeToString([]byte("credential123"))

	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			SignCount: 5,
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      validCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"passkey.get"}`)),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("authenticator-data")),
		Signature:         base64.RawURLEncoding.EncodeToString([]byte("signature")),
		UserHandle:        "",
		SessionToken:      testSessionToken,
	}
	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidAuthenticatorResponse.Code, err.Code)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_SkipAssertion() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID),
		RelyingPartyID: testRelyingPartyID,
	}

	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      "credential123",
		CredentialType:    "public-key",
		ClientDataJSON:    "valid-client-data",
		AuthenticatorData: "valid-auth-data",
		Signature:         "valid-signature",
		UserHandle:        "",
		SessionToken:      testSessionToken,
	}
	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
}

func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_UsernameBasedFlow_WithUserHandle() {
	sessionData := &sessionData{
		Challenge:      "challenge123",
		UserID:         []byte(testUserID), // UserID present in session
		RelyingPartyID: testRelyingPartyID,
	}

	// UserHandle may still be provided but should be ignored in username-based flow
	userHandle := base64.StdEncoding.EncodeToString([]byte("different-user"))

	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			SignCount: 5,
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	suite.mockSessionStore.On("deleteSession", testSessionToken).
		Return(nil).Maybe()

	validCredentialID := base64.RawURLEncoding.EncodeToString([]byte("credential123"))

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:      validCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"passkey.get"}`)),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("authenticator-data")),
		Signature:         base64.RawURLEncoding.EncodeToString([]byte("signature")),
		UserHandle:        userHandle, // Provided but should be ignored
		SessionToken:      testSessionToken,
	}

	result, err := suite.service.FinishAuthentication(context.Background(), req)

	// Will fail at WebAuthn validation but verifies username-based flow uses session userID
	suite.Nil(result)
	suite.NotNil(err)
}

// TestFinishAuthentication_ValidatePasskeyLogin_ReachesValidation tests that the usernameless
func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_ValidatePasskeyLogin_ReachesValidation() {
	sessionData := &sessionData{
		Challenge:      "dGVzdC1jaGFsbGVuZ2U",
		UserID:         nil, // Empty for usernameless
		RelyingPartyID: testRelyingPartyID,
	}

	userHandle := base64.StdEncoding.EncodeToString([]byte(testUserID))

	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			SignCount: 5,
			AAGUID:    []byte("aaguid123"),
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
		OUID:     "org123",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	validCredentialID := base64.RawURLEncoding.EncodeToString([]byte("credential123"))

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:   validCredentialID,
		CredentialType: "public-key",
		ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
			`{"type":"webauthn.get","challenge":"dGVzdC1jaGFsbGVuZ2U","origin":"http://` + testRelyingPartyID + `"}`)),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(make([]byte, 37)),
		Signature:         base64.RawURLEncoding.EncodeToString([]byte("test-signature")),
		UserHandle:        userHandle,
		SessionToken:      testSessionToken,
	}

	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.True(err.Code == ErrorInvalidSignature.Code || err.Code == ErrorInvalidAuthenticatorResponse.Code)
}

// TestFinishAuthentication_ValidateLogin_ReachesValidation tests that the username-based
func (suite *WebAuthnServiceTestSuite) TestFinishAuthentication_ValidateLogin_ReachesValidation() {
	sessionData := &sessionData{
		Challenge:      "dGVzdC1jaGFsbGVuZ2U",
		UserID:         []byte(testUserID), // UserID present for username-based flow
		RelyingPartyID: testRelyingPartyID,
	}

	mockCredential := webauthnCredential{
		ID:        []byte("credential123"),
		PublicKey: []byte("publickey123"),
		Authenticator: authenticator{
			SignCount: 5,
			AAGUID:    []byte("aaguid123"),
		},
	}
	credentialJSON, _ := json.Marshal(mockCredential)

	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
		Type:     "person",
		OUID:     "org123",
	}

	suite.mockSessionStore.On("retrieveSession", testSessionToken).
		Return(sessionData, nil).Once()

	suite.mockEntityService.On("GetEntity", mock.Anything, testUserID).
		Return(testEntity, nil).Once()
	suite.mockEntityService.On("GetCredentialsByType", mock.Anything, testUserID, "passkey").
		Return([]entity.StoredCredential{{Value: string(credentialJSON)}}, nil).Once()

	validCredentialID := base64.RawURLEncoding.EncodeToString([]byte("credential123"))

	req := &PasskeyAuthenticationFinishRequest{
		CredentialID:   validCredentialID,
		CredentialType: "public-key",
		ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
			`{"type":"webauthn.get","challenge":"dGVzdC1jaGFsbGVuZ2U","origin":"http://` + testRelyingPartyID + `"}`)),
		AuthenticatorData: base64.RawURLEncoding.EncodeToString(make([]byte, 37)),
		Signature:         base64.RawURLEncoding.EncodeToString([]byte("test-signature")),
		UserHandle:        "", // Empty for username-based flow
		SessionToken:      testSessionToken,
	}

	result, err := suite.service.FinishAuthentication(context.Background(), req)

	suite.Nil(result)
	suite.NotNil(err)
	suite.True(err.Code == ErrorInvalidSignature.Code || err.Code == ErrorInvalidAuthenticatorResponse.Code)
}
