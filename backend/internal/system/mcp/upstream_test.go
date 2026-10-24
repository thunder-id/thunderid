// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

const (
	testCallerAuthorization  = "Bearer caller-token"
	testOwnInstructions      = "Create applications with thunderid_create_application."
	testUpstreamInstructions = "After creating an application, deploy it with deploy_to_environment."
)

type echoInput struct {
	Environment string `json:"environment"`
}

type echoOutput struct {
	Server      string `json:"server"`
	Environment string `json:"environment"`
}

// answering registers a tool that answers with which server it ran on.
func answering(server *mcpsdk.Server, name, which string) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: name, Description: name},
		func(_ context.Context, _ *mcpsdk.CallToolRequest, in echoInput) (*mcpsdk.CallToolResult, *echoOutput, error) {
			return nil, &echoOutput{Server: which, Environment: in.Environment}, nil
		})
}

type UpstreamTestSuite struct {
	suite.Suite
	upstream *httptest.Server
	// seen are the Authorization headers the upstream received.
	mu   sync.Mutex
	seen []string
}

func TestUpstreamTestSuite(t *testing.T) {
	suite.Run(t, new(UpstreamTestSuite))
}

// SetupTest starts an upstream MCP server that answers only the caller's token, with a tool of its
// own and one named like a tool of this server's.
func (s *UpstreamTestSuite) SetupTest() {
	s.seen = nil
	upstream := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "upstream", Version: "1.0.0"},
		&mcpsdk.ServerOptions{Instructions: testUpstreamInstructions})
	answering(upstream, "list_environments", "upstream")
	answering(upstream, "thunderid_local_tool", "upstream")
	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return upstream },
		&mcpsdk.StreamableHTTPOptions{Stateless: true})
	s.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.seen = append(s.seen, r.Header.Get("Authorization"))
		s.mu.Unlock()
		if r.Header.Get("Authorization") != testCallerAuthorization {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	s.T().Cleanup(s.upstream.Close)
}

// connect serves a server with its own tools, configured with cfg, and connects a client to it over
// HTTP with the given Authorization header.
func (s *UpstreamTestSuite) connect(cfg config.MCPServerConfig, authorization string) *mcpsdk.ClientSession {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "thunderid", Version: "1.0.0"},
		&mcpsdk.ServerOptions{Instructions: testOwnInstructions})
	answering(server, "thunderid_local_tool", "local")
	answering(server, "thunderid_hidden_tool", "local")
	serveConfiguredTools(server, cfg)
	srv := httptest.NewServer(mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return server },
		&mcpsdk.StreamableHTTPOptions{Stateless: true}))
	s.T().Cleanup(srv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "agent", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), &mcpsdk.StreamableClientTransport{
		Endpoint:   srv.URL,
		HTTPClient: &http.Client{Transport: &asCaller{authorization: authorization}},
	}, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = session.Close() })
	return session
}

func (s *UpstreamTestSuite) toolNames(session *mcpsdk.ClientSession) []string {
	var names []string
	for tool, err := range session.Tools(context.Background(), nil) {
		s.Require().NoError(err)
		names = append(names, tool.Name)
	}
	return names
}

func (s *UpstreamTestSuite) callFrom(session *mcpsdk.ClientSession, name string) (string, *mcpsdk.CallToolResult) {
	result, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name: name, Arguments: map[string]any{"environment": "dev"},
	})
	s.Require().NoError(err)
	output, _ := result.StructuredContent.(map[string]any)
	server, _ := output["server"].(string)
	return server, result
}

func (s *UpstreamTestSuite) withUpstream() config.MCPServerConfig {
	return config.MCPServerConfig{
		Upstreams:     []config.MCPUpstreamConfig{{URL: s.upstream.URL}},
		ExcludedTools: []string{"thunderid_hidden_tool"},
	}
}

// One server serves its own tools and the upstream's, without the excluded ones, and its own tool
// keeps its name against the upstream's.
func (s *UpstreamTestSuite) TestListsTheUpstreamsToolsBesideItsOwn() {
	session := s.connect(s.withUpstream(), testCallerAuthorization)

	s.ElementsMatch([]string{"thunderid_local_tool", "list_environments"}, s.toolNames(session))
}

