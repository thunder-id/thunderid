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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	httpEmailClientLoggerComponentName = "HTTPEmailClient"
)

// HTTPEmailClient implements the EmailClientInterface for sending emails via a custom HTTP webhook.
type HTTPEmailClient struct {
	name       string
	config     httpWebhookConfig
	httpClient syshttp.HTTPClientInterface
}

// newHTTPEmailClient creates a new instance of HTTPEmailClient.
func newHTTPEmailClient(ctx context.Context, sender common.NotificationSenderDTO) (NotificationClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, httpEmailClientLoggerComponentName))

	client := &HTTPEmailClient{}
	client.name = sender.Name

	config, err := parseHTTPWebhookConfig(ctx, sender, logger)
	if err != nil {
		return nil, err
	}
	client.config = config
	client.httpClient = syshttp.NewHTTPClientWithTimeout(httpClientTimeout)

	return client, nil
}

// GetName returns the name of the HTTP Email client.
func (c *HTTPEmailClient) GetName() string {
	return c.name
}

// Send dispatches an email notification via the custom webhook.
func (c *HTTPEmailClient) Send(ctx context.Context, data common.EmailData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, httpEmailClientLoggerComponentName))
	logger.Debug(ctx, "Sending Email via HTTP client", log.MaskedString("to", strings.Join(data.To, ",")))

	var req *http.Request
	var err error

	if strings.ToUpper(c.config.contentType) == "JSON" {
		payload := map[string]interface{}{
			"to":      data.To,
			"cc":      data.CC,
			"bcc":     data.BCC,
			"subject": data.Subject,
			"body":    data.Body,
			"is_html": data.IsHTML,
		}
		jsonBytes, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal JSON payload: %w", marshalErr)
		}

		req, err = http.NewRequest(c.config.httpMethod, c.config.url, bytes.NewBuffer(jsonBytes))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	} else if strings.ToUpper(c.config.contentType) == "FORM" {
		formData := url.Values{}
		formData.Add("to", strings.Join(data.To, ","))
		if len(data.CC) > 0 {
			formData.Add("cc", strings.Join(data.CC, ","))
		}
		if len(data.BCC) > 0 {
			formData.Add("bcc", strings.Join(data.BCC, ","))
		}
		formData.Add("subject", data.Subject)
		formData.Add("body", data.Body)
		if data.IsHTML {
			formData.Add("is_html", "true")
		} else {
			formData.Add("is_html", "false")
		}

		req, err = http.NewRequest(c.config.httpMethod, c.config.url, strings.NewReader(formData.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeFormURLEncoded)
	} else {
		return fmt.Errorf("unsupported content type: %s", c.config.contentType)
	}

	for key, value := range c.config.httpHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error(ctx, "Failed to close response body", log.Error(closeErr))
		}
	}()

	logger.Debug(ctx, "Received response from HTTP Email provider", log.Int("statusCode", resp.StatusCode))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error(ctx, "Failed to send Email via HTTP client", log.Int("statusCode", resp.StatusCode),
			log.String("response", string(bodyBytes)))
		return fmt.Errorf("HTTP Email send failed, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
