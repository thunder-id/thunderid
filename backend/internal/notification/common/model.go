// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

// OTPConfig holds optional OTP generation overrides.
// Nil pointer = use server default.
type OTPConfig struct {
	Length                *int
	UseNumericOnly        *bool
	ValidityPeriodSeconds *int
}
