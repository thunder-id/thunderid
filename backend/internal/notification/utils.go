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

package notification

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

var matchString = regexp.MatchString

// validateNotificationSender validates the notification sender data.
func validateNotificationSender(sender common.NotificationSenderDTO) *serviceerror.ServiceError {
	if sender.Name == "" {
		return &ErrorInvalidSenderName
	}

	switch sender.Type {
	case common.NotificationSenderTypeMessage:
		return validateMessageNotificationSender(sender)
	default:
		return &ErrorInvalidSenderType
	}
}

// validateMessageNotificationSender validates a message notification sender.
func validateMessageNotificationSender(sender common.NotificationSenderDTO) *serviceerror.ServiceError {
	if sender.Provider == "" {
		return &ErrorInvalidProvider
	}
	if sender.Provider != common.MessageProviderTypeTwilio &&
		sender.Provider != common.MessageProviderTypeVonage &&
		sender.Provider != common.MessageProviderTypeCustom {
		return &ErrorInvalidProvider
	}

	if err := validateMessageNotificationSenderProperties(sender); err != nil {
		svcErr := ErrorInvalidRequestFormat
		svcErr.ErrorDescription = core.I18nMessage{
			Key:          "error.notificationservice.sender_property_validation_failed_description",
			DefaultValue: err.Error(),
		}
		return &svcErr
	}

	return nil
}

// validateMessageNotificationSenderProperties validates the properties of a message notification sender.
func validateMessageNotificationSenderProperties(sender common.NotificationSenderDTO) error {
	if len(sender.Properties) == 0 {
		return errors.New("message notification sender properties cannot be empty")
	}

	switch sender.Provider {
	case common.MessageProviderTypeTwilio:
		return validateTwilioProperties(sender.Properties)
	case common.MessageProviderTypeVonage:
		return validateVonageProperties(sender.Properties)
	case common.MessageProviderTypeCustom:
		return validateCustomProperties(sender.Properties)
	default:
		return errors.New("unsupported message notification sender")
	}
}

// validateTwilioProperties validates the message notification sender properties for a Twilio client.
func validateTwilioProperties(properties []cmodels.Property) error {
	requiredProps := map[string]bool{
		"account_sid": false,
		"auth_token":  false,
		"sender_id":   false,
	}
	err := validateSenderProperties(properties, requiredProps)
	if err != nil {
		return err
	}

	// Validate the account SID format
	sIDRegex := `^AC[0-9a-fA-F]{32}$`
	sid := ""
	for _, prop := range properties {
		if prop.GetName() == common.TwilioPropKeyAccountSID {
			propValue, err := prop.GetValue()
			if err == nil {
				sid = propValue
			}
			break
		}
	}
	matched, err := matchString(sIDRegex, sid)
	if err != nil {
		return fmt.Errorf("failed to validate Twilio account SID: %w", err)
	}
	if !matched {
		return errors.New("invalid Twilio account SID format")
	}

	return nil
}

// validateVonageProperties validates the message notification sender properties for a Vonage client.
func validateVonageProperties(properties []cmodels.Property) error {
	requiredProps := map[string]bool{
		"api_key":    false,
		"api_secret": false,
		"sender_id":  false,
	}
	return validateSenderProperties(properties, requiredProps)
}

// validateCustomProperties validates the message notification sender properties for a custom client.
func validateCustomProperties(properties []cmodels.Property) error {
	validHTTPMethods := []string{http.MethodGet, http.MethodPost}
	validContentTypes := []string{"JSON", "FORM"}

	url := ""
	httpMethod := ""
	contentType := ""
	for _, prop := range properties {
		if prop.GetName() == "" {
			return errors.New("properties must have non-empty name")
		}
		propValue, err := prop.GetValue()
		if err != nil {
			continue
		}
		switch prop.GetName() {
		case common.CustomPropKeyURL:
			url = propValue
		case common.CustomPropKeyHTTPMethod:
			httpMethod = strings.ToUpper(propValue)
		case common.CustomPropKeyContentType:
			contentType = strings.ToUpper(propValue)
		}
	}
	if url == "" {
		return errors.New("custom provider must have a URL property")
	}
	if httpMethod != "" && !slices.Contains(validHTTPMethods, httpMethod) {
		return errors.New("custom provider must have a valid HTTP method")
	}
	if contentType != "" && !slices.Contains(validContentTypes, contentType) {
		return errors.New("custom provider must have a valid content type (JSON or FORM)")
	}

	return nil
}

// validateSenderProperties validates the properties for a notification sender.
func validateSenderProperties(properties []cmodels.Property, requiredProperties map[string]bool) error {
	for _, prop := range properties {
		if prop.GetName() == "" {
			return errors.New("properties must have non-empty name")
		}
		if _, exists := requiredProperties[prop.GetName()]; exists {
			requiredProperties[prop.GetName()] = true
		}
	}

	// Check if all required properties are present
	for key, found := range requiredProperties {
		if !found {
			return errors.New("required property missing for the provider: " + key)
		}
	}
	return nil
}