// A call to an upstream tool is forwarded with the caller's token and answered as the upstream
// answered; a call to the server's own tool stays here.
func (s *UpstreamTestSuite) TestForwardsACallAsTheCaller() {
	session := s.connect(s.withUpstream(), testCallerAuthorization)

	server, result := s.callFrom(session, "list_environments")
	s.False(result.IsError)
	s.Equal("upstream", server)
	s.mu.Lock()
	s.NotEmpty(s.seen)
	for _, authorization := range s.seen {
		s.Equal(testCallerAuthorization, authorization)
	}
	s.mu.Unlock()

	server, _ = s.callFrom(session, "thunderid_local_tool")
	s.Equal("local", server)
}

// A caller the upstream refuses sees only the server's own tools.
func (s *UpstreamTestSuite) TestAnUpstreamThatRefusesTheCallerIsLeftOut() {
	session := s.connect(s.withUpstream(), "Bearer someone-else")

	s.Equal([]string{"thunderid_local_tool"}, s.toolNames(session))
}

// An upstream that is down leaves the server's own tools served.
func (s *UpstreamTestSuite) TestAnUpstreamThatIsDownIsLeftOut() {
	cfg := s.withUpstream()
	s.upstream.Close()
	session := s.connect(cfg, testCallerAuthorization)

	s.Equal([]string{"thunderid_local_tool"}, s.toolNames(session))
	_, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "list_environments"})
	s.Error(err, "a tool no server serves is unknown")
}

// Without upstreams or exclusions the server serves its own tools as they are.
func (s *UpstreamTestSuite) TestWithoutConfigurationNothingChanges() {
	session := s.connect(config.MCPServerConfig{}, testCallerAuthorization)

	s.ElementsMatch([]string{"thunderid_local_tool", "thunderid_hidden_tool"}, s.toolNames(session))
}

// A client learns how to use the upstream's tools from the one server it connects to: the upstream's
// instructions follow the server's own.
func (s *UpstreamTestSuite) TestTheUpstreamsInstructionsFollowTheServersOwn() {
	session := s.connect(s.withUpstream(), testCallerAuthorization)

	s.Equal(testOwnInstructions+"\n\n"+testUpstreamInstructions, session.InitializeResult().Instructions)
}

// A caller the upstream refuses is given the server's own instructions alone.
func (s *UpstreamTestSuite) TestAnUpstreamThatRefusesTheCallerAddsNoInstructions() {
	session := s.connect(s.withUpstream(), "Bearer someone-else")

	s.Equal(testOwnInstructions, session.InitializeResult().Instructions)
}

// An upstream that redirects is not followed, so the caller's token reaches no host but the configured
// one, and the upstream is left out.
func (s *UpstreamTestSuite) TestARedirectingUpstreamIsNotFollowed() {
	var leaked []string
	var leakedMu sync.Mutex
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leakedMu.Lock()
		leaked = append(leaked, r.Header.Get("Authorization"))
		leakedMu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	s.T().Cleanup(elsewhere.Close)
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL, http.StatusTemporaryRedirect)
	}))
	s.T().Cleanup(redirecting.Close)

	session := s.connect(config.MCPServerConfig{Upstreams: []config.MCPUpstreamConfig{{URL: redirecting.URL}}},
		testCallerAuthorization)

	s.ElementsMatch([]string{"thunderid_local_tool", "thunderid_hidden_tool"}, s.toolNames(session))
	leakedMu.Lock()
	defer leakedMu.Unlock()
	s.Empty(leaked, "the redirect was followed")
}

// An upstream that accepts the connection and never answers is left out once the read times out,
// rather than holding the server's own tools back.
func (s *UpstreamTestSuite) TestAHungUpstreamIsLeftOut() {
	previous := upstreamReadTimeout
	upstreamReadTimeout = 200 * time.Millisecond
	s.T().Cleanup(func() { upstreamReadTimeout = previous })
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	s.T().Cleanup(func() { close(release); hung.Close() })

	session := s.connect(config.MCPServerConfig{Upstreams: []config.MCPUpstreamConfig{{URL: hung.URL}}},
		testCallerAuthorization)

	started := time.Now()
	s.ElementsMatch([]string{"thunderid_local_tool", "thunderid_hidden_tool"}, s.toolNames(session))
	s.Less(time.Since(started), 3*time.Second)
}
