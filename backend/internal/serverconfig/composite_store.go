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

package serverconfig

import "context"

// compositeServerConfigStore combines the file-based (read-only) and database (writable) stores.
// Reads take the readOnly layer from the file store and the writable layer from the db store; writes
// go to the db store only.
type compositeServerConfigStore struct {
	fileStore serverConfigStoreInterface
	dbStore   serverConfigStoreInterface
}

// newCompositeServerConfigStore creates a composite store over the file and database stores.
func newCompositeServerConfigStore(fileStore, dbStore serverConfigStoreInterface) serverConfigStoreInterface {
	return &compositeServerConfigStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

func (c *compositeServerConfigStore) GetServerConfig(ctx context.Context,
	name ConfigName) (storeLayers, error) {
	fileLayers, err := c.fileStore.GetServerConfig(ctx, name)
	if err != nil {
		return storeLayers{}, err
	}
	dbLayers, err := c.dbStore.GetServerConfig(ctx, name)
	if err != nil {
		return storeLayers{}, err
	}
	return storeLayers{ReadOnly: fileLayers.ReadOnly, Writable: dbLayers.Writable}, nil
}

func (c *compositeServerConfigStore) UpsertServerConfig(ctx context.Context, cfg ServerConfig) error {
	return c.dbStore.UpsertServerConfig(ctx, cfg)
}
