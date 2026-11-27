// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"bytes"
	"context"
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

	req, err := buildUserEmailRequest(context.Background(), endpoint, accessToken, suite.logger)
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
	req, err := buildUserEmailRequest(context.Background(), endpoint, "token", suite.logger)
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

func (suite *GithubUtilsTestSuite) TestBuildUserTeamsRequest() {
	endpoint := "https://api.github.com/user/teams"
	accessToken := "test_token"

	req, err := buildUserTeamsRequest(context.Background(), endpoint, accessToken, suite.logger)
	suite.Nil(err)
	suite.NotNil(req)
	suite.Equal(http.MethodGet, req.Method)
	suite.Equal("https://api.github.com/user/teams?per_page=100", req.URL.String())
	suite.Equal("Bearer test_token", req.Header.Get("Authorization"))
	suite.Equal("application/json", req.Header.Get("Accept"))

	// Preserves existing per_page query param
	reqWithPage, err := buildUserTeamsRequest(
		context.Background(), "https://api.github.com/user/teams?page=2&per_page=50", accessToken, suite.logger)
	suite.Nil(err)
	suite.NotNil(reqWithPage)
	suite.Equal("https://api.github.com/user/teams?page=2&per_page=50", reqWithPage.URL.String())
}

func (suite *GithubUtilsTestSuite) TestBuildUserTeamsRequestError() {
	req, err := buildUserTeamsRequest(context.Background(), "http://\x00", "token", suite.logger)
	suite.Nil(req)
	suite.NotNil(err)
}

func (suite *GithubUtilsTestSuite) TestSendUserTeamsRequestSuccessAndLinkPagination() {
	body := `[{"slug":"platform-engineers","organization":{"login":"my-org"}}]`
	headers := http.Header{}
	headers.Set(linkHeaderName,
		`<https://api.github.com/user/teams?page=1>; rel="prev", `+
			`<https://api.github.com/user/teams?page=3&per_page=100>; rel="next"`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/teams?page=2&per_page=100", nil)
	suite.mockHTTPClient.On("Do", req).Return(resp, nil).Once()

	teams, nextURL, err := sendUserTeamsRequest(req, suite.mockHTTPClient, suite.logger)
	suite.Nil(err)
	suite.Len(teams, 1)
	suite.Equal("platform-engineers", teams[0].Slug)
	suite.Equal("my-org", teams[0].Organization.Login)
	suite.Equal("https://api.github.com/user/teams?page=3&per_page=100", nextURL)
}

func (suite *GithubUtilsTestSuite) TestSendUserTeamsRequestNon2xxError() {
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"message":"Forbidden"}`))),
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/user/teams?per_page=100", nil)
	suite.mockHTTPClient.On("Do", req).Return(resp, nil).Once()

	teams, nextURL, err := sendUserTeamsRequest(req, suite.mockHTTPClient, suite.logger)
	suite.Nil(teams)
	suite.Empty(nextURL)
	suite.NotNil(err)
}
