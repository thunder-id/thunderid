// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourceserverprovidermock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type deviceAuthFlowProvider struct {
	flow *providers.CompleteFlowDefinition
}

func (p deviceAuthFlowProvider) GetFlow(context.Context, string) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError,
) {
	return p.flow, nil
}

func (p deviceAuthFlowProvider) GetFlowByHandle(context.Context, string, providers.FlowType) (
	*providers.CompleteFlowDefinition, *tidcommon.ServiceError,
) {
	return nil, nil
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

func applicationFlowProviders(t *testing.T, applicationID, flowID string,
	flowType providers.FlowType) (providers.ActorProvider, providers.FlowProvider) {
	t.Helper()
	actorProvider := actorprovidermock.NewActorProviderMock(t)
	actorProvider.EXPECT().GetInboundClientByID(mock.Anything, applicationID).
		Return(&providers.InboundClient{ID: applicationID, AuthFlowID: flowID}, nil)
	flowProvider := deviceAuthFlowProvider{flow: &providers.CompleteFlowDefinition{
		ID: flowID, FlowType: flowType,
		Nodes: []providers.NodeDefinition{{
			ID: "consent", Type: string(flowcm.NodeTypeTaskExecution),
			Executor: &providers.ExecutorDefinition{Name: executormeta.ExecutorNameConsent},
		}},
	}}
	return actorProvider, flowProvider
}

func TestCreateGeneratesCodesAndPersistsRequest(t *testing.T) {
	cfg := testhelpers.OAuthConfig()
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes:  []providers.GrantType{providers.GrantTypeDeviceCode},
		ScopeClaims: map[string][]string{"profile": {"name"}},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			UserAttributes: []string{"name"}}}}
	actorProvider, flowProvider := applicationFlowProviders(t, client.ID, "authentication-flow-1",
		providers.FlowTypeAuthentication)
	service := newDeviceAuthService(store, actorProvider, flowProvider, nil, nil, cfg)

	result, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid profile"}, client)

	require.NoError(t, err)
	require.Len(t, result.DeviceCode, 43)
	require.Regexp(t, regexp.MustCompile(`^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$`), result.UserCode)
	require.Equal(t, int64(600), result.ExpiresIn)
	require.Equal(t, int64(5), result.Interval)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	require.NoError(t, err)
	require.Equal(t, "https://localhost:3000/device?applicationId=application-1", result.VerificationURI)
	require.Equal(t, "https://localhost:3000/device?applicationId=application-1&user_code="+result.UserCode,
		result.VerificationURIComplete)
	require.Equal(t, client.ClientID, record.ClientID)
	require.NotEmpty(t, record.RequestID)
	require.Equal(t, client.ID, record.ApplicationID)
	require.Equal(t, "authentication-flow-1", record.ApplicationAuthFlowID)
	require.Equal(t, hashDeviceCode(result.DeviceCode), record.DeviceCodeHash)
	require.Equal(t, "openid profile", record.StandardScopes)
	require.Equal(t, "name", record.RequiredOptionalAttributes)
	require.Empty(t, record.RequestedPermissions)
	require.Empty(t, record.Resources)
	require.Equal(t, DeviceStatePending, record.State)
	require.Equal(t, int64(5), record.PollInterval)
	require.Zero(t, record.UserID)
	require.Zero(t, record.AuthorizedScopes)
}

func TestCreateRejectsClientWithoutDeviceGrant(t *testing.T) {
	service := newDeviceAuthService(newDeviceAuthStore(inmemory.Initialize("test-deployment")), nil, nil, nil, nil,
		testhelpers.OAuthConfig())
	client := &providers.OAuthClient{
		ClientID: "client", GrantTypes: []providers.GrantType{providers.GrantTypeAuthorizationCode},
	}

	_, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid"}, client)

	require.EqualError(t, err, "unauthorized client")
}

func TestVerifyUserCodeUsesStoredApplication(t *testing.T) {
	runtimeStore := inmemory.Initialize("test-deployment")
	store := newDeviceAuthStore(runtimeStore)
	record := &DeviceAuthRequest{
		RequestID: "request-1", ApplicationID: "application-1", ApplicationAuthFlowID: "authentication-flow-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK",
		State: DeviceStatePending, ExpiryTime: time.Now().Add(2 * time.Minute),
		RequestedPermissions: "booking:read", Resources: []string{"https://api.example.com"},
		RequiredOptionalAttributes: "email",
	}
	require.NoError(t, store.Add(context.Background(), record))
	verifier := NewUserCodeVerifier(runtimeStore)

	result, err := verifier.VerifyUserCode(context.Background(), "bcdf-ghjk")
	require.NoError(t, err)
	require.Equal(t, "application-1", result.ApplicationID)
	require.Equal(t, "authentication-flow-1", result.ApplicationAuthFlowID)
	require.Equal(t, "request-1", result.AuthID)
	require.Equal(t, "booking:read", result.RequestedPermissions)
	require.Equal(t, "https://api.example.com", result.ResourceServerIdentifier)
	require.Equal(t, "email", result.RequiredOptionalAttributes)
	stored, err := store.GetByRequestID(context.Background(), record.RequestID)
	require.NoError(t, err)
	require.Equal(t, DeviceStateRedeeming, stored.State)
}

