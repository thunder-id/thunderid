// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthZENPDPEvaluateAccess(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var request authZENEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "user", request.Subject.Type)
		require.Equal(t, "user-1", request.Subject.ID)
		require.Equal(t, "finance", request.Subject.Properties["department_name"])
		require.Equal(t, []interface{}{"travel-agent"}, request.Subject.Properties["group_ids"])
		require.NotContains(t, request.Subject.Properties, "department")
		require.NotContains(t, request.Subject.Properties, "email")
		require.NotContains(t, request.Subject.Properties, subjectGroupsProperty)
		require.Equal(t, "external-resource", request.Resource.Type)
		require.Equal(t, "read", request.Action.Name)
		require.Equal(t, "authorization_code", request.Context["grant_type"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true,"context":{"policy":"allow-read"}}`))
	}))
	defer server.Close()

	engine, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:                evaluationEndpoint(server),
		BatchEndpoint:           batchEndpoint(server),
		SubjectProperties:       []string{"department", subjectGroupsProperty},
		SubjectPropertyMappings: map[string]string{"department": "department_name", subjectGroupsProperty: "group_ids"},
	}, server.Client())
	require.NoError(t, err)

	response, err := engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		Subject: Subject{
			Type:     "user",
			ID:       "user-1",
			GroupIDs: []string{"travel-agent"},
			Properties: map[string]interface{}{
				"department": "finance",
				"email":      "user@example.com",
			},
		},
		ResourceServer: ResourceServer{
			ID:   "resource-server-1",
			Type: "external-resource",
		},
		Permission: Permission{Name: "read"},
		Context:    map[string]interface{}{"grant_type": "authorization_code"},
	})
	require.NoError(t, err)
	require.True(t, response.Decision)
	require.Equal(t, "allow-read", response.Context["policy"])
}

func TestAuthZENPDPEvaluateAccessDeny(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":false,"context":{"reason":"denied"}}`))
	}))
	defer server.Close()

	engine, err := newAuthZENPDP(authZENPDPConfig(server), server.Client())
	require.NoError(t, err)

	response, err := engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		Subject:        Subject{ID: "user-1"},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "delete"},
	})
	require.NoError(t, err)
	require.False(t, response.Decision)
	require.Equal(t, "denied", response.Context["reason"])
}

func TestAuthZENPDPDoesNotSendUnconfiguredSubjectProperties(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		var request authZENEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Nil(t, request.Subject.Properties)
		require.NotContains(t, request.Subject.Properties, "department")
		require.NotContains(t, request.Subject.Properties, "email")
		require.NotContains(t, request.Subject.Properties, subjectGroupsProperty)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true}`))
	}))
	defer server.Close()

	engine, err := newAuthZENPDP(authZENPDPConfig(server), server.Client())
	require.NoError(t, err)

	response, err := engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		Subject: Subject{
			Type:     "user",
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

func TestAuthZENPDPEvaluateAccessUsesSubjectTypeAttributeMapping(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		var request authZENEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "Customer", request.Subject.Type)
		require.Equal(t, "active", request.Subject.Properties["customer_status"])
		require.NotContains(t, request.Subject.Properties, "agent_status")
		require.NotContains(t, request.Subject.Properties, "status")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"decision":true}`))
	}))
	defer server.Close()

	engine, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      evaluationEndpoint(server),
		BatchEndpoint: batchEndpoint(server),
		SubjectAttributeMappings: []SubjectAttributeMapping{
			{
				UserType:   "Agent",
				Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "agent_status"}},
			},
			{
				UserType:   "Customer",
				Attributes: []SubjectAttributeRow{{Attribute: "status", PDPAttribute: "customer_status"}},
			},
		},
	}, server.Client())
	require.NoError(t, err)

	response, err := engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{
		Subject: Subject{
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

func TestAuthZENPDPEvaluateAccessBatch(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluations", r.URL.Path)

		var request authZENBatchEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Evaluations, 2)
		require.Equal(t, "read", request.Evaluations[0].Action.Name)
		require.Equal(t, "cancel", request.Evaluations[1].Action.Name)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true},{"decision":false}]}`))
	}))
	defer server.Close()

	pdp, err := newAuthZENPDP(authZENPDPConfig(server), server.Client())
	require.NoError(t, err)

	response, err := pdp.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{
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

func TestAuthZENPDPEvaluateAccessBatchEmpty(t *testing.T) {
	pdp, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
	}, http.DefaultClient)
	require.NoError(t, err)

	response, err := pdp.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{})
	require.NoError(t, err)
	require.Empty(t, response.Evaluations)
}

func TestAuthZENPDPEvaluateAccessBatchRejectsMismatchedResponse(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"evaluations":[]}`))
	}))
	defer server.Close()

	pdp, err := newAuthZENPDP(authZENPDPConfig(server), server.Client())
	require.NoError(t, err)

	_, err = pdp.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{{
			Subject:        Subject{ID: "user-1"},
			ResourceServer: ResourceServer{ID: "resource-1"},
			Permission:     Permission{Name: "read"},
		}},
	})
	require.EqualError(t, err, "AuthZEN PDP returned 0 evaluations for 1 requests")
}

