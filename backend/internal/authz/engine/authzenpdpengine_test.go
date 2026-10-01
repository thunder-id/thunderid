// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authzen"
	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/system/config"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type legacyAuthZENPDP struct {
	*authZENPDPEngine
	service *legacyAuthZENPDPService
}

type AuthZENPDPEngineTestSuite struct {
	suite.Suite
}

func TestAuthZENPDPEngineTestSuite(t *testing.T) {
	suite.Run(t, new(AuthZENPDPEngineTestSuite))
}

type legacyAuthZENPDPService struct {
	authzenpdp.AuthZENPDPServiceInterface
	mu          sync.RWMutex
	connections map[string]authzenpdp.AuthZENPDPConnection
}

func (s *legacyAuthZENPDPService) GetAuthZENPDP(
	_ context.Context, id string,
) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connection := s.connections[id]
	return &connection, nil
}

func newLegacyAuthZENPDP(client httpservice.HTTPClientInterface) *legacyAuthZENPDP {
	service := &legacyAuthZENPDPService{connections: map[string]authzenpdp.AuthZENPDPConnection{}}
	pdpEngine, ok := newAuthZENPDPEngine(service, client).(*authZENPDPEngine)
	if !ok {
		panic("unexpected AuthZEN PDP engine type")
	}
	return &legacyAuthZENPDP{authZENPDPEngine: pdpEngine, service: service}
}

func (p *legacyAuthZENPDP) EvaluateAccess(
	ctx context.Context, config AuthZENPDPConfig, request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	p.configure(config, &request.ResourceServer)
	return p.authZENPDPEngine.EvaluateAccess(ctx, request)
}

func (p *legacyAuthZENPDP) EvaluateAccessBatch(
	ctx context.Context, config AuthZENPDPConfig, request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	for index := range request.Evaluations {
		p.configure(config, &request.Evaluations[index].ResourceServer)
	}
	return p.authZENPDPEngine.EvaluateAccessBatch(ctx, request)
}

func (p *legacyAuthZENPDP) configure(config AuthZENPDPConfig, resourceServer *ResourceServer) {
	connectionID := config.ResourceType
	if connectionID == "" {
		connectionID = "test-pdp"
	}
	connection := authzenpdp.AuthZENPDPConnection{
		ID: connectionID, Endpoint: config.Endpoint, BatchEndpoint: config.BatchEndpoint,
		TimeoutMS: int(config.Timeout.Milliseconds()), RetryCount: config.RetryCount,
		SubjectAttributeMappings: config.SubjectAttributeMappings,
	}
	p.service.mu.Lock()
	p.service.connections[connectionID] = connection
	p.service.mu.Unlock()
	if resourceServer.Properties == nil {
		resourceServer.Properties = map[string]interface{}{}
	}
	resourceServer.Identifier = config.ResourceType
	resourceServer.Engine.Properties.PDPConnectionID = connectionID
}

