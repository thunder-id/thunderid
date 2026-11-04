// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type HandlerTestSuite struct {
	suite.Suite
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

func (s *HandlerTestSuite) TestWritesOAuthError() {
	service := NewDeviceAuthServiceInterfaceMock(s.T())
	client := &clientauth.OAuthClientInfo{
		ClientID: "device-client",
		OAuthApp: &providers.OAuthClient{ClientID: "device-client"},
	}
	service.EXPECT().InitiateDeviceAuthorization(mock.Anything, mock.MatchedBy(
		func(request *DeviceAuthorizationRequest) bool {
			return request.Scope == "openid" && len(request.Resources) == 2
		}), client.OAuthApp).Return(nil, &DeviceAuthError{
		Code:    oauth2const.ErrorInvalidTarget,
		Message: "Only a single resource parameter is supported",
	})

	request := httptest.NewRequest(http.MethodPost, oauth2const.OAuth2DeviceAuthorizationEndpoint,
		strings.NewReader(
			"scope=openid&resource=https%3A%2F%2Fapi.example.com&resource=https%3A%2F%2Fapi2.example.com",
		))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(context.WithValue(request.Context(), clientauth.OAuthClientKey, client))
	response := httptest.NewRecorder()

	newHandler(service).HandleDeviceAuthorizationRequest(response, request)

	s.Equal(http.StatusBadRequest, response.Code)
	var body map[string]string
	s.Require().NoError(json.NewDecoder(response.Body).Decode(&body))
	s.Equal(oauth2const.ErrorInvalidTarget, body["error"])
	s.Equal("Only a single resource parameter is supported", body["error_description"])
}

func (s *HandlerTestSuite) TestWritesSuccessfulResponse() {
	service := NewDeviceAuthServiceInterfaceMock(s.T())
	client := &clientauth.OAuthClientInfo{
		ClientID: "device-client",
		OAuthApp: &providers.OAuthClient{ClientID: "device-client"},
	}
	result := &DeviceAuthorizationResponse{
		DeviceCode:      "device-code",
		UserCode:        "BCDF-GHJK",
		VerificationURI: "https://id.example.com/gate/device",
		ExpiresIn:       600,
		Interval:        5,
	}
	service.EXPECT().InitiateDeviceAuthorization(mock.Anything, mock.MatchedBy(
		func(request *DeviceAuthorizationRequest) bool {
			return request.Scope == "openid profile" && len(request.Resources) == 0
		}), client.OAuthApp).Return(result, nil)

	request := httptest.NewRequest(http.MethodPost, oauth2const.OAuth2DeviceAuthorizationEndpoint,
		strings.NewReader("scope=openid+profile"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(context.WithValue(request.Context(), clientauth.OAuthClientKey, client))
	response := httptest.NewRecorder()

	newHandler(service).HandleDeviceAuthorizationRequest(response, request)

	s.Equal(http.StatusOK, response.Code)
	s.Equal("no-store", response.Header().Get("Cache-Control"))
	s.Equal("no-cache", response.Header().Get("Pragma"))
	var body DeviceAuthorizationResponse
	s.Require().NoError(json.NewDecoder(response.Body).Decode(&body))
	s.Equal(*result, body)
}

func (s *HandlerTestSuite) TestRejectsMissingClientContext() {
	service := NewDeviceAuthServiceInterfaceMock(s.T())
	request := httptest.NewRequest(http.MethodPost, oauth2const.OAuth2DeviceAuthorizationEndpoint, nil)
	response := httptest.NewRecorder()

	newHandler(service).HandleDeviceAuthorizationRequest(response, request)

	s.Equal(http.StatusInternalServerError, response.Code)
	var body map[string]string
	s.Require().NoError(json.NewDecoder(response.Body).Decode(&body))
	s.Equal(oauth2const.ErrorServerError, body["error"])
}
