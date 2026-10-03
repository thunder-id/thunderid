// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const createTemplateReqBody = `{"handle":"otp","displayName":"OTP","content":{"body":"b"}}`

type HandlerTestSuite struct {
	suite.Suite
	mockService *NotificationTemplateServiceInterfaceMock
	handler     *notificationTemplateHandler
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) SetupTest() {
	s.mockService = NewNotificationTemplateServiceInterfaceMock(s.T())
	s.handler = newNotificationTemplateHandler(s.mockService)
}

func errorCode(body []byte) string {
	var resp struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.Code
}

func (s *HandlerTestSuite) TestHandleTemplateList_OK_DefaultsLimit() {
	// The handler applies the default page size of 30 and offset 0 when none are supplied; the
	// expectation below matching 30/0 is what proves it.
	s.mockService.On("ListTemplates", mock.Anything, ChannelTypeEmail, 30, 0).
		Return(&TemplateListResponse{
			TotalResults: 1,
			Count:        1,
			Templates:    []TemplateSummary{{ID: "1", Handle: "otp", DisplayName: "OTP"}},
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/notification-templates/email/templates", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplateListRequest(w, req)

	s.Require().Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleTemplateList_PassesPagination() {
	s.mockService.On("ListTemplates", mock.Anything, ChannelTypeEmail, 5, 10).
		Return(&TemplateListResponse{Templates: []TemplateSummary{}}, nil)

	req := httptest.NewRequest(http.MethodGet, "/notification-templates/email/templates?limit=5&offset=10", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplateListRequest(w, req)

	s.Require().Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleTemplateList_BadLimit() {
	req := httptest.NewRequest(http.MethodGet, "/notification-templates/email/templates?limit=abc", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplateListRequest(w, req)

	s.Require().Equal(http.StatusBadRequest, w.Code)
	s.Require().Equal(ErrorInvalidLimit.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplatePost_Created() {
	s.mockService.On("CreateTemplate", mock.Anything, ChannelTypeEmail, mock.Anything).
		Return(&Template{ID: "abc", Handle: "otp", DisplayName: "OTP"}, nil)

	body := createTemplateReqBody
	req := httptest.NewRequest(http.MethodPost, "/notification-templates/email/templates", strings.NewReader(body))
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplatePostRequest(w, req)

	s.Require().Equal(http.StatusCreated, w.Code)
	s.Require().Equal(buildSelf(ChannelTypeEmail, "abc"), w.Header().Get("Location"))
}

func (s *HandlerTestSuite) TestHandleTemplatePost_BadJSON() {
	req := httptest.NewRequest(http.MethodPost, "/notification-templates/email/templates", strings.NewReader("{"))
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplatePostRequest(w, req)

	s.Require().Equal(http.StatusBadRequest, w.Code)
	s.Require().Equal(ErrorInvalidTemplateData.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplatePost_HandleConflict() {
	s.mockService.On("CreateTemplate", mock.Anything, ChannelTypeEmail, mock.Anything).
		Return(nil, &ErrorTemplateHandleConflict)

	body := createTemplateReqBody
	req := httptest.NewRequest(http.MethodPost, "/notification-templates/email/templates", strings.NewReader(body))
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplatePostRequest(w, req)

	s.Require().Equal(http.StatusConflict, w.Code)
	s.Require().Equal(ErrorTemplateHandleConflict.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplatePost_MissingSubject() {
	s.mockService.On("CreateTemplate", mock.Anything, ChannelTypeEmail, mock.Anything).
		Return(nil, &ErrorMissingSubject)

	body := createTemplateReqBody
	req := httptest.NewRequest(http.MethodPost, "/notification-templates/email/templates", strings.NewReader(body))
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplatePostRequest(w, req)

	s.Require().Equal(http.StatusBadRequest, w.Code)
	s.Require().Equal(ErrorMissingSubject.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplateGet_NotFound() {
	s.mockService.On("GetTemplate", mock.Anything, ChannelTypeEmail, "x").Return(nil, &ErrorTemplateNotFound)

	req := httptest.NewRequest(http.MethodGet, "/notification-templates/email/templates/x", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	s.handler.HandleTemplateGetRequest(w, req)

	s.Require().Equal(http.StatusNotFound, w.Code)
	s.Require().Equal(ErrorTemplateNotFound.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplateGet_OK() {
	s.mockService.On("GetTemplate", mock.Anything, ChannelTypeEmail, "1").
		Return(&Template{ID: "1", Handle: "otp", DisplayName: "OTP"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/notification-templates/email/templates/1", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handler.HandleTemplateGetRequest(w, req)

	s.Require().Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleTemplatePut_OK() {
	s.mockService.On("UpdateTemplate", mock.Anything, ChannelTypeEmail, "1", mock.Anything).
		Return(&Template{ID: "1", Handle: "otp", DisplayName: "Renamed"}, nil)

	body := `{"displayName":"Renamed","content":{"body":"b"}}`
	req := httptest.NewRequest(http.MethodPut, "/notification-templates/email/templates/1", strings.NewReader(body))
	req.SetPathValue("channel", string(ChannelTypeEmail))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handler.HandleTemplatePutRequest(w, req)

	s.Require().Equal(http.StatusOK, w.Code)
}

func (s *HandlerTestSuite) TestHandleTemplateDelete_NoContent() {
	s.mockService.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "1").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/notification-templates/email/templates/1", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handler.HandleTemplateDeleteRequest(w, req)

	s.Require().Equal(http.StatusNoContent, w.Code)
}

func (s *HandlerTestSuite) TestHandleTemplateDelete_InUse() {
	s.mockService.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, "1").Return(&ErrorTemplateInUse)

	req := httptest.NewRequest(http.MethodDelete, "/notification-templates/email/templates/1", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handler.HandleTemplateDeleteRequest(w, req)

	s.Require().Equal(http.StatusConflict, w.Code)
	s.Require().Equal(ErrorTemplateInUse.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplateList_ServiceError_500() {
	s.mockService.On("ListTemplates", mock.Anything, ChannelTypeEmail, 30, 0).
		Return(nil, &tidcommon.InternalServerError)

	req := httptest.NewRequest(http.MethodGet, "/notification-templates/email/templates", nil)
	req.SetPathValue("channel", string(ChannelTypeEmail))
	w := httptest.NewRecorder()
	s.handler.HandleTemplateListRequest(w, req)

	s.Require().Equal(http.StatusInternalServerError, w.Code)
}

func (s *HandlerTestSuite) TestHandleTemplateGet_InvalidChannel_400() {
	s.mockService.On("GetTemplate", mock.Anything, ChannelType("push"), "1").Return(nil, &ErrorInvalidChannel)

	req := httptest.NewRequest(http.MethodGet, "/notification-templates/push/templates/1", nil)
	req.SetPathValue("channel", "push")
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handler.HandleTemplateGetRequest(w, req)

	s.Require().Equal(http.StatusBadRequest, w.Code)
	s.Require().Equal(ErrorInvalidChannel.Code, errorCode(w.Body.Bytes()))
}

func (s *HandlerTestSuite) TestHandleTemplatePut_NotFound() {
	s.mockService.On("UpdateTemplate", mock.Anything, ChannelTypeEmail, "1", mock.Anything).
		Return(nil, &ErrorTemplateNotFound)

	body := `{"displayName":"x","content":{"body":"b"}}`
	req := httptest.NewRequest(http.MethodPut, "/notification-templates/email/templates/1", strings.NewReader(body))
	req.SetPathValue("channel", string(ChannelTypeEmail))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	s.handler.HandleTemplatePutRequest(w, req)

	s.Require().Equal(http.StatusNotFound, w.Code)
	s.Require().Equal(ErrorTemplateNotFound.Code, errorCode(w.Body.Bytes()))
}
