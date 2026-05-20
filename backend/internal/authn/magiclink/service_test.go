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

package magiclink

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testUserOUID    = "test-ou"
	testExecutionID = "flow-123"
	testToken       = "jwt-token-123" // nolint:gosec // G101: test data, not a real secret
	testIssuedAt    = int64(1609459200)
)

// testValidJWT is a valid JWT with recipient and user_id in the standard subclaim.
// nolint:gosec // G101: test data, not a real secret
var testValidJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
	"eyJyZWNpcGllbnQiOiJ0ZXN0QGV4YW1wbGUuY29tIiwic3ViIjoidXNlci0xMjMifQ." +
	"test-signature"

var testMissingSubJWT = "eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9." +
	"eyJyZWNpcGllbnQiOiAidGVzdEBleGFtcGxlLmNvbSJ9." +
	"test-signature"

var testMismatchedUserIDJWT = "eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9." +
	"eyJyZWNpcGllbnQiOiAidGVzdEBleGFtcGxlLmNvbSIsICJzdWIiOiAidXNlci00NTYifQ." +
	"test-signature"

var (
	testUserID   = "user-123"
	runtimeMutex sync.Mutex
)

func createMagicLinkJWTWithSubject(subject string) string {
	header := `{"alg":"HS256","typ":"JWT"}`
	payload := fmt.Sprintf(`{"sub":%q}`, subject)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))

	return headerB64 + "." + payloadB64 + ".test-signature"
}

func initializeTestRuntime(root string) error {
	testConfig := &config.Config{
		Server: config.ServerConfig{
			Hostname: "localhost",
			Port:     8090,
		},
		JWT: config.JWTConfig{
			Issuer: "magiclink-svc",
		},
		GateClient: config.GateClientConfig{
			Hostname:  "localhost",
			Port:      8090,
			Scheme:    "https",
			LoginPath: "/gate/signin",
		},
	}
	return config.InitializeServerRuntime(root, testConfig)
}

type MagicLinkServiceTestSuite struct {
	suite.Suite
	mockJWTService  *jwtmock.JWTServiceInterfaceMock
	mockUserService *entityprovidermock.EntityProviderInterfaceMock
	service         MagicLinkAuthnServiceInterface
}

func TestMagicLinkServiceTestSuite(t *testing.T) {
	suite.Run(t, new(MagicLinkServiceTestSuite))
}

func (suite *MagicLinkServiceTestSuite) SetupSuite() {
	runtimeMutex.Lock()
	config.ResetServerRuntime()
	suite.Require().NoError(initializeTestRuntime(suite.T().TempDir()))
}

func (suite *MagicLinkServiceTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
	runtimeMutex.Unlock()
}

func (suite *MagicLinkServiceTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockUserService = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.service = newMagicLinkAuthnService(suite.mockJWTService, suite.mockUserService)
}

func (suite *MagicLinkServiceTestSuite) TestGenerateMagicLinkSuccess() {
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		testUserID, mock.Anything, int64(DefaultExpirySeconds),
		mock.MatchedBy(func(claims interface{}) bool {
			m, ok := claims.(map[string]interface{})
			if !ok {
				return false
			}
			if executionID, ok := m["executionId"]; ok && executionID == testExecutionID {
				if aud, audOk := m["aud"].(string); audOk && aud == tokenAudience {
					return true
				}
			}
			return false
		}),
		jwt.TokenTypeJWT,
		"",
	).Return(testToken, testIssuedAt, nil)

	magicLinkURL, err := suite.service.GenerateMagicLink(
		context.Background(), testUserID, int64(DefaultExpirySeconds), map[string]string{"id": testExecutionID},
		map[string]interface{}{"executionId": testExecutionID}, "")
	suite.Nil(err)
	parsedURL, parseErr := url.Parse(magicLinkURL)
	suite.Require().NoError(parseErr)
	suite.Equal(testExecutionID, parsedURL.Query().Get("id"))
	suite.Equal(testToken, parsedURL.Query().Get("token"))
}

