// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
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

	return validateSMTPProperties(sender.Properties)
}

// validateSMTPProperties validates the email notification sender properties for an SMTP client.
// Every value is checked here rather than at send time, so a sender that cannot deliver is
// rejected when it is configured instead of during a password reset.
func validateSMTPProperties(properties []cmodels.Property) error {
	values := make(map[string]string, len(properties))
	for _, prop := range properties {
		if prop.GetName() == "" {
			return errors.New("properties must have non-empty name")
		}
		propValue, err := prop.GetValue()
		if err != nil {
			return fmt.Errorf("failed to read property %s", prop.GetName())
		}
		values[prop.GetName()] = propValue
	}

	if strings.TrimSpace(values[common.SMTPPropKeyHost]) == "" {
		return errors.New("required property missing for the provider: " + common.SMTPPropKeyHost)
	}

	rawPort := strings.TrimSpace(values[common.SMTPPropKeyPort])
	if rawPort == "" {
		return errors.New("required property missing for the provider: " + common.SMTPPropKeyPort)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be an integer between 1 and 65535, got: %s", rawPort)
	}

	fromAddress := strings.TrimSpace(values[common.SMTPPropKeyFromAddress])
	if fromAddress == "" {
		return errors.New("required property missing for the provider: " + common.SMTPPropKeyFromAddress)
	}
	if !common.IsValidEmailAddress(fromAddress) {
		return fmt.Errorf("invalid from address: %s", fromAddress)
	}

	// The display name is optional, but it lands in the From header, so it must not be able to
	// end that header and start one of its own.
	if !common.IsValidHeaderText(values[common.SMTPPropKeyFromName]) {
		return errors.New("from name must not contain line breaks")
	}

	tlsMode, ok := common.ParseTLSMode(values[common.SMTPPropKeyTLS])
	if !ok {
		return fmt.Errorf("tls must be one of none, starttls or implicit, got: %s",
			values[common.SMTPPropKeyTLS])
	}

	authConfig := outboundauth.FromValues(values)
	if err := outboundauth.Validate(authConfig, common.SMTPSupportedAuthTypes); err != nil {
		return err
	}
	// Transport policy, not a generic rule: credentials must not travel in the clear.
	if authConfig.Enabled() && tlsMode == common.TLSModeNone {
		return errors.New("tls must be enabled when authentication is configured")
	}

	return nil
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

	values := make(map[string]string, len(properties))
	for _, prop := range properties {
		if prop.GetName() == "" {
			return errors.New("properties must have non-empty name")
		}
		propValue, err := prop.GetValue()
		if err != nil {
			continue
		}
		values[prop.GetName()] = propValue
	}

	rawURL := values[common.CustomPropKeyURL]
	httpMethod := strings.ToUpper(values[common.CustomPropKeyHTTPMethod])
	contentType := strings.ToUpper(values[common.CustomPropKeyContentType])

	if rawURL == "" {
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
