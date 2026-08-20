// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package importer provides functionality for importing resources into the server.
package importer

import (
	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/application"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// notFoundErrorCodes is the set of service error codes that represent a resource-not-found condition
// across all domain packages used by the importer. Used to distinguish upsert fallback (create after
// update-not-found) from other update errors.
var notFoundErrorCodes = map[string]struct{}{
	application.ErrorApplicationNotFound.Code:  {},
	idp.ErrorIDPNotFound.Code:                  {},
	notification.ErrorSenderNotFound.Code:      {},
	flowmgt.ErrorFlowNotFound.Code:             {},
	ou.ErrorOrganizationUnitNotFound.Code:      {},
	entitytype.ErrorEntityTypeNotFound.Code:    {},
	role.ErrorRoleNotFound.Code:                {},
	group.ErrorGroupNotFound.Code:              {},
	resource.ErrorResourceServerNotFound.Code:  {},
	thememgt.ErrorThemeNotFound.Code:           {},
	layoutmgt.ErrorLayoutNotFound.Code:         {},
	user.ErrorUserNotFound.Code:                {},
	agent.ErrorAgentNotFound.Code:              {},
	presentation.ErrorDefinitionNotFound.Code:  {},
	credential.ErrorConfigurationNotFound.Code: {},
}

var (
	// ErrorInvalidImportRequest represents malformed import requests.
	ErrorInvalidImportRequest = tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IMP-1001",
		Error: tidcommon.I18nMessage{Key: "error.import.invalidRequest", DefaultValue: "Invalid import request"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.invalidRequest.description",
			DefaultValue: "The provided import request is invalid or malformed",
		},
	}

	// ErrorInvalidYAMLContent represents invalid YAML payloads.
	ErrorInvalidYAMLContent = tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IMP-1002",
		Error: tidcommon.I18nMessage{Key: "error.import.invalidYaml", DefaultValue: "Invalid YAML content"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.invalidYaml.description",
			DefaultValue: "The provided YAML content cannot be parsed",
		},
	}

	// ErrorTemplateResolutionFailed represents template resolution failures.
	ErrorTemplateResolutionFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IMP-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.import.templateResolutionFailed",
			DefaultValue: "Template resolution failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.templateResolutionFailed.description",
			DefaultValue: "Failed to resolve one or more template variables in YAML content",
		},
	}

	// ErrorDeleteNotSupported represents a deletion requested for a resource type that cannot be
	// removed at runtime.
	ErrorDeleteNotSupported = tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IMP-1005",
		Error: tidcommon.I18nMessage{Key: "error.import.deleteNotSupported", DefaultValue: "Deletion not supported"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.deleteNotSupported.description",
			DefaultValue: "The requested resource type does not support runtime deletion",
		},
	}

	// ErrorAdapterNotConfigured represents missing runtime adapter wiring.
	ErrorAdapterNotConfigured = tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IMP-1004",
		Error: tidcommon.I18nMessage{Key: "error.import.adapterNotConfigured", DefaultValue: "Adapter not configured"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.import.adapterNotConfigured.description",
			DefaultValue: "The required resource adapter is not configured",
		},
	}
)
