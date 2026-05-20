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

package inboundclient

import (
	"context"
	"fmt"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// loadDeclarativeResources loads inbound clients from the consumer's YAML directory into the
// inboundclient file store. Mirrors entity.loadDeclarativeResources: the file store is
// extracted from the underlying store hierarchy; consumers don't manage it.
//
// Returns nil silently when the inboundclient store has no file-backed component.
func loadDeclarativeResources(
	ctx context.Context, store inboundClientStoreInterface, cfg inboundmodel.DeclarativeLoaderConfig) error {
	fileStore := extractFileStore(store)
	if fileStore == nil {
		return nil
	}

	resourceCfg := declarativeresource.ResourceConfig{
		ResourceType:  cfg.ResourceType,
		DirectoryName: cfg.DirectoryName,
		Parser: func(data []byte) (interface{}, error) {
			return cfg.Parser(data)
		},
		IDExtractor: func(data interface{}) string {
			if c, ok := data.(*inboundmodel.InboundClient); ok {
				return c.ID
			}
			return ""
		},
		Validator: func(data interface{}) error {
			c, ok := data.(*inboundmodel.InboundClient)
			if !ok {
				return fmt.Errorf("unexpected data type: %T", data)
			}
			if cfg.Validator != nil {
				if err := cfg.Validator(c); err != nil {
					return err
				}
			}
			return validateUniqueInboundClientID(ctx, store, c)
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceCfg, fileStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load inbound client declarative resources: %w", err)
	}
	return nil
}

// extractFileStore walks the store hierarchy and returns the file-backed leaf, or nil when
// the configuration has no declarative side.
func extractFileStore(store inboundClientStoreInterface) *fileBasedStore {
	switch s := store.(type) {
	case *fileBasedStore:
		return s
	case *compositeStore:
		if fs, ok := s.fileStore.(*fileBasedStore); ok {
			return fs
		}
	}
	return nil
}

// validateUniqueInboundClientID rejects an inbound client whose entity ID already exists. In
// composite mode, this checks both the file store and the database store.
func validateUniqueInboundClientID(ctx context.Context, store inboundClientStoreInterface,
	c *inboundmodel.InboundClient) error {
	exists, err := store.InboundClientExists(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("failed to check inbound client existence: %w", err)
	}
	if exists {
		return fmt.Errorf("duplicate entity ID '%s': an inbound client with this ID already exists", c.ID)
	}
	return nil
}
