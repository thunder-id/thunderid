// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	// ErrorVersionNotFound is returned when the deployment has no version of that number.
	ErrorVersionNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1013",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.version_not_found",
			DefaultValue: "Configuration version not found",
		},
	}
	// ErrorInvalidVersion is returned when a version is named by something other than a hash, a prefix
	// of at least seven of its characters, or "latest".
	ErrorInvalidVersion = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1014",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_version",
			DefaultValue: "A version is named by its hash, at least seven of its characters, or latest",
		},
	}
	// ErrorNoVersions is returned when something is to be applied before anything was captured.
	ErrorNoVersions = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1015",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.no_versions",
			DefaultValue: "No configuration version has been captured",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.no_versions.description",
			DefaultValue: "Capture a version of this deployment's configuration before applying one.",
		},
	}
	// ErrorAmbiguousVersion is returned when a prefix names more than one version.
	ErrorAmbiguousVersion = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1020",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.ambiguous_version",
			DefaultValue: "More than one version starts with that hash",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.ambiguous_version.description",
			DefaultValue: "Give more characters of the version's hash.",
		},
	}
	// ErrorNothingToRevert is returned when a gateway holds no earlier version to go back to.
	ErrorNothingToRevert = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1016",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.nothing_to_revert",
			DefaultValue: "This gateway holds no earlier version to revert to",
		},
	}
	// ErrorVersionRemoved is returned when the version being applied was removed by a capture before
	// the apply could record it. The gateway received it, so applying a kept version records the
	// gateway's state again.
	ErrorVersionRemoved = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.version_removed",
			DefaultValue: "The version was removed while it was applied",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.version_removed.description",
			DefaultValue: "Only the newest versions are kept, and this one was removed by a capture. " +
				"Apply a version that is still kept.",
		},
	}
	// ErrorAppliedChanged is returned when another apply to the same gateway finished between this one
	// reading what the gateway held and recording what it now holds.
	ErrorAppliedChanged = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1019",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.applied_changed",
			DefaultValue: "Another apply to this gateway finished first",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.applied_changed.description",
			DefaultValue: "Check which version the gateway holds now, then apply again.",
		},
	}
	// ErrorMissingValues is returned when a version refers to variables or secrets the gateway does not
	// hold. A dry run of the same apply lists them.
	ErrorMissingValues = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1017",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.missing_values",
			DefaultValue: "The gateway does not hold every value this version refers to",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.missing_values.description",
			DefaultValue: "Set the missing variables and secrets on the gateway, then apply again. " +
				"A dry run lists them.",
		},
	}
	// ErrorInvalidSelection is returned when an apply's selection names a resource the diff does not
	// report, or would leave out a resource too long to keep.
	ErrorInvalidSelection = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_selection",
			DefaultValue: "The selection names a resource this apply does not report",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.invalid_selection.description",
			DefaultValue: "Select changes by the keys a diff of this version reports for this gateway. " +
				"A resource whose key is longer than 512 characters, or whose id or name is longer than 255, " +
				"cannot be left out.",
		},
	}
	// ErrorGatewayUnreachable is returned when a gateway could not be called or refused the import.
	ErrorGatewayUnreachable = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "GTW-5001",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.gateway_unreachable",
			DefaultValue: "The gateway could not be reached, or refused the request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.gateway_unreachable.description",
			DefaultValue: "Check that the gateway is running at its base URL, that it trusts this " +
				"control plane's key, and that its certificate verifies.",
		},
	}
)