func TestMain(m *testing.M) {
	data, err := os.ReadFile("../../../cmd/server/config/default.json")
	if err != nil {
		panic(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		panic(err)
	}
	if err := config.InitializeServerRuntime("", &cfg); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPConcurrentConfigurations() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request authzen.AccessEvaluationsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if len(request.Evaluations) != 1 {
			t.Errorf("expected one evaluation, got %d", len(request.Evaluations))
			http.Error(w, "invalid batch", http.StatusBadRequest)
			return
		}
		evaluation := request.Evaluations[0]
		name := r.URL.Path[1:]
		if evaluation.Resource.Type != name || evaluation.Subject.Properties[name] != "engineering" {
			t.Errorf("configuration crossed requests: path=%s evaluation=%+v", name, evaluation)
		}
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true}]}`))
	}))
	t.Cleanup(server.Close)
	pdp := newLegacyAuthZENPDP(server.Client())
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			settings := AuthZENPDPConfig{
				ResourceType:  name,
				Endpoint:      server.URL + "/" + name,
				BatchEndpoint: server.URL + "/" + name,
				Timeout:       time.Second,
				SubjectAttributeMappings: []authzenpdp.SubjectAttributeMapping{{
					EntityType: "user",
					Attributes: []authzenpdp.SubjectAttributeRow{{Attribute: "department", PDPAttribute: name}},
				}},
			}
			for i := 0; i < 10; i++ {
				response, err := pdp.EvaluateAccessBatch(context.Background(), settings, AccessEvaluationsRequest{
					Evaluations: []AccessEvaluationRequest{{
						Subject: Subject{
							Category: "user", ID: "user", Type: "user",
							Properties: map[string]interface{}{"department": "engineering"},
						},
					}},
				})
				require.NoError(t, err)
				require.Len(t, response.Evaluations, 1)
				require.True(t, response.Evaluations[0].Decision)
			}
		})
	}
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccess() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluations", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var request authzen.AccessEvaluationsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Evaluations, 1)
		evaluation := request.Evaluations[0]
		require.Equal(t, "user", evaluation.Subject.Type)
		require.Equal(t, "user-1", evaluation.Subject.ID)
		require.Equal(t, "finance", evaluation.Subject.Properties["department_name"])
		require.Equal(t, []interface{}{"travel-agent"}, evaluation.Subject.Properties["group_ids"])
		require.NotContains(t, evaluation.Subject.Properties, "department")
		require.NotContains(t, evaluation.Subject.Properties, "email")
		require.NotContains(t, evaluation.Subject.Properties, subjectGroupsProperty)
		require.Equal(t, "external-resource", evaluation.Resource.Type)
		require.Equal(t, "read", evaluation.Action.Name)
		require.Equal(t, "authorization_code", evaluation.Context["grant_type"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true,"context":{"policy":"allow-read"}}]}`))
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{
		ResourceType:  "external-resource",
		Endpoint:      evaluationEndpoint(server),
		BatchEndpoint: batchEndpoint(server),
		SubjectAttributeMappings: []authzenpdp.SubjectAttributeMapping{{
			EntityType: "user",
			Attributes: []authzenpdp.SubjectAttributeRow{
				{Attribute: "department", PDPAttribute: "department_name"},
				{Attribute: subjectGroupsProperty, PDPAttribute: "group_ids"},
			},
		}},
	}
	engine := newLegacyAuthZENPDP(server.Client())

	response, err := engine.EvaluateAccess(context.Background(), settings, AccessEvaluationRequest{
		Subject: Subject{
			Category: "user", Type: "user",
			ID:       "user-1",
			GroupIDs: []string{"travel-agent"},
			Properties: map[string]interface{}{
				"department": "finance",
				"email":      "user@example.com",
			},
		},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "read"},
		Context:        map[string]interface{}{"grant_type": "authorization_code"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
	require.Equal(t, "allow-read", response.Context["policy"])
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessDeny() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":false,"context":{"reason":"denied"}}`))
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{Endpoint: evaluationEndpoint(server)}
	engine := newLegacyAuthZENPDP(server.Client())

	response, err := engine.EvaluateAccess(context.Background(), settings, AccessEvaluationRequest{
		Subject:        Subject{ID: "user-1"},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "delete"},
	})
	require.NoError(t, err)
	require.False(t, response.Decision)
	require.Equal(t, "denied", response.Context["reason"])
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPDoesNotSendUnconfiguredSubjectProperties() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		var request authzen.AccessEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Nil(t, request.Subject.Properties)
		require.NotContains(t, request.Subject.Properties, "department")
		require.NotContains(t, request.Subject.Properties, "email")
		require.NotContains(t, request.Subject.Properties, subjectGroupsProperty)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true}`))
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{Endpoint: evaluationEndpoint(server)}
	engine := newLegacyAuthZENPDP(server.Client())

	response, err := engine.EvaluateAccess(context.Background(), settings, AccessEvaluationRequest{
		Subject: Subject{
			Category: "user",
			ID:       "user-1",
			GroupIDs: []string{"travel-agent"},
			Properties: map[string]interface{}{
				"department": "finance",
				"email":      "user@example.com",
			},
		},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "read"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessUsesEntityTypeAttributeMapping() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		var request authzen.AccessEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "user", request.Subject.Type)
		require.Equal(t, "active", request.Subject.Properties["customer_status"])
		require.NotContains(t, request.Subject.Properties, "agent_status")
		require.NotContains(t, request.Subject.Properties, "status")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true}`))
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{
		Endpoint: evaluationEndpoint(server),
		SubjectAttributeMappings: []authzenpdp.SubjectAttributeMapping{{
			EntityType: "Customer",
			Attributes: []authzenpdp.SubjectAttributeRow{{
				Attribute: "status", PDPAttribute: "customer_status",
			}},
		}},
	}
	engine := newLegacyAuthZENPDP(server.Client())

	response, err := engine.EvaluateAccess(context.Background(), settings, AccessEvaluationRequest{
		Subject: Subject{
			Category:   "user",
			Type:       "Customer",
			ID:         "customer-1",
			Properties: map[string]interface{}{"status": "active"},
		},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "read"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessBatch() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluations", r.URL.Path)

		var request authzen.AccessEvaluationsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Evaluations, 2)
		require.Equal(t, "read", request.Evaluations[0].Action.Name)
		require.Equal(t, "cancel", request.Evaluations[1].Action.Name)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true},{"decision":false}]}`))
	}))
	defer server.Close()

	settings := authZENPDPConfig(server)
	pdp := newLegacyAuthZENPDP(server.Client())

	response, err := pdp.EvaluateAccessBatch(context.Background(), settings, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"},
				Permission: Permission{Name: "read"}},
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"},
				Permission: Permission{Name: "cancel"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, response.Evaluations, 2)
	require.True(t, response.Evaluations[0].Decision)
	require.False(t, response.Evaluations[1].Decision)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessBatchFallsBackToSingleEndpoint() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		var request authzen.AccessEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fmt.Appendf(nil, `{"decision":%t}`, request.Action.Name == "read"))
	}))
	defer server.Close()

	pdp := newLegacyAuthZENPDP(server.Client())
	response, err := pdp.EvaluateAccessBatch(context.Background(), AuthZENPDPConfig{
		Endpoint: evaluationEndpoint(server),
	}, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"},
				Permission: Permission{Name: "read"}},
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"},
				Permission: Permission{Name: "cancel"}},
		},
	})

	require.NoError(t, err)
	require.Len(t, response.Evaluations, 2)
	require.True(t, response.Evaluations[0].Decision)
	require.False(t, response.Evaluations[1].Decision)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessBatchEmpty() {
	t := suite.T()
	settings := AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
	}
	pdp := newLegacyAuthZENPDP(http.DefaultClient)

	response, err := pdp.EvaluateAccessBatch(context.Background(), settings, AccessEvaluationsRequest{})
	require.NoError(t, err)
	require.Empty(t, response.Evaluations)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessBatchRejectsMismatchedResponse() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"evaluations":[]}`))
	}))
	defer server.Close()

	settings := authZENPDPConfig(server)
	pdp := newLegacyAuthZENPDP(server.Client())

	_, err := pdp.EvaluateAccessBatch(context.Background(), settings, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{{
			Subject:        Subject{ID: "user-1"},
			ResourceServer: ResourceServer{ID: "resource-1"},
			Permission:     Permission{Name: "read"},
		}},
	})
	require.EqualError(t, err, "AuthZEN PDP returned 0 evaluations for 1 requests")
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPEvaluateAccessBatchUsesConfiguredBatchEndpoint() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/authzen/custom-batch", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		var request authzen.AccessEvaluationsRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Evaluations, 2)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true},{"decision":false}]}`))
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{
		Endpoint:      evaluationEndpoint(server),
		BatchEndpoint: server.URL + "/authzen/custom-batch",
	}
	pdp := newLegacyAuthZENPDP(server.Client())

	response, err := pdp.EvaluateAccessBatch(context.Background(), settings, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"},
				Permission: Permission{Name: "read"}},
			{Subject: Subject{ID: "user-1"}, ResourceServer: ResourceServer{ID: "resource-1"},
				Permission: Permission{Name: "cancel"}},
		},
	})

	require.NoError(t, err)
	require.Len(t, response.Evaluations, 2)
	require.True(t, response.Evaluations[0].Decision)
	require.False(t, response.Evaluations[1].Decision)
}

