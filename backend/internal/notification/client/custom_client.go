// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
)

const (
	customClientLoggerComponentName = "CustomMessageClient"
)

// CustomClient implements the NotificationClientInterface for sending messages via a custom message provider.
type CustomClient struct {
	name          string
	url           string
	httpMethod    string
	legacyHeaders map[string]string
	authenticator outboundauthn.RequestAuthenticator
	contentType   string
	httpClient    syshttp.HTTPClientInterface
}

// newCustomClient creates a new instance of CustomClient.
func newCustomClient(ctx context.Context, sender common.NotificationSenderDTO) (NotificationClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, customClientLoggerComponentName))

	client := &CustomClient{}
	client.name = sender.Name

	apiKeyHeaders := make(map[string]string)
	for _, prop := range sender.Properties {
		value, err := prop.GetValue()
		if err != nil {
			return nil, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		if prop.GetName() == common.CustomPropKeyAPIKeyHeaders {
			if !prop.IsSecret() {
				client.legacyHeaders, err = parseLegacyHTTPHeaders(value)
				if err != nil {
					return nil, err
				}
				continue
			}
			var headers []outboundauthn.APIKeyHeader
			if err := json.Unmarshal([]byte(value), &headers); err != nil {
				return nil, fmt.Errorf("failed to decode API key headers: %w", err)
			}
			for _, header := range headers {
				apiKeyHeaders[header.Name] = header.Value
			}
			continue
		}

		switch prop.GetName() {
		case common.CustomPropKeyURL:
			client.url = value
		case common.CustomPropKeyHTTPMethod:
			client.httpMethod = strings.ToUpper(value)
		case common.CustomPropKeyContentType:
			client.contentType = strings.ToUpper(value)
		default:
			logger.Warn(ctx, "Unknown property for Custom client", log.String("property", prop.GetName()))
		}
	}
	scheme := outboundauthn.SchemeNone
	if len(apiKeyHeaders) > 0 {
		scheme = outboundauthn.SchemeAPIKey
	}
	authenticator, err := outboundauthn.NewRequestAuthenticator(outboundauthn.Config{
		Scheme: scheme, APIKeyHeaders: apiKeyHeaders,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to configure API key headers: %w", err)
	}
	client.authenticator = authenticator
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
func (c *CustomClient) Send(ctx context.Context, channel common.ChannelType, data common.NotificationData) error {
	switch channel {
	case common.ChannelTypeSMS:
		return c.sendSMS(ctx, data)
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}

// sendSMS sends an SMS via the custom webhook.
func (c *CustomClient) sendSMS(ctx context.Context, data common.NotificationData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, customClientLoggerComponentName))
	logger.Debug(ctx, "Sending SMS via custom client", log.MaskedString("to", data.Recipient))

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

	if c.authenticator != nil {
		c.authenticator.ApplyAuthentication(req)
	}
	for name, value := range c.legacyHeaders {
		req.Header.Set(name, value)
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

	logger.Debug(ctx, "Received response from custom provider", log.Int("statusCode", resp.StatusCode))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error(ctx, "Failed to send SMS via custom client", log.Int("statusCode", resp.StatusCode),
			log.String("response", string(bodyBytes)))
		return fmt.Errorf("custom SMS send failed, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func parseLegacyHTTPHeaders(value string) (map[string]string, error) {
	headers := make(map[string]string)
	for _, segment := range strings.Split(value, ",") {
		parts := strings.SplitN(segment, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("failed to parse legacy HTTP headers: invalid header format")
		}
		name := strings.TrimSpace(parts[0])
		headerValue := strings.TrimSpace(parts[1])
		if name == "" || headerValue == "" {
			return nil, fmt.Errorf("failed to parse legacy HTTP headers: invalid header format")
		}
		headers[name] = headerValue
	}
	return headers, nil
}
