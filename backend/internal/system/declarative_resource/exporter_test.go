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

package declarativeresource

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

			got := ValidateResourceName(tt.resourceName, tt.resourceType, tt.resourceID, tt.errorCode, logger)
			assert.Equal(t, tt.wantError, got)
		})
	}
}
