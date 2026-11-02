// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/client"
	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

var matchString = regexp.MatchString

// validateNotificationSender validates the notification sender data.
func validateNotificationSender(sender common.NotificationSenderDTO) *tidcommon.ServiceError {
	if sender.Name == "" {
		return &ErrorInvalidSenderName
	}

	switch sender.Type {
	case common.NotificationSenderTypeMessage:
		return validateMessageNotificationSender(sender)
	case common.NotificationSenderTypeEmail:
		return validateEmailNotificationSender(sender)
	default:
		return &ErrorInvalidSenderType
	}
}

// ValidateNotificationSender runs the checks the live /connections create and update APIs run on
// a sender. For use by the declarative connection loader, which otherwise writes straight to the
// file store without running this validation.
func ValidateNotificationSender(sender common.NotificationSenderDTO) error {
	if svcErr := validateNotificationSender(sender); svcErr != nil {
		if svcErr.ErrorDescription.DefaultValue != "" {
			return errors.New(svcErr.ErrorDescription.DefaultValue)
		}
		return errors.New(svcErr.Error.DefaultValue)
	}
	return nil
}

// invalidPropertiesError wraps a property validation failure in the client-facing error.
func invalidPropertiesError(err error) *tidcommon.ServiceError {
	svcErr := ErrorInvalidRequestFormat
	svcErr.ErrorDescription = tidcommon.I18nMessage{
		Key:          "error.notificationservice.sender_property_validation_failed_description",
		DefaultValue: err.Error(),
	}
	return &svcErr
}

// validateMessageNotificationSender validates a message notification sender.
func validateMessageNotificationSender(sender common.NotificationSenderDTO) *tidcommon.ServiceError {
	if sender.Provider == "" {
		return &ErrorInvalidProvider
	}
	if sender.Provider != common.NotificationProviderTypeTwilio &&
		sender.Provider != common.NotificationProviderTypeVonage &&
		sender.Provider != common.NotificationProviderTypeCustom {
		return &ErrorInvalidProvider
	}

	if err := validateMessageNotificationSenderProperties(sender); err != nil {
		return invalidPropertiesError(err)
	}

	return nil
}

// validateEmailNotificationSender validates an email notification sender.
func validateEmailNotificationSender(sender common.NotificationSenderDTO) *tidcommon.ServiceError {
	if sender.Provider != common.NotificationProviderTypeSMTP {
		return &ErrorInvalidProvider
	}

	if err := validateEmailNotificationSenderProperties(sender); err != nil {
		return invalidPropertiesError(err)
	}

	return nil
}

// validateEmailNotificationSenderProperties validates the properties of an email notification sender.
func validateEmailNotificationSenderProperties(sender common.NotificationSenderDTO) error {
	if len(sender.Properties) == 0 {
		return errors.New("email notification sender properties cannot be empty")
	}

	for _, prop := range sender.Properties {
		if prop.GetName() == common.SenderPropertySupportedChannels {
			val, err := prop.GetValue()
			if err != nil {
				return errors.New("failed to read supported channels property")
			}
			// An email sender currently only supports the "email" channel.
			if val != string(common.ChannelTypeEmail) {
				return fmt.Errorf("invalid supported channel: %s", val)
			}
			break
		}
	}

	return client.ValidateSMTPProperties(sender.Properties)
}

// validateMessageNotificationSenderProperties validates the properties of a message notification sender.
func validateMessageNotificationSenderProperties(sender common.NotificationSenderDTO) error {
	if len(sender.Properties) == 0 {
		return errors.New("message notification sender properties cannot be empty")
	}

	for _, prop := range sender.Properties {
		if prop.GetName() == common.SenderPropertySupportedChannels {
			val, err := prop.GetValue()
			if err != nil {
				return errors.New("failed to read supported channels property")
			}
			// A message sender currently only supports the "sms" channel.
			if val != string(common.ChannelTypeSMS) {
				return fmt.Errorf("invalid supported channel: %s", val)
			}
			break
		}
	}

	switch sender.Provider {
	case common.NotificationProviderTypeTwilio:
		return validateTwilioProperties(sender.Properties)
	case common.NotificationProviderTypeVonage:
		return validateVonageProperties(sender.Properties)
	case common.NotificationProviderTypeCustom:
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

// applyDefaultSenderProperties applies default properties to the notification sender.
func applyDefaultSenderProperties(sender *common.NotificationSenderDTO) {
	hasSupportedChannels := false
	for _, prop := range sender.Properties {
		if prop.GetName() == common.SenderPropertySupportedChannels {
			hasSupportedChannels = true
			break
		}
	}
	if !hasSupportedChannels {
		defaultChannel := common.ChannelTypeSMS
		if sender.Type == common.NotificationSenderTypeEmail {
			defaultChannel = common.ChannelTypeEmail
		}
		prop, _ := cmodels.NewProperty(common.SenderPropertySupportedChannels, string(defaultChannel), false)
		sender.Properties = append(sender.Properties, *prop)
	}
}
