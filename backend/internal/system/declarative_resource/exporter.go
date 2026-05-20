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
	"context"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// ResourceRules defines variables and array variables to parameterize for a resource type.
type ResourceRules struct {
	Variables             []string `yaml:"Variables,omitempty"`
	ArrayVariables        []string `yaml:"ArrayVariables,omitempty"`
	DynamicPropertyFields []string `yaml:"DynamicPropertyFields,omitempty"`
}

// ResourceExporter defines the interface that each resource type must implement
// to be exportable. This makes it easy to add new resources to the export functionality.
type ResourceExporter interface {
	// GetResourceType returns the type identifier for this resource (e.g., "application", "identity_provider")
	GetResourceType() string

	// GetParameterizerType returns the type name used by the parameterizer (e.g., "Application", "IdentityProvider")
	GetParameterizerType() string

	// GetAllResourceIDs retrieves all resource IDs for wildcard export
	GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError)

	// GetResourceByID retrieves a single resource by its ID
	// Returns: resource object, resource name, error
	GetResourceByID(ctx context.Context, id string) (interface{}, string, *serviceerror.ServiceError)

	// ValidateResource validates the resource and extracts its name
	// Returns: resource name, export error
	ValidateResource(resource interface{}, id string, logger *log.Logger) (string, *ExportError)

	// GetResourceRules returns the parameterization rules for this resource type
	GetResourceRules() *ResourceRules
}

// PerResourceRuler is an optional interface that an exporter can implement to provide
// resource-instance-specific parameterization rules. When implemented, the export service
// will call GetResourceRulesForResource instead of GetResourceRules, allowing the exporter
// to tailor rules based on the resource's own data (e.g. omit client_secret for public clients).
type PerResourceRuler interface {
	GetResourceRulesForResource(resource interface{}) *ResourceRules
}

// ExportError represents errors that occurred during export.
type ExportError struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	Error        string `json:"error"`
	Code         string `json:"code,omitempty"`
}

// CreateTypeError creates a standardized type assertion error.
func CreateTypeError(resourceType, resourceID string) *ExportError {
	return &ExportError{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Error:        "Invalid resource type",
		Code:         "INVALID_TYPE",
	}
}

// ValidateResourceName validates that a resource name is not empty and returns an error if it is.
func ValidateResourceName(name, resourceType, resourceID, errorCode string, logger *log.Logger) *ExportError {
	if name == "" {
		logger.Warn(resourceType+" missing name, skipping export",
			log.String("resourceID", resourceID))
		return &ExportError{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Error:        resourceType + " name is empty",
			Code:         errorCode,
		}
	}
	return nil
}
