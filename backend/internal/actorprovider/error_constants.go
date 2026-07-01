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

package actorprovider

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ErrorActorNotFound is returned when the requested actor cannot be resolved.
var ErrorActorNotFound = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "ACP-1001",
	Error: tidcommon.I18nMessage{
		Key:          "error.actor_not_found",
		DefaultValue: "Actor not found",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.actor_not_found_description",
		DefaultValue: "The requested actor does not exist",
	},
}

// ErrorEntityNotFound is returned when the backing entity record cannot be resolved.
var ErrorEntityNotFound = tidcommon.ServiceError{
	Type: tidcommon.ClientErrorType,
	Code: "ACP-1002",
	Error: tidcommon.I18nMessage{
		Key:          "error.entity_not_found",
		DefaultValue: "Entity not found",
	},
	ErrorDescription: tidcommon.I18nMessage{
		Key:          "error.entity_not_found_description",
		DefaultValue: "The requested entity does not exist",
	},
}
