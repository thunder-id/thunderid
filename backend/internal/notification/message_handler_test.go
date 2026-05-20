/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package notification

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
)

type MessageHandlerTestSuite struct {
	suite.Suite
}

func TestMessageHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(MessageHandlerTestSuite))
}

func (suite *MessageHandlerTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *MessageHandlerTestSuite) TestHandleSenderListRequest() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	sender1 := common.NotificationSenderDTO{ID: "id1", Name: "s1", Provider: common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{createTestProperty("k", "v", false)}}
	sender2 := common.NotificationSenderDTO{ID: "id2", Name: "s2", Provider: common.MessageProviderTypeVonage,
		Properties: []cmodels.Property{createTestProperty("k2", "v2", false)}}

	m.On("ListSenders", mock.Anything).Return([]common.NotificationSenderDTO{sender1, sender2}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/senders", nil)
	rr := httptest.NewRecorder()

	handler.HandleSenderListRequest(rr, req)

	suite.Equal(http.StatusOK, rr.Code)

	var res []common.NotificationSenderResponse
	suite.NoError(json.Unmarshal(rr.Body.Bytes(), &res))
	suite.Len(res, 2)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderListRequest_ServiceError() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	m.On("ListSenders", mock.Anything).Return(nil, &serviceerror.InternalServerError).Once()

	req := httptest.NewRequest(http.MethodGet, "/senders", nil)
	rr := httptest.NewRecorder()

	handler.HandleSenderListRequest(rr, req)

	suite.Equal(http.StatusInternalServerError, rr.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderCreateRequest() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	reqBody := common.NotificationSenderRequest{
		Name:       "New Sender",
		Provider:   "twilio",
		Properties: []cmodels.PropertyDTO{{Name: "k", Value: "v", IsSecret: false}},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	created := common.NotificationSenderDTO{ID: "created-id", Name: "New Sender",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{createTestProperty("k", "v", false)}}

	m.On("CreateSender", mock.Anything, mock.Anything).Return(&created, nil).Once()
	req := httptest.NewRequest(http.MethodPost, "/senders", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleSenderCreateRequest(rr, req)
	suite.Equal(http.StatusCreated, rr.Code)

	var res common.NotificationSenderResponse
	suite.NoError(json.Unmarshal(rr.Body.Bytes(), &res))
	suite.Equal("created-id", res.ID)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderCreateRequest_Duplicate() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	reqBody := common.NotificationSenderRequest{Name: "New Sender", Provider: "twilio"}
	bodyBytes, _ := json.Marshal(reqBody)

	m.On("CreateSender", mock.Anything, mock.Anything).Return(nil, &ErrorDuplicateSenderName).Once()
	reqDup := httptest.NewRequest(http.MethodPost, "/senders", bytes.NewBuffer(bodyBytes))
	reqDup.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.HandleSenderCreateRequest(rr2, reqDup)
	suite.Equal(http.StatusConflict, rr2.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderGetRequest() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	dto := &common.NotificationSenderDTO{ID: "s-1", Name: "ns",
		Provider:   common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{createTestProperty("k", "v", false)}}

	m.On("GetSender", mock.Anything, "s-1").Return(dto, nil).Once()
	reqGet := httptest.NewRequest(http.MethodGet, "/senders/s-1", nil)
	reqGet.SetPathValue("id", "s-1")
	rrGet := httptest.NewRecorder()
	handler.HandleSenderGetRequest(rrGet, reqGet)
	suite.Equal(http.StatusOK, rrGet.Code)

	var res common.NotificationSenderResponse
	suite.NoError(json.Unmarshal(rrGet.Body.Bytes(), &res))
	suite.Equal("s-1", res.ID)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderGetRequest_NotFound() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	m.On("GetSender", mock.Anything, "missing").Return(nil, nil).Once()
	reqGet2 := httptest.NewRequest(http.MethodGet, "/senders/missing", nil)
	reqGet2.SetPathValue("id", "missing")
	rrGet2 := httptest.NewRecorder()
	handler.HandleSenderGetRequest(rrGet2, reqGet2)
	suite.Equal(http.StatusNotFound, rrGet2.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderUpdateRequest() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	updateReq := common.NotificationSenderRequest{Name: "Updated",
		Provider: "twilio", Properties: []cmodels.PropertyDTO{}}
	body, _ := json.Marshal(updateReq)
	updatedDTO := common.NotificationSenderDTO{ID: "s-1", Name: "Updated",
		Provider: common.MessageProviderTypeTwilio, Properties: []cmodels.Property{}}
	m.On("UpdateSender", mock.Anything, "s-1", mock.Anything).Return(&updatedDTO, nil).Once()
	reqUpd := httptest.NewRequest(http.MethodPut, "/senders/s-1", bytes.NewBuffer(body))
	reqUpd.SetPathValue("id", "s-1")
	rrUpd := httptest.NewRecorder()
	handler.HandleSenderUpdateRequest(rrUpd, reqUpd)
	suite.Equal(http.StatusOK, rrUpd.Code)

	var res common.NotificationSenderResponse
	suite.NoError(json.Unmarshal(rrUpd.Body.Bytes(), &res))
	suite.Equal("s-1", res.ID)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderDeleteRequest() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	m.On("DeleteSender", mock.Anything, "s-1").Return(nil).Once()
	reqDel := httptest.NewRequest(http.MethodDelete, "/senders/s-1", nil)
	reqDel.SetPathValue("id", "s-1")
	rrDel := httptest.NewRecorder()
	handler.HandleSenderDeleteRequest(rrDel, reqDel)
	suite.Equal(http.StatusNoContent, rrDel.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleOTPSendRequest() {
	mOtp := NewOTPServiceInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(nil, mOtp)

	sendReq := common.SendOTPRequest{Recipient: "+123", SenderID: "s-1", Channel: "sms"}
	body, _ := json.Marshal(sendReq)
	mOtp.On("SendOTP", mock.Anything, mock.Anything).Return(&common.SendOTPResultDTO{SessionToken: "tok-1"}, nil).Once()
	req := httptest.NewRequest(http.MethodPost, "/otp/send", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	handler.HandleOTPSendRequest(rr, req)
	suite.Equal(http.StatusOK, rr.Code)
	var resp common.SendOTPResponse
	suite.NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	suite.Equal("tok-1", resp.SessionToken)
}

func (suite *MessageHandlerTestSuite) TestHandleOTPVerifyRequest() {
	mOtp := NewOTPServiceInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(nil, mOtp)

	verifyReq := common.VerifyOTPRequest{SessionToken: "tok-1", OTPCode: "1234"}
	vbody, _ := json.Marshal(verifyReq)
	mOtp.On("VerifyOTP", mock.Anything, mock.Anything).
		Return(&common.VerifyOTPResultDTO{Status: common.OTPVerifyStatus("SUCCESS")}, nil).Once()
	req2 := httptest.NewRequest(http.MethodPost, "/otp/verify", bytes.NewBuffer(vbody))
	rr2 := httptest.NewRecorder()
	handler.HandleOTPVerifyRequest(rr2, req2)
	suite.Equal(http.StatusOK, rr2.Code)
	var vresp common.VerifyOTPResponse
	suite.NoError(json.Unmarshal(rr2.Body.Bytes(), &vresp))
	suite.Equal("SUCCESS", vresp.Status)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderCreateRequest_InvalidJSON() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	req := httptest.NewRequest(http.MethodPost, "/senders", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.HandleSenderCreateRequest(rr, req)
	suite.Equal(http.StatusBadRequest, rr.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderCreateRequest_InvalidProvider() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	validBody := common.NotificationSenderRequest{Name: "s", Provider: "twilio"}
	bb, _ := json.Marshal(validBody)
	m.On("CreateSender", mock.Anything, mock.Anything).Return(nil, &ErrorInvalidProvider).Once()
	req2 := httptest.NewRequest(http.MethodPost, "/senders", bytes.NewBuffer(bb))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.HandleSenderCreateRequest(rr2, req2)
	suite.Equal(http.StatusBadRequest, rr2.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderGetRequest_ServiceError() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	m.On("GetSender", mock.Anything, "err").Return(nil, &serviceerror.InternalServerError).Once()
	req := httptest.NewRequest(http.MethodGet, "/senders/err", nil)
	req.SetPathValue("id", "err")
	rr := httptest.NewRecorder()
	handler.HandleSenderGetRequest(rr, req)
	suite.Equal(http.StatusInternalServerError, rr.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderUpdateRequest_InvalidJSON() {
	handler := newMessageNotificationSenderHandler(nil, nil)

	req2 := httptest.NewRequest(http.MethodPut, "/senders/s1", bytes.NewBufferString("invalid"))
	req2.SetPathValue("id", "s1")
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.HandleSenderUpdateRequest(rr2, req2)
	suite.Equal(http.StatusBadRequest, rr2.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleSenderUpdateRequest_SenderNotFound() {
	m := NewNotificationSenderMgtSvcInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(m, nil)

	upd := common.NotificationSenderRequest{Name: "u", Provider: "twilio"}
	b, _ := json.Marshal(upd)
	m.On("UpdateSender", mock.Anything, "s1", mock.Anything).Return(nil, &ErrorSenderNotFound).Once()
	req3 := httptest.NewRequest(http.MethodPut, "/senders/s1", bytes.NewBuffer(b))
	req3.SetPathValue("id", "s1")
	req3.Header.Set("Content-Type", "application/json")
	rr3 := httptest.NewRecorder()
	handler.HandleSenderUpdateRequest(rr3, req3)
	suite.Equal(http.StatusNotFound, rr3.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleOTPSendRequest_InvalidJSON() {
	handler := newMessageNotificationSenderHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/otp/send", bytes.NewBufferString("invalid"))
	rr := httptest.NewRecorder()
	handler.HandleOTPSendRequest(rr, req)
	suite.Equal(http.StatusBadRequest, rr.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleOTPSendRequest_InvalidRecipient() {
	mOtp := NewOTPServiceInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(nil, mOtp)
	sendReq := common.SendOTPRequest{Recipient: "+1", SenderID: "s1", Channel: "sms"}
	b, _ := json.Marshal(sendReq)
	mOtp.On("SendOTP", mock.Anything, mock.Anything).Return(nil, &ErrorInvalidRecipient).Once()
	req2 := httptest.NewRequest(http.MethodPost, "/otp/send", bytes.NewBuffer(b))
	rr2 := httptest.NewRecorder()
	handler.HandleOTPSendRequest(rr2, req2)
	suite.Equal(http.StatusBadRequest, rr2.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleOTPVerifyRequest_InvalidJSON() {
	handler := newMessageNotificationSenderHandler(nil, nil)
	req3 := httptest.NewRequest(http.MethodPost, "/otp/verify", bytes.NewBufferString("invalid"))
	rr3 := httptest.NewRecorder()
	handler.HandleOTPVerifyRequest(rr3, req3)
	suite.Equal(http.StatusBadRequest, rr3.Code)
}

func (suite *MessageHandlerTestSuite) TestHandleOTPVerifyRequest_InvalidOTP() {
	mOtp := NewOTPServiceInterfaceMock(suite.T())
	handler := newMessageNotificationSenderHandler(nil, mOtp)
	vreq := common.VerifyOTPRequest{SessionToken: "t", OTPCode: "c"}
	vb, _ := json.Marshal(vreq)
	mOtp.On("VerifyOTP", mock.Anything, mock.Anything).Return(nil, &ErrorInvalidOTP).Once()
	req4 := httptest.NewRequest(http.MethodPost, "/otp/verify", bytes.NewBuffer(vb))
	rr4 := httptest.NewRecorder()
	handler.HandleOTPVerifyRequest(rr4, req4)
	suite.Equal(http.StatusBadRequest, rr4.Code)
}

func (suite *MessageHandlerTestSuite) TestValidateSenderID_EmptyID() {
	handler := newMessageNotificationSenderHandler(nil, nil)

	rr := httptest.NewRecorder()
	ok := handler.validateSenderID(rr, "")

	suite.False(ok)
	suite.Equal(400, rr.Code)

	var errResp apierror.ErrorResponse
	suite.NoError(json.Unmarshal(rr.Body.Bytes(), &errResp))
	suite.Equal(ErrorInvalidSenderID.Code, errResp.Code)
}

func (suite *MessageHandlerTestSuite) TestValidateSenderID_NonEmptyID() {
	handler := newMessageNotificationSenderHandler(nil, nil)
	rr := httptest.NewRecorder()
	ok := handler.validateSenderID(rr, "sender-1")
	suite.True(ok)
	suite.Equal(0, rr.Body.Len())
}

func (suite *MessageHandlerTestSuite) TestHandleError() {
	handler := newMessageNotificationSenderHandler(nil, nil)

	cases := []struct {
		name          string
		svcErr        *serviceerror.ServiceError
		customDesc    string
		expectedCode  int
		expectedError string
	}{
		{
			name:          "sender not found -> 404",
			svcErr:        &ErrorSenderNotFound,
			customDesc:    "",
			expectedCode:  404,
			expectedError: ErrorSenderNotFound.Error.String(),
		},
		{
			name:          "duplicate sender name -> 409",
			svcErr:        &ErrorDuplicateSenderName,
			customDesc:    "",
			expectedCode:  409,
			expectedError: ErrorDuplicateSenderName.Error.String(),
		},
		{
			name: "generic client error -> 400",
			svcErr: &serviceerror.ServiceError{
				Type:             serviceerror.ClientErrorType,
				Code:             "MNS-1999",
				Error:            i18ncore.I18nMessage{DefaultValue: "Some client error"},
				ErrorDescription: i18ncore.I18nMessage{DefaultValue: "details"},
			},
			customDesc:    "custom desc",
			expectedCode:  400,
			expectedError: "Some client error",
		},
		{
			name:          "server error -> 500",
			svcErr:        &serviceerror.InternalServerError,
			customDesc:    "internal happened",
			expectedCode:  500,
			expectedError: serviceerror.InternalServerError.Error.String(),
		},
	}

	for _, tc := range cases {
		rr := httptest.NewRecorder()
		handler.handleError(rr, tc.svcErr, tc.customDesc)

		suite.Equal(tc.expectedCode, rr.Code, tc.name)

		var resp apierror.ErrorResponse
		suite.NoError(json.Unmarshal(rr.Body.Bytes(), &resp), tc.name)
		suite.Equal(tc.svcErr.Code, resp.Code, tc.name)
		suite.Equal(tc.expectedError, resp.Message.DefaultValue, tc.name)
		if tc.customDesc != "" {
			suite.Equal(tc.customDesc, resp.Description.DefaultValue, tc.name)
		} else {
			suite.Equal(tc.svcErr.ErrorDescription.String(), resp.Description.DefaultValue, tc.name)
		}
	}
}

// writer that always errors when writing to exercise the encode error path
type errWriter struct{}

func (e *errWriter) Header() http.Header        { return http.Header{} }
func (e *errWriter) Write([]byte) (int, error)  { return 0, io.ErrClosedPipe }
func (e *errWriter) WriteHeader(statusCode int) {}

func (suite *MessageHandlerTestSuite) TestHandleError_EncodeFailure() {
	handler := newMessageNotificationSenderHandler(nil, nil)
	ew := &errWriter{}
	handler.handleError(ew, &serviceerror.InternalServerError, "boom")
}

func (suite *MessageHandlerTestSuite) TestGetDTOFromSenderRequest() {
	request := &common.NotificationSenderRequest{
		Name:        "Test Sender",
		Description: "Test Description",
		Provider:    "twilio",
		Properties: []cmodels.PropertyDTO{
			{
				Name:     "account_sid",
				Value:    "AC12345",
				IsSecret: true,
			},
			{
				Name:     "auth_token",
				Value:    "token123",
				IsSecret: true,
			},
			{
				Name:     "sender_id",
				Value:    "+15551234567",
				IsSecret: false,
			},
		},
	}

	dto, err := getDTOFromSenderRequest(request)

	suite.NoError(err)
	suite.NotNil(dto)
	suite.Equal("Test Sender", dto.Name)
	suite.Equal("Test Description", dto.Description)
	suite.Equal(common.MessageProviderTypeTwilio, dto.Provider)
	suite.Equal(common.NotificationSenderTypeMessage, dto.Type)
	suite.Len(dto.Properties, 3)
}

func (suite *MessageHandlerTestSuite) TestGetDTOFromSenderRequest_EmptyProperties() {
	request := &common.NotificationSenderRequest{
		Name:        "Test Sender",
		Description: "Test Description",
		Provider:    "vonage",
		Properties:  []cmodels.PropertyDTO{},
	}

	dto, err := getDTOFromSenderRequest(request)

	suite.NoError(err)
	suite.NotNil(dto)
	suite.Equal("Test Sender", dto.Name)
	suite.Len(dto.Properties, 0)
}

func (suite *MessageHandlerTestSuite) TestGetDTOFromSenderRequest_Sanitization() {
	request := &common.NotificationSenderRequest{
		Name:        "  Test Sender  ",
		Description: "  Test Description  ",
		Provider:    "  twilio  ",
		Properties:  []cmodels.PropertyDTO{},
	}

	dto, err := getDTOFromSenderRequest(request)

	suite.NoError(err)
	suite.NotNil(dto)
	suite.Equal("Test Sender", dto.Name)
	suite.Equal("Test Description", dto.Description)
	suite.Equal(common.MessageProviderType("twilio"), dto.Provider)
}

func (suite *MessageHandlerTestSuite) TestGetSenderResponseFromDTO() {
	dto := &common.NotificationSenderDTO{
		ID:          "sender-123",
		Name:        "Test Sender",
		Description: "Test Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC12345", true),
			createTestProperty("auth_token", "token123", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}

	response, err := getSenderResponseFromDTO(dto)

	suite.NoError(err)
	suite.Equal("sender-123", response.ID)
	suite.Equal("Test Sender", response.Name)
	suite.Equal("Test Description", response.Description)
	suite.Equal(common.MessageProviderTypeTwilio, response.Provider)
	suite.Len(response.Properties, 3)

	// Check that secrets are masked
	for _, prop := range response.Properties {
		if prop.Name == "account_sid" || prop.Name == "auth_token" {
			suite.Equal("******", prop.Value)
			suite.True(prop.IsSecret)
		}
	}

	// Check that non-secrets are not masked
	for _, prop := range response.Properties {
		if prop.Name == "sender_id" {
			suite.Equal("+15551234567", prop.Value)
			suite.False(prop.IsSecret)
		}
	}
}

func (suite *MessageHandlerTestSuite) TestGetSenderResponseFromDTO_AllSecrets() {
	dto := &common.NotificationSenderDTO{
		ID:          "sender-123",
		Name:        "Test Sender",
		Description: "Test Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC12345", true),
			createTestProperty("auth_token", "token123", true),
		},
	}

	response, err := getSenderResponseFromDTO(dto)

	suite.NoError(err)
	suite.Len(response.Properties, 2)

	// All should be masked
	for _, prop := range response.Properties {
		suite.Equal("******", prop.Value)
		suite.True(prop.IsSecret)
	}
}

func (suite *MessageHandlerTestSuite) TestGetSenderResponseFromDTO_NoSecrets() {
	dto := &common.NotificationSenderDTO{
		ID:          "sender-123",
		Name:        "Test Sender",
		Description: "Test Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeCustom,
		Properties: []cmodels.Property{
			createTestProperty("url", "https://api.example.com", false),
			createTestProperty("method", "POST", false),
		},
	}

	response, err := getSenderResponseFromDTO(dto)

	suite.NoError(err)
	suite.Len(response.Properties, 2)

	// None should be masked
	for _, prop := range response.Properties {
		suite.NotEqual("******", prop.Value)
		suite.False(prop.IsSecret)
	}
}

func (suite *MessageHandlerTestSuite) TestGetSenderResponseFromDTO_EmptyProperties() {
	dto := &common.NotificationSenderDTO{
		ID:          "sender-123",
		Name:        "Test Sender",
		Description: "Test Description",
		Type:        common.NotificationSenderTypeMessage,
		Provider:    common.MessageProviderTypeTwilio,
		Properties:  []cmodels.Property{},
	}

	response, err := getSenderResponseFromDTO(dto)

	suite.NoError(err)
	suite.Len(response.Properties, 0)
}
