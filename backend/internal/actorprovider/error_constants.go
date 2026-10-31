// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package actorprovider

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorActorNotFound is returned when the requested actor cannot be resolved.
var ErrorActorNotFound = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "ACP-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.actor_not_found",
		DefaultValue: "Actor not found",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.actor_not_found_description",
		DefaultValue: "The requested actor does not exist",
	},
}

// ErrorEntityNotFound is returned when the backing entity record cannot be resolved.
var ErrorEntityNotFound = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "ACP-1002",
	Error: tidcommon.I18nMessage{
		Key:          "error.entity_not_found",
		DefaultValue: "Entity not found",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.entity_not_found_description",
		DefaultValue: "The requested entity does not exist",
	},
}

// ErrorActorLookupNotSupported is returned when the configured entity provider cannot look up
// entities, so an actor cannot be resolved for a reason other than a failure.
var ErrorActorLookupNotSupported = tidcommon.ServiceError{
	Type: tidcommon.ServerErrorType,
	Code: "ACP-1003",
	Error: tidcommon.I18nMessage{
		Key:          "error.actor_lookup_not_supported",
		DefaultValue: "Actor lookup not supported",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.actor_lookup_not_supported_description",
		DefaultValue: "The configured entity provider cannot look up entities",
	},
}
