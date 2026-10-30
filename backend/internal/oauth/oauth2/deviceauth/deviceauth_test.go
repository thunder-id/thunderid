// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourceserverprovidermock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

func newDeviceAuthServiceForApplication(
	t *testing.T,
	store DeviceAuthRequestStoreInterface,
	resourceService providers.ResourceServerProvider,
	jwtService jwt.JWTServiceInterface,
	cfg oauthconfig.Config,
	applicationID string,
) DeviceAuthServiceInterface {
	t.Helper()
	actorProvider := actorprovidermock.NewActorProviderMock(t)
	actorProvider.EXPECT().GetInboundClientByID(mock.Anything, applicationID).
		Return(&providers.InboundClient{DeviceGrantFlowID: "device-grant-flow"}, nil)
	return newDeviceAuthService(store, resourceService, jwtService, cfg, actorProvider)
}

func buildDeviceAssertion(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "none"})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

type DeviceAuthTestSuite struct {
	suite.Suite
}

func TestDeviceAuthTestSuite(t *testing.T) {
	suite.Run(t, new(DeviceAuthTestSuite))
}

func (s *DeviceAuthTestSuite) TestCreateGeneratesCodesAndPersistsRequest() {
	cfg := testhelpers.OAuthConfig()
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes:  []providers.GrantType{providers.GrantTypeDeviceCode},
		ScopeClaims: map[string][]string{"profile": {"name"}},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			UserAttributes: []string{"name"}}}}
	service := newDeviceAuthServiceForApplication(s.T(), store, nil, nil, cfg, client.ID)

	result, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid profile"}, client)

	s.Require().NoError(err)
	s.Require().Len(result.DeviceCode, 43)
	s.Require().Regexp(regexp.MustCompile(`^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$`), result.UserCode)
	s.Require().Equal(int64(600), result.ExpiresIn)
	s.Require().Equal(int64(5), result.Interval)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	s.Require().NoError(err)
	s.Require().Equal("https://localhost:3000/device?applicationId=application-1", result.VerificationURI)
	s.Require().Equal("https://localhost:3000/device?applicationId=application-1&user_code="+result.UserCode,
		result.VerificationURIComplete)
	s.Require().Equal(client.ClientID, record.ClientID)
	s.Require().NotEmpty(record.RequestID)
	s.Require().Equal(client.ID, record.ApplicationID)
	s.Require().Equal(hashDeviceCode(result.DeviceCode), record.DeviceCodeHash)
	s.Require().Equal("openid profile", record.StandardScopes)
	s.Require().Equal("name", record.RequiredOptionalAttributes)
	s.Require().Empty(record.RequestedPermissions)
	s.Require().Empty(record.Resources)
	s.Require().Equal(DeviceStatePending, record.State)
	s.Require().Equal(int64(5), record.PollInterval)
	s.Require().Zero(record.UserID)
	s.Require().Zero(record.AuthorizedScopes)
}

func (s *DeviceAuthTestSuite) TestCreateRejectsClientWithoutDeviceGrant() {
	service := newDeviceAuthService(newTestDeviceAuthStore(inmemory.Initialize("test-deployment")), nil, nil,
		testhelpers.OAuthConfig())
	client := &providers.OAuthClient{
		ClientID: "client", GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	_, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid"}, client)

	s.Require().EqualError(err, "unauthorized client")
}

func (s *DeviceAuthTestSuite) TestCreateReturnsServerErrorWhenApplicationLookupFails() {
	actorProvider := actorprovidermock.NewActorProviderMock(s.T())
	actorProvider.EXPECT().GetInboundClientByID(mock.Anything, "application-1").
		Return(nil, &tidcommon.ServiceError{Code: "APP-ERROR"})
	service := newDeviceAuthService(newTestDeviceAuthStore(inmemory.Initialize("test-deployment")),
		nil, nil, testhelpers.OAuthConfig(), actorProvider)
	client := &providers.OAuthClient{ID: "application-1",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}

	_, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{}, client)

	var deviceAuthErr *DeviceAuthError
	s.Require().ErrorAs(err, &deviceAuthErr)
	s.Equal(oauth2const.ErrorServerError, deviceAuthErr.Code)
}

