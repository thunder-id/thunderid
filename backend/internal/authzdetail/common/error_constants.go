// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorAuthorizationDetailTypeChanged is the error returned when a type changes during a request.
var ErrorAuthorizationDetailTypeChanged = tidcommon.ServiceError{
	Type: tidcommon.ServerErrorType,
	Code: "RAR-5002",
	Error: tidcommon.I18nMessage{
		Key:          "error.authorizationdetailservice.authorization_detail_type_changed",
		DefaultValue: "Authorization details type changed",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.authorizationdetailservice.authorization_detail_type_changed_description",
		DefaultValue: "An authorization details type was changed or deleted while the request was in progress",
	},
}

// ErrChangeRefused is returned when ApplyChanges refuses a change.
var ErrChangeRefused = errors.New("authorization detail change refused")

// errLimitOrder is returned when a field's minimum is greater than its maximum.
var errLimitOrder = errors.New("the minimum cannot be greater than the maximum")

// FieldError is a schema error about the field at Path, which is empty for the schema itself.
type FieldError struct {
	Path string
	Err  error
}

// Error returns the message of the underlying error.
func (e *FieldError) Error() string { return e.Err.Error() }

// Unwrap returns the underlying error.
func (e *FieldError) Unwrap() error { return e.Err }

// fieldError returns a FieldError for the field at path.
func fieldError(path, format string, args ...any) error {
	return &FieldError{Path: path, Err: fmt.Errorf(format, args...)}
}
