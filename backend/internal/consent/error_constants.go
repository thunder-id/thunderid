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

package consent

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client-facing service errors.
var (
	// ErrorInvalidRequestFormat is returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1001",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}

	// ErrorConsentServiceReturnedUnauthorized is returned when the consent service returns a unauthorized response.
	ErrorConsentServiceReturnedUnauthorized = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1002",
		Error: core.I18nMessage{
			Key:          "error.consentservice.unauthorized",
			DefaultValue: "Unauthorized to access consent service",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.unauthorized_description",
			DefaultValue: "The consent service returned an unauthorized response",
		},
	}

	// ErrorConsentServiceReturnedForbidden is returned when the consent service returns a forbidden response.
	ErrorConsentServiceReturnedForbidden = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1003",
		Error: core.I18nMessage{
			Key:          "error.consentservice.forbidden",
			DefaultValue: "Forbidden to access consent service",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.forbidden_description",
			DefaultValue: "The consent service returned a forbidden response",
		},
	}

	// ErrorInvalidConsentRequest is returned when the consent backend rejects a request as invalid (400).
	ErrorInvalidConsentRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1004",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_request",
			DefaultValue: "Invalid consent request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_request_description",
			DefaultValue: "The request sent to the consent service was invalid",
		},
	}

	// ErrorInvalidConsentElementRequest is returned when the consent service rejects a consent element request.
	ErrorInvalidConsentElementRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1005",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_element_request",
			DefaultValue: "Invalid consent element request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_element_request_description",
			DefaultValue: "The consent element request was rejected by the consent service as invalid",
		},
	}

	// ErrorConsentElementAlreadyExists is returned when a consent element with the same name already exists.
	ErrorConsentElementAlreadyExists = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1006",
		Error: core.I18nMessage{
			Key:          "error.consentservice.element_already_exists",
			DefaultValue: "Consent element already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.element_already_exists_description",
			DefaultValue: "A consent element with the same name already exists",
		},
	}

	// ErrorConsentElementNotFound is returned when a consent element is not found.
	ErrorConsentElementNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1007",
		Error: core.I18nMessage{
			Key:          "error.consentservice.element_not_found",
			DefaultValue: "Consent element not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.element_not_found_description",
			DefaultValue: "The consent element with the specified ID does not exist",
		},
	}

	// ErrorDeletingConsentElementWithAssociatedPurpose is returned when attempting to delete a consent element
	// that is still associated with a purpose.
	ErrorDeletingConsentElementWithAssociatedPurpose = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1008",
		Error: core.I18nMessage{
			Key:          "error.consentservice.cannot_delete_element",
			DefaultValue: "Cannot delete consent element",
		},
		ErrorDescription: core.I18nMessage{
			Key: "error.consentservice.delete_element_with_associated_purpose_description",
			DefaultValue: "The consent element cannot be deleted because it is still associated with " +
				"one or more consent purposes.",
		},
	}

	// ErrorInvalidConsentPurposeRequest is returned when the consent service rejects a consent purpose request.
	ErrorInvalidConsentPurposeRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1009",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_purpose_request",
			DefaultValue: "Invalid consent purpose request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_purpose_request_description",
			DefaultValue: "The consent purpose request was rejected by the consent service as invalid",
		},
	}

	// ErrorConsentPurposeAlreadyExists is returned when a consent purpose with the same name already exists.
	ErrorConsentPurposeAlreadyExists = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1010",
		Error: core.I18nMessage{
			Key:          "error.consentservice.purpose_already_exists",
			DefaultValue: "Consent purpose already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.purpose_already_exists_description",
			DefaultValue: "A consent purpose with the same name already exists for this resource",
		},
	}

	// ErrorConsentPurposeNotFound is returned when a consent purpose is not found.
	ErrorConsentPurposeNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1011",
		Error: core.I18nMessage{
			Key:          "error.consentservice.purpose_not_found",
			DefaultValue: "Consent purpose not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.purpose_not_found_description",
			DefaultValue: "The consent purpose with the specified ID does not exist",
		},
	}

	// ErrorDeletingConsentPurposeWithAssociatedRecords is returned when attempting to delete a consent purpose that
	// has associated consent records.
	ErrorDeletingConsentPurposeWithAssociatedRecords = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1012",
		Error: core.I18nMessage{
			Key:          "error.consentservice.cannot_delete_purpose",
			DefaultValue: "Cannot delete consent purpose",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.delete_purpose_with_associated_records_description",
			DefaultValue: "The consent purpose cannot be deleted as it is associated with one or more consent records",
		},
	}

	// ErrorInvalidConsentRecordRequest is returned when the consent service rejects a consent request.
	ErrorInvalidConsentRecordRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1013",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_consent_request",
			DefaultValue: "Invalid consent request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_consent_request_description",
			DefaultValue: "The consent request was rejected by the consent service as invalid",
		},
	}

	// ErrorConsentRecordNotFound is returned when a consent record is not found.
	ErrorConsentRecordNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1014",
		Error: core.I18nMessage{
			Key:          "error.consentservice.consent_not_found",
			DefaultValue: "Consent not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.consent_not_found_description",
			DefaultValue: "The consent record with the specified ID does not exist",
		},
	}

	// ErrorInvalidConsentSearchFilter is returned when the consent service rejects a consent search filter.
	ErrorInvalidConsentSearchFilter = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1015",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_search_filter",
			DefaultValue: "Invalid consent search filter",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_search_filter_description",
			DefaultValue: "The consent search filter was rejected by the consent service as invalid",
		},
	}

	// ErrorInvalidConsentValidationRequest is returned when the consent service rejects a consent validation request.
	ErrorInvalidConsentValidationRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1016",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_validation_request",
			DefaultValue: "Invalid consent validation request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_validation_request_description",
			DefaultValue: "The consent validation request was rejected by the consent service as invalid",
		},
	}

	// ErrorInvalidConsentRevokeRequest is returned when the consent service rejects a consent revoke request.
	ErrorInvalidConsentRevokeRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1017",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_revoke_request",
			DefaultValue: "Invalid consent revoke request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_revoke_request_description",
			DefaultValue: "The consent revoke request was rejected by the consent service as invalid",
		},
	}

	// ErrorInvalidConsentUpdateRequest is returned when the consent service rejects a consent update request.
	ErrorInvalidConsentUpdateRequest = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CSE-1018",
		Error: core.I18nMessage{
			Key:          "error.consentservice.invalid_update_request",
			DefaultValue: "Invalid consent update request",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.consentservice.invalid_update_request_description",
			DefaultValue: "The consent update request was rejected by the consent service as invalid",
		},
	}
)
