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

package tenant

import (
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

var (
	// ErrorInvalidRequestFormat is returned when the request body cannot be parsed.
	ErrorInvalidRequestFormat = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1001",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_request_format",
			DefaultValue: "Invalid request format",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_request_format_description",
			DefaultValue: "The request body is malformed or contains invalid data",
		},
	}
	// ErrorTenantNotFound is returned when a managed tenant does not exist.
	ErrorTenantNotFound = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1002",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_not_found",
			DefaultValue: "Tenant not found",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_not_found_description",
			DefaultValue: "The tenant with the specified deployment id does not exist",
		},
	}
	// ErrorTenantConflict is returned when a tenant with the same deployment id already exists.
	ErrorTenantConflict = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1003",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_conflict",
			DefaultValue: "Tenant already exists",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.tenant_conflict_description",
			DefaultValue: "A tenant with the same deployment id is already provisioned",
		},
	}
	// ErrorInvalidDeploymentID is returned when the requested deployment id is invalid.
	ErrorInvalidDeploymentID = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1004",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_deployment_id",
			DefaultValue: "Invalid deployment id",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.tenantservice.invalid_deployment_id_description",
			DefaultValue: "The deployment id must contain only letters, digits, '-', '_', ':', or '.' " +
				"and must not be the reserved system tenant id",
		},
	}
	// ErrorReservedSystemTenant is returned when an operation targets the reserved system tenant.
	ErrorReservedSystemTenant = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1005",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.reserved_system_tenant",
			DefaultValue: "Reserved system tenant",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.tenantservice.reserved_system_tenant_description",
			DefaultValue: "The system tenant is provisioned only at Control Plane start-up and cannot be " +
				"created or deleted through this API",
		},
	}
	// ErrorNotSystemTenant is returned when the caller is not the system tenant.
	ErrorNotSystemTenant = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1006",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.not_system_tenant",
			DefaultValue: "Not authorized",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.not_system_tenant_description",
			DefaultValue: "Only the system tenant may manage tenants",
		},
	}
	// ErrorSeedFailed is returned when a later environment of an organization could not be copied from
	// its first. The reason names what went wrong, because it is almost always something about the
	// source tenant that the caller can act on rather than a fault in this server.
	ErrorSeedFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1007",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.seed_failed",
			DefaultValue: "Could not copy the organization's configuration",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.seed_failed_description",
			DefaultValue: "The new environment could not be seeded from the organization's first environment",
		},
	}
	// ErrorEnvironmentRegistrationFailed is returned when the tenant was created but could not be
	// recorded as an environment of its organization, so it would not take part in promotion.
	ErrorEnvironmentRegistrationFailed = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1008",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.environment_registration_failed",
			DefaultValue: "Could not register the environment",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.environment_registration_failed_description",
			DefaultValue: "The tenant could not be registered as an environment of its organization",
		},
	}
	// ErrorInvalidDataPlane is returned when an environment is registered without naming the
	// deployment its configuration is applied to.
	ErrorInvalidDataPlane = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1009",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_data_plane",
			DefaultValue: "Invalid data plane",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.invalid_data_plane_description",
			DefaultValue: "dataPlane.id is required to register an environment",
		},
	}
	// ErrorEnvironmentRegistrationUnavailable is returned when this server hosts no environment
	// manager, so there is nowhere to register an environment.
	ErrorEnvironmentRegistrationUnavailable = tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "TNT-1010",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.environment_registration_unavailable",
			DefaultValue: "Environment registration is not available",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.tenantservice.environment_registration_unavailable_description",
			DefaultValue: "This server hosts no environment manager, so an environment cannot be " +
				"registered here",
		},
	}
	// ErrorInternalServer is returned for unexpected server-side errors.
	ErrorInternalServer = tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "TNT-5001",
		Error: tidcommon.I18nMessage{
			Key:          "error.tenantservice.internal_server_error",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.tenantservice.internal_server_error_description",
			DefaultValue: "An unexpected error occurred while managing the tenant",
		},
	}
)

// errTenantNotFound is the sentinel error returned by the store when a registry row is absent.
var errTenantNotFound = errors.New("tenant not found")