func (suite *AuthZENPDPEngineTestSuite) TestNormalizeSubjectAttributeMappings() {
	t := suite.T()
	result := normalizeSubjectAttributeMappings([]authzenpdp.SubjectAttributeMapping{
		{EntityType: " Customer ", Attributes: []authzenpdp.SubjectAttributeRow{
			{Attribute: " email ", PDPAttribute: " mail "},
			{Attribute: " groups "},
			{Attribute: "   "},
		}},
		{EntityType: "Empty", Attributes: []authzenpdp.SubjectAttributeRow{{Attribute: " "}}},
	})

	require.Equal(t, []authzenpdp.SubjectAttributeMapping{{
		EntityType: "Customer",
		Attributes: []authzenpdp.SubjectAttributeRow{
			{Attribute: "email", PDPAttribute: "mail"},
			{Attribute: "groups", PDPAttribute: "groups"},
		},
	}}, result)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPRequestRetries() {
	t := suite.T()
	var attemptsMu sync.Mutex
	attempts := 0
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		attemptsMu.Lock()
		attempts++
		currentAttempt := attempts
		attemptsMu.Unlock()
		if currentAttempt < 3 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true}`))
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{
		Endpoint:   evaluationEndpoint(server),
		RetryCount: 2,
	}
	pdp := newLegacyAuthZENPDP(server.Client())

	response, err := pdp.EvaluateAccess(context.Background(), settings, AccessEvaluationRequest{
		Subject:        Subject{ID: "user-1"},
		ResourceServer: ResourceServer{ID: "resource-1"},
		Permission:     Permission{Name: "read"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
	attemptsMu.Lock()
	totalAttempts := attempts
	attemptsMu.Unlock()
	require.Equal(t, 3, totalAttempts)
}

func (suite *AuthZENPDPEngineTestSuite) TestAuthZENPDPRejectsPDPError() {
	t := suite.T()
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	settings := AuthZENPDPConfig{Endpoint: evaluationEndpoint(server)}
	engine := newLegacyAuthZENPDP(server.Client())

	_, err := engine.EvaluateAccess(context.Background(), settings, AccessEvaluationRequest{})
	require.ErrorContains(t, err, "HTTP 503")
}

func (suite *AuthZENPDPEngineTestSuite) TestNewAuthZENPDPRejectsInvalidEndpoint() {
	t := suite.T()
	_, err := prepareAuthZENPDPSettings(AuthZENPDPConfig{Endpoint: "not-a-url"})
	require.Error(t, err)
}

func (suite *AuthZENPDPEngineTestSuite) TestNewAuthZENPDPAppliesDefaultTimeout() {
	t := suite.T()
	pdp, err := prepareAuthZENPDPSettings(AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
	})
	require.NoError(t, err)

	require.Equal(t, time.Second, pdp.timeout)
}

func (suite *AuthZENPDPEngineTestSuite) TestNewAuthZENPDPRequiresHTTPClient() {
	t := suite.T()
	_, err := newLegacyAuthZENPDP(nil).EvaluateAccess(context.Background(), AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
	}, AccessEvaluationRequest{})
	require.EqualError(t, err, "HTTP client is required")
}

func (suite *AuthZENPDPEngineTestSuite) TestNewAuthZENPDPRejectsInvalidBatchEndpoint() {
	t := suite.T()
	_, err := prepareAuthZENPDPSettings(AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "not-a-url",
	})
	require.EqualError(t, err, "invalid AuthZEN access evaluations endpoint: endpoint must be an absolute URL")
}

func (suite *AuthZENPDPEngineTestSuite) TestNewAuthZENPDPAcceptsMissingBatchEndpoint() {
	t := suite.T()
	settings, err := prepareAuthZENPDPSettings(AuthZENPDPConfig{
		Endpoint: "http://localhost:9000/access/v1/evaluation",
	})
	require.NoError(t, err)
	require.Empty(t, settings.batchEndpoint)
}

func (suite *AuthZENPDPEngineTestSuite) TestToAuthZENEvaluationRequestPreservesProxyResource() {
	t := suite.T()
	request := toAuthZENEvaluationRequest(AccessEvaluationRequest{
		Subject: Subject{Category: "user", ID: "user-1"},
		ResourceServer: ResourceServer{
			ID: "resource-server-1",
			Properties: map[string]interface{}{
				DelegatedResourceIDProperty: "booking-123",
				"status":                    "confirmed",
			},
		},
		Permission: Permission{Name: "booking:cancel"},
	}, "travel-booking-api", nil)

	require.Equal(t, "travel-booking-api", request.Resource.Type)
	require.Equal(t, "booking-123", request.Resource.ID)
	require.Equal(t, "confirmed", request.Resource.Properties["status"])
	require.NotContains(t, request.Resource.Properties, DelegatedResourceIDProperty)
}

func (suite *AuthZENPDPEngineTestSuite) TestEvaluationRequestDefaultsResourceFields() {
	t := suite.T()
	request := toAuthZENEvaluationRequest(AccessEvaluationRequest{
		Subject:        Subject{Category: "user", ID: "user-1"},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "read"},
	}, "", nil)

	require.Equal(t, "resource-server-1", request.Resource.Type)
	require.Equal(t, "resource-server-1", request.Resource.ID)
}

func evaluationEndpoint(server *httptest.Server) string {
	return server.URL + "/access/v1/evaluation"
}

func batchEndpoint(server *httptest.Server) string {
	return server.URL + "/access/v1/evaluations"
}

func authZENPDPConfig(server *httptest.Server) AuthZENPDPConfig {
	return AuthZENPDPConfig{
		Endpoint:      evaluationEndpoint(server),
		BatchEndpoint: batchEndpoint(server),
	}
}

func newAuthZENTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}