func TestCreateStoresRequestedProfileAttributes(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes:  []providers.GrantType{providers.GrantTypeDeviceCode},
		ScopeClaims: map[string][]string{"email": {"email"}, "profile": {"name"}},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			UserAttributes: []string{"email", "name"}}}}
	actorProvider, flowProvider := applicationFlowProviders(t, client.ID, "authentication-flow-1",
		providers.FlowTypeAuthentication)
	service := newDeviceAuthService(store, actorProvider, flowProvider, nil, nil, testhelpers.OAuthConfig())
	result, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid email profile"}, client)
	require.NoError(t, err)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	require.NoError(t, err)
	require.Equal(t, "email name", record.RequiredOptionalAttributes)
}

func TestCreateResolvesScopesAndResourceBinding(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	resourceService := resourceserverprovidermock.NewResourceServerProviderMock(t)
	resourceService.EXPECT().GetResourceServerByIdentifier(mock.Anything, "https://api.example.com").
		Return(&providers.ResourceServer{ID: "resource-server-1", Identifier: "https://api.example.com"}, nil)
	resourceService.EXPECT().ValidatePermissions(mock.Anything, "resource-server-1",
		[]string{"booking:read", "booking:delete"}).Return([]string{"booking:delete"}, nil)
	client := &providers.OAuthClient{
		ID:         "application-1",
		ClientID:   "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode},
	}
	actorProvider, flowProvider := applicationFlowProviders(t, client.ID, "authentication-flow-1",
		providers.FlowTypeAuthentication)
	service := newDeviceAuthService(store, actorProvider, flowProvider, resourceService, nil,
		testhelpers.OAuthConfig())

	result, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{
		Scope:     "openid profile booking:read booking:delete",
		Resources: []string{"https://api.example.com"},
	}, client)

	require.NoError(t, err)
	record, err := store.GetByDeviceCode(context.Background(), result.DeviceCode)
	require.NoError(t, err)
	require.Equal(t, "openid profile", record.StandardScopes)
	require.Equal(t, "booking:read", record.RequestedPermissions)
	require.Equal(t, []string{"https://api.example.com"}, record.Resources)
}

func TestCreateRejectsInvalidApplicationAuthenticationFlow(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}
	actorProvider, flowProvider := applicationFlowProviders(t, client.ID, "system-flow-1",
		providers.FlowTypeSystem)
	service := newDeviceAuthService(store, actorProvider, flowProvider, nil, nil, testhelpers.OAuthConfig())

	_, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid"}, client)

	var deviceAuthErr *DeviceAuthError
	require.ErrorAs(t, err, &deviceAuthErr)
	require.Equal(t, oauth2const.ErrorServerError, deviceAuthErr.Code)
}

func TestCreateRejectsRequestWithoutConsentPurpose(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}
	actorProvider, flowProvider := applicationFlowProviders(t, client.ID, "authentication-flow-1",
		providers.FlowTypeAuthentication)
	service := newDeviceAuthService(store, actorProvider, flowProvider, nil, nil, testhelpers.OAuthConfig())

	_, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid"}, client)

	var deviceAuthErr *DeviceAuthError
	require.ErrorAs(t, err, &deviceAuthErr)
	require.Equal(t, oauth2const.ErrorInvalidRequest, deviceAuthErr.Code)
}

func TestCreateRejectsApplicationFlowWithoutConsentStep(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	client := &providers.OAuthClient{ID: "application-1", ClientID: "device-client",
		GrantTypes:  []providers.GrantType{providers.GrantTypeDeviceCode},
		ScopeClaims: map[string][]string{"profile": {"name"}},
		Token: &providers.OAuthTokenConfig{IDToken: &providers.IDTokenConfig{
			UserAttributes: []string{"name"}}}}
	actorProvider := actorprovidermock.NewActorProviderMock(t)
	actorProvider.EXPECT().GetInboundClientByID(mock.Anything, client.ID).
		Return(&providers.InboundClient{ID: client.ID, AuthFlowID: "authentication-flow-1"}, nil)
	flowProvider := deviceAuthFlowProvider{flow: &providers.CompleteFlowDefinition{
		ID: "authentication-flow-1", FlowType: providers.FlowTypeAuthentication,
	}}
	service := newDeviceAuthService(store, actorProvider, flowProvider, nil, nil, testhelpers.OAuthConfig())

	_, err := service.InitiateDeviceAuthorization(context.Background(),
		&DeviceAuthorizationRequest{Scope: "openid profile"}, client)

	var deviceAuthErr *DeviceAuthError
	require.ErrorAs(t, err, &deviceAuthErr)
	require.Equal(t, oauth2const.ErrorInvalidRequest, deviceAuthErr.Code)
}

