// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

// NotificationSenderType defines the type of notification sender.
type NotificationSenderType string

const (
	// NotificationSenderTypeMessage represents a message notification sender.
	NotificationSenderTypeMessage NotificationSenderType = "MESSAGE"
	// NotificationSenderTypeEmail represents an email notification sender.
	NotificationSenderTypeEmail NotificationSenderType = "EMAIL"
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
)

// ChannelType defines the type of communication channel.
type ChannelType string

const (
	// ChannelTypeSMS represents the SMS channel.
	ChannelTypeSMS ChannelType = "sms"
	// ChannelTypeEmail represents the email channel.
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
	// CustomPropKeyAPIKeyHeaders is the property key for custom-provider API key headers.
	// The legacy key is retained so existing SMS gateway connections remain readable.
	CustomPropKeyAPIKeyHeaders = "http_headers" // #nosec G101
	// CustomPropKeyContentType is the property key for the content type.
	CustomPropKeyContentType = "content_type"
)

const (
	// SMTPPropKeyHost is the property key for the SMTP host.
	SMTPPropKeyHost = "host"
	// SMTPPropKeyPort is the property key for the SMTP port.
	SMTPPropKeyPort = "port"
	// SMTPPropKeyFromAddress is the property key for the SMTP from address.
	SMTPPropKeyFromAddress = "from_address"
	// SMTPPropKeyFromName is the property key for the display name shown beside the from
	// address in the From header. Optional: without it the From header carries the bare address.
	SMTPPropKeyFromName = "from_name"
	// SMTPPropKeyTLS is the property key for the SMTP TLS mode.
	SMTPPropKeyTLS = "tls"
)

// TLSMode defines how the transport to an SMTP server is secured.
type TLSMode string

const (
	// TLSModeNone sends over a plaintext connection.
	TLSModeNone TLSMode = "none"
	// TLSModeSTARTTLS upgrades a plaintext connection with the STARTTLS command.
	TLSModeSTARTTLS TLSMode = "starttls"
	// TLSModeImplicit dials a TLS connection directly (SMTPS).
	TLSModeImplicit TLSMode = "implicit"
)

const (
	// SenderPropertySupportedChannels is the property key for the supported channels.
	SenderPropertySupportedChannels = "supported_channels"
)