func TestAuthZENPDPEvaluateAccessBatchUsesConfiguredBatchEndpoint(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/authzen/custom-batch", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		var request authZENBatchEvaluationRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Len(t, request.Evaluations, 2)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"evaluations":[{"decision":true},{"decision":false}]}`))
	}))
	defer server.Close()

	pdp, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      evaluationEndpoint(server),
		BatchEndpoint: server.URL + "/authzen/custom-batch",
	}, server.Client())
	require.NoError(t, err)

	response, err := pdp.EvaluateAccessBatch(context.Background(), AccessEvaluationsRequest{
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

func TestAuthZENEvaluationAuditFieldsMaskOptionalValues(t *testing.T) {
	request := authZENEvaluationRequest{
		Subject: authZENSubject{
			Type: "user", ID: "user-123",
			Properties: map[string]interface{}{"email": "user@example.com"},
		},
		Resource: authZENResource{
			Type: "booking", ID: "booking-123",
			Properties: map[string]interface{}{"owner": "user-123"},
		},
		Action:  authZENAction{Name: "read", Properties: map[string]interface{}{"source": "api"}},
		Context: map[string]interface{}{"ip": "127.0.0.1"},
	}

	fields := authZENEvaluationLogFields(request)
	require.Len(t, fields, 9)
	require.Equal(t, "subject_id", fields[0].Key)
	require.NotEqual(t, request.Subject.ID, fields[0].Value)
	require.Equal(t, "subject_type", fields[1].Key)
	require.Equal(t, request.Subject.Type, fields[1].Value)
}

func TestAuditAuthZENEvaluationsMasksSensitiveValues(t *testing.T) {
	evaluations := auditAuthZENEvaluations([]authZENBatchEvaluation{{
		Subject: authZENSubject{
			Type: "user", ID: "user-123",
			Properties: map[string]interface{}{"email": "user@example.com"},
		},
		Resource: authZENResource{
			Type: "booking", ID: "booking-123",
			Properties: map[string]interface{}{"owner": "user-123"},
		},
		Action:  authZENAction{Name: "read", Properties: map[string]interface{}{"source": "api"}},
		Context: map[string]interface{}{"ip": "127.0.0.1"},
	}})

	require.Len(t, evaluations, 1)
	require.Equal(t, "user", evaluations[0]["subject"].(map[string]interface{})["type"])
	require.NotEqual(t, "user-123", evaluations[0]["subject"].(map[string]interface{})["id"])
	subjectProperties := evaluations[0]["subject"].(map[string]interface{})["properties"].(map[string]interface{})
	require.NotEqual(t, "user@example.com", subjectProperties["email"])
}

func TestNormalizeSubjectAttributeMappings(t *testing.T) {
	result := normalizeSubjectAttributeMappings([]SubjectAttributeMapping{
		{UserType: " Customer ", Attributes: []SubjectAttributeRow{
			{Attribute: " email ", PDPAttribute: " mail "},
			{Attribute: " groups "},
			{Attribute: "   "},
		}},
		{UserType: "Empty", Attributes: []SubjectAttributeRow{{Attribute: " "}}},
	})

	require.Equal(t, []SubjectAttributeMapping{{
		UserType: "Customer",
		Attributes: []SubjectAttributeRow{
			{Attribute: "email", PDPAttribute: "mail"},
			{Attribute: "groups", PDPAttribute: "groups"},
		},
	}}, result)
}

func TestAuthZENPDPRequestRetries(t *testing.T) {
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

	pdp, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      evaluationEndpoint(server),
		BatchEndpoint: batchEndpoint(server),
		RetryCount:    2,
	}, server.Client())
	require.NoError(t, err)

	response, err := pdp.EvaluateAccess(context.Background(), AccessEvaluationRequest{
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

func TestAuthZENPDPRejectsPDPError(t *testing.T) {
	server := newAuthZENTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/access/v1/evaluation", r.URL.Path)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	engine, err := newAuthZENPDP(authZENPDPConfig(server), server.Client())
	require.NoError(t, err)

	_, err = engine.EvaluateAccess(context.Background(), AccessEvaluationRequest{})
	require.ErrorContains(t, err, "HTTP 503")
}

func TestNewAuthZENPDPRejectsInvalidEndpoint(t *testing.T) {
	_, err := newAuthZENPDP(AuthZENPDPConfig{Endpoint: "not-a-url"}, http.DefaultClient)
	require.Error(t, err)
}

func TestNewAuthZENPDPAppliesDefaultTimeout(t *testing.T) {
	authorizationEngine, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
	}, http.DefaultClient)
	require.NoError(t, err)

	pdp, ok := authorizationEngine.(*authZENPDP)
	require.True(t, ok)
	require.Equal(t, defaultAuthZENPDPTimeout, pdp.timeout)
}

func TestNewAuthZENPDPRequiresHTTPClient(t *testing.T) {
	_, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "http://localhost:9000/access/v1/evaluations",
	}, nil)
	require.EqualError(t, err, "HTTP client is required")
}

