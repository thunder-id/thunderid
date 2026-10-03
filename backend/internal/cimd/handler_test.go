// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type CIMDHandlerTestSuite struct {
	suite.Suite
	mockService *CIMDServiceInterfaceMock
	handler     *cimdHandler
}

func TestCIMDHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(CIMDHandlerTestSuite))
}

func (s *CIMDHandlerTestSuite) SetupTest() {
	s.mockService = NewCIMDServiceInterfaceMock(s.T())
	s.handler = newCIMDHandler(s.mockService)
}

func (s *CIMDHandlerTestSuite) sendPreview(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/cimd/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handler.HandlePreviewRequest(w, req)
	return w
}

func (s *CIMDHandlerTestSuite) TestHandlePreviewRequest_Success() {
	s.mockService.EXPECT().Preview(mock.Anything, testCIMDClientID).Return(&PreviewResponse{
		Name: "Client",
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type:        providers.OAuthInboundAuthType,
			OAuthConfig: publicCIMDConfig(),
		}},
	}, nil)

	w := s.sendPreview(`{"clientId":"` + testCIMDClientID + `"}`)

	s.Equal(http.StatusOK, w.Code)
	var response PreviewResponse
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &response))
	s.Equal("Client", response.Name)
	s.True(response.InboundAuthConfig[0].OAuthConfig.ClientIDMetadataDocument)
}

func (s *CIMDHandlerTestSuite) TestHandlePreviewRequest_InvalidBody() {
	w := s.sendPreview(`{`)

	s.Equal(http.StatusBadRequest, w.Code)
	s.Contains(w.Body.String(), ErrorInvalidRequestFormat.Code)
}

func (s *CIMDHandlerTestSuite) TestHandlePreviewRequest_RejectedURL() {
	s.mockService.EXPECT().Preview(mock.Anything, "http://client.example.com/client.json").
		Return(nil, &ErrorInvalidClientID)

	w := s.sendPreview(`{"clientId":"http://client.example.com/client.json"}`)

	s.Equal(http.StatusBadRequest, w.Code)
	s.Contains(w.Body.String(), ErrorInvalidClientID.Code)
}

func (s *CIMDHandlerTestSuite) TestHandlePreviewRequest_ServerError() {
	s.mockService.EXPECT().Preview(mock.Anything, testCIMDClientID).Return(nil, &tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "CIMD-5000",
		Error: tidcommon.I18nMessage{DefaultValue: "Internal server error"},
	})

	w := s.sendPreview(`{"clientId":"` + testCIMDClientID + `"}`)

	s.Equal(http.StatusInternalServerError, w.Code)
	s.Contains(w.Body.String(), "CIMD-5000")
}
