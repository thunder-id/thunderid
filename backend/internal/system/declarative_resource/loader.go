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
	"flag"
	"fmt"
	"path/filepath"
	"strings"

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

// LoadResources loads all resources using the following precedence:
//  1. A file supplied via the --resources startup argument.
//  2. YAML files found directly in config/resources/ (multi-document format with
//     "resource_type: <type>" fields), when present.
//  3. Individual YAML files inside the config/resources/<DirectoryName>/ subdirectory.
//
// Returns an error if any step fails.
func (l *ResourceLoader) LoadResources() error {
	var configs [][]byte
	var err error

	resourceType := strings.TrimSuffix(l.config.DirectoryName, "s")

	resourcesFile := ""
	if f := flag.Lookup("resources"); f != nil {
		if v := f.Value.String(); v != "" {
			absPath, absErr := filepath.Abs(v)
			if absErr == nil {
				resourcesFile = absPath
			}
		}
	}

	if resourcesFile != "" {
		configs, err = GetConfigsFromFile(resourcesFile, resourceType)
	} else {
		var rootConfigs [][]byte
		rootConfigs, err = GetConfigsFromRootDir(resourceType)
		if err == nil && len(rootConfigs) > 0 {
			configs = rootConfigs
		} else if err == nil {
			configs, err = GetConfigs(l.config.DirectoryName)
		}
	}
	if err != nil {
		return err
	}

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