func TestNewAuthZENPDPRejectsInvalidBatchEndpoint(t *testing.T) {
	_, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:      "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint: "not-a-url",
	}, http.DefaultClient)
	require.EqualError(t, err, "invalid AuthZEN access evaluations endpoint: endpoint must be an absolute URL")
}

func TestNewAuthZENPDPRequiresBatchEndpoint(t *testing.T) {
	_, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint: "http://localhost:9000/access/v1/evaluation",
	}, http.DefaultClient)
	require.EqualError(t, err, "AuthZEN access evaluations endpoint is required")
}

func TestNewAuthZENPDPRejectsMappingForDisallowedProperty(t *testing.T) {
	_, err := newAuthZENPDP(AuthZENPDPConfig{
		Endpoint:                "http://localhost:9000/access/v1/evaluation",
		BatchEndpoint:           "http://localhost:9000/access/v1/evaluations",
		SubjectPropertyMappings: map[string]string{"department": "department_name"},
	}, http.DefaultClient)
	require.EqualError(t, err, "subject property \"department\" has a mapping but is not allowed")
}

func TestToAuthZENEvaluationRequestPreservesProxyResource(t *testing.T) {
	request := toAuthZENEvaluationRequest(AccessEvaluationRequest{
		Subject: Subject{Type: "user", ID: "user-1"},
		ResourceServer: ResourceServer{
			ID:         "resource-server-1",
			Type:       "travel-booking-api",
			ResourceID: "booking-123",
			Properties: map[string]interface{}{"status": "confirmed"},
		},
		Permission: Permission{Name: "booking:cancel"},
	}, nil, nil, nil)

	require.Equal(t, "travel-booking-api", request.Resource.Type)
	require.Equal(t, "booking-123", request.Resource.ID)
	require.Equal(t, "confirmed", request.Resource.Properties["status"])
}

func TestToAuthZENEvaluationRequestUsesResourceServerIDWhenResourceFieldsAreMissing(t *testing.T) {
	request := toAuthZENEvaluationRequest(AccessEvaluationRequest{
		Subject:        Subject{Type: "user", ID: "user-1"},
		ResourceServer: ResourceServer{ID: "resource-server-1"},
		Permission:     Permission{Name: "read"},
	}, nil, nil, nil)

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
