// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"fmt"
)

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
