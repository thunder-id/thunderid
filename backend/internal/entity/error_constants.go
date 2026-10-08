// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "errors"

// Error variables for entity operations.
var (
	// ErrEntityNotFound is returned when the entity is not found in the system.
	ErrEntityNotFound = errors.New("entity not found")

	// ErrAuthenticationFailed is returned when entity credential verification fails.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrSchemaValidationFailed is returned when entity attributes fail schema validation.
	ErrSchemaValidationFailed = errors.New("schema validation failed")

	// ErrAttributeConflict is returned when entity attributes conflict with an existing entity.
	ErrAttributeConflict = errors.New("attribute conflict")

	// ErrInvalidCredential is returned when a credential value is invalid.
	ErrInvalidCredential = errors.New("invalid credential")

	// ErrAmbiguousEntity is returned when multiple entities match the provided filters.
	ErrAmbiguousEntity = errors.New("ambiguous entity")

	// ErrIndexedValueLimitExceeded is returned when an indexed attribute has more values than an
	// entity may index under one name.
	ErrIndexedValueLimitExceeded = errors.New("indexed value limit exceeded")

	// ErrBadAttributesInRequest is returned when the attributes in the request are invalid.
	ErrBadAttributesInRequest = errors.New("failed to marshal attributes")

	// errResultLimitExceededInCompositeMode is returned when the result limit is exceeded in composite mode.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)
