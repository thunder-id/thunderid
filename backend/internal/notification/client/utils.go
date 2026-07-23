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
	"errors"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// httpWebhookConfig holds the configuration for HTTP webhook clients.
type httpWebhookConfig struct {
	url         string
	httpMethod  string
	httpHeaders map[string]string
	contentType string
}

// parseHTTPWebhookConfig parses the HTTP webhook configuration from the given notification sender properties.
func parseHTTPWebhookConfig(
	ctx context.Context,
	sender common.NotificationSenderDTO,
	logger *log.Logger,
) (httpWebhookConfig, error) {
	config := httpWebhookConfig{}

	for _, prop := range sender.Properties {
		value, err := prop.GetValue()
		if err != nil {
			return config, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.CustomPropKeyURL:
			config.url = value
		case common.CustomPropKeyHTTPMethod:
			config.httpMethod = strings.ToUpper(value)
		case common.CustomPropKeyHTTPHeaders:
			headers, err := parseHTTPHeaders(value)
			if err != nil {
				return config, fmt.Errorf("failed to parse HTTP headers: %w", err)
			}
			config.httpHeaders = headers
		case common.CustomPropKeyContentType:
			config.contentType = strings.ToUpper(value)
		case common.SenderPropertySupportedChannels:
			// Ignored here as it is a generic sender property
		default:
			logger.Warn(ctx, "Unknown property for HTTP webhook client", log.String("property", prop.GetName()))
		}
	}

	if config.url == "" {
		return config, errors.New("custom provider must have a URL property")
	}

	return config, nil
}

// parseHTTPHeaders parses a comma-separated string of HTTP headers into a map.
func parseHTTPHeaders(headersString string) (map[string]string, error) {
	headers := make(map[string]string)
	if strings.TrimSpace(headersString) == "" {
		return headers, nil
	}
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
