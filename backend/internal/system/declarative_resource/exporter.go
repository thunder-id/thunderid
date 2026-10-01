// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package declarativeresource

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// ResourceRules defines variables and array variables to parameterize for a resource type.
type ResourceRules struct {
	Variables      []string `yaml:"Variables,omitempty"`
	ArrayVariables []string `yaml:"ArrayVariables,omitempty"`
	// SecretVariables are parameterized exactly as Variables are, and additionally reported as
	// credentials.
	//
	// The distinction is not cosmetic. A value placed in a data plane's variable collection is
	// returned by a read; one placed in its secret collection never is. A client secret listed
	// under Variables would therefore be readable by anything allowed to list variables, which is
	// the difference the two collections exist to make.
	//
	// A dynamic property carries this on itself, through IsSecret. A field named by a rule has
	// nothing to carry it, so it is named here instead.
	SecretVariables       []string `yaml:"SecretVariables,omitempty"`
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
	GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError)

	// GetResourceByID retrieves a single resource by its ID
	// Returns: resource object, resource name, error
	GetResourceByID(ctx context.Context, id string) (interface{}, string, *tidcommon.ServiceError)

	// ValidateResource validates the resource and extracts its name
	// Returns: resource name, export error
	ValidateResource(ctx context.Context, resource interface{}, id string, logger *log.Logger) (string, *ExportError)

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
func ValidateResourceName(
	ctx context.Context, name, resourceType, resourceID, errorCode string, logger *log.Logger) *ExportError {
	if name == "" {
		logger.Warn(ctx, resourceType+" missing name, skipping export",
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
