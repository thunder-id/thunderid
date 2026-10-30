// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/agent/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
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

func TestHandleAgentListRequest_WithOrganizationUnitFilter(t *testing.T) {
	var gotFilters map[string]interface{}
	stubService := &InlineStubAgentService{
		OnGetAgentList: func(
			ctx context.Context, limit, offset int, filters map[string]interface{}, inc bool,
		) (*model.AgentListResponse, *tidcommon.ServiceError) {
			gotFilters = filters
			return &model.AgentListResponse{}, nil
		},
	}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, `/agents?filter=ouId+eq+%22ou-1%22`, nil)
	w := httptest.NewRecorder()

	handler.HandleAgentListRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, map[string]interface{}{"ouId": "ou-1"}, gotFilters)
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