func TestHandleCallbackApprovesRedeemingRequest(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK", ClientID: "client-1",
		StandardScopes: "openid profile", State: DeviceStateRedeeming,
		ExpiryTime: time.Now().Add(2 * time.Minute)}
	require.NoError(t, store.Add(context.Background(), record))

	assertion := buildDeviceAssertion(t, map[string]interface{}{
		"sub": "user-1", "aci": "cache-1", "completed_auth_class": "urn:acr:pwd",
		"authorization_request_id": "request-1", "authorized_permissions": "booking:read",
		oauth2const.ClaimConsentID: "consent-1", oauth2const.ClaimConsentApproved: true,
		oauth2const.ClaimSessionID: "session-1",
		"iat":                      float64(time.Now().Unix()),
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, nil, nil, jwtService, testhelpers.OAuthConfig())

	require.Nil(t, service.HandleCallback(context.Background(), "request-1", assertion))
	stored, err := store.GetByRequestID(context.Background(), "request-1")
	require.NoError(t, err)
	require.Equal(t, DeviceStateApproved, stored.State)
	require.Equal(t, "user-1", stored.UserID)
	require.Equal(t, "openid profile booking:read", stored.AuthorizedScopes)
	require.Equal(t, "cache-1", stored.AttributeCacheID)
	require.Equal(t, "urn:acr:pwd", stored.CompletedACR)
	require.Equal(t, "session-1", stored.SessionID)
}

func TestHandleCallbackDoesNotApproveWithoutRecordedConsent(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK",
		State: DeviceStateRedeeming, ExpiryTime: time.Now().Add(2 * time.Minute)}
	require.NoError(t, store.Add(context.Background(), record))

	assertion := buildDeviceAssertion(t, map[string]interface{}{
		"sub": "user-1", "authorization_request_id": "request-1",
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, nil, nil, jwtService, testhelpers.OAuthConfig())

	err := service.HandleCallback(context.Background(), "request-1", assertion)
	var deviceAuthErr *DeviceAuthError
	require.ErrorAs(t, err, &deviceAuthErr)
	require.Equal(t, oauth2const.ErrorAccessDenied, deviceAuthErr.Code)
	stored, getErr := store.GetByRequestID(context.Background(), "request-1")
	require.NoError(t, getErr)
	require.Equal(t, DeviceStateFailed, stored.State)
}

func TestHandleCallbackDeniesRedeemingRequest(t *testing.T) {
	store := newDeviceAuthStore(inmemory.Initialize("test-deployment"))
	record := &DeviceAuthRequest{RequestID: "request-1", ApplicationID: "application-1",
		DeviceCodeHash: hashDeviceCode("device-code"), UserCode: "BCDF-GHJK",
		State: DeviceStateRedeeming, ExpiryTime: time.Now().Add(2 * time.Minute)}
	require.NoError(t, store.Add(context.Background(), record))

	assertion := buildDeviceAssertion(t, map[string]interface{}{
		"authorization_request_id": "request-1", "flow_error_type": flowcm.FlowErrorTypeEndUser,
	})
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtService.EXPECT().VerifyJWT(mock.Anything, assertion, "application-1", "").Return(nil)
	service := newDeviceAuthService(store, nil, nil, nil, jwtService, testhelpers.OAuthConfig())

	require.Nil(t, service.HandleCallback(context.Background(), "request-1", assertion))
	stored, err := store.GetByRequestID(context.Background(), "request-1")
	require.NoError(t, err)
	require.Equal(t, DeviceStateDenied, stored.State)
}

func TestCreateRejectsMultipleResources(t *testing.T) {
	service := newDeviceAuthService(newDeviceAuthStore(inmemory.Initialize("test-deployment")), nil, nil, nil, nil,
		testhelpers.OAuthConfig())
	client := &providers.OAuthClient{ClientID: "device-client",
		GrantTypes: []providers.GrantType{providers.GrantTypeDeviceCode}}

	_, err := service.InitiateDeviceAuthorization(context.Background(), &DeviceAuthorizationRequest{
		Resources: []string{"https://api.example.com", "https://api2.example.com"},
	}, client)

	var deviceAuthErr *DeviceAuthError
	require.ErrorAs(t, err, &deviceAuthErr)
	require.Equal(t, oauth2const.ErrorInvalidTarget, deviceAuthErr.Code)
}
