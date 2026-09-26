// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) SetupSuite() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *UtilsTestSuite) TestValidateNotificationSender_EmptyName() {
	sender := common.NotificationSenderDTO{
		Name:     "",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeTwilio,
	}

	err := validateNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderName.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateNotificationSender_InvalidType() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     "INVALID_TYPE",
		Provider: common.NotificationProviderTypeTwilio,
	}

	err := validateNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidSenderType.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_EmptyProvider() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: "",
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidProvider.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_InvalidProvider() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: "invalid-provider",
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidProvider.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_Twilio() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Twilio",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_Vonage() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Vonage",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeVonage,
		Properties: []cmodels.Property{
			createTestProperty("api_key", "test-key", true),
			createTestProperty("api_secret", "test-secret", true),
			createTestProperty("sender_id", "TestSender", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_Custom() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Custom",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeCustom,
		Properties: []cmodels.Property{
			createTestProperty("url", "https://api.example.com/sms", false),
			createTestProperty("http_method", "POST", false),
			createTestProperty("content_type", "JSON", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_MissingAccountSID() {
	properties := []cmodels.Property{
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "account_sid")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_MissingAuthToken() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "auth_token")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_MissingSenderID() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("auth_token", "test-token", true),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "sender_id")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_InvalidAccountSIDFormat() {
	properties := []cmodels.Property{
		createTestProperty("account_sid", "invalid-sid", true),
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "account SID format")
}

func (suite *UtilsTestSuite) TestValidateVonageProperties() {
	properties := []cmodels.Property{
		createTestProperty("api_key", "test-key", true),
		createTestProperty("api_secret", "test-secret", true),
		createTestProperty("sender_id", "TestSender", false),
	}

	err := validateVonageProperties(properties)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateVonageProperties_MissingAPIKey() {
	properties := []cmodels.Property{
		createTestProperty("api_secret", "test-secret", true),
		createTestProperty("sender_id", "TestSender", false),
	}

	err := validateVonageProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "api_key")
}

func (suite *UtilsTestSuite) TestValidateVonageProperties_MissingAPISecret() {
	properties := []cmodels.Property{
		createTestProperty("api_key", "test-key", true),
		createTestProperty("sender_id", "TestSender", false),
	}

	err := validateVonageProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "api_secret")
}

func (suite *UtilsTestSuite) TestValidateVonageProperties_MissingSenderID() {
	properties := []cmodels.Property{
		createTestProperty("api_key", "test-key", true),
		createTestProperty("api_secret", "test-secret", true),
	}

	err := validateVonageProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "sender_id")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "JSON", false),
	}

	err := validateCustomProperties(properties)

	suite.Nil(err)
}

// Plaintext stays allowed: the gateway carries any credential in its own headers.
func (suite *UtilsTestSuite) TestValidateCustomProperties_PlaintextAllowed() {
	properties := []cmodels.Property{
		createTestProperty("url", "http://api.example.com/sms", false),
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "JSON", false),
	}

	suite.Nil(validateCustomProperties(properties))
}

// An Authorization header is how a gateway carries a credential today.
func (suite *UtilsTestSuite) TestValidateCustomProperties_AuthorizationHeaderAllowed() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "JSON", false),
		createTestProperty("http_headers", "Authorization:Bearer token", false),
	}

	suite.Nil(validateCustomProperties(properties))
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_MissingURL() {
	properties := []cmodels.Property{
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "JSON", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "URL")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_InvalidHTTPMethod() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "DELETE", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "HTTP method")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_InvalidContentType() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("http_method", "POST", false),
		createTestProperty("content_type", "XML", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "content type")
}

func (suite *UtilsTestSuite) TestValidateCustomProperties_EmptyPropertyName() {
	properties := []cmodels.Property{
		createTestProperty("url", "https://api.example.com/sms", false),
		createTestProperty("", "value", false),
	}

	err := validateCustomProperties(properties)

	suite.NotNil(err)
	suite.Contains(err.Error(), "non-empty name")
}

