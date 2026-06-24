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
	attributecache "github.com/thunder-id/thunderid/internal/attributecache/config"
	"github.com/thunder-id/thunderid/internal/system/database/dbtypes"
	"github.com/thunder-id/thunderid/internal/system/database/redisstore"
)

// Initialize initializes the attribute cache service and returns an instance of AttributeCacheServiceInterface.
func Initialize(cfg attributecache.Config) AttributeCacheServiceInterface {
	var store AttributeCacheStoreInterface
	if cfg.StoreConfig.Runtime.Type == dbtypes.DataSourceTypeRedis {
		store = newRedisAttributeCacheStore(cfg.DeploymentID, redisstore.GetRedisProvider())
	} else {
		store = newAttributeCacheStore(cfg.DeploymentID, cfg.StoreConfig)
	}
	return newAttributeCacheService(store)
}
