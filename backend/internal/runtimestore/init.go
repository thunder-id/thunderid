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

// Package runtimestore provides the factory that selects a runtime store backend.
package runtimestore

import (
	"github.com/thunder-id/thunderid/internal/runtimestore/dbstore"
	"github.com/thunder-id/thunderid/internal/runtimestore/redisstore"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize returns the runtime store provider backing the given runtime datasource type.
// Redis-backed runtimes use the Redis store; all others use the relational database store.
func Initialize(runtimeTransientDBType, deploymentID string) (
	providers.RuntimeStoreProvider, transaction.Transactioner, error) {
	if runtimeTransientDBType == dbprovider.DataSourceTypeRedis {
		return redisstore.Initialize(deploymentID)
	}
	return dbstore.Initialize(deploymentID)
}
