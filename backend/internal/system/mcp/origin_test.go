// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/cors/corstest"
)

const (
	// testServerOrigin is the origin of the server URL derived from testHostname and testPort.
	testServerOrigin = "https://localhost:8090"
	testCORSOrigin   = "https://app.example.com"
	// testPublicURL stands in for server.public_url behind a reverse proxy. Its path is not part of
	// its origin.
	testPublicURL    = "https://id.example.com/thunderid"
	testPublicOrigin = "https://id.example.com"
)

type OriginTestSuite struct {
	suite.Suite
}

func TestOriginTestSuite(t *testing.T) {
	suite.Run(t, new(OriginTestSuite))
}

func (suite *OriginTestSuite) SetupTest() {
	config.ResetServerRuntime()
}

func (suite *OriginTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// initRuntime installs a server runtime listening on testHostname:testPort, with publicURL as
// server.public_url when it is not empty.
func (suite *OriginTestSuite) initRuntime(publicURL string) {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.Server.Hostname = testHostname
	cfg.Server.Port = testPort
	cfg.Server.PublicURL = publicURL
	suite.Require().NoError(config.InitializeServerRuntime("", cfg))
}

// allowCORSOrigin installs a CORS configuration that allows origin, for the rest of the test.
func (suite *OriginTestSuite) allowCORSOrigin(origin string) {
	var entries cors.OriginEntries
	suite.Require().NoError(yaml.Unmarshal([]byte("- "+origin), &entries))
	corstest.InstallMatcherEntries(suite.T(), entries)
}

// serve sends a request carrying the given Origin header values through requireAllowedOrigin and
// reports the response status and whether the request reached the next handler.
func (suite *OriginTestSuite) serve(origins ...string) (int, bool) {
	reached := false
	handler := requireAllowedOrigin(serverOrigin(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, MCPEndpointPath, nil)
	for _, origin := range origins {
		req.Header.Add(constants.OriginHeaderName, origin)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Code, reached
}

func (suite *OriginTestSuite) TestRequireAllowedOrigin_NoOriginPasses() {
	suite.initRuntime("")
	status, reached := suite.serve()

	suite.Equal(http.StatusOK, status)
	suite.True(reached)
}

func (suite *OriginTestSuite) TestRequireAllowedOrigin() {
	cases := []struct {
		name        string
		publicURL   string
		corsAllowed bool
		origins     []string
		allowed     bool
	}{
		{name: "server origin", origins: []string{testServerOrigin}, allowed: true},
		{name: "server origin with an upper-case host", origins: []string{"https://LOCALHOST:8090"},
			allowed: true},
		{name: "CORS allowed origin", corsAllowed: true, origins: []string{testCORSOrigin}, allowed: true},
		{name: "public URL origin", publicURL: testPublicURL, origins: []string{testPublicOrigin},
			allowed: true},
		{name: "bind address origin when a public URL is set", publicURL: testPublicURL,
			origins: []string{testServerOrigin}},
		{name: "untrusted origin", corsAllowed: true, origins: []string{"https://evil.example"}},
		{name: "server host on another port", origins: []string{"https://localhost:9443"}},
		{name: "server host over another scheme", origins: []string{"http://localhost:8090"}},
		{name: "origin with a path", origins: []string{testServerOrigin + "/mcp"}},
		{name: "malformed origin", origins: []string{"not-an-origin"}},
		{name: "empty origin", origins: []string{""}},
		{name: "null origin", origins: []string{"null"}},
		{name: "more than one origin", origins: []string{testServerOrigin, testServerOrigin}},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.initRuntime(tc.publicURL)
			if tc.corsAllowed {
				suite.allowCORSOrigin(testCORSOrigin)
			}

			status, reached := suite.serve(tc.origins...)

			if tc.allowed {
				suite.Equal(http.StatusOK, status)
				suite.True(reached)
			} else {
				suite.Equal(http.StatusForbidden, status)
				suite.False(reached)
			}
		})
	}
}

func (suite *OriginTestSuite) TestInitialize_UntrustedOriginRefusedBeforeGuard() {
	suite.initRuntime("")
	guardReached := false
	guard := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			guardReached = true
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
	mux := http.NewServeMux()
	Initialize(mux, NewServer(), guard, nil)

	for _, path := range []string{MCPEndpointPath, MCPEndpointPath + "/"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set(constants.OriginHeaderName, "https://evil.example")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, req)

		suite.Equal(http.StatusForbidden, recorder.Code, path)
	}
	suite.False(guardReached)
}
