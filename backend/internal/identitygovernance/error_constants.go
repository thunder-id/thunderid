// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client errors for identity governance operations. A refusal at an access check is returned as a
// Decision, not as one of these.
var (
	// ErrorMissingSubject is returned when an operation names no identity.
	ErrorMissingSubject = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IGS-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.identitygovernanceservice.missing_subject",
			DefaultValue: "Invalid request",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.identitygovernanceservice.missing_subject_description",
			DefaultValue: "An identity is required",
		},
	}
	// ErrorNotApplicable is returned when the operation does not apply to the identity's current
	// state, such as unsuspending an identity that is not suspended.
	ErrorNotApplicable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IGS-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.identitygovernanceservice.not_applicable",
			DefaultValue: "Operation does not apply",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.identitygovernanceservice.not_applicable_description",
			DefaultValue: "The operation does not apply to the identity's current state",
		},
	}
	// ErrorEntityNotFound is returned when no identity has the given ID.
	ErrorEntityNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "IGS-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.identitygovernanceservice.entity_not_found",
			DefaultValue: "Identity not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.identitygovernanceservice.entity_not_found_description",
			DefaultValue: "No identity exists with the given ID",
		},
	}
)

// ErrEntityNotFound is returned by an EntityStateProvider when no identity has the given ID.
var ErrEntityNotFound = errors.New("governed entity not found")

// errNoEntityStateProvider is returned when the service is built without an entity state provider.
var errNoEntityStateProvider = errors.New("identity governance requires an entity state provider")

// errDeclarativeAccountAccessLocked is returned when a write targets a declaratively set section.
var errDeclarativeAccountAccessLocked = errors.New(
	"accountAccess is set declaratively and cannot be overridden")
