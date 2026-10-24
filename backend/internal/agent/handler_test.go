// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/agent/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/tests/mocks/agentmock"
)

type InlineStubAgentService struct {
	OnCreateAgent func(
		ctx context.Context, agent *providers.Agent,
	) (*model.AgentCompleteResponse, *tidcommon.ServiceError)
	OnUpdateAgent func(
		ctx context.Context, id string, req *model.UpdateAgentRequest,
	) (*model.AgentCompleteResponse, *tidcommon.ServiceError)
	OnGetAgent func(
		ctx context.Context, id string, inc bool,
	) (*model.AgentGetResponse, *tidcommon.ServiceError)
	OnDeleteAgent  func(ctx context.Context, id string) *tidcommon.ServiceError
	OnGetAgentList func(
		ctx context.Context, limit, offset int, filters map[string]interface{}, inc bool,
	) (*model.AgentListResponse, *tidcommon.ServiceError)
	OnGetAgentGroups func(
		ctx context.Context, id string, limit, offset int,
	) (*model.AgentGroupListResponse, *tidcommon.ServiceError)
	OnGetAgentRoles func(
		ctx context.Context, id string, limit, offset int,
	) (*model.AgentRoleListResponse, *tidcommon.ServiceError)
}

func (s *InlineStubAgentService) CreateAgent(
	ctx context.Context, agent *providers.Agent) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if s.OnCreateAgent != nil {
		return s.OnCreateAgent(ctx, agent)
	}
	return &model.AgentCompleteResponse{}, nil
}

func (s *InlineStubAgentService) UpdateAgent(
	ctx context.Context, id string, req *model.UpdateAgentRequest,
) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
	if s.OnUpdateAgent != nil {
		return s.OnUpdateAgent(ctx, id, req)
	}
	return &model.AgentCompleteResponse{}, nil
}

func (s *InlineStubAgentService) GetAgent(
	ctx context.Context, id string, inc bool) (*model.AgentGetResponse, *tidcommon.ServiceError) {
	if s.OnGetAgent != nil {
		return s.OnGetAgent(ctx, id, inc)
	}
	return &model.AgentGetResponse{}, nil
}

func (s *InlineStubAgentService) DeleteAgent(
	ctx context.Context, id string) *tidcommon.ServiceError {
	if s.OnDeleteAgent != nil {
		return s.OnDeleteAgent(ctx, id)
	}
	return nil
}

func (s *InlineStubAgentService) GetAgentList(
	ctx context.Context, limit, offset int, filters map[string]interface{}, inc bool,
) (*model.AgentListResponse, *tidcommon.ServiceError) {
	if s.OnGetAgentList != nil {
		return s.OnGetAgentList(ctx, limit, offset, filters, inc)
	}
	return &model.AgentListResponse{Agents: []model.BasicAgentResponse{}, Links: []utils.Link{}}, nil
}

func (s *InlineStubAgentService) GetAgentGroups(
	ctx context.Context, id string, limit, offset int) (*model.AgentGroupListResponse, *tidcommon.ServiceError) {
	if s.OnGetAgentGroups != nil {
		return s.OnGetAgentGroups(ctx, id, limit, offset)
	}
	return &model.AgentGroupListResponse{}, nil
}

func (s *InlineStubAgentService) GetAgentRoles(
	ctx context.Context, id string, limit, offset int) (*model.AgentRoleListResponse, *tidcommon.ServiceError) {
	if s.OnGetAgentRoles != nil {
		return s.OnGetAgentRoles(ctx, id, limit, offset)
	}
	return &model.AgentRoleListResponse{}, nil
}

func (s *InlineStubAgentService) ValidateAgent(
	ctx context.Context, agent *providers.Agent, flowID string,
) (string, string, inboundmodel.InboundClient, *tidcommon.ServiceError) {
	return "", "", inboundmodel.InboundClient{}, nil
}

func (s *InlineStubAgentService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	return nil, nil
}

func (s *InlineStubAgentService) SetDependencyRegistry(resourcedependency.Registry) {}

