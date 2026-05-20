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

package entity

import (
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cryptolab/hash"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the entity service.
// The entity store is always composite: a DB store backed by an in-memory file store.
// Declarative resources are loaded on demand by consumer packages (e.g. user, application)
// based on their own store mode configuration.
func Initialize(
	cacheManager cache.CacheManagerInterface,
	hashService hash.HashServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
) (EntityServiceInterface, error) {
	store, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, err
	}

	svc := newEntityService(store, hashService, entityTypeService, ouService, transactioner)
	return svc, nil
}

// initializeStore always creates a composite store (DB + in-memory file store).
func initializeStore(cacheManager cache.CacheManagerInterface) (
	entityStoreInterface, transaction.Transactioner, error) {
	fileStore := newEntityFileBasedStore()
	dbStore, transactioner, err := newEntityDBStore()
	if err != nil {
		return nil, nil, err
	}
	entityByIDCache := cache.GetCache[*Entity](cacheManager, "EntityByIDCache")
	cacheBackedEntityStore := newCacheBackedEntityStore(dbStore, entityByIDCache)
	return newEntityCompositeStore(fileStore, cacheBackedEntityStore), transactioner, nil
}
