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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package openid4vp

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client-facing API errors for the OpenID4VP verifier endpoints.
var (
	// ErrorInvalidRequest indicates a malformed or incomplete endpoint request.
	ErrorInvalidRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "EUDI-1001",
		Error: core.I18nMessage{
			Key:          "error.eudi.invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.eudi.invalid_request_description",
			DefaultValue: "The request is missing required parameters or is malformed",
		},
	}

	// ErrorUnknownState indicates the state value is unknown or has expired.
	ErrorUnknownState = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "EUDI-1002",
		Error: core.I18nMessage{
			Key:          "error.eudi.unknown_state",
			DefaultValue: "Unknown or expired request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.eudi.unknown_state_description",
			DefaultValue: "No active request was found for the supplied state value",
		},
	}

	// ErrorVerificationFailed indicates the presentation failed verification.
	ErrorVerificationFailed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "EUDI-1003",
		Error: core.I18nMessage{
			Key:          "error.eudi.verification_failed",
			DefaultValue: "Presentation verification failed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.eudi.verification_failed_description",
			DefaultValue: "The presented credential could not be verified",
		},
	}

	// ErrorUnknownDefinition indicates the requested presentation_definition_id is not registered.
	ErrorUnknownDefinition = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "EUDI-1004",
		Error: core.I18nMessage{
			Key:          "error.eudi.unknown_definition",
			DefaultValue: "Unknown presentation definition",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.eudi.unknown_definition_description",
			DefaultValue: "No presentation definition is registered for the supplied id",
		},
	}
)

// toServiceError maps an internal verifier error to a client-facing service error.
func toServiceError(err error) *serviceerror.ServiceError {
	switch {
	case errors.Is(err, ErrUnknownState):
		return &ErrorUnknownState
	case errors.Is(err, ErrInvalidResponse),
		errors.Is(err, ErrInvalidPresentation),
		errors.Is(err, ErrStateMismatch),
		errors.Is(err, ErrUntrustedIssuer),
		errors.Is(err, ErrUnexpectedVCT),
		errors.Is(err, ErrUnrequestedClaim),
		errors.Is(err, ErrMissingMandatoryClaim):
		return &ErrorVerificationFailed
	default:
		return &serviceerror.InternalServerError
	}
}