func (s *DeviceAuthTestSuite) TestCreateReturnsServerErrorWhenApplicationHasNoDeviceGrantFlow() {
	actorProvider := actorprovidermock.NewActorProviderMock(s.T())
	actorProvider.EXPECT().GetInboundClientByID(mock.Anything, "application-1").
		Return(&providers.InboundClient{}, nil)
	service := newDeviceAuthService(newTestDeviceAuthStore(inmemory.Initialize("test-deployment")),
		nil, nil, testhelpers.OAuthConfig(), actorProvider)
	client := &providers.OAuthClient{ID: "application-1",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}

	_, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{}, client)

	var deviceAuthErr *DeviceAuthError
	s.Require().ErrorAs(err, &deviceAuthErr)
	s.Equal(oauth2const.ErrorServerError, deviceAuthErr.Code)
}

func (s *DeviceAuthTestSuite) TestCreatePropagatesNonCollisionStoreFailure() {
	storeFailure := errors.New("runtime store unavailable")
	store := NewDeviceAuthRequestStoreInterfaceMock(s.T())
	store.EXPECT().Add(mock.Anything, mock.Anything).Return(storeFailure)
	service := newDeviceAuthServiceForApplication(s.T(), store, nil, nil,
		testhelpers.OAuthConfig(), "application-1")
	client := &providers.OAuthClient{ID: "application-1",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}

	_, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{}, client)

	s.ErrorIs(err, storeFailure)
}

func (s *DeviceAuthTestSuite) TestVerifyUserCodeUsesStoredApplication() {
	runtimeStore := inmemory.Initialize("test-deployment")
	store := newTestDeviceAuthStore(runtimeStore)
	record := &DeviceAuthRequest{
		RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK",
		State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute),
		RequestedPermissions: "booking:read", Resources: []string{"https://api.example.com"},
		RequiredOptionalAttributes: "email",
	}
	s.Require().NoError(store.Add(context.Background(), record))
	verifier := NewUserCodeVerifier(runtimeStore, []byte(testUserCodeIndexKey))

	result, err := verifier.VerifyUserCode(context.Background(), "bcdf-ghjk")
	s.Require().NoError(err)
	s.Require().Equal("application-1", result.ApplicationID)
	s.Require().Equal("request-1", result.AuthID)
	s.Require().Equal("booking:read", result.RequestedPermissions)
	s.Require().Equal("https://api.example.com", result.ResourceServerIdentifier)
	s.Require().Equal("email", result.RequiredOptionalAttributes)
	stored, err := store.GetByRequestID(context.Background(), record.RequestID)
	s.Require().NoError(err)
	s.Require().Equal(DeviceStateRedeeming, stored.State)
}

func (s *DeviceAuthTestSuite) TestCreateStoresRequestedProfileAttributes() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes:  []providers.GrantType{providers.GrantTypeDeviceCode},
		ScopeClaims: map[string][]string{"email": {"email"}, "profile": {"name"}},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			UserAttributes: []string{"email", "name"}}}}
	service := newDeviceAuthServiceForApplication(s.T(), store, nil, nil, testhelpers.OAuthConfig(), client.ID)
	result, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid email profile"}, client)
	s.Require().NoError(err)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	s.Require().NoError(err)
	s.Require().Equal("email name", record.RequiredOptionalAttributes)
}

