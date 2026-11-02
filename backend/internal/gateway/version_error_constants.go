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
	// ErrorVersionRemoved is returned when the version being applied was removed, or replaced as the
	// current version, by a capture. Found before the import, when the version cannot be kept as it
	// is, nothing was sent to the gateway. Found after it, when the apply records it, the gateway
	// received it, so applying a kept version records the gateway's state again.
	ErrorVersionRemoved = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1018",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.version_removed",
			DefaultValue: "The version was removed or replaced while it was applied",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.version_removed.description",
			DefaultValue: "Only the newest versions are kept, and a capture removed this one or replaced it " +
				"as the current version. Apply a version that is still kept.",
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
	// ErrorMisplacedSecret is returned when the configuration being captured refers to a secret from a
	// field that holds no credential. A gateway would refuse the resource on import, and resolved
	// there the secret would be stored as an ordinary value.
	ErrorMisplacedSecret = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1026",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.misplaced_secret",
			DefaultValue: "A secret reference stands outside a credential field",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.misplaced_secret.description",
			DefaultValue: "A secret reference may only stand in a field that holds a credential",
		},
	}
	// ErrorApplyInProgress is returned when another apply or revert to the gateway was still running
	// after this one had waited as long as an import may take.
	ErrorApplyInProgress = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1029",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.apply_in_progress",
			DefaultValue: "Another apply to this gateway is still running",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.apply_in_progress.description",
			DefaultValue: "Wait for it to finish, then apply again.",
		},
	}
	// ErrorApplyTookTooLong is returned when an apply took longer to prepare than its lease allows for
	// an import, so it sent nothing rather than risk overlapping an apply that took the lease over.
	ErrorApplyTookTooLong = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "GTW-5003",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.apply_took_too_long",
			DefaultValue: "The apply took too long to prepare and sent nothing to the gateway",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.apply_took_too_long.description",
			DefaultValue: "Reading the gateway's values or the version took too long. Apply again.",
		},
	}
	// ErrorRemovalsNeedConfirmation is returned when an apply would remove more resources from the
	// gateway than the deployment's threshold and was not confirmed.
	ErrorRemovalsNeedConfirmation = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1030",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.removals_need_confirmation",
			DefaultValue: "This apply would remove many resources from the gateway",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.removals_need_confirmation.description",
			DefaultValue: "Review the removals with a dry run, then apply again with confirmRemovals set " +
				"to go ahead.",
		},
	}
	// ErrorInvalidSelection is returned when an apply's selection names a resource the diff does not
	// report, or would leave out a resource too long to keep.
	ErrorInvalidSelection = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1025",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.invalid_selection",
			DefaultValue: "The selection or skip names a resource this apply does not report",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.invalid_selection.description",
			DefaultValue: "Select or skip changes by the keys a diff of this version reports for this gateway. " +
				"A change cannot be both selected and skipped. A resource whose key is longer than 512 characters, " +
				"or whose id or name is longer than 255, cannot be left out.",
		},
	}
	// ErrorGatewayPrivateAddress is returned when a call to a gateway was refused before it was made,
	// because its host resolves to a loopback, link-local or private address and the deployment does
	// not allow gateways on such addresses.
	ErrorGatewayPrivateAddress = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "GTW-5002",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.gateway_private_address",
			DefaultValue: "The gateway's address resolves to a private address this deployment does not reach",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.gateway_private_address.description",
			DefaultValue: "The gateway's host resolves to a loopback, link-local or private address. Set " +
				"gateway.allow_private_addresses to reach gateways on such addresses, or register the gateway " +
				"on a public one.",
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
	// ErrorResourceNotViewable names an applied resource whose type cannot be shown from a version.
	ErrorResourceNotViewable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1021",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.resource_not_viewable",
			DefaultValue: "This resource type cannot be shown from a version",
		},
	}
	// ErrorResourceNotReadable names an applied document that does not read as its resource: one
	// that is not YAML as it stands, as a document captured with template placeholders is not until its
	// values are filled in, or one its exporter cannot show.
	ErrorResourceNotReadable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1022",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.resource_not_readable",
			DefaultValue: "The version's document cannot be read as a resource",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.gatewayservice.resource_not_readable.description",
			DefaultValue: "A document captured with template placeholders, as a deployment that is not a " +
				"control plane captures it, is not YAML until its values are filled in.",
		},
	}
)
