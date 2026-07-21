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

package agent

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/agent/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

type InlineStubAgentService struct {
	OnCreateAgent func(
		ctx context.Context, agent *model.Agent,
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
	ctx context.Context, agent *model.Agent) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
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
	ctx context.Context, agent *model.Agent, flowID string,
) (string, string, inboundmodel.InboundClient, *tidcommon.ServiceError) {
	return "", "", inboundmodel.InboundClient{}, nil
}

func (s *InlineStubAgentService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	return nil, nil
}

func (s *InlineStubAgentService) SetDependencyRegistry(resourcedependency.Registry) {}

func TestHandleAgentPostRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnCreateAgent: func(
			ctx context.Context, agent *model.Agent,
		) (*model.AgentCompleteResponse, *tidcommon.ServiceError) {
			return &model.AgentCompleteResponse{ID: "agent-123"}, nil
		},
	}
	handler := newAgentHandler(stubService)
	goodJSON := `{"ouId": "ou-123", "type": "worker", "name": "Valid Agent Name", "owner": "admin-id"}`
	req := httptest.NewRequest(http.MethodPost, "/agents", bytes.NewBufferString(goodJSON))
	w := httptest.NewRecorder()

	handler.HandleAgentPostRequest(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleAgentPostRequest_ValidationError(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	badJSON := `{"name": "ab", "type": "worker"}`
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
	goodJSON := `{"name": "Updated Valid Name"}`
	req := httptest.NewRequest(http.MethodPut, "/agents/agent-123", bytes.NewBufferString(goodJSON))
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentPutRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
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
	badJSON := `{"name": "a"}`
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
