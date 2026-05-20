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

package message

import (
	"bytes"
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
	customClientLoggerComponentName = "CustomMessageClient"
)

// CustomClient implements the NotificationClientInterface for sending messages via a custom message provider.
type CustomClient struct {
	name        string
	url         string
	httpMethod  string
	httpHeaders map[string]string
	contentType string
	httpClient  syshttp.HTTPClientInterface
}

// NewCustomClient creates a new instance of CustomClient.
func NewCustomClient(sender common.NotificationSenderDTO) (NotificationClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, customClientLoggerComponentName))

	client := &CustomClient{}
	client.name = sender.Name

	for _, prop := range sender.Properties {
		value, err := prop.GetValue()
		if err != nil {
			return nil, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.CustomPropKeyURL:
			client.url = value
		case common.CustomPropKeyHTTPMethod:
			client.httpMethod = strings.ToUpper(value)
		case common.CustomPropKeyHTTPHeaders:
			headers, err := client.getHeadersFromString(value)
			if err != nil {
				return nil, fmt.Errorf("failed to parse HTTP headers: %w", err)
			}
			client.httpHeaders = headers
		case common.CustomPropKeyContentType:
			client.contentType = strings.ToUpper(value)
		default:
			logger.Warn("Unknown property for Custom client", log.String("property", prop.GetName()))
		}
	}
	client.httpClient = syshttp.NewHTTPClientWithTimeout(httpClientTimeout)

	return client, nil
}

// GetName returns the name of the Custom client.
func (c *CustomClient) GetName() string {
	return c.name
}

// IsChannelSupported reports whether the given channel is supported by the custom client.
func (c *CustomClient) IsChannelSupported(channel common.ChannelType) bool {
	return channel == common.ChannelTypeSMS
}

// Send dispatches a notification via the requested channel.
func (c *CustomClient) Send(channel common.ChannelType, data common.NotificationData) error {
	switch channel {
	case common.ChannelTypeSMS:
		return c.sendSMS(data)
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}

// sendSMS sends an SMS via the custom webhook.
func (c *CustomClient) sendSMS(data common.NotificationData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, customClientLoggerComponentName))
	logger.Debug("Sending SMS via custom client", log.MaskedString("to", data.Recipient))

	var req *http.Request
	var err error

	if strings.ToUpper(c.contentType) == "JSON" {
		req, err = http.NewRequest(c.httpMethod, c.url, bytes.NewBufferString(data.Body))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	} else if strings.ToUpper(c.contentType) == "FORM" {
		formData := url.Values{}
		lines := strings.Split(data.Body, "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				formData.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
		req, err = http.NewRequest(c.httpMethod, c.url, strings.NewReader(formData.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeFormURLEncoded)
	} else {
		return fmt.Errorf("unsupported content type: %s", c.contentType)
	}

	for key, value := range c.httpHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("Failed to close response body", log.Error(closeErr))
		}
	}()

	logger.Debug("Received response from custom provider", log.Int("statusCode", resp.StatusCode))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error("Failed to send SMS via custom client", log.Int("statusCode", resp.StatusCode),
			log.String("response", string(bodyBytes)))
		return fmt.Errorf("custom SMS send failed, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// getHeadersFromString parses a string of HTTP headers into a map.
func (c *CustomClient) getHeadersFromString(headersString string) (map[string]string, error) {
	headers := make(map[string]string)
	for _, header := range strings.Split(headersString, ",") {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		} else {
			return nil, fmt.Errorf("invalid HTTP header format: %s", header)
		}
	}
	return headers, nil
}
