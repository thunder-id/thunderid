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

// ErrorActorUnavailable is returned when the actor resolves but an administrative hold withholds it.
// Machine-facing callers must fold it into the unregistered-client answer; only the browser
// authorization funnel shows it, as an unavailable page.
var ErrorActorUnavailable = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "ACP-1003",
	Error: tidcommon.I18nMessage{
		Key:          "error.actor_unavailable",
		DefaultValue: "Actor unavailable",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.actor_unavailable_description",
		DefaultValue: "The requested actor is currently unavailable",
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