func (suite *UtilsTestSuite) TestValidateSenderProperties() {
	properties := []cmodels.Property{
		createTestProperty("prop1", "value1", false),
		createTestProperty("prop2", "value2", false),
	}
	requiredProps := map[string]bool{
		"prop1": false,
		"prop2": false,
	}

	err := validateSenderProperties(properties, requiredProps)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateSenderProperties_MissingRequired() {
	properties := []cmodels.Property{
		createTestProperty("prop1", "value1", false),
	}
	requiredProps := map[string]bool{
		"prop1": false,
		"prop2": false,
	}

	err := validateSenderProperties(properties, requiredProps)

	suite.NotNil(err)
	suite.Contains(err.Error(), "prop2")
}

func (suite *UtilsTestSuite) TestValidateSenderProperties_EmptyName() {
	properties := []cmodels.Property{
		createTestProperty("", "value1", false),
	}
	requiredProps := map[string]bool{}

	err := validateSenderProperties(properties, requiredProps)

	suite.NotNil(err)
	suite.Contains(err.Error(), "non-empty name")
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_EmptyProperties() {
	sender := common.NotificationSenderDTO{
		Name:       "Test Twilio Empty Props",
		Type:       common.NotificationSenderTypeMessage,
		Provider:   common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{},
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "message notification sender properties cannot be empty")
}

func (suite *UtilsTestSuite) TestValidateNotificationSender() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
		},
	}

	err := validateNotificationSender(sender)
	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSenderProperties_UnsupportedProvider() {
	sender := common.NotificationSenderDTO{
		Provider:   "unsupported-provider",
		Properties: []cmodels.Property{createTestProperty("k", "v", false)},
	}

	err := validateMessageNotificationSenderProperties(sender)
	suite.NotNil(err)
	suite.Contains(err.Error(), "unsupported message notification sender")
}

func (suite *UtilsTestSuite) TestValidateTwilioProperties_RegexError() {
	// patch matchString to return an error
	orig := matchString
	matchString = func(pattern, s string) (bool, error) {
		return false, errors.New("regex fail")
	}
	defer func() { matchString = orig }()

	properties := []cmodels.Property{
		createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
		createTestProperty("auth_token", "test-token", true),
		createTestProperty("sender_id", "+15551234567", false),
	}

	err := validateTwilioProperties(properties)
	suite.NotNil(err)
	suite.Contains(err.Error(), "failed to validate Twilio account SID")
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_InvalidSupportedChannel() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Sender Invalid Channel",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeTwilio,
		Properties: []cmodels.Property{
			createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true),
			createTestProperty("auth_token", "test-token", true),
			createTestProperty("sender_id", "+15551234567", false),
			createTestProperty(common.SenderPropertySupportedChannels, "email", false),
		},
	}

	err := validateMessageNotificationSender(sender)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidRequestFormat.Code, err.Code)
	suite.Contains(err.ErrorDescription.DefaultValue, "invalid supported channel: email")
}

func (suite *UtilsTestSuite) TestValidateMessageNotificationSender_SupportedChannelReadError() {
	properties, err := cmodels.DeserializePropertiesFromJSONObject(
		`{"supported_channels": {"value": "invalid-secret", "isSecret": true}}`)
	suite.NoError(err)

	properties = append(properties, createTestProperty("account_sid", "AC00112233445566778899aabbccddeeff", true))
	properties = append(properties, createTestProperty("auth_token", "test-token", true))
	properties = append(properties, createTestProperty("sender_id", "+15551234567", false))

	sender := common.NotificationSenderDTO{
		Name:       "Test Sender Read Error",
		Type:       common.NotificationSenderTypeMessage,
		Provider:   common.NotificationProviderTypeTwilio,
		Properties: properties,
	}

	errSvc := validateMessageNotificationSender(sender)

	suite.NotNil(errSvc)
	suite.Equal(ErrorInvalidRequestFormat.Code, errSvc.Code)
	suite.Contains(errSvc.ErrorDescription.DefaultValue, "failed to read supported channels property")
}

