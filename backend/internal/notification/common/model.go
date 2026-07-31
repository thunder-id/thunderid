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

import (
	"errors"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cmodels"
)

// MessageData holds the channel-agnostic payload for sending an SMS or message.
type MessageData struct {
	Recipient string
	Body      string
}

// EmailData holds the payload for sending an email.
type EmailData struct {
	To      []string
	CC      []string
	BCC     []string
	Subject string
	Body    string
	IsHTML  bool
}

// NotificationSenderDTO represents the data transfer object for a notification sender.
type NotificationSenderDTO struct {
	ID          string                   `yaml:"id,omitempty"`
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description,omitempty"`
	Type        NotificationSenderType   `yaml:"-"`
	Provider    NotificationProviderType `yaml:"provider"`
	Properties  []cmodels.Property       `yaml:"properties,omitempty"`
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

// Validate cleans and validates the email data payload.
func (e *EmailData) Validate() error {
	trimSlice := func(s []string) []string {
		if s == nil {
			return nil
		}
		res := make([]string, len(s))
		for i, v := range s {
			res[i] = strings.TrimSpace(v)
		}
		return res
	}

	e.Subject = strings.TrimSpace(e.Subject)
	e.To = trimSlice(e.To)
	e.CC = trimSlice(e.CC)
	e.BCC = trimSlice(e.BCC)

	if len(e.To) == 0 || len(e.To[0]) == 0 {
		return errors.New("recipient address cannot be empty")
	}

	for _, addressList := range [][]string{e.To, e.CC, e.BCC} {
		for _, address := range addressList {
			if strings.ContainsAny(address, CRLF) {
				return errors.New("recipient address contains invalid characters")
			}
		}
	}

	if strings.ContainsAny(e.Subject, CRLF) {
		return errors.New("subject contains invalid characters")
	}

	return nil
}
