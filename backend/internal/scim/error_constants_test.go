// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewMissingRequiredAttributesError tests New Missing Required Attributes Error.
func TestNewMissingRequiredAttributesError(t *testing.T) {
	svcErr := newMissingRequiredAttributesError("employee", []string{"department", "employee_id"})

	require.Equal(t, ErrorSchemaValidationFailed.Code, svcErr.Code)
	require.Equal(t, ErrorSchemaValidationFailed.Type, svcErr.Type)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, `"employee"`)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, "department, employee_id")
}

// TestNewUndeclaredAttributesError tests New Undeclared Attributes Error.
func TestNewUndeclaredAttributesError(t *testing.T) {
	svcErr := newUndeclaredAttributesError("employee", []string{"extra1", "extra2"})

	require.Equal(t, ErrorSchemaValidationFailed.Code, svcErr.Code)
	require.Equal(t, ErrorSchemaValidationFailed.Type, svcErr.Type)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, `"employee"`)
	require.Contains(t, svcErr.ErrorDescription.DefaultValue, "extra1, extra2")
}
