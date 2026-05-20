/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package cert

import (
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Client errors for the certificate service.
var (
	// ErrorInvalidCertificateID is the error for an invalid certificate ID.
	ErrorInvalidCertificateID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1001",
		Error: core.I18nMessage{
			Key:          "error.certservice.invalid_certificate_id",
			DefaultValue: "Invalid certificate ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.invalid_certificate_id_description",
			DefaultValue: "The provided certificate ID is invalid",
		},
	}
	// ErrorInvalidReferenceType is the error for an invalid certificate reference type.
	ErrorInvalidReferenceType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1002",
		Error: core.I18nMessage{
			Key:          "error.certservice.invalid_reference_type",
			DefaultValue: "Invalid certificate reference type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.invalid_reference_type_description",
			DefaultValue: "The provided certificate reference type is invalid",
		},
	}
	// ErrorInvalidReferenceID is the error for an invalid certificate reference ID.
	ErrorInvalidReferenceID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1003",
		Error: core.I18nMessage{
			Key:          "error.certservice.invalid_reference_id",
			DefaultValue: "Invalid certificate reference ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.invalid_reference_id_description",
			DefaultValue: "The provided certificate reference ID is invalid",
		},
	}
	// ErrorInvalidCertificateType is the error for an invalid certificate type.
	ErrorInvalidCertificateType = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1004",
		Error: core.I18nMessage{
			Key:          "error.certservice.invalid_certificate_type",
			DefaultValue: "Invalid certificate type",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.invalid_certificate_type_description",
			DefaultValue: "The provided certificate type is invalid",
		},
	}
	// ErrorInvalidCertificateValue is the error for an invalid certificate value.
	ErrorInvalidCertificateValue = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1005",
		Error: core.I18nMessage{
			Key:          "error.certservice.invalid_certificate_value",
			DefaultValue: "Invalid certificate value",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.invalid_certificate_value_description",
			DefaultValue: "The provided certificate value is invalid",
		},
	}
	// ErrorCertificateNotFound is the error when a certificate is not found.
	ErrorCertificateNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1006",
		Error: core.I18nMessage{
			Key:          "error.certservice.certificate_not_found",
			DefaultValue: "Certificate not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.certificate_not_found_description",
			DefaultValue: "The requested certificate could not be found",
		},
	}
	// ErrorCertificateAlreadyExists is the error when a certificate already exists.
	ErrorCertificateAlreadyExists = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1007",
		Error: core.I18nMessage{
			Key:          "error.certservice.certificate_already_exists",
			DefaultValue: "Certificate already exists",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.certificate_already_exists_description",
			DefaultValue: "A certificate with the same reference type and ID already exists",
		},
	}
	// ErrorReferenceUpdateIsNotAllowed is the error when trying to update a certificate's reference type or ID.
	ErrorReferenceUpdateIsNotAllowed = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "CES-1008",
		Error: core.I18nMessage{
			Key:          "error.certservice.reference_update_not_allowed",
			DefaultValue: "Reference update is not allowed",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.certservice.reference_update_not_allowed_description",
			DefaultValue: "Updating the reference type or ID of an existing certificate is not allowed",
		},
	}
)
