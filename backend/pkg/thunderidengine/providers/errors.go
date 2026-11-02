// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package providers

import (
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Organization unit provider errors. The runtime branches on these codes, so an
// OrganizationUnitProvider implementation must return them for the corresponding conditions;
// any other code is surfaced to the user as a generic failure.
var (
	// ErrorOrganizationUnitNameConflict is the error an OrganizationUnitProvider returns when the
	// requested name is already taken by a sibling organization unit.
	ErrorOrganizationUnitNameConflict = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "OU-1004",
		Error: common.I18nMessage{
			Key:          "error.ouservice.organization_unit_name_conflict",
			DefaultValue: "Organization unit name conflict",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.ouservice.organization_unit_name_conflict_description",
			DefaultValue: "An organization unit with the same name exists under the same parent",
		},
	}

	// ErrorOrganizationUnitHandleConflict is the error an OrganizationUnitProvider returns when the
	// requested handle is already taken by a sibling organization unit.
	ErrorOrganizationUnitHandleConflict = common.ServiceError{
		Type: common.ClientErrorType,
		Code: "OU-1008",
		Error: common.I18nMessage{
			Key:          "error.ouservice.organization_unit_handle_conflict",
			DefaultValue: "Organization unit handle conflict",
		},
		ErrorDescription: common.I18nMessage{
			Key:          "error.ouservice.organization_unit_handle_conflict_description",
			DefaultValue: "An organization unit with the same handle already exists under the same parent",
		},
	}
)
