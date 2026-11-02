// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

// ErrorCode represents an entity provider error code.
type ErrorCode string

// Error codes for entity provider operations.
const (
	ErrorCodeSystemError            ErrorCode = "EP-0001"
	ErrorCodeEntityNotFound         ErrorCode = "EP-0002"
	ErrorCodeInvalidRequestFormat   ErrorCode = "EP-0003"
	ErrorCodeAttributeConflict      ErrorCode = "EP-0004"
	ErrorCodeMissingRequiredFields  ErrorCode = "EP-0005"
	ErrorCodeMissingCredentials     ErrorCode = "EP-0006"
	ErrorCodeNotImplemented         ErrorCode = "EP-0007"
	ErrorCodeAmbiguousEntity        ErrorCode = "EP-0008"
	ErrorCodeSchemaValidationFailed ErrorCode = "EP-0009"
	ErrorCodeSyncRejected           ErrorCode = "EP-0010"
)

// EntityProviderError represents an error returned by the entity provider.
type EntityProviderError struct {
	Code        ErrorCode `json:"code"`
	Message     string    `json:"message"`
	Description string    `json:"description"`
}

func (e *EntityProviderError) Error() string {
	return e.Message + ": " + e.Description
}

// NewEntityProviderError creates a new entity provider error.
func NewEntityProviderError(code ErrorCode, message string, description string) *EntityProviderError {
	return &EntityProviderError{
		Code:        code,
		Message:     message,
		Description: description,
	}
}
