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

package group

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for group management operations.
var (
	// ErrorInvalidRequestFormat is the error returned when the request format is invalid.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorMissingGroupID is the error returned when group ID is missing.
	ErrorMissingGroupID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.missing_group_id",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.missing_group_id_description",
			DefaultValue: "Group ID is required",
		},
	}
	// ErrorGroupNotFound is the error returned when a group is not found.
	ErrorGroupNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.group_not_found",
			DefaultValue: "Group not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.group_not_found_description",
			DefaultValue: "The group with the specified id does not exist",
		},
	}
	// ErrorGroupNameConflict is the error returned when a group name conflicts.
	ErrorGroupNameConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.group_name_conflict",
			DefaultValue: "Group name conflict",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.group_name_conflict_description",
			DefaultValue: "A group with the same name exists under the same parent",
		},
	}
	// ErrorInvalidOUID is the error returned when parent is not found.
	ErrorInvalidOUID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_ou_id",
			DefaultValue: "Invalid OU ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_ou_id_description",
			DefaultValue: "Organization unit does not exists",
		},
	}
	// ErrorCannotDeleteGroup is the error returned when group cannot be deleted.
	ErrorCannotDeleteGroup = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.cannot_delete_group",
			DefaultValue: "Cannot delete group",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.cannot_delete_group_description",
			DefaultValue: "Cannot delete group with child groups",
		},
	}
	// ErrorInvalidMemberID is the error returned when a user or app member ID is invalid.
	ErrorInvalidMemberID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_member_id",
			DefaultValue: "Invalid member ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.groupservice.invalid_member_id_description",
			DefaultValue: "One or more user or app member IDs in the request do not exist " +
				"or do not match the claimed type",
		},
	}
	// ErrorInvalidGroupMemberID is the error returned when group member ID is invalid.
	ErrorInvalidGroupMemberID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_group_member_id",
			DefaultValue: "Invalid group member ID",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_group_member_id_description",
			DefaultValue: "One or more group member IDs in the request do not exist",
		},
	}
	// ErrorInvalidLimit is the error returned when limit parameter is invalid.
	ErrorInvalidLimit = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1011",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_limit_parameter",
			DefaultValue: "Invalid limit parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_limit_parameter_description",
			DefaultValue: "The limit parameter must be a positive integer",
		},
	}
	// ErrorInvalidOffset is the error returned when offset parameter is invalid.
	ErrorInvalidOffset = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1012",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_offset_parameter",
			DefaultValue: "Invalid offset parameter",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_offset_parameter_description",
			DefaultValue: "The offset parameter must be a non-negative integer",
		},
	}
	// ErrorEmptyMembers is the error returned when the members list is empty.
	ErrorEmptyMembers = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.empty_members_list",
			DefaultValue: "Empty members list",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.empty_members_list_description",
			DefaultValue: "The members list cannot be empty",
		},
	}
	// ErrorInvalidMemberType is the error returned when a member type is not a valid API value.
	ErrorInvalidMemberType = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_member_type",
			DefaultValue: "Invalid member type",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.invalid_member_type_description",
			DefaultValue: "The member type must be 'user', 'group', or 'app'",
		},
	}
)

// Declarative mode errors for group management operations.
var (
	// ErrorImmutableGroup is the error returned when attempting to modify a declarative group.
	ErrorImmutableGroup = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.cannot_modify_declarative_group",
			DefaultValue: "Cannot modify declarative group",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.groupservice.cannot_modify_declarative_group_description",
			DefaultValue: "The group is defined in declarative configuration and cannot be modified",
		},
	}
	// ErrorDeclarativeModeGroupCreateNotAllowed is the error returned when attempting to create
	// a group in declarative-only mode.
	ErrorDeclarativeModeGroupCreateNotAllowed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GRP-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.groupservice.cannot_create_group_in_declarative_only_mode",
			DefaultValue: "Cannot create group in declarative-only mode",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.groupservice.cannot_create_group_in_declarative_only_mode_description",
			DefaultValue: "Group creation is not allowed when running in declarative-only mode. " +
				"Groups must be defined in declarative configuration files",
		},
	}
)

// Internal error constants for group management operations.
var (
	// ErrGroupNotFound is returned when the group is not found in the system.
	ErrGroupNotFound = errors.New("group not found")

	// ErrGroupNameConflict is returned when a group with the same name exists under the same parent.
	ErrGroupNameConflict = errors.New("a group with the same name exists under the same parent")

	// ErrGroupDataCorrupted is returned when group data in the declarative store is malformed.
	ErrGroupDataCorrupted = errors.New("group data is corrupted")

	// errResultLimitExceededInCompositeMode is returned when composite store results exceed the configured limit.
	errResultLimitExceededInCompositeMode = errors.New("result limit exceeded in composite mode")
)
