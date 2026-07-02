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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

func TestParseHTTPHeaders_Valid(t *testing.T) {
	headersString := "Authorization: Bearer token, X-Custom-Header: custom_value"
	headers, err := parseHTTPHeaders(headersString)

	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Equal(t, "Bearer token", headers["Authorization"])
	assert.Equal(t, "custom_value", headers["X-Custom-Header"])
}

func TestParseHTTPHeaders_EmptyString(t *testing.T) {
	headersString := "   "
	headers, err := parseHTTPHeaders(headersString)

	require.NoError(t, err)
	require.NotNil(t, headers)
	assert.Empty(t, headers)
}

func TestParseHTTPHeaders_Invalid(t *testing.T) {
	headersString := "Invalid Header Format"
	headers, err := parseHTTPHeaders(headersString)

	require.Error(t, err)
	require.Nil(t, headers)
	assert.Contains(t, err.Error(), "invalid HTTP header format")
}

func TestParseHTTPWebhookConfig_Success(t *testing.T) {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyURL, "https://example.com/webhook", false),
		createProperty(common.CustomPropKeyHTTPMethod, "post", false),
		createProperty(common.CustomPropKeyHTTPHeaders, "Authorization: Bearer token", false),
		createProperty(common.CustomPropKeyContentType, "application/json", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	logger := log.GetLogger()

	cfg, err := parseHTTPWebhookConfig(context.Background(), sender, logger)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/webhook", cfg.url)
	assert.Equal(t, "POST", cfg.httpMethod)
	assert.Equal(t, "APPLICATION/JSON", cfg.contentType)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, cfg.httpHeaders)
}

func TestParseHTTPWebhookConfig_MissingURL(t *testing.T) {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyHTTPMethod, "post", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	logger := log.GetLogger()

	_, err := parseHTTPWebhookConfig(context.Background(), sender, logger)

	require.Error(t, err)
	assert.Equal(t, "custom provider must have a URL property", err.Error())
}

func TestParseHTTPWebhookConfig_InvalidHeaders(t *testing.T) {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyURL, "https://example.com/webhook", false),
		createProperty(common.CustomPropKeyHTTPHeaders, "InvalidHeaderFormat", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	logger := log.GetLogger()

	_, err := parseHTTPWebhookConfig(context.Background(), sender, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse HTTP headers: invalid HTTP header format")
}

func TestParseHTTPWebhookConfig_UnknownProperty(t *testing.T) {
	props := []cmodels.Property{
		createProperty(common.CustomPropKeyURL, "https://example.com/webhook", false),
		createProperty("unknown_prop", "some_value", false),
	}

	sender := common.NotificationSenderDTO{Properties: props}
	_ = config.InitializeServerRuntime("", &config.Config{})
	logger := log.GetLogger()

	cfg, err := parseHTTPWebhookConfig(context.Background(), sender, logger)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/webhook", cfg.url)
}
