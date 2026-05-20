/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package declarativeresource

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// Storer defines the interface for storing resources.
type Storer interface {
	Create(id string, data interface{}) error
}

// ResourceLoader handles loading declarative resources from YAML files.
type ResourceLoader struct {
	config ResourceConfig
	store  Storer
	logger *log.Logger
}

// NewResourceLoader creates a new resource loader.
func NewResourceLoader(config ResourceConfig, store Storer) *ResourceLoader {
	logger := log.GetLogger().With(
		log.String(log.LoggerKeyComponentName, fmt.Sprintf("%sLoader", config.ResourceType)))

	return &ResourceLoader{
		config: config,
		store:  store,
		logger: logger,
	}
}

// LoadResources loads all resources from the configured directory.
// It reads YAML files, parses them, validates them, and stores them.
// Returns an error if any step fails.
func (l *ResourceLoader) LoadResources() error {
	// Read configuration files from the directory
	configs, err := GetConfigs(l.config.DirectoryName)
	if err != nil {
		return err
	}

	// Process each configuration file
	for _, cfg := range configs {
		if err := l.loadSingleResource(cfg); err != nil {
			return err
		}
	}

	return nil
}

// loadSingleResource loads a single resource from YAML data.
func (l *ResourceLoader) loadSingleResource(data []byte) error {
	// Parse YAML to DTO
	dto, err := l.config.Parser(data)
	if err != nil {
		return err
	}

	// Extract resource name/ID for logging
	resourceID := l.config.IDExtractor(dto)

	// Validate the DTO if validator is provided
	if l.config.Validator != nil {
		if err := l.config.Validator(dto); err != nil {
			return err
		}
	}

	// Validate dependencies if dependency validator is provided
	if l.config.DependencyValidator != nil {
		if err := l.config.DependencyValidator(dto); err != nil {
			return err
		}
	}

	// Store the resource
	if err := l.store.Create(resourceID, dto); err != nil {
		return err
	}

	return nil
}
