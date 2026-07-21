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

// NotificationSenderDTO represents the data transfer object for a notification sender.
type NotificationSenderDTO struct {
	ID          string                 `yaml:"id,omitempty"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description,omitempty"`
	Type        NotificationSenderType `yaml:"-"`
	Provider    MessageProviderType    `yaml:"provider"`
	Properties  []cmodels.Property     `yaml:"properties,omitempty"`
}

// VerifyOTPDTO represents the service layer data structure for verifying an OTP.
type VerifyOTPDTO struct {
	SessionToken string
	OTPCode      string
}

// VerifyOTPResultDTO represents the service layer result for OTP verify operation.
type VerifyOTPResultDTO struct {
	Status        OTPVerifyStatus
	Recipient     string
	RecipientAttr string
}
