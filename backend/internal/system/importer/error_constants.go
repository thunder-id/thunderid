/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package importer provides functionality for importing resources into the server.
package importer

import (
	"github.com/thunder-id/thunderid/internal/application"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/user"
)

// notFoundErrorCodes is the set of service error codes that represent a resource-not-found condition
// across all domain packages used by the importer. Used to distinguish upsert fallback (create after
// update-not-found) from other update errors.
var notFoundErrorCodes = map[string]struct{}{
	application.ErrorApplicationNotFound.Code: {},
	idp.ErrorIDPNotFound.Code:                 {},
	flowmgt.ErrorFlowNotFound.Code:            {},
	ou.ErrorOrganizationUnitNotFound.Code:     {},
	entitytype.ErrorEntityTypeNotFound.Code:   {},
	role.ErrorRoleNotFound.Code:               {},
	group.ErrorGroupNotFound.Code:             {},
	resource.ErrorResourceServerNotFound.Code: {},
	thememgt.ErrorThemeNotFound.Code:          {},
	layoutmgt.ErrorLayoutNotFound.Code:        {},
	user.ErrorUserNotFound.Code:               {},
}

var (
	// ErrorInvalidImportRequest represents malformed import requests.
	ErrorInvalidImportRequest = serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "IMP-1001",
		Error: core.I18nMessage{Key: "error.import.invalidRequest", DefaultValue: "Invalid import request"},
		ErrorDescription: core.I18nMessage{
			Key:          "error.import.invalidRequest.description",
			DefaultValue: "The provided import request is invalid or malformed",
		},
	}

	// ErrorInvalidYAMLContent represents invalid YAML payloads.
	ErrorInvalidYAMLContent = serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "IMP-1002",
		Error: core.I18nMessage{Key: "error.import.invalidYaml", DefaultValue: "Invalid YAML content"},
		ErrorDescription: core.I18nMessage{
			Key:          "error.import.invalidYaml.description",
			DefaultValue: "The provided YAML content cannot be parsed",
		},
	}

	// ErrorTemplateResolutionFailed represents template resolution failures.
	ErrorTemplateResolutionFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "IMP-1003",
		Error: core.I18nMessage{
			Key:          "error.import.templateResolutionFailed",
			DefaultValue: "Template resolution failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.import.templateResolutionFailed.description",
			DefaultValue: "Failed to resolve one or more template variables in YAML content",
		},
	}

	// ErrorAdapterNotConfigured represents missing runtime adapter wiring.
	ErrorAdapterNotConfigured = serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "IMP-1004",
		Error: core.I18nMessage{Key: "error.import.adapterNotConfigured", DefaultValue: "Adapter not configured"},
		ErrorDescription: core.I18nMessage{
			Key:          "error.import.adapterNotConfigured.description",
			DefaultValue: "The required resource adapter is not configured",
		},
	}
)
