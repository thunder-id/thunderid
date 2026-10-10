// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package declarativeresource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/log"
)

func TestCreateTypeError(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		resourceID   string
		wantError    *ExportError
	}{
		{
			name:         "Create type error for application",
			resourceType: "application",
			resourceID:   "app-123",
			wantError: &ExportError{
				ResourceType: "application",
				ResourceID:   "app-123",
				Error:        "Invalid resource type",
				Code:         "INVALID_TYPE",
			},
		},
		{
			name:         "Create type error for IDP",
			resourceType: "identity_provider",
			resourceID:   "idp-456",
			wantError: &ExportError{
				ResourceType: "identity_provider",
				ResourceID:   "idp-456",
				Error:        "Invalid resource type",
				Code:         "INVALID_TYPE",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateTypeError(tt.resourceType, tt.resourceID)
			assert.Equal(t, tt.wantError, got)
		})
	}
}

func TestValidateResourceName(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		resourceType string
		resourceID   string
		errorCode    string
		wantError    *ExportError
	}{
		{
			name:         "Valid resource name",
			resourceName: "MyApp",
			resourceType: "application",
			resourceID:   "app-123",
			errorCode:    "APP_VALIDATION_ERROR",
			wantError:    nil,
		},
		{
			name:         "Empty resource name",
			resourceName: "",
			resourceType: "application",
			resourceID:   "app-123",
			errorCode:    "APP_VALIDATION_ERROR",
			wantError: &ExportError{
				ResourceType: "application",
				ResourceID:   "app-123",
				Error:        "application name is empty",
				Code:         "APP_VALIDATION_ERROR",
			},
		},
		{
			name:         "Empty IDP name",
			resourceName: "",
			resourceType: "identity_provider",
			resourceID:   "idp-456",
			errorCode:    "IDP_VALIDATION_ERROR",
			wantError: &ExportError{
				ResourceType: "identity_provider",
				ResourceID:   "idp-456",
				Error:        "identity_provider name is empty",
				Code:         "IDP_VALIDATION_ERROR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a logger for testing
			logger := log.GetLogger()

			got := ValidateResourceName(
				context.Background(), tt.resourceName, tt.resourceType, tt.resourceID, tt.errorCode, logger)
			assert.Equal(t, tt.wantError, got)
		})
	}
}

func decodeViewDocument(t *testing.T, content string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	return root.Content[0]
}

type decodeViewTarget struct {
	Name string `yaml:"name"`
}

// DecodeView shows what the document decodes to, and returns what the view returns.
func TestDecodeViewShowsTheDecodedDocument(t *testing.T) {
	view, err := DecodeView(decodeViewDocument(t, "name: orders"), func(d *decodeViewTarget) (interface{}, error) {
		return "shown " + d.Name, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "shown orders", view)

	viewErr := errors.New("cannot show")
	_, err = DecodeView(decodeViewDocument(t, "name: orders"), func(*decodeViewTarget) (interface{}, error) {
		return nil, viewErr
	})
	assert.ErrorIs(t, err, viewErr)
}

// A document that does not decode is refused without being shown.
func TestDecodeViewRefusesADocumentThatDoesNotDecode(t *testing.T) {
	shown := false
	view, err := DecodeView(decodeViewDocument(t, "name: [not, a, name]"),
		func(*decodeViewTarget) (map[string]interface{}, error) {
			shown = true
			return map[string]interface{}{}, nil
		})
	assert.Error(t, err)
	assert.Nil(t, view)
	assert.False(t, shown)
}
