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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Client-facing service errors for the consent enforcer service.
var (
	// ErrorConsentPurposeFetchFailed is returned when the consent service rejects the
	// request to list consent purposes with a client error.
	ErrorConsentPurposeFetchFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.purpose_fetch_failed",
			DefaultValue: "Failed to fetch consent purposes",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.purpose_fetch_failed_description",
			DefaultValue: "Error while fetching consent purposes from the consent service",
		},
	}

	// ErrorConsentSearchFailed is returned when the consent service rejects the
	// request to search for consent records with a client error.
	ErrorConsentSearchFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_search_failed",
			DefaultValue: "Failed to search consent records",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_search_failed_description",
			DefaultValue: "Error while searching for consent records from the consent service",
		},
	}

	// ErrorConsentUpdateFailed is returned when the consent service rejects the
	// request to update an existing consent record with a client error.
	ErrorConsentUpdateFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_update_failed",
			DefaultValue: "Failed to update consent record",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_update_failed_description",
			DefaultValue: "Error while updating consent record in the consent service",
		},
	}

	// ErrorConsentCreateFailed is returned when the consent service rejects the
	// request to create a new consent record with a client error.
	ErrorConsentCreateFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_create_failed",
			DefaultValue: "Failed to create consent record",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_create_failed_description",
			DefaultValue: "Error while creating consent record in the consent service",
		},
	}

	// ErrorConsentSessionInvalid is returned when the consent session token is missing,
	// expired, or cannot be verified.
	ErrorConsentSessionInvalid = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_session_invalid",
			DefaultValue: "Invalid consent session",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.consent_session_invalid_description",
			DefaultValue: "The consent session token is invalid or has expired",
		},
	}

	// ErrorEssentialConsentDenied is returned when the user denied one or more essential consent attributes.
	ErrorEssentialConsentDenied = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.essential_consent_denied",
			DefaultValue: "Essential consent denied",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.essential_consent_denied_description",
			DefaultValue: "One or more essential consent attributes were denied",
		},
	}

	// ErrorConsentPurposeCreateFailed is returned when the consent service rejects the
	// request to create a consent purpose with a client error.
	ErrorConsentPurposeCreateFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.purpose_create_failed",
			DefaultValue: "Failed to create consent purpose",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.purpose_create_failed_description",
			DefaultValue: "Error while creating consent purpose in the consent service",
		},
	}

	// ErrorConsentPurposeUpdateFailed is returned when the consent service rejects the
	// request to update an existing consent purpose with a client error.
	ErrorConsentPurposeUpdateFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "AUTH-CES-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.purpose_update_failed",
			DefaultValue: "Failed to update consent purpose",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.consentenforcerservice.purpose_update_failed_description",
			DefaultValue: "Error while updating consent purpose in the consent service",
		},
	}
)
