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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	twilioURL                 = "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json"
	twilioLoggerComponentName = "TwilioClient"
	sIDRegex                  = `^AC[0-9a-fA-F]{32}$`
)

// TwilioClient implements the NotificationClientInterface for sending messages via Twilio API.
type TwilioClient struct {
	name       string
	url        string
	accountSID string
	authToken  string
	senderID   string
	httpClient syshttp.HTTPClientInterface
}

// NewTwilioClient creates a new instance of TwilioClient.
func NewTwilioClient(sender common.NotificationSenderDTO) (NotificationClientInterface, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, twilioLoggerComponentName))

	client := &TwilioClient{}
	client.name = sender.Name

	for _, prop := range sender.Properties {
		value, err := prop.GetValue()
		if err != nil {
			return nil, fmt.Errorf("failed to get property value for %s: %w", prop.GetName(), err)
		}

		switch prop.GetName() {
		case common.TwilioPropKeyAccountSID:
			client.accountSID = value
		case common.TwilioPropKeyAuthToken:
			client.authToken = value
		case common.TwilioPropKeySenderID:
			client.senderID = value
		default:
			logger.Warn("Unknown property for Twilio client", log.String("property", prop.GetName()))
		}
	}
	client.url = fmt.Sprintf(twilioURL, client.accountSID)
	client.httpClient = syshttp.NewHTTPClientWithTimeout(httpClientTimeout)

	return client, nil
}

// GetName returns the name of the Twilio client.
func (c *TwilioClient) GetName() string {
	return c.name
}

// IsChannelSupported reports whether the given channel is supported by Twilio.
func (c *TwilioClient) IsChannelSupported(channel common.ChannelType) bool {
	return channel == common.ChannelTypeSMS
}

// Send dispatches a notification via the requested channel.
func (c *TwilioClient) Send(channel common.ChannelType, data common.NotificationData) error {
	switch channel {
	case common.ChannelTypeSMS:
		return c.sendSMS(data)
	default:
		return fmt.Errorf("unsupported channel: %s", channel)
	}
}

// sendSMS sends an SMS via the Twilio API.
func (c *TwilioClient) sendSMS(data common.NotificationData) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, twilioLoggerComponentName))
	logger.Debug("Sending SMS via Twilio", log.MaskedString("to", data.Recipient))

	formData := url.Values{}
	formData.Set("To", data.Recipient)
	formData.Set("From", c.senderID)
	formData.Set("Body", data.Body)

	req, err := http.NewRequest(http.MethodPost, c.url, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.accountSID, c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("Failed to close response body", log.Error(closeErr))
		}
	}()

	logger.Debug("Received response from Twilio", log.Int("statusCode", resp.StatusCode))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logger.Error("Failed to send SMS via Twilio", log.Int("statusCode", resp.StatusCode),
			log.String("response", string(bodyBytes)))
		return fmt.Errorf("twilio SMS send failed, status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