func (s *InlineStubAgentService) GetClientRules(
	ctx context.Context, agentID string,
) (*providers.ClientRules, *tidcommon.ServiceError) {
	return nil, nil
}

func (s *InlineStubAgentService) GetOwnedAgents(
	ctx context.Context, ownerID string, kind providers.AgentKind, limit, offset int,
) (*model.OwnedAgentListResponse, *tidcommon.ServiceError) {
	return nil, nil
}

func (s *InlineStubAgentService) GetOwnedAgent(
	ctx context.Context, ownerID, agentID string, kind providers.AgentKind,
) (*model.OwnedAgentResponse, *tidcommon.ServiceError) {
	return nil, nil
}

func (s *InlineStubAgentService) UpdateOwnedAgent(
	ctx context.Context, ownerID, agentID string, kind providers.AgentKind, req *model.UpdateOwnedAgentRequest,
) (*model.OwnedAgentResponse, *tidcommon.ServiceError) {
	return nil, nil
}

func (s *InlineStubAgentService) DeleteOwnedAgent(
	ctx context.Context, ownerID, agentID string, kind providers.AgentKind,
) *tidcommon.ServiceError {
	return nil
}

func TestHandleAgentPostRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnCreateAgent: func(
			ctx context.Context, agent *providers.Agent,
		) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
			return &model.AgentCompleteResponse{ID: "agent-123"}, nil
		},
	}
	handler := newAgentHandler(stubService)
	goodJSON := `{"ouId": "ou-123", "type": "worker", "attributes": {"name": "Valid Agent Name"}, "owner": "admin-id"}`
	req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewBufferString(goodJSON))
	w := httptest.NewRecorder()

	handler.HandleAgentPostRequest(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleAgentPostRequest_PassesDelegatedToTheService(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		delegated bool
	}{
		{"DelegatedSet", `{"ouId": "ou-123", "type": "worker", "delegated": true}`, true},
		{"DelegatedAbsent", `{"ouId": "ou-123", "type": "worker"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var received *providers.Agent
			stubService := &InlineStubAgentService{
				OnCreateAgent: func(
					ctx context.Context, agent *providers.Agent,
				) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
					received = agent
					return &model.AgentCompleteResponse{ID: "agent-123"}, nil
				},
			}
			handler := newAgentHandler(stubService)
			req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			handler.HandleAgentPostRequest(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)
			if assert.NotNil(t, received) {
				assert.Equal(t, tt.delegated, received.Delegated)
			}
		})
	}
}

func TestHandleAgentPostRequest_ValidationError(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	badJSON := `{"type": "worker", "attributes": {"name": "ab"}}`
	req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewBufferString(badJSON))
	w := httptest.NewRecorder()

	handler.HandleAgentPostRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAgentPostRequest_InvalidJSONFormat(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewBufferString(`{bad`))
	w := httptest.NewRecorder()

	handler.HandleAgentPostRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAgentPutRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	goodJSON := `{"attributes": {"name": "Updated Valid Name"}}`
	req := httptest.NewRequest(http.MethodPut, "/agents/agent-123", bytes.NewBufferString(goodJSON))
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAgentPutRequest_ReportsTheDerivedStatus(t *testing.T) {
	mockSvc := agentmock.NewAgentServiceInterfaceMock(t)
	mockSvc.EXPECT().UpdateAgent(mock.Anything, "agent-123", mock.MatchedBy(func(req *model.UpdateAgentRequest) bool {
		return string(req.Attributes) == `{"name":"Updated Valid Name"}`
	})).Return(&model.AgentCompleteResponse{ID: "agent-123",
		Status: &providers.AccessStatusReport{Value: providers.AccessStatusActive}}, nil).Once()
	handler := newAgentHandler(mockSvc)
	body := `{"attributes":{"name":"Updated Valid Name"},"status":{"value":"SUSPENDED"}}`
	req := httptest.NewRequest(http.MethodPut, "/agents/agent-123", bytes.NewBufferString(body))
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response struct {
		Status json.RawMessage `json:"status"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.JSONEq(t, `{"value":"ACTIVE"}`, string(response.Status))
}

func TestHandleAgentPutRequest_ValidationError(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnUpdateAgent: func(
			ctx context.Context, id string, req *model.UpdateAgentRequest,
		) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
			return nil, &tidcommon.ServiceError{
				Code: "AGENT-4001",
				Type: tidcommon.ClientErrorType,
				Error: tidcommon.I18nMessage{
					Key:          "error.agent.validation_failed",
					DefaultValue: "Validation Failed",
				},
				ErrorDescription: tidcommon.I18nMessage{
					Key:          "error.agent.invalid_name_length",
					DefaultValue: "Agent name must be between 3 and 100 characters",
				},
			}
		},
	}
	handler := newAgentHandler(stubService)
	badJSON := `{"attributes": {"name": "a"}}`
	req := httptest.NewRequest(http.MethodPut, "/agents/agent-123", bytes.NewBufferString(badJSON))
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAgentPutRequest_MissingID(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodPut, "/agents/", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAgentPutRequest_InvalidJSONFormat(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodPut, "/agents/agent-123", bytes.NewBufferString(`{bad`))
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// The handler forwards the kind as given; the agent service refuses an unknown kind and applies the rules
// for a personal agent (an owner that is a user, and the personal grant types).
func TestHandleAgentPostRequest_KindForwarded(t *testing.T) {
	for _, kind := range []providers.AgentKind{providers.AgentKindPersonalAssistant, "personal_assistant"} {
		t.Run(string(kind), func(t *testing.T) {
			var forwarded providers.AgentKind
			stubService := &InlineStubAgentService{
				OnCreateAgent: func(
					ctx context.Context, agent *providers.Agent,
				) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
					forwarded = agent.Kind
					return &model.AgentCompleteResponse{}, nil
				},
			}
			handler := newAgentHandler(stubService)
			body := `{"ouId": "ou-123", "type": "worker", "attributes": {"name": "Valid Agent Name"},
				"kind": "` + string(kind) + `"}`
			req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewBufferString(body))
			w := httptest.NewRecorder()

			handler.HandleAgentPostRequest(w, req)
			assert.Equal(t, http.StatusCreated, w.Code)
			assert.Equal(t, kind, forwarded)
		})
	}
}

func TestHandleAgentPutRequest_PersonalAgentForbidden(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnUpdateAgent: func(
			ctx context.Context, id string, req *model.UpdateAgentRequest,
		) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
			return nil, &ErrorCannotUpdatePersonalAgent
		},
	}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodPut, "/agents/agent-123",
		bytes.NewBufferString(`{"attributes": {"name": "Updated Valid Name"}}`))
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleAgentGetRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent-123", nil)
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentGetRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAgentGetRequest_MissingID(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleAgentGetRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAgentDeleteRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodDelete, "/agents/agent-123", nil)
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentDeleteRequest(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandleAgentDeleteRequest_MissingID(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodDelete, "/agents/", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleAgentDeleteRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleAgentListRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	w := httptest.NewRecorder()

	handler.HandleAgentListRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAgentListRequest_InvalidFilter(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents?filter=invalidfilter", nil)
	w := httptest.NewRecorder()

	handler.HandleAgentListRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrorInvalidFilter.Code)
}

// ownerRequest returns a GET request whose token is for subject, delegated to an actor when delegated is set.
func ownerRequest(target, subject string, delegated bool) *http.Request {
	return ownerRequestWithBody(http.MethodGet, target, "", subject, delegated)
}

// ownerRequestWithBody returns a request with the given method and JSON body whose token is for subject,
// delegated to an actor when delegated is set.
func ownerRequestWithBody(method, target, body, subject string, delegated bool) *http.Request {
	var attributes map[string]interface{}
	if delegated {
		attributes = map[string]interface{}{"act": map[string]interface{}{"sub": "agent-1"}}
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if subject == "" {
		return req
	}
	return req.WithContext(security.WithSecurityContextTest(req.Context(),
		security.NewSecurityContextForTest(subject, "", "token", nil, attributes)))
}

// The owner routes need a user's own token: none is 401, and a delegated one, such as an agent's token for its
// owner, is 403. Neither reaches the service.
func TestHandleOwnedAgentRequests_CallerGuards(t *testing.T) {
	handler := newAgentHandler(agentmock.NewAgentServiceInterfaceMock(t))
	cases := []struct {
		name      string
		subject   string
		delegated bool
		status    int
		code      string
	}{
		{"no token", "", false, http.StatusUnauthorized, ErrorAuthenticationRequired.Code},
		{"delegated token", "user-1", true, http.StatusForbidden, ErrorDelegatedTokenNotAllowed.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.HandleOwnedAgentListRequest(w, ownerRequest("/users/me/agents",
				tc.subject, tc.delegated))
			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, w.Body.String(), tc.code)

			w = httptest.NewRecorder()
			req := ownerRequest("/users/me/agents/agent-1", tc.subject, tc.delegated)
			req.SetPathValue("id", "agent-1")
			handler.HandleOwnedAgentGetRequest(w, req)
			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, w.Body.String(), tc.code)
		})
	}
}

