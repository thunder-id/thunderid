// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorAttributeNotVerifiable is returned when an attribute cannot be pinned or recorded as verified.
var ErrorAttributeNotVerifiable = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "AVS-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.attributeverificationservice.attribute_not_verifiable",
		DefaultValue: "Attribute cannot be verified",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.attributeverificationservice.attribute_not_verifiable_description",
		DefaultValue: "The attribute is not a current email or mobile contact of a user that can be verified",
	},
}

// errNilEventBus is returned when the service is initialized without the event bus it audits on.
var errNilEventBus = errors.New("the attribute verification service needs an event bus")
