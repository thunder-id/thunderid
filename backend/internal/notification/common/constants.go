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

package common

// NotificationSenderType defines the type of notification sender.
type NotificationSenderType string

const (
	// NotificationSenderTypeMessage represents a message notification sender.
	NotificationSenderTypeMessage NotificationSenderType = "MESSAGE"
	// NotificationSenderTypeEmail represents an email notification sender.
	NotificationSenderTypeEmail NotificationSenderType = "EMAIL"

	// CRLF is theStandard line ending used in network protocols (e.g., SMTP, HTTP).
	CRLF = "\r\n"
)

// NotificationProviderType defines the type of messaging provider.
type NotificationProviderType string

const (
	// NotificationProviderTypeVonage represents the Vonage messaging provider.
	NotificationProviderTypeVonage NotificationProviderType = "vonage"
	// NotificationProviderTypeTwilio represents the Twilio messaging provider.
	NotificationProviderTypeTwilio NotificationProviderType = "twilio"
	// NotificationProviderTypeCustom represents a custom messaging provider.
	NotificationProviderTypeCustom NotificationProviderType = "custom"

	// NotificationProviderTypeSMTP represents the SMTP email provider.
	NotificationProviderTypeSMTP NotificationProviderType = "smtp"
	// NotificationProviderTypeHTTP represents an HTTP webhook email provider.
	NotificationProviderTypeHTTP NotificationProviderType = "http"
)

// ChannelType defines the type of communication channel.
type ChannelType string

const (
	// ChannelTypeSMS represents the SMS channel.
	ChannelTypeSMS ChannelType = "sms"
	// ChannelTypeEmail represents the Email channel.
	ChannelTypeEmail ChannelType = "email"
)

// OTPVerifyStatus defines the status of OTP verification.
type OTPVerifyStatus string

const (
	// OTPVerifyStatusVerified indicates a successful OTP verification.
	OTPVerifyStatusVerified OTPVerifyStatus = "VERIFIED"
	// OTPVerifyStatusInvalid indicates an invalid OTP verification attempt.
	OTPVerifyStatusInvalid OTPVerifyStatus = "INVALID"
)

const (
	// VonagePropKeyAPIKey is the property key for the Vonage API key.
	VonagePropKeyAPIKey = "api_key"
	// VonagePropKeyAPISecret is the property key for the Vonage API secret.
	VonagePropKeyAPISecret = "api_secret"
	// VonagePropKeySenderID is the property key for the Vonage sender ID.
	VonagePropKeySenderID = "sender_id"
)

const (
	// TwilioPropKeyAccountSID is the property key for the Twilio account SID.
	TwilioPropKeyAccountSID = "account_sid"
	// TwilioPropKeyAuthToken is the property key for the Twilio auth token.
	TwilioPropKeyAuthToken = "auth_token"
	// TwilioPropKeySenderID is the property key for the Twilio sender ID.
	TwilioPropKeySenderID = "sender_id"
)

const (
	// CustomPropKeyURL is the property key for the custom URL.
	CustomPropKeyURL = "url"
	// CustomPropKeyHTTPMethod is the property key for the HTTP method.
	CustomPropKeyHTTPMethod = "http_method"
	// CustomPropKeyHTTPHeaders is the property key for the HTTP headers.
	CustomPropKeyHTTPHeaders = "http_headers"
	// CustomPropKeyContentType is the property key for the content type.
	CustomPropKeyContentType = "content_type"
)

const (
	// SMTPPropKeyHost is the property key for the SMTP host.
	SMTPPropKeyHost = "host"
	// SMTPPropKeyPort is the property key for the SMTP port.
	SMTPPropKeyPort = "port"
	// SMTPPropKeyUsername is the property key for the SMTP username.
	SMTPPropKeyUsername = "username"
	// SMTPPropKeyPassword is the property key for the SMTP password.
	SMTPPropKeyPassword = "password"
	// SMTPPropKeyFromAddress is the property key for the SMTP from address.
	SMTPPropKeyFromAddress = "from_address"
	// SMTPPropKeyTLS is the property key for the SMTP TLS mode.
	SMTPPropKeyTLS = "tls"
	// SMTPPropKeyEnableAuth is the property key to enable authentication.
	SMTPPropKeyEnableAuth = "enable_authentication"
)

// TLSMode defines the TLS mode for SMTP connections.
type TLSMode string

const (
	// TLSModeNone represents a plaintext connection.
	TLSModeNone TLSMode = "none"
	// TLSModeSTARTTLS represents an explicit TLS connection using STARTTLS.
	TLSModeSTARTTLS TLSMode = "starttls"
	// TLSModeImplicit represents an implicit TLS connection (SMTPS).
	TLSModeImplicit TLSMode = "implicit"
)

const (
	// SenderPropertySupportedChannels is the property key for the supported channels.
	SenderPropertySupportedChannels = "supported_channels"
)
