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
	"encoding/json"
	"net/url"
	"sync"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testExecutionID = "flow-123"
	testToken       = "jwt-token-123" // nolint:gosec // G101: test data, not a real secret
	testIssuedAt    = int64(1609459200)
)

var (
	testUserID   = "user-123"
	runtimeMutex sync.Mutex
)

func createMagicLinkJWTWithClaims(subject, executionID, jti string) string {
	payloadMap := make(map[string]interface{})
	if subject != "" {
		payloadMap["sub"] = subject
	}
	if executionID != "" {
		payloadMap["nonce"] = executionID
	}
	if jti != "" {
		payloadMap["jti"] = jti
	}
	return createMagicLinkJWTWithRawPayload(payloadMap)
}

func createMagicLinkJWTWithRawPayload(payloadMap map[string]interface{}) string {
	header := `{"alg":"HS256","typ":"JWT"}`
	payloadBytes, _ := json.Marshal(payloadMap)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	return headerB64 + "." + payloadB64 + ".test-signature"
}

func initializeTestRuntime(root string) error {
	testConfig := &config.Config{
		Server: engineconfig.ServerConfig{
			Hostname: "localhost",
			Port:     8090,
		},
		JWT: engineconfig.JWTConfig{
			Issuer: "magiclink-svc",
		},
		GateClient: engineconfig.GateClientConfig{
			Hostname:     "localhost",
			Port:         8090,
			Scheme:       "https",
			LoginPath:    "/gate/signin",
			CallbackPath: "/gate/callback",
		},
	}
	return config.InitializeServerRuntime(root, testConfig)
}

type MagicLinkServiceTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	service        MagicLinkAuthnServiceInterface
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
	suite.service = newMagicLinkAuthnService(suite.mockJWTService)
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
	).Return("", int64(0), &tidcommon.ServiceError{Code: tidcommon.InternalServerError.Code})

	magicLinkURL, err := suite.service.GenerateMagicLink(context.Background(), testUserID, 0,
		map[string]string{"id": testExecutionID}, nil, "")
	suite.NotNil(err)
	suite.Equal(ErrorTokenGenerationFailed.Code, err.Code)
	suite.Empty(magicLinkURL)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateEmptyToken() {
	result, err := suite.service.Authenticate(context.Background(), "", "", "", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateExpiredToken() {
	expiredErr := &tidcommon.ServiceError{
		Code: jwt.ErrorTokenExpired.Code,
	}
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testToken, tokenAudience, mock.Anything).Return(expiredErr)

	result, err := suite.service.Authenticate(context.Background(), testToken, testExecutionID, "", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorExpiredToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateInvalidToken() {
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testToken, tokenAudience, mock.Anything).
		Return(&tidcommon.ServiceError{
			Code: "JWT_INVALID",
		})

	result, err := suite.service.Authenticate(context.Background(), testToken, testExecutionID, "", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateSuccess() {
	testJWT := createMagicLinkJWTWithClaims(testUserID, testExecutionID, "jti-123")
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testJWT, tokenAudience, mock.Anything).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testJWT, testExecutionID, "", "")
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(testUserID, result.Token[common.UserAttributeUserID])
	suite.Equal(testUserID, result.AuthenticatedClaims[common.UserAttributeUserID])
	suite.Equal("jti-123", result.AuthenticatedClaims[ClaimMagicLinkUsedJti])
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateSuccessWithSubjectAttribute() {
	const (
		workEmailAttr  = "workemail"
		workEmailValue = "johnwork@company.lk"
	)
	testWorkEmailJWT := createMagicLinkJWTWithClaims(workEmailValue, testExecutionID, "jti-work")
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testWorkEmailJWT, tokenAudience, mock.Anything).Return(nil)

	result, err := suite.service.Authenticate(
		context.Background(), testWorkEmailJWT, testExecutionID, "", workEmailAttr)
	suite.Nil(err)
	suite.NotNil(result)
	suite.Equal(workEmailValue, result.Token[workEmailAttr])
	suite.Equal(workEmailValue, result.AuthenticatedClaims[workEmailAttr])
	suite.Equal("jti-work", result.AuthenticatedClaims[ClaimMagicLinkUsedJti])
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateExecutionIDMismatch() {
	testJWT := createMagicLinkJWTWithClaims(testUserID, "wrong-exec-id", "jti-123")
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testJWT, tokenAudience, mock.Anything).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testJWT, testExecutionID, "", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateAlreadyUsedJTI() {
	testJWT := createMagicLinkJWTWithClaims(testUserID, testExecutionID, "jti-used")
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testJWT, tokenAudience, mock.Anything).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testJWT, testExecutionID, "jti-used", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateMissingSubjectClaim() {
	testMissingSubJWT := createMagicLinkJWTWithClaims("", testExecutionID, "jti-nosub")
	suite.mockJWTService.On("VerifyJWT", mock.Anything, testMissingSubJWT, tokenAudience, mock.Anything).Return(nil)

	result, err := suite.service.Authenticate(context.Background(), testMissingSubJWT, testExecutionID, "", "")
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorMalformedTokenClaims.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestAuthenticateNonStringClaims() {
	ctx := context.Background()

	// Non-string sub claim (number)
	tokenNonStringSub := createMagicLinkJWTWithRawPayload(map[string]interface{}{
		"sub": 12345, "nonce": testExecutionID, "jti": "jti-1",
	})
	suite.mockJWTService.On("VerifyJWT", mock.Anything, tokenNonStringSub, tokenAudience, mock.Anything).Return(nil)
	res, err := suite.service.Authenticate(ctx, tokenNonStringSub, testExecutionID, "", "")
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorMalformedTokenClaims.Code, err.Code)

	// Non-string nonce claim (boolean)
	tokenNonStringNonce := createMagicLinkJWTWithRawPayload(map[string]interface{}{
		"sub": testUserID, "nonce": true, "jti": "jti-2",
	})
	suite.mockJWTService.On("VerifyJWT", mock.Anything, tokenNonStringNonce, tokenAudience, mock.Anything).Return(nil)
	res, err = suite.service.Authenticate(ctx, tokenNonStringNonce, testExecutionID, "", "")
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)

	// Non-string jti claim (number)
	tokenNonStringJTI := createMagicLinkJWTWithRawPayload(map[string]interface{}{
		"sub": testUserID, "nonce": testExecutionID, "jti": 999,
	})
	suite.mockJWTService.On("VerifyJWT", mock.Anything, tokenNonStringJTI, tokenAudience, mock.Anything).Return(nil)
	res, err = suite.service.Authenticate(ctx, tokenNonStringJTI, testExecutionID, "", "")
	suite.Nil(res)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidToken.Code, err.Code)
}

func (suite *MagicLinkServiceTestSuite) TestGetAuthenticatorMetadata() {
	metadata := suite.service.(*magicLinkAuthnService).getMetadata()
	suite.Equal(common.AuthenticatorMagicLink, metadata.Name)
	suite.Len(metadata.Factors, 1)
	suite.Contains(metadata.Factors, common.FactorPossession)
}

func (suite *MagicLinkServiceTestSuite) TestBuildMagicLinkURLUsesQueryParams() {
	service := suite.service.(*magicLinkAuthnService)

	result := service.buildMagicLinkURL(context.Background(), "", testToken, map[string]string{"id": testExecutionID})
	parsedURL, err := url.Parse(result)

	suite.Require().NoError(err)
	suite.Equal("/gate/callback", parsedURL.Path)
	suite.Equal(testExecutionID, parsedURL.Query().Get("id"))
	suite.Equal(testToken, parsedURL.Query().Get("token"))
}

func (suite *MagicLinkServiceTestSuite) TestBuildMagicLinkURLUsesQueryParamsForCustomURL() {
	service := suite.service.(*magicLinkAuthnService)
	result := service.buildMagicLinkURL(context.Background(), "https://example.com/signin?tenant=alpha", testToken,
		map[string]string{"id": testExecutionID})
	parsedURL, err := url.Parse(result)

	suite.Require().NoError(err)
	suite.Equal("alpha", parsedURL.Query().Get("tenant"))
	suite.Equal(testExecutionID, parsedURL.Query().Get("id"))
	suite.Equal(testToken, parsedURL.Query().Get("token"))
}

func (suite *MagicLinkServiceTestSuite) TestBuildMagicLinkURLDefaultURLIsNotMutated() {
	service := suite.service.(*magicLinkAuthnService)

	result1 := service.buildMagicLinkURL(context.Background(), "", "token-aaa", map[string]string{"id": "flow-aaa"})
	result2 := service.buildMagicLinkURL(context.Background(), "", "token-bbb", map[string]string{"id": "flow-bbb"})

	parsedURL1, err1 := url.Parse(result1)
	parsedURL2, err2 := url.Parse(result2)

	suite.Require().NoError(err1)
	suite.Require().NoError(err2)

	suite.Equal("flow-aaa", parsedURL1.Query().Get("id"))
	suite.Equal("token-aaa", parsedURL1.Query().Get("token"))

	suite.Equal("flow-bbb", parsedURL2.Query().Get("id"))
	suite.Equal("token-bbb", parsedURL2.Query().Get("token"))
}
