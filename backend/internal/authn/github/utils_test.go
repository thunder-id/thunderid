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

package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
)

type GithubUtilsTestSuite struct {
	suite.Suite
	mockHTTPClient *httpmock.HTTPClientInterfaceMock
	logger         *log.Logger
}

func TestGithubUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(GithubUtilsTestSuite))
}

func (suite *GithubUtilsTestSuite) SetupTest() {
	suite.mockHTTPClient = httpmock.NewHTTPClientInterfaceMock(suite.T())
	suite.logger = log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, "GithubUtilsTest"),
	)
}

func (suite *GithubUtilsTestSuite) TestBuildUserEmailRequest() {
	endpoint := "https://api.github.com/user/emails"
	accessToken := "test_token"

	req, err := buildUserEmailRequest(endpoint, accessToken, suite.logger)
	suite.Nil(err)
	suite.NotNil(req)

	suite.Equal(http.MethodGet, req.Method)
	suite.Equal(endpoint, req.URL.String())
	suite.Equal("Bearer test_token", req.Header.Get("Authorization"))
	suite.Equal("application/json", req.Header.Get("Accept"))
}

func (suite *GithubUtilsTestSuite) TestSendUserEmailRequestSuccess() {
	emailData := []map[string]interface{}{
		{
			"email":    "test@example.com",
			"primary":  true,
			"verified": true,
		},
		{
			"email":    "secondary@example.com",
			"primary":  false,
			"verified": true,
		},
	}
	emailJSON, _ := json.Marshal(emailData)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(emailJSON)),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	suite.mockHTTPClient.On("Do", req).Return(resp, nil)

	emails, err := sendUserEmailRequest(req, suite.mockHTTPClient, suite.logger)
	suite.Nil(err)
	suite.NotNil(emails)

	suite.Len(emails, 2)
	suite.Equal("test@example.com", emails[0]["email"])
	suite.Equal(true, emails[0]["primary"])
}

func (suite *GithubUtilsTestSuite) TestSendUserEmailRequestHTTPError() {
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	suite.mockHTTPClient.On("Do", req).Return(nil, errors.New("network error"))

	emails, err := sendUserEmailRequest(req, suite.mockHTTPClient, suite.logger)
	suite.Nil(emails)
	suite.NotNil(err)
}

func (suite *GithubUtilsTestSuite) TestSendUserEmailRequestErrorResponse() {
	tests := []struct {
		name      string
		resp      *http.Response
		mockErr   error
		expectErr bool
		expectNil bool
		expectLen int
	}{
		{
			name: "Non200Status",
			resp: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"message":"Unauthorized"}`))),
			},
			mockErr:   nil,
			expectErr: true,
			expectNil: true,
		},
		{
			name: "InvalidJSON",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`invalid json`))),
			},
			mockErr:   nil,
			expectErr: true,
			expectNil: true,
		},
		{
			name: "EmptyResponse",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`[]`))),
			},
			mockErr:   nil,
			expectErr: false,
			expectNil: false,
			expectLen: 0,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			fresh := httpmock.NewHTTPClientInterfaceMock(suite.T())

			req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
			fresh.On("Do", req).Return(tc.resp, tc.mockErr)

			emails, err := sendUserEmailRequest(req, fresh, suite.logger)
			if tc.expectErr {
				suite.Nil(emails)
				suite.NotNil(err)
			} else {
				suite.Nil(err)
				suite.NotNil(emails)
				suite.Len(emails, tc.expectLen)
			}
		})
	}
}

func (suite *GithubUtilsTestSuite) TestSendUserEmailRequestNon200LargeBody() {
	// Create a body larger than 4096 bytes to exercise io.LimitReader path
	largeBody := bytes.Repeat([]byte("x"), 5000)
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(bytes.NewReader(largeBody)),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	fresh := httpmock.NewHTTPClientInterfaceMock(suite.T())
	fresh.On("Do", req).Return(resp, nil)

	emails, err := sendUserEmailRequest(req, fresh, suite.logger)
	suite.Nil(emails)
	suite.NotNil(err)
}

func (suite *GithubUtilsTestSuite) TestSendUserEmailRequestDoReturnsRespAndError() {
	// If Do returns an error (even if resp is non-nil), the function should return an error
	emailData := []map[string]interface{}{{"email": "a@b.com", "primary": true}}
	emailJSON, _ := json.Marshal(emailData)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(emailJSON))}

	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	fresh := httpmock.NewHTTPClientInterfaceMock(suite.T())
	fresh.On("Do", req).Return(resp, errors.New("network failure"))

	emails, err := sendUserEmailRequest(req, fresh, suite.logger)
	suite.Nil(emails)
	suite.NotNil(err)
}

func (suite *GithubUtilsTestSuite) TestBuildUserEmailRequestError() {
	// Use an invalid URL containing a control character to force NewRequest to fail.
	endpoint := "http://\x00"
	req, err := buildUserEmailRequest(endpoint, "token", suite.logger)
	suite.Nil(req)
	suite.NotNil(err)
}

type badCloseReadCloser struct {
	io.Reader
}

func (b badCloseReadCloser) Close() error { return errors.New("close failed") }

func (suite *GithubUtilsTestSuite) TestSendUserEmailRequestCloseError() {
	emailData := []map[string]interface{}{
		{"email": "test@example.com", "primary": true},
	}
	emailJSON, _ := json.Marshal(emailData)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       badCloseReadCloser{bytes.NewReader(emailJSON)},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/emails", nil)
	fresh := httpmock.NewHTTPClientInterfaceMock(suite.T())
	fresh.On("Do", req).Return(resp, nil)

	emails, err := sendUserEmailRequest(req, fresh, suite.logger)
	suite.Nil(err)
	suite.NotNil(emails)
	suite.Len(emails, 1)
}
