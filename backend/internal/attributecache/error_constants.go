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

package attributecache

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// Store-level errors.
var (
	// errAttributeCacheNotFound is returned when an attribute cache entry is not found.
	errAttributeCacheNotFound = errors.New("attribute cache not found")
)

// Client-facing service errors.
var (
	// ErrorInvalidRequestFormat is returned when the request format is invalid.
	ErrorInvalidRequestFormat = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ACS-1001",
		Error: core.I18nMessage{
			Key:          "error.attributecache.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.attributecache.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}

	// ErrorMissingCacheID is returned when cache ID is missing.
	ErrorMissingCacheID = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ACS-1002",
		Error: core.I18nMessage{
			Key:          "error.attributecache.missing_cache_id",
			DefaultValue: "Missing cache ID",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.attributecache.missing_cache_id_description",
			DefaultValue: "Cache ID is required",
		},
	}

	// ErrorAttributeCacheNotFound is returned when an attribute cache entry is not found.
	ErrorAttributeCacheNotFound = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ACS-1003",
		Error: core.I18nMessage{
			Key:          "error.attributecache.cache_not_found",
			DefaultValue: "Attribute cache not found",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.attributecache.cache_not_found_description",
			DefaultValue: "The attribute cache entry with the specified ID does not exist",
		},
	}

	// ErrorMissingAttributes is returned when attributes are missing.
	ErrorMissingAttributes = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ACS-1004",
		Error: core.I18nMessage{
			Key:          "error.attributecache.missing_attributes",
			DefaultValue: "Missing attributes",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.attributecache.missing_attributes_description",
			DefaultValue: "Attributes are required",
		},
	}

	// ErrorInvalidExpiryTime is returned when expiry time is invalid.
	ErrorInvalidExpiryTime = serviceerror.ServiceError{
		Type: serviceerror.ClientErrorType,
		Code: "ACS-1005",
		Error: core.I18nMessage{
			Key:          "error.attributecache.invalid_expiry_time",
			DefaultValue: "Invalid expiry time",
		},
		ErrorDescription: core.I18nMessage{
			Key:          "error.attributecache.invalid_expiry_time_description",
			DefaultValue: "Expiry time must be in the future",
		},
	}
)
