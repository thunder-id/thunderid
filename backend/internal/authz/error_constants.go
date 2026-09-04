// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

var (
	// ErrorInvalidAuthorizationRequest indicates that an authorization engine rejected the request shape.
	ErrorInvalidAuthorizationRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTHZ-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.authorization.invalid_request",
			DefaultValue: "Invalid authorization request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.authorization.invalid_request_description",
			DefaultValue: "The authorization request is missing required policy evaluation data",
		},
	}
)
