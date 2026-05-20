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
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/internal/notification/common"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	vonageURL                 = "https://api.nexmo.com/v1/messages"
	vonageLoggerComponentName = "VonageClient"
)

// VonageClient implements the NotificationClientInterface for sending messages via Vonage API.
type VonageClient struct {
	name       string
	url        string
	apiKey     string
	apiSecret  string
	senderID   string
	httpClient syshttp.HTTPClientInterface
}

// NewVonageClient creates a new instance of VonageClient.
func NewVonageClient(sender common.NotificationSenderDTO) (NotificationClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, vonageLoggerComponentName))

	client := &VonageClient{}
	client.name = sender.Name
	client.url = vonageURL

	for _, prop := range sender.Properties {
		value, err := prop.GetValue()
		if err != nil {
			return nil, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.VonagePropKeyAPIKey:
			client.apiKey = value
		case common.VonagePropKeyAPISecret:
			client.apiSecret = value
		case common.VonagePropKeySenderID:
			client.senderID = value
		default:
			logger.Warn("Unknown property for Vonage client", log.String("property", prop.GetName()))
		}
	}
	client.httpClient = syshttp.NewHTTPClientWithTimeout(httpClientTimeout)

	return client, nil
}

// GetName returns the name of the Vonage client.
func (v *VonageClient) GetName() string {
	return v.name
}

// IsChannelSupported reports whether the given channel is supported by Vonage.
func (v *VonageClient) IsChannelSupported(channel common.ChannelType) bool {
	return channel == common.ChannelTypeSMS
}

// Send dispatches a notification via the requested channel.
func (v *VonageClient) Send(channel common.ChannelType, data common.NotificationData) error {
	switch channel {
	case common.ChannelTypeSMS:
		return v.sendSMS(data)
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}

// sendSMS sends an SMS via the Vonage API.
func (v *VonageClient) sendSMS(data common.NotificationData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, vonageLoggerComponentName))
	logger.Debug("Sending SMS via Vonage", log.MaskedString("to", data.Recipient))

	formattedPhoneNumber := v.formatPhoneNumber(data.Recipient)

	payload := map[string]interface{}{
		"message_type": "text",
		"channel":      "sms",
		"from":         v.senderID,
		"to":           formattedPhoneNumber,
		"text":         data.Body,
		"sms": map[string]interface{}{
			"encoding_type": "text",
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, v.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(v.apiKey, v.apiSecret)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("Failed to close response body", log.Error(closeErr))
		}
	}()

	logger.Debug("Received response from Vonage", log.Int("statusCode", resp.StatusCode))

	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error("Failed to send SMS via Vonage", log.Int("statusCode", resp.StatusCode),
			log.String("response", string(bodyBytes)))
		return fmt.Errorf("vonage SMS send failed, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// formatPhoneNumber formats a phone number to comply with Vonage E.164 requirements
// by removing any leading '+' or '00' from the number.
func (v *VonageClient) formatPhoneNumber(phoneNumber string) string {
	if len(phoneNumber) > 0 && phoneNumber[0] == '+' {
		return phoneNumber[1:]
	}
	if len(phoneNumber) > 1 && phoneNumber[0:2] == "00" {
		return phoneNumber[2:]
	}
	return phoneNumber
}
