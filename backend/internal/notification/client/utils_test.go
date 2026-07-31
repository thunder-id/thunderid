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

package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (s *UtilsTestSuite) TestParseHTTPHeaders_Valid() {
	headersString := "Authorization: Bearer token, X-Custom-Header: custom_value"
	headers, err := parseHTTPHeaders(headersString)

	s.NoError(err)
	s.NotNil(headers)
	s.Equal("Bearer token", headers["Authorization"])
	s.Equal("custom_value", headers["X-Custom-Header"])
}

func (s *UtilsTestSuite) TestParseHTTPHeaders_EmptyString() {
	headersString := "   "
	headers, err := parseHTTPHeaders(headersString)

	s.NoError(err)
	s.NotNil(headers)
	s.Empty(headers)
}

func (s *UtilsTestSuite) TestParseHTTPHeaders_Invalid() {
	headersString := "Invalid Header Format"
	headers, err := parseHTTPHeaders(headersString)

	s.Error(err)
	s.Nil(headers)
	s.Contains(err.Error(), "invalid HTTP header format")
}

func (s *UtilsTestSuite) TestParseHTTPTransportConfig_Success() {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyURL, "https://example.com/webhook", false),
		createProperty(common.CustomPropKeyHTTPMethod, "post", false),
		createProperty(common.CustomPropKeyHTTPHeaders, "Authorization: Bearer token", false),
		createProperty(common.CustomPropKeyContentType, "application/json", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	logger := log.GetLogger()

	cfg, err := parseHTTPTransportConfig(context.Background(), sender, logger)

	s.NoError(err)
	s.Equal("https://example.com/webhook", cfg.url)
	s.Equal("POST", cfg.httpMethod)
	s.Equal("APPLICATION/JSON", cfg.contentType)
	s.Equal(map[string]string{"Authorization": "Bearer token"}, cfg.httpHeaders)
}

func (s *UtilsTestSuite) TestParseHTTPTransportConfig_MissingURL() {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyHTTPMethod, "post", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	logger := log.GetLogger()

	_, err := parseHTTPTransportConfig(context.Background(), sender, logger)

	s.Error(err)
	s.Equal("custom provider must have a URL property", err.Error())
}

func (s *UtilsTestSuite) TestParseHTTPTransportConfig_InvalidHeaders() {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyURL, "https://example.com/webhook", false),
		createProperty(common.CustomPropKeyHTTPHeaders, "InvalidHeaderFormat", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	logger := log.GetLogger()

	_, err := parseHTTPTransportConfig(context.Background(), sender, logger)

	s.Error(err)
	s.Contains(err.Error(), "failed to parse HTTP headers: invalid HTTP header format")
}

func (s *UtilsTestSuite) TestParseHTTPTransportConfig_UnknownProperty() {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyURL, "https://example.com/webhook", false),
		createProperty("unknown_prop", "some_value", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	_ = config.InitializeServerRuntime("", &config.Config{})
	logger := log.GetLogger()

	cfg, err := parseHTTPTransportConfig(context.Background(), sender, logger)

	s.NoError(err)
	s.Equal("https://example.com/webhook", cfg.url)
}
