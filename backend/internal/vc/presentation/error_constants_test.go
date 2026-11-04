// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinitionClientErrorStatus(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		expected int
	}{
		{"NotFound", ErrorDefinitionNotFound.Code, http.StatusNotFound},
		{"AlreadyExists", ErrorDefinitionAlreadyExists.Code, http.StatusConflict},
		{"Immutable", ErrorDefinitionImmutable.Code, http.StatusConflict},
		{"IDConflict", ErrorDefinitionIDConflict.Code, http.StatusConflict},
		{"InvalidRequestDefaultsToBadRequest", ErrorDefinitionInvalidRequest.Code, http.StatusBadRequest},
		{"UnknownDefaultsToBadRequest", "VP-9999", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, definitionClientErrorStatus(tc.code))
		})
	}
}
