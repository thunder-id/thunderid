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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for notification sender operations.
var (
	// ErrorSenderNotFound is the error returned when a notification sender is not found.
	ErrorSenderNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.sender_not_found",
			DefaultValue: "Sender not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.sender_not_found_description",
			DefaultValue: "The requested notification sender could not be found",
		},
	}
	// ErrorInvalidSenderID is the error returned when an invalid sender ID is provided.
	ErrorInvalidSenderID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_sender_id",
			DefaultValue: "Invalid sender ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_sender_id_description",
			DefaultValue: "The provided sender ID is invalid",
		},
	}
	// ErrorInvalidSenderName is the error returned when an invalid sender name is provided.
	ErrorInvalidSenderName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_sender_name",
			DefaultValue: "Invalid sender name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_sender_name_description",
			DefaultValue: "The provided sender name is invalid",
		},
	}
	// ErrorInvalidProvider is the error returned when an invalid provider is specified.
	ErrorInvalidProvider = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_notification_provider",
			DefaultValue: "Invalid notification provider",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_notification_provider_description",
			DefaultValue: "The specified notification provider is invalid or unsupported",
		},
	}
	// ErrorDuplicateSenderName is the error returned when a sender with the same name already exists.
	ErrorDuplicateSenderName = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.duplicate_sender_name",
			DefaultValue: "Duplicate sender name",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.duplicate_sender_name_description",
			DefaultValue: "A sender with the same name already exists",
		},
	}
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorInvalidSenderType is the error returned when an invalid sender type is provided.
	ErrorInvalidSenderType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_sender_type",
			DefaultValue: "Invalid sender type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_sender_type_description",
			DefaultValue: "The provided sender type is invalid or unsupported",
		},
	}
	// ErrorSenderTypeUpdateNotAllowed is the error when trying to update the sender type.
	ErrorSenderTypeUpdateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.update_not_allowed",
			DefaultValue: "Update not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.update_not_allowed_description",
			DefaultValue: "Updating the sender type is not allowed",
		},
	}
	// ErrorRequestedSenderIsNotOfExpectedType is the error when the requested sender is not of the expected type.
	ErrorRequestedSenderIsNotOfExpectedType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.sender_type_mismatch",
			DefaultValue: "Sender type mismatch",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.sender_type_mismatch_description",
			DefaultValue: "The requested sender is not of the expected type",
		},
	}
	// ErrorInvalidRecipient is the error returned when an invalid recipient is provided.
	ErrorInvalidRecipient = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_recipient",
			DefaultValue: "Invalid recipient",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_recipient_description",
			DefaultValue: "The provided recipient is invalid",
		},
	}
	// ErrorInvalidChannel is the error returned when an invalid channel is provided.
	ErrorInvalidChannel = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_channel",
			DefaultValue: "Invalid channel",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_channel_description",
			DefaultValue: "The provided channel is invalid",
		},
	}
	// ErrorUnsupportedChannel is the error returned when an unsupported channel is provided.
	ErrorUnsupportedChannel = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.unsupported_channel",
			DefaultValue: "Unsupported channel",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.unsupported_channel_description",
			DefaultValue: "The provided channel is not supported",
		},
	}
	// ErrorInvalidOTP is the error returned when an invalid OTP is provided.
	ErrorInvalidOTP = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_otp",
			DefaultValue: "Invalid OTP",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_otp_description",
			DefaultValue: "The provided OTP is invalid",
		},
	}
	// ErrorInvalidSessionToken is the error returned when an invalid session token is provided.
	ErrorInvalidSessionToken = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_session_token",
			DefaultValue: "Invalid session token",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.invalid_session_token_description",
			DefaultValue: "The provided session token is invalid, malformed, or expired",
		},
	}
	// ErrorClientErrorWhileRetrievingMessageClient is the error returned when a client error occurs
	// while retrieving the message client.
	ErrorClientErrorWhileRetrievingMessageClient = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.error_while_retrieving_message_client",
			DefaultValue: "Error while retrieving message client",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.error_while_retrieving_message_client_description",
			DefaultValue: "An error occurred while retrieving the message client",
		},
	}
	// ErrorSenderHasBlockingDependencies is returned when a notification sender cannot be deleted
	// because other resources depend on it in a way that forbids deletion (e.g. flows that reference it).
	ErrorSenderHasBlockingDependencies = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.sender_has_blocking_dependencies",
			DefaultValue: "Notification sender cannot be deleted",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.notificationservice.sender_has_blocking_dependencies_description",
			DefaultValue: "The notification sender cannot be deleted because other resources depend on it. " +
				"Remove or reassign them first.",
		},
	}
	// ErrorMaxOTPAttemptsExceeded is the error returned when maximum OTP attempts are reached.
	ErrorMaxOTPAttemptsExceeded = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "MNS-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.notificationservice.max_otp_attempts_exceeded",
			DefaultValue: "Maximum OTP attempts reached",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.notificationservice.max_otp_attempts_exceeded_description",
			DefaultValue: "The maximum number of OTP generation attempts has been reached",
		},
	}
)
