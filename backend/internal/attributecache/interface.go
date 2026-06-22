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
	"context"
)

// AttributeCacheStoreInterface defines the interface for the attribute cache store.
type AttributeCacheStoreInterface interface {
	// CreateAttributeCache creates a new attribute cache entry in the store.
	CreateAttributeCache(ctx context.Context, cache AttributeCache) error

	// GetAttributeCache retrieves an attribute cache entry by ID from the store.
	GetAttributeCache(ctx context.Context, id string) (AttributeCache, error)

	// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry in the store.
	ExtendAttributeCacheTTL(ctx context.Context, id string, ttlSeconds int) error

	// DeleteAttributeCache deletes an attribute cache entry by ID from the store.
	DeleteAttributeCache(ctx context.Context, id string) error
}