// The owner routes serve the caller's personal agents: the caller is the owner and the kind is fixed.
func TestHandleOwnedAgentRequests_PassTheCallerAndKind(t *testing.T) {
	svc := agentmock.NewAgentServiceInterfaceMock(t)
	svc.EXPECT().GetOwnedAgents(mock.Anything, "user-1", providers.AgentKindPersonalAssistant, 5, 10).
		Return(&model.OwnedAgentListResponse{TotalResults: 1}, nil).Once()
	svc.EXPECT().GetOwnedAgent(mock.Anything, "user-1", "agent-1", providers.AgentKindPersonalAssistant).
		Return(&model.OwnedAgentResponse{ID: "agent-1"}, nil).Once()
	handler := newAgentHandler(svc)

	w := httptest.NewRecorder()
	handler.HandleOwnedAgentListRequest(w,
		ownerRequest("/users/me/agents?limit=5&offset=10", "user-1", false))
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req := ownerRequest("/users/me/agents/agent-1", "user-1", false)
	req.SetPathValue("id", "agent-1")
	handler.HandleOwnedAgentGetRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"agent-1"`)
}

// The owner's update passes the body through; a field outside the editable set is refused before the service.
func TestHandleOwnedAgentPutRequest(t *testing.T) {
	svc := agentmock.NewAgentServiceInterfaceMock(t)
	svc.EXPECT().UpdateOwnedAgent(mock.Anything, "user-1", "agent-1", providers.AgentKindPersonalAssistant,
		mock.MatchedBy(func(req *model.UpdateOwnedAgentRequest) bool {
			return req.Description == "new" &&
				slices.Equal(req.GrantTypes, []providers.GrantType{providers.GrantTypeAuthorizationCode})
		})).Return(&model.OwnedAgentResponse{ID: "agent-1"}, nil).Once()
	handler := newAgentHandler(svc)

	req := ownerRequestWithBody(http.MethodPut, "/users/me/agents/agent-1",
		`{"description":"new","attributes":{"name":"x"},"grantTypes":["authorization_code"],`+
			`"redirectUris":["https://a.example.com/cb"]}`, "user-1", false)
	req.SetPathValue("id", "agent-1")
	w := httptest.NewRecorder()
	handler.HandleOwnedAgentPutRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	for _, body := range []string{`{"authFlowId":"flow-1"}`, `{"owner":"user-2"}`, `not json`} {
		req = ownerRequestWithBody(http.MethodPut, "/users/me/agents/agent-1", body, "user-1", false)
		req.SetPathValue("id", "agent-1")
		w = httptest.NewRecorder()
		handler.HandleOwnedAgentPutRequest(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
	}

	req = ownerRequestWithBody(http.MethodPut, "/users/me/agents/agent-1", `{"description":"x"}`, "user-1", true)
	req.SetPathValue("id", "agent-1")
	w = httptest.NewRecorder()
	handler.HandleOwnedAgentPutRequest(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// The owner's delete answers 204, and a delegated token never reaches the service.
func TestHandleOwnedAgentDeleteRequest(t *testing.T) {
	svc := agentmock.NewAgentServiceInterfaceMock(t)
	svc.EXPECT().DeleteOwnedAgent(mock.Anything, "user-1", "agent-1", providers.AgentKindPersonalAssistant).
		Return(nil).Once()
	handler := newAgentHandler(svc)

	req := ownerRequestWithBody(http.MethodDelete, "/users/me/agents/agent-1", "", "user-1", false)
	req.SetPathValue("id", "agent-1")
	w := httptest.NewRecorder()
	handler.HandleOwnedAgentDeleteRequest(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	req = ownerRequestWithBody(http.MethodDelete, "/users/me/agents/agent-1", "", "user-1", true)
	req.SetPathValue("id", "agent-1")
	w = httptest.NewRecorder()
	handler.HandleOwnedAgentDeleteRequest(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Not found from the service is 404, so another owner's agent is indistinguishable from a missing one.
func TestHandleOwnedAgentGetRequest_NotFound(t *testing.T) {
	svc := agentmock.NewAgentServiceInterfaceMock(t)
	svc.EXPECT().GetOwnedAgent(mock.Anything, "user-1", "agent-2", providers.AgentKindPersonalAssistant).
		Return(nil, &ErrorAgentNotFound).Once()
	req := ownerRequest("/users/me/agents/agent-2", "user-1", false)
	req.SetPathValue("id", "agent-2")
	w := httptest.NewRecorder()

	newAgentHandler(svc).HandleOwnedAgentGetRequest(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleAgentListRequest_ServiceError(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnGetAgentList: func(
			ctx context.Context, limit, offset int, filters map[string]interface{}, inc bool,
		) (*model.AgentListResponse, *tidcommon.ServiceError) {
			return nil, &tidcommon.InternalServerError
		},
	}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents", nil)
	w := httptest.NewRecorder()

	handler.HandleAgentListRequest(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleAgentGroupsRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent-123/groups", nil)
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentGroupsRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAgentGroupsRequest_MissingID(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents//groups", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleAgentGroupsRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrorMissingAgentID.Code)
}

func TestHandleAgentGroupsRequest_InvalidLimit(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent1/groups?limit=abc", nil)
	req.SetPathValue("id", "agent1")
	w := httptest.NewRecorder()

	handler.HandleAgentGroupsRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrorInvalidLimit.Code)
}

func TestHandleAgentGroupsRequest_ServiceError(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnGetAgentGroups: func(
			ctx context.Context, id string, limit, offset int,
		) (*model.AgentGroupListResponse, *tidcommon.ServiceError) {
			return nil, &ErrorAgentNotFound
		},
	}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent1/groups", nil)
	req.SetPathValue("id", "agent1")
	w := httptest.NewRecorder()

	handler.HandleAgentGroupsRequest(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), ErrorAgentNotFound.Code)
}

func TestHandleAgentRolesRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent-123/roles", nil)
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentRolesRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleAgentRolesRequest_MissingID(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents//roles", nil)
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	handler.HandleAgentRolesRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrorMissingAgentID.Code)
}

func TestHandleAgentRolesRequest_InvalidLimit(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent1/roles?limit=abc", nil)
	req.SetPathValue("id", "agent1")
	w := httptest.NewRecorder()

	handler.HandleAgentRolesRequest(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), ErrorInvalidLimit.Code)
}

func TestHandleAgentRolesRequest_ServiceError(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnGetAgentRoles: func(
			ctx context.Context, id string, limit, offset int,
		) (*model.AgentRoleListResponse, *tidcommon.ServiceError) {
			return nil, &ErrorAgentNotFound
		},
	}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent1/roles", nil)
	req.SetPathValue("id", "agent1")
	w := httptest.NewRecorder()

	handler.HandleAgentRolesRequest(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), ErrorAgentNotFound.Code)
}