// smtpProperties builds a valid SMTP property set, applying the given overrides. A nil value
// removes the property.
func (suite *UtilsTestSuite) smtpProperties(overrides map[string]*string) []cmodels.Property {
	values := map[string]string{
		common.SMTPPropKeyHost:        "smtp.example.com",
		common.SMTPPropKeyPort:        "587",
		common.SMTPPropKeyFromAddress: "noreply@example.com",
	}
	for name, value := range overrides {
		if value == nil {
			delete(values, name)
			continue
		}
		values[name] = *value
	}

	properties := make([]cmodels.Property, 0, len(values))
	for name, value := range values {
		isSecret := name == outboundauth.PropertyKey(outboundauth.FieldBasicPassword)
		properties = append(properties, createTestProperty(name, value, isSecret))
	}
	return properties
}

// basicAuthOverrides configures the basic method with the given credentials. A nil value drops
// the property, matching the override convention used throughout this suite.
func basicAuthOverrides(username, password *string) map[string]*string {
	return map[string]*string{
		outboundauth.PropertyKeyType:                              strPtr(string(outboundauth.TypeBasic)),
		outboundauth.PropertyKey(outboundauth.FieldBasicUsername): username,
		outboundauth.PropertyKey(outboundauth.FieldBasicPassword): password,
	}
}

func (suite *UtilsTestSuite) smtpSender(overrides map[string]*string) common.NotificationSenderDTO {
	return common.NotificationSenderDTO{
		Name:       "Test SMTP Sender",
		Type:       common.NotificationSenderTypeEmail,
		Provider:   common.NotificationProviderTypeSMTP,
		Properties: suite.smtpProperties(overrides),
	}
}

func strPtr(value string) *string { return &value }

// withTLS sets the transport security mode on an override map and returns it.
func withTLS(overrides map[string]*string, mode common.TLSMode) map[string]*string {
	overrides[common.SMTPPropKeyTLS] = strPtr(string(mode))
	return overrides
}

func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_Valid() {
	suite.Nil(validateNotificationSender(suite.smtpSender(nil)))
}

func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_WithAuthOverTLS() {
	overrides := basicAuthOverrides(strPtr("user"), strPtr("pass"))
	overrides[common.SMTPPropKeyTLS] = strPtr(string(common.TLSModeSTARTTLS))
	suite.Nil(validateNotificationSender(suite.smtpSender(overrides)))
}

func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_InvalidProvider() {
	sender := suite.smtpSender(nil)
	sender.Provider = common.NotificationProviderTypeTwilio

	errSvc := validateNotificationSender(sender)
	suite.Require().NotNil(errSvc)
	suite.Equal(ErrorInvalidProvider.Code, errSvc.Code)
}

func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_EmptyProvider() {
	sender := suite.smtpSender(nil)
	sender.Provider = ""

	errSvc := validateNotificationSender(sender)
	suite.Require().NotNil(errSvc)
	suite.Equal(ErrorInvalidProvider.Code, errSvc.Code)
}

func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_NoProperties() {
	sender := suite.smtpSender(nil)
	sender.Properties = nil

	errSvc := validateNotificationSender(sender)
	suite.Require().NotNil(errSvc)
	suite.Equal(ErrorInvalidRequestFormat.Code, errSvc.Code)
	suite.Contains(errSvc.ErrorDescription.DefaultValue, "cannot be empty")
}

func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_WrongSupportedChannel() {
	sender := suite.smtpSender(map[string]*string{
		common.SenderPropertySupportedChannels: strPtr(string(common.ChannelTypeSMS)),
	})

	errSvc := validateNotificationSender(sender)
	suite.Require().NotNil(errSvc)
	suite.Contains(errSvc.ErrorDescription.DefaultValue, "invalid supported channel: sms")
}

