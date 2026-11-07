// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/revocationcache"
	"github.com/thunder-id/thunderid/internal/system/security"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testDeploymentID = "test-deployment"
	testIssuer       = "https://audit.test"
	testAPIKey       = "pat-0123456789abcdef"
	testRemoteAddr   = "192.0.2.10:52814"
	testClientIP     = "192.0.2.10"
	testUserAgent    = "audit-test/1.0"
	testUserID       = "01900000-0000-7000-8000-000000000030"
	testAppID        = "01900000-0000-7000-8000-000000000031"
	testAgentID      = "01900000-0000-7000-8000-000000000032"
	testRoleID       = "01900000-0000-7000-8000-000000000033"
	testGroupID      = "01900000-0000-7000-8000-000000000034"
	testGatewayID    = "01900000-0000-7000-8000-000000000035"
	eventWait        = 3 * time.Second
)

var (
	testTarget      = Ref{Type: TargetTypeUser, ID: testUserID}
	testApplication = Ref{Type: TargetTypeApplication, ID: testAppID}
	testRole        = Ref{Type: TargetTypeRole, ID: testRoleID}
	testGroup       = Ref{Type: TargetTypeGroup, ID: testGroupID}
	testGateway     = Ref{Type: TargetTypeGateway, ID: testGatewayID}
)

// received is one audit event a sink was handed.
type received struct {
	env     eventbus.Envelope
	payload Payload
}

// channelSink forwards every event it is handed to its channel.
type channelSink chan received

func (c channelSink) Handle(_ context.Context, env eventbus.Envelope, payload Payload) error {
	c <- received{env: env, payload: payload}
	return nil
}

// newAuditBus builds a bus with a sink subscribed through Subscribe that forwards every event it
// receives to the returned channel.
func newAuditBus(t *testing.T) (eventbus.EventBusInterface, <-chan received) {
	t.Helper()
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	events := make(channelSink, 32)
	require.NoError(t, Subscribe(registry, "test-sink", events))
	return bus, events
}

// awaitEvent returns the next event the sink received.
func awaitEvent(t *testing.T, events <-chan received) received {
	t.Helper()
	select {
	case evt := <-events:
		return evt
	case <-time.After(eventWait):
		t.Fatal("no audit event was delivered")
		return received{}
	}
}

// withServerRuntime initializes the server runtime the security layer reads, for the duration of the
// test.
func withServerRuntime(t *testing.T) {
	t.Helper()
	config.ResetServerRuntime()
	cfg := &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
		JWT:    engineconfig.JWTConfig{Issuer: testIssuer},
	}
	require.NoError(t, config.InitializeServerRuntime("", cfg))
	t.Cleanup(config.ResetServerRuntime)
}

// accessToken builds a compact access token carrying claims, as this server issues them. The
// signature is not checked: the JWT service mock accepts every token.
func accessToken(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	claims["iss"] = testIssuer
	if _, ok := claims["scope"]; !ok {
		claims["scope"] = "system"
	}
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	encode := base64.RawURLEncoding.EncodeToString
	return encode(header) + "." + encode(payload) + "." + encode([]byte("signature"))
}

// newRequest builds a request from the test client address with the given credentials.
func newRequest(method, target, bearer, apiKey string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	r.RemoteAddr = testRemoteAddr
	r.Header.Set("User-Agent", testUserAgent)
	if bearer != "" {
		r.Header.Set(constants.AuthorizationHeaderName, constants.AuthSchemeBearer+bearer)
	}
	if apiKey != "" {
		r.Header.Set(constants.APIKeyHeaderName, apiKey)
	}
	return r
}

// securedContext returns the context the client middleware and the security layer hand down for r,
// built by the real middleware and authenticators.
func securedContext(t *testing.T, r *http.Request) context.Context {
	t.Helper()
	withServerRuntime(t)
	security.InitSystemPermissions("")

	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	jwtService.EXPECT().VerifyJWT(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	enforcer, _, err := revocationcache.Initialize(revocationcache.Config{})
	require.NoError(t, err)
	secure, err := security.Initialize(jwtService, enforcer, "", cryptolib.HashToken(testAPIKey))
	require.NoError(t, err)

	var ctx context.Context
	handler := middleware.ClientMiddleware(secure(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, r)
	require.NotNil(t, ctx, "the security layer refused the request with %d", recorder.Code)
	return ctx
}

// securedCallContext returns the context of a management call with the given credentials, on a path
// that admits both a token and the management API key.
func securedCallContext(t *testing.T, bearer, apiKey string) context.Context {
	t.Helper()
	return securedContext(t, newRequest(http.MethodPut, "/variables/API_URL", bearer, apiKey))
}
