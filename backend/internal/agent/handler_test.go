package agent

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thunder-id/thunderid/internal/agent/model"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

type InlineStubAgentService struct {
	OnCreateAgent func(ctx context.Context, agent *model.Agent) (*model.AgentCompleteResponse, *serviceerror.ServiceError)
	OnUpdateAgent func(ctx context.Context, id string, req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *serviceerror.ServiceError)
	OnGetAgent    func(ctx context.Context, id string, inc bool) (*model.AgentGetResponse, *serviceerror.ServiceError)
	OnDeleteAgent func(ctx context.Context, id string) *serviceerror.ServiceError
}

func (s *InlineStubAgentService) CreateAgent(
	ctx context.Context, agent *model.Agent) (*model.AgentCompleteResponse, *serviceerror.ServiceError) {
	if s.OnCreateAgent != nil {
		return s.OnCreateAgent(ctx, agent)
	}
	return &model.AgentCompleteResponse{}, nil
}

func (s *InlineStubAgentService) UpdateAgent(
	ctx context.Context, id string, req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *serviceerror.ServiceError) {
	if s.OnUpdateAgent != nil {
		return s.OnUpdateAgent(ctx, id, req)
	}
	return &model.AgentCompleteResponse{}, nil
}

func (s *InlineStubAgentService) GetAgent(
	ctx context.Context, id string, inc bool) (*model.AgentGetResponse, *serviceerror.ServiceError) {
	if s.OnGetAgent != nil {
		return s.OnGetAgent(ctx, id, inc)
	}
	return &model.AgentGetResponse{}, nil
}

func (s *InlineStubAgentService) DeleteAgent(
	ctx context.Context, id string) *serviceerror.ServiceError {
	if s.OnDeleteAgent != nil {
		return s.OnDeleteAgent(ctx, id)
	}
	return nil
}

func (s *InlineStubAgentService) GetAgentList(
	ctx context.Context, limit, offset int, filters map[string]interface{}, inc bool) (*model.AgentListResponse, *serviceerror.ServiceError) {
	return &model.AgentListResponse{Agents: []model.BasicAgentResponse{}, Links: []utils.Link{}}, nil
}

func (s *InlineStubAgentService) GetAgentGroups(
	ctx context.Context, id string, limit, offset int) (*model.AgentGroupListResponse, *serviceerror.ServiceError) {
	return &model.AgentGroupListResponse{}, nil
}

func (s *InlineStubAgentService) ValidateAgent(
	ctx context.Context, agent *model.Agent, flowID string) (string, string, inboundmodel.InboundClient, *serviceerror.ServiceError) {
	return "", "", inboundmodel.InboundClient{}, nil
}

func TestHandleAgentPostRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{
		OnCreateAgent: func(ctx context.Context, agent *model.Agent) (*model.AgentCompleteResponse, *serviceerror.ServiceError) {
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
		OnUpdateAgent: func(ctx context.Context, id string, req *model.UpdateAgentRequest) (*model.AgentCompleteResponse, *serviceerror.ServiceError) {
			return nil, &serviceerror.ServiceError{
				Code: "AGENT-4001",
				Type: serviceerror.ClientErrorType,
				// ◄── THE FIX: Initialize using I18nMessage fields
				Error: core.I18nMessage{
					Key:          "error.agent.validation_failed",
					DefaultValue: "Validation Failed",
				},
				ErrorDescription: core.I18nMessage{
					Key:          "error.agent.invalid_name_length",
					DefaultValue: "Agent name must be between 3 and 100 characters",
				},
			}
		},
	}
	handler := newAgentHandler(stubService)
	badJSON := `{"name": "a"}` // Too short
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

func TestHandleAgentGroupsRequest_Success(t *testing.T) {
	stubService := &InlineStubAgentService{}
	handler := newAgentHandler(stubService)
	req := httptest.NewRequest(http.MethodGet, "/agents/agent-123/groups", nil)
	req.SetPathValue("id", "agent-123")
	w := httptest.NewRecorder()

	handler.HandleAgentGroupsRequest(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