func (s *DeviceAuthTestSuite) TestCreateResolvesScopesAndResourceBinding() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	resourceService := resourceserverprovidermock.NewResourceServerProviderMock(s.T())
	resourceService.EXPECT().GetResourceServerByIdentifier(mock.Anything, "https://api.example.com").
		Return(&providers.ResourceServer{ID: "resource-server-1", Identifier: "https://api.example.com"}, nil)
	resourceService.EXPECT().ValidatePermissions(mock.Anything, "resource-server-1",
		[]string{"booking:read", "booking:delete"}).Return([]string{"booking:delete"}, nil)
	client := &providers.OAuthClient{
		ID:         "application-1",
		ClientID:   "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode},
	}
	service := newDeviceAuthServiceForApplication(s.T(), store, resourceService, nil,
		testhelpers.OAuthConfig(), client.ID)

	result, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{
		Scope:     "openid profile booking:read booking:delete",
		Resources: []string{"https://api.example.com"},
	}, client)

	s.Require().NoError(err)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	s.Require().NoError(err)
	s.Require().Equal("openid profile", record.StandardScopes)
	s.Require().Equal("booking:read", record.RequestedPermissions)
	s.Require().Equal([]string{"https://api.example.com"}, record.Resources)
}

func (s *DeviceAuthTestSuite) TestCreateAllowsRequestWithoutConsentPurpose() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}
	service := newDeviceAuthServiceForApplication(s.T(), store, nil, nil, testhelpers.OAuthConfig(), client.ID)

	result, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid"}, client)

	s.Require().NoError(err)
	s.Require().NotEmpty(result.DeviceCode)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	s.Require().NoError(err)
	s.Require().Equal("openid", record.StandardScopes)
	s.Require().Empty(record.RequestedPermissions)
	s.Require().Empty(record.RequiredOptionalAttributes)
}

func (s *DeviceAuthTestSuite) TestHandleCallbackApprovesRedeemingRequest() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK", ClientID: "client-1",
		StandardScopes: "openid profile", State: DeviceStateRedeeming,
		ExpiryTime: time.Now().Add(2 * time.Minute)}
	s.Require().NoError(store.Add(context.Background(), record))

	assertion := buildDeviceAssertion(s.T(), map[string]interface{}{
		"sub": "user-1", "aci": "cache-1", "completed_auth_class": "urn:acr:pwd",
		"authorization_request_id": "request-1", "authorized_permissions": "booking:read",
		oauth2const.ClaimSessionID: "session-1",
		"iat":                      float64(time.Now().Unix()),
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, jwtService, testhelpers.OAuthConfig())

	s.Require().Nil(service.HandleCallback(context.Background(), "request-1", assertion))
	stored, err := store.GetByRequestID(context.Background(), "request-1")
	s.Require().NoError(err)
	s.Require().Equal(DeviceStateApproved, stored.State)
	s.Require().Equal("user-1", stored.UserID)
	s.Require().Equal("openid profile booking:read", stored.AuthorizedScopes)
	s.Require().Equal("cache-1", stored.AttributeCacheID)
	s.Require().Equal("urn:acr:pwd", stored.CompletedACR)
	s.Require().Equal("session-1", stored.SessionID)
}

func (s *DeviceAuthTestSuite) TestHandleCallbackApprovesWithoutConsentClaims() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK",
		State: DeviceStateRedeeming, ExpiryTime: time.Now().Add(2 * time.Minute)}
	s.Require().NoError(store.Add(context.Background(), record))

	assertion := buildDeviceAssertion(s.T(), map[string]interface{}{
		"sub": "user-1", "authorization_request_id": "request-1",
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, jwtService, testhelpers.OAuthConfig())

	s.Require().Nil(service.HandleCallback(context.Background(), "request-1", assertion))
	stored, getErr := store.GetByRequestID(context.Background(), "request-1")
	s.Require().NoError(getErr)
	s.Require().Equal(DeviceStateApproved, stored.State)
}

func (s *DeviceAuthTestSuite) TestHandleCallbackDeniesRedeemingRequest() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK",
		State: DeviceStateRedeeming, ExpiryTime: time.Now().Add(2 * time.Minute)}
	s.Require().NoError(store.Add(context.Background(), record))

	assertion := buildDeviceAssertion(s.T(), map[string]interface{}{
		"authorization_request_id": "request-1", "flow_error_type": flowcm.FlowErrorTypeEndUser,
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, jwtService, testhelpers.OAuthConfig())

	s.Require().Nil(service.HandleCallback(context.Background(), "request-1", assertion))
	stored, err := store.GetByRequestID(context.Background(), "request-1")
	s.Require().NoError(err)
	s.Require().Equal(DeviceStateDenied, stored.State)
}

