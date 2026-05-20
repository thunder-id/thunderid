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

// Package common contains the common models and constants for notification package.
package common

import "github.com/thunder-id/thunderid/internal/system/cmodels"

// SMSData represents the data structure for a SMS message.
type SMSData struct {
	To   string `json:"to"`
	Body string `json:"body"`
}

// NotificationData holds the channel-agnostic payload for sending a notification.
type NotificationData struct {
	Recipient string
	Body      string
}

// OTP represents the data structure for an OTP (One-Time Password).
type OTP struct {
	Value                  string `json:"value"`
	GeneratedTimeInMillis  int64  `json:"generated_time_in_millis"`
	ValidityPeriodInMillis int64  `json:"validity_period_in_millis"`
	ExpiryTimeInMillis     int64  `json:"expiry_time_in_millis"`
	AttemptCount           int    `json:"attempt_count"`
}

// NotificationSenderDTO represents the data transfer object for a notification sender.
type NotificationSenderDTO struct {
	ID          string                 `yaml:"id,omitempty"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description,omitempty"`
	Type        NotificationSenderType `yaml:"-"`
	Provider    MessageProviderType    `yaml:"provider"`
	Properties  []cmodels.Property     `yaml:"properties,omitempty"`
}

// NotificationSenderRequest represents the request structure for creating or updating a notification sender.
type NotificationSenderRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Provider    string                `json:"provider"`
	Properties  []cmodels.PropertyDTO `json:"properties"`
}

// NotificationSenderResponse represents the response structure for a notification sender.
type NotificationSenderResponse struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Provider    MessageProviderType   `json:"provider"`
	Properties  []cmodels.PropertyDTO `json:"properties"`
}

// SendOTPRequest represents the request structure for sending an OTP.
type SendOTPRequest struct {
	Recipient string `json:"recipient"`
	SenderID  string `json:"senderId"`
	Channel   string `json:"channel"`
}

// SendOTPResponse represents the response structure for OTP send request.
type SendOTPResponse struct {
	SessionToken string `json:"sessionToken"`
	Status       string `json:"status"`
}

// VerifyOTPRequest represents the request structure for verifying an OTP.
type VerifyOTPRequest struct {
	SessionToken string `json:"sessionToken"`
	OTPCode      string `json:"otpCode"`
}

// VerifyOTPResponse represents the response structure for OTP verification.
type VerifyOTPResponse struct {
	Status string `json:"status"`
}

// SendOTPDTO represents the service layer data structure for sending an OTP.
type SendOTPDTO struct {
	Recipient string
	SenderID  string
	Channel   string
}

// SendOTPResultDTO represents the service layer result for OTP send operation.
type SendOTPResultDTO struct {
	SessionToken string
}

// VerifyOTPDTO represents the service layer data structure for verifying an OTP.
type VerifyOTPDTO struct {
	SessionToken string
	OTPCode      string
}

// VerifyOTPResultDTO represents the service layer result for OTP verify operation.
type VerifyOTPResultDTO struct {
	Status    OTPVerifyStatus
	Recipient string
}

// OTPSessionData represents the data stored in the OTP session token.
type OTPSessionData struct {
	Recipient  string `json:"recipient"`
	Channel    string `json:"channel"`
	SenderID   string `json:"senderId"`
	OTPValue   string `json:"otp_value"`
	ExpiryTime int64  `json:"expiry_time"`
}

// NotificationSenderRequestWithID represents the request structure for creating a notification sender
// from file-based config.
type NotificationSenderRequestWithID struct {
	ID          string                `yaml:"id"`
	Name        string                `yaml:"name"`
	Description string                `yaml:"description,omitempty"`
	Provider    string                `yaml:"provider"`
	Properties  []cmodels.PropertyDTO `yaml:"properties,omitempty"`
}
