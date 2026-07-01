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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client-facing API errors for the OpenID4VP verifier endpoints.
var (
	// ErrorInvalidRequest indicates a malformed or incomplete endpoint request.
	ErrorInvalidRequest = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EUDI-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.eudi.invalid_request",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.eudi.invalid_request_description",
			DefaultValue: "The request is missing required parameters or is malformed",
		},
	}

	// ErrorUnknownState indicates the state value is unknown or has expired.
	ErrorUnknownState = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EUDI-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.eudi.unknown_state",
			DefaultValue: "Unknown or expired request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.eudi.unknown_state_description",
			DefaultValue: "No active request was found for the supplied state value",
		},
	}

	// ErrorVerificationFailed indicates the presentation failed verification.
	ErrorVerificationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EUDI-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.eudi.verification_failed",
			DefaultValue: "Presentation verification failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.eudi.verification_failed_description",
			DefaultValue: "The presented credential could not be verified",
		},
	}

	// ErrorExpiredState is distinct from ErrorUnknownState: state exists but is past its TTL.
	ErrorExpiredState = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EUDI-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.eudi.expired_state",
			DefaultValue: "Expired request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.eudi.expired_state_description",
			DefaultValue: "The request associated with the supplied state value has expired",
		},
	}

	// ErrorUnknownDefinition indicates the requested presentation_definition_id is not registered.
	ErrorUnknownDefinition = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "EUDI-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.eudi.unknown_definition",
			DefaultValue: "Unknown presentation definition",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.eudi.unknown_definition_description",
			DefaultValue: "No presentation definition is registered for the supplied id",
		},
	}
)

// Sentinel errors returned by the verifier. HTTP-facing service errors live in error_constants.go.
var (
	ErrUntrustedIssuer       = errors.New("openid4vp: untrusted credential issuer")
	ErrUnexpectedVCT         = errors.New("openid4vp: unexpected credential type (vct)")
	ErrUnrequestedClaim      = errors.New("openid4vp: disclosed claim was not requested")
	ErrMissingMandatoryClaim = errors.New("openid4vp: mandatory claim missing")
	ErrClaimValueNotAllowed  = errors.New("openid4vp: disclosed claim value not in the allowed set")
	ErrInvalidPresentation   = errors.New("openid4vp: invalid presentation")
	ErrInvalidResponse       = errors.New("openid4vp: invalid authorization response")
	ErrPolicy                = errors.New("openid4vp: invalid verification policy")
	ErrUnknownDefinition     = errors.New("openid4vp: unknown presentation definition")
	ErrUnknownState          = errors.New("openid4vp: unknown or expired request state")
	ErrExpiredState          = errors.New("openid4vp: request state expired")
	ErrStateMismatch         = errors.New("openid4vp: response state mismatch")
)

// toServiceError maps an internal verifier error to a client-facing service error.
func toServiceError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, ErrUnknownState):
		return &ErrorUnknownState
	case errors.Is(err, ErrExpiredState):
		return &ErrorExpiredState
	case errors.Is(err, ErrUnknownDefinition):
		return &ErrorUnknownDefinition
	case errors.Is(err, ErrPolicy):
		return &tidcommon.InternalServerError
	case errors.Is(err, ErrInvalidResponse),
		errors.Is(err, ErrInvalidPresentation),
		errors.Is(err, ErrStateMismatch),
		errors.Is(err, ErrUntrustedIssuer),
		errors.Is(err, ErrUnexpectedVCT),
		errors.Is(err, ErrUnrequestedClaim),
		errors.Is(err, ErrMissingMandatoryClaim),
		errors.Is(err, ErrClaimValueNotAllowed):
		return &ErrorVerificationFailed
	default:
		return &tidcommon.InternalServerError
	}
}
