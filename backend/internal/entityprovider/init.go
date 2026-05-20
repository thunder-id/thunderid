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

package entityprovider

import (
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/config"
)

// InitializeEntityProvider initializes the entity provider.
func InitializeEntityProvider(
	entitySvc entity.EntityServiceInterface,
) EntityProviderInterface {
	entityProviderConfig := config.GetServerRuntime().Config.EntityProvider
	switch entityProviderConfig.Type {
	case "disabled":
		return initializeDisabledEntityProvider()
	default:
		return initializeDefaultEntityProvider(entitySvc)
	}
}

// initializeDefaultEntityProvider initializes the default entity provider.
func initializeDefaultEntityProvider(
	entitySvc entity.EntityServiceInterface,
) EntityProviderInterface {
	return newDefaultEntityProvider(entitySvc)
}

// initializeDisabledEntityProvider initializes the disabled entity provider.
func initializeDisabledEntityProvider() EntityProviderInterface {
	return newDisabledEntityProvider()
}