// Everything the client needs is checked when the sender is written, so a provider that
// cannot deliver is rejected at configuration time rather than during a password reset.
func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_PropertyErrors() {
	cases := []struct {
		name      string
		overrides map[string]*string
		contains  string
	}{
		{"missing host", map[string]*string{common.SMTPPropKeyHost: nil}, "host"},
		{"blank host", map[string]*string{common.SMTPPropKeyHost: strPtr("   ")}, "host"},
		{"missing port", map[string]*string{common.SMTPPropKeyPort: nil}, "port"},
		{"non numeric port", map[string]*string{common.SMTPPropKeyPort: strPtr("abc")}, "between 1 and 65535"},
		{"zero port", map[string]*string{common.SMTPPropKeyPort: strPtr("0")}, "between 1 and 65535"},
		{"port too large", map[string]*string{common.SMTPPropKeyPort: strPtr("70000")}, "between 1 and 65535"},
		{"missing from address", map[string]*string{common.SMTPPropKeyFromAddress: nil}, "from_address"},
		{"invalid from address", map[string]*string{
			common.SMTPPropKeyFromAddress: strPtr("not-an-address")}, "invalid from address"},
		{"display name from address", map[string]*string{
			common.SMTPPropKeyFromAddress: strPtr(`"Bot" <bot@example.com>`)}, "invalid from address"},
		{"from name with a line break", map[string]*string{
			common.SMTPPropKeyFromName: strPtr("Acme\r\nBcc: attacker@evil.test")}, "from name"},
		{"invalid tls mode", map[string]*string{
			common.SMTPPropKeyTLS: strPtr("true")}, "none, starttls or implicit"},
		{"auth without tls", withTLS(
			basicAuthOverrides(strPtr("user"), strPtr("pass")), common.TLSModeNone,
		), "tls must be enabled"},
		{"auth without credentials", withTLS(
			basicAuthOverrides(nil, nil), common.TLSModeSTARTTLS,
		), "username"},
		{"auth without password", withTLS(
			basicAuthOverrides(strPtr("user"), nil), common.TLSModeSTARTTLS,
		), "password"},
		{"unsupported auth type", map[string]*string{
			outboundauth.PropertyKeyType: strPtr("bearer"),
		}, "unsupported authentication type"},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			errSvc := validateNotificationSender(suite.smtpSender(tc.overrides))
			suite.Require().NotNil(errSvc)
			suite.Equal(ErrorInvalidRequestFormat.Code, errSvc.Code)
			suite.Contains(errSvc.ErrorDescription.DefaultValue, tc.contains)
		})
	}
}

// An omitted tls property is the secure default, so it must validate as STARTTLS rather than
// being treated as plaintext.
func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_OmittedTLSAllowsAuth() {
	sender := suite.smtpSender(basicAuthOverrides(strPtr("user"), strPtr("pass")))
	suite.Nil(validateNotificationSender(sender))
}

// Plaintext transport is only rejected when credentials would actually travel over it.
func (suite *UtilsTestSuite) TestValidateEmailNotificationSender_NoAuthAllowsPlaintext() {
	sender := suite.smtpSender(map[string]*string{
		common.SMTPPropKeyTLS:        strPtr(string(common.TLSModeNone)),
		outboundauth.PropertyKeyType: strPtr(string(outboundauth.TypeNone)),
	})
	suite.Nil(validateNotificationSender(sender))
}

func (suite *UtilsTestSuite) TestApplyDefaultSenderProperties_EmailDefaultsToEmailChannel() {
	sender := suite.smtpSender(nil)
	applyDefaultSenderProperties(&sender)

	value, found := "", false
	for _, prop := range sender.Properties {
		if prop.GetName() == common.SenderPropertySupportedChannels {
			value, _ = prop.GetValue()
			found = true
		}
	}
	suite.True(found)
	suite.Equal(string(common.ChannelTypeEmail), value)
}

func (suite *UtilsTestSuite) TestApplyDefaultSenderProperties_MessageDefaultsToSMSChannel() {
	sender := common.NotificationSenderDTO{
		Name:     "Test Twilio",
		Type:     common.NotificationSenderTypeMessage,
		Provider: common.NotificationProviderTypeTwilio,
	}
	applyDefaultSenderProperties(&sender)

	value, found := "", false
	for _, prop := range sender.Properties {
		if prop.GetName() == common.SenderPropertySupportedChannels {
			value, _ = prop.GetValue()
			found = true
		}
	}
	suite.True(found)
	suite.Equal(string(common.ChannelTypeSMS), value)
}
