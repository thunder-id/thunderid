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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package managedresource

import (
	"context"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorResourceManaged is returned when a caller tries to change a resource the control plane owns.
// One code covers every resource type, so a client can recognize the condition without knowing which
// kind of resource it was working with.
var ErrorResourceManaged = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "MRS-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.managedresource.resource_is_managed",
		DefaultValue: "Resource is managed by the control plane",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key: "error.managedresource.resource_is_managed_description",
		DefaultValue: "The resource was applied from the control plane and can only be changed there. " +
			"A change made here would be overwritten by the next promotion",
	},
}

// Guard returns the error when the resource is owned by the control plane, and nil otherwise. It is
// the call a service makes before changing something.
func Guard(ctx context.Context, resourceType, resourceID string) *tidcommon.ServiceError {
	if IsManaged(ctx, resourceType, resourceID) {
		return &ErrorResourceManaged
	}
	return nil
}