func (s *DeviceAuthTestSuite) TestHandleCallbackRejectsInvalidAssertionSignature() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), State: DeviceStateRedeeming,
		ExpiryTime: time.Now().Add(2 * time.Minute)}
	s.Require().NoError(store.Add(context.Background(), record))
	assertion := buildDeviceAssertion(s.T(), map[string]interface{}{
		"authorization_request_id": "request-1", "sub": "user-1",
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").
		Return(&tidcommon.ServiceError{Code: "JWT-ERROR"})
	service := newDeviceAuthService(store, nil, jwtService, testhelpers.OAuthConfig())

	err := service.HandleCallback(context.Background(), "request-1", assertion)

	s.Require().NotNil(err)
	s.Equal(oauth2const.ErrorInvalidRequest, err.Code)
	s.Equal("Invalid assertion signature", err.Message)
	stored, getErr := store.GetByRequestID(context.Background(), "request-1")
	s.Require().NoError(getErr)
	s.Equal(DeviceStateRedeeming, stored.State)
}

func (s *DeviceAuthTestSuite) TestHandleCallbackRejectsMismatchedAuthorizationRequestID() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), State: DeviceStateRedeeming,
		ExpiryTime: time.Now().Add(2 * time.Minute)}
	s.Require().NoError(store.Add(context.Background(), record))
	assertion := buildDeviceAssertion(s.T(), map[string]interface{}{
		"authorization_request_id": "another-request", "sub": "user-1",
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(s.T())
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, jwtService, testhelpers.OAuthConfig())

	err := service.HandleCallback(context.Background(), "request-1", assertion)

	s.Require().NotNil(err)
	s.Equal(oauth2const.ErrorAccessDenied, err.Code)
	s.Equal("Assertion does not match the device authorization request", err.Message)
	stored, getErr := store.GetByRequestID(context.Background(), "request-1")
	s.Require().NoError(getErr)
	s.Equal(DeviceStateRedeeming, stored.State)
}

func (s *DeviceAuthTestSuite) TestHandleCallbackRejectsExpiredOrNonRedeemingRequest() {
	for _, testCase := range []struct {
		name        string
		expiry      time.Time
		state       DeviceRequestState
		wantCode    string
		wantMessage string
	}{
		{name: "expired", expiry: time.Now().Add(-time.Second), state: DeviceStateRedeeming,
			wantCode: oauth2const.ErrorExpiredToken, wantMessage: "Device authorization request has expired"},
		{name: "wrong state", expiry: time.Now().Add(time.Minute), state: DeviceStatePending,
			wantCode: oauth2const.ErrorInvalidRequest, wantMessage: "Device authorization request is not redeeming"},
	} {
		s.Run(testCase.name, func() {
			store := NewDeviceAuthRequestStoreInterfaceMock(s.T())
			store.EXPECT().GetByRequestID(mock.Anything, "request-1").Return(&DeviceAuthRequest{
				RequestID: "request-1", State: testCase.state, ExpiryTime: testCase.expiry,
			}, nil)
			service := newDeviceAuthService(store, nil, nil, testhelpers.OAuthConfig())

			err := service.HandleCallback(context.Background(), "request-1", "assertion")

			s.Require().NotNil(err)
			s.Equal(testCase.wantCode, err.Code)
			s.Equal(testCase.wantMessage, err.Message)
		})
	}
}

func (s *DeviceAuthTestSuite) TestCreateRejectsMultipleResources() {
	store := newTestDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}
	service := newDeviceAuthServiceForApplication(s.T(), store, nil, nil, testhelpers.OAuthConfig(), client.ID)

	_, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{
		Resources: []string{"https://api.example.com", "https://api2.example.com"},
	}, client)

	var deviceAuthErr *DeviceAuthError
	s.Require().ErrorAs(err, &deviceAuthErr)
	s.Require().Equal(oauth2const.ErrorInvalidTarget, deviceAuthErr.Code)
}
