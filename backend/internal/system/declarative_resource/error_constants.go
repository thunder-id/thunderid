// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package declarativeresource

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorDeclarativeResourceCreateOperation is the error returned when
	// a declarative resource create operation is attempted.
	ErrorDeclarativeResourceCreateOperation = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DCR-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.declarative_resource.create_operation_not_allowed",
			DefaultValue: "Declarative resource create operation is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.declarative_resource.create_operation_not_allowed_description",
			DefaultValue: "Creating declarative resources is not permitted",
		},
	}

	// ErrorDeclarativeResourceUpdateOperation is the error returned when
	// a declarative resource update operation is attempted.
	ErrorDeclarativeResourceUpdateOperation = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DCR-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.declarative_resource.update_operation_not_allowed",
			DefaultValue: "Declarative resource update operation is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.declarative_resource.update_operation_not_allowed_description",
			DefaultValue: "Updating declarative resources is not permitted",
		},
	}

	// ErrorDeclarativeResourceDeleteOperation is the error returned when
	// a declarative resource delete operation is attempted.
	ErrorDeclarativeResourceDeleteOperation = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "DCR-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.declarative_resource.delete_operation_not_allowed",
			DefaultValue: "Declarative resource delete operation is not allowed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.declarative_resource.delete_operation_not_allowed_description",
			DefaultValue: "Deleting declarative resources is not permitted",
		},
	}

	// ErrorDataPlaneNotReady is returned when a write is refused because the default gateway, the data
	// plane the values it produces are written to, has not yet answered a health check: it is still
	// being provisioned. A deploy to a gateway in that state is refused with it too.
	ErrorDataPlaneNotReady = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1034",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.data_plane_not_ready",
			DefaultValue: "The data plane is not ready yet",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.data_plane_not_ready.description",
			DefaultValue: "Your data plane is still being provisioned. Try again once it is ready.",
		},
	}

	// ErrorNoDataPlane is returned when a write is refused because no default gateway is registered, so
	// the values it produces have no data plane to be written to.
	ErrorNoDataPlane = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "GTW-1038",
		Error: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.no_data_plane",
			DefaultValue: "No data plane is connected yet",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.gatewayservice.no_data_plane.description",
			DefaultValue: "Register a data plane and wait until it is ready, then try again.",
		},
	}
)

// IsDataPlaneRefusal reports whether a service error is a write refused until a data plane is ready,
// which a handler answers with 409: the request is sound, but the deployment is not ready for it yet.
func IsDataPlaneRefusal(svcErr *tidcommon.ServiceError) bool {
	return svcErr != nil && (svcErr.Code == ErrorDataPlaneNotReady.Code || svcErr.Code == ErrorNoDataPlane.Code)
}