func (suite *MagicLinkServiceTestSuite) TestGenerateMagicLinkEmptyUserID() {
	_, err := suite.service.GenerateMagicLink(
		context.Background(), "", 0, map[string]string{"id": testExecutionID}, nil, "")
	suite.NotNil(err)
	suite.Equal(ErrorTokenGenerationFailed.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestGenerateMagicLinkJWTGenerationError() {
	suite.mockJWTService.On("GenerateJWT",
		mock.Anything,
		testUserID,
		mock.Anything,
		int64(DefaultExpirySeconds),
		mock.Anything,
		jwt.TokenTypeJWT,
		"",
	).Return("", int64(0), &serviceerror.ServiceError{Code: serviceerror.InternalServerError.Code})

	magicLinkURL, err := suite.service.GenerateMagicLink(context.Background(), testUserID, 0,
		map[string]string{"id": testExecutionID}, nil, "")
	suite.NotNil(err)
	suite.Equal(ErrorTokenGenerationFailed.Code, err.Code)
	suite.Empty(magicLinkURL)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkEmptyToken() {
	result, err := suite.service.VerifyMagicLink(context.Background(), "", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkExpiredToken() {
	expiredErr := &serviceerror.ServiceError{
		Code: jwt.ErrorTokenExpired.Code,
	}
	suite.mockJWTService.On("VerifyJWT", testToken, tokenAudience, mock.Anything).Return(expiredErr)

	result, err := suite.service.VerifyMagicLink(context.Background(), testToken, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorExpiredToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkInvalidToken() {
	suite.mockJWTService.On("VerifyJWT", testToken, tokenAudience, mock.Anything).Return(&serviceerror.ServiceError{
		Code: "JWT_INVALID",
	})

	result, err := suite.service.VerifyMagicLink(context.Background(), testToken, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkSuccess() {
	suite.mockJWTService.On("VerifyJWT", testValidJWT, tokenAudience, mock.Anything).Return(nil)

	testUser := &entityprovider.Entity{
		ID:   testUserID,
		OUID: testUserOUID,
		Type: "person",
	}
	suite.mockUserService.On("GetEntity", testUserID).Return(testUser, nil)

	result, err := suite.service.VerifyMagicLink(context.Background(), testValidJWT, "")
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.ID)
	suite.Equal(testUserOUID, result.OUID)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkSuccessWithDestinationAttribute() {
	const (
		workEmailAttr  = "workemail"
		workEmailValue = "johnwork@company.lk"
	)
	workEmailUser := "user-work"
	testWorkEmailJWT := createMagicLinkJWTWithSubject(workEmailValue)
	suite.mockJWTService.On("VerifyJWT", testWorkEmailJWT, tokenAudience, mock.Anything).Return(nil)
	suite.mockUserService.On("IdentifyEntity", map[string]interface{}{
		workEmailAttr: workEmailValue,
	}).Return(&workEmailUser, nil)

	testUser := &entityprovider.Entity{
		ID:   workEmailUser,
		OUID: testUserOUID,
		Type: "person",
	}
	suite.mockUserService.On("GetEntity", workEmailUser).Return(testUser, nil)

	result, err := suite.service.VerifyMagicLink(context.Background(), testWorkEmailJWT, workEmailAttr)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(workEmailUser, result.ID)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkMissingSubjectClaim() {
	suite.mockJWTService.On("VerifyJWT", testMissingSubJWT, tokenAudience, mock.Anything).Return(nil)

	result, err := suite.service.VerifyMagicLink(context.Background(), testMissingSubJWT, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMalformedTokenClaims.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkUserIDMismatchClaim() {
	suite.mockJWTService.On("VerifyJWT", testMismatchedUserIDJWT, tokenAudience, mock.Anything).Return(nil)
	suite.mockUserService.On("GetEntity", "user-456").Return(nil, &entityprovider.EntityProviderError{
		Code:    entityprovider.ErrorCodeEntityNotFound,
		Message: "Entity not found",
	})

	result, err := suite.service.VerifyMagicLink(context.Background(), testMismatchedUserIDJWT, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorUserNotFound.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkUserNotFoundOnVerify() {
	suite.mockJWTService.On("VerifyJWT", testValidJWT, tokenAudience, mock.Anything).Return(nil)
	suite.mockUserService.On("GetEntity", testUserID).Return(nil, &entityprovider.EntityProviderError{
		Code:    entityprovider.ErrorCodeEntityNotFound,
		Message: "Entity not found",
	})

	result, err := suite.service.VerifyMagicLink(context.Background(), testValidJWT, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(common.ErrorUserNotFound.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkGetUserError() {
	suite.mockJWTService.On("VerifyJWT", testValidJWT, tokenAudience, mock.Anything).Return(nil)
	suite.mockUserService.On("GetEntity", testUserID).Return(nil, &entityprovider.EntityProviderError{
		Code:    entityprovider.ErrorCodeInvalidRequestFormat,
		Message: "Invalid request",
	})

	result, err := suite.service.VerifyMagicLink(context.Background(), testValidJWT, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorClientErrorWhileResolvingUser.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestVerifyMagicLinkEntityProviderSystemError() {
	suite.mockJWTService.On("VerifyJWT", testValidJWT, tokenAudience, mock.Anything).Return(nil)
	suite.mockUserService.On("GetEntity", testUserID).Return(nil, &entityprovider.EntityProviderError{
		Code:        entityprovider.ErrorCodeSystemError,
		Message:     "System error",
		Description: "Database connection failed",
	})

	result, err := suite.service.VerifyMagicLink(context.Background(), testValidJWT, "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(serviceerror.InternalServerError.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestGetAuthenticatorMetadata() {
	metadata := suite.service.(*magicLinkAuthnService).getMetadata()
	suite.Equal(common.AuthenticatorMagicLink, metadata.Name)
	suite.Len(metadata.Factors, 1)
	suite.Contains(metadata.Factors, common.FactorPossession)
}

func (suite *MagicLinkServiceTestSuite) TestBuildMagicLinkURLUsesQueryParams() {
	service := suite.service.(*magicLinkAuthnService)

	result := service.buildMagicLinkURL("", testToken, map[string]string{"id": testExecutionID})
	parsedURL, err := url.Parse(result)

	suite.Require().NoError(err)
	suite.Equal("/gate/signin", parsedURL.Path)
	suite.Equal(testExecutionID, parsedURL.Query().Get("id"))
	suite.Equal(testToken, parsedURL.Query().Get("token"))
}

func (suite *MagicLinkServiceTestSuite) TestBuildMagicLinkURLUsesQueryParamsForCustomURL() {
	service := suite.service.(*magicLinkAuthnService)
	result := service.buildMagicLinkURL("https://example.com/signin?tenant=alpha", testToken,
		map[string]string{"id": testExecutionID})
	parsedURL, err := url.Parse(result)

	suite.Require().NoError(err)
	suite.Equal("alpha", parsedURL.Query().Get("tenant"))
	suite.Equal(testExecutionID, parsedURL.Query().Get("id"))
	suite.Equal(testToken, parsedURL.Query().Get("token"))
}

func (suite *MagicLinkServiceTestSuite) TestBuildMagicLinkURLDefaultURLIsNotMutated() {
	service := suite.service.(*magicLinkAuthnService)

	result1 := service.buildMagicLinkURL("", "token-aaa", map[string]string{"id": "flow-aaa"})
	result2 := service.buildMagicLinkURL("", "token-bbb", map[string]string{"id": "flow-bbb"})

	parsedURL1, err1 := url.Parse(result1)
	parsedURL2, err2 := url.Parse(result2)

	suite.Require().NoError(err1)
	suite.Require().NoError(err2)

	suite.Equal("flow-aaa", parsedURL1.Query().Get("id"))
	suite.Equal("token-aaa", parsedURL1.Query().Get("token"))

	suite.Equal("flow-bbb", parsedURL2.Query().Get("id"))
	suite.Equal("token-bbb", parsedURL2.Query().Get("token"))
}
