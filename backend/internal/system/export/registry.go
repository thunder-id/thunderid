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

package export

import declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"

// ResourceExporterRegistry holds all registered resource exporters.
type ResourceExporterRegistry struct {
	exporters map[string]declarativeresource.ResourceExporter
}

// NewResourceExporterRegistry creates a new registry for resource exporters.
func newResourceExporterRegistry() *ResourceExporterRegistry {
	return &ResourceExporterRegistry{
		exporters: make(map[string]declarativeresource.ResourceExporter),
	}
}

// Register adds a new resource exporter to the registry.
func (r *ResourceExporterRegistry) Register(exporter declarativeresource.ResourceExporter) {
	r.exporters[exporter.GetResourceType()] = exporter
}

// Get retrieves a resource exporter by type.
func (r *ResourceExporterRegistry) Get(resourceType string) (declarativeresource.ResourceExporter, bool) {
	exporter, exists := r.exporters[resourceType]
	return exporter, exists
}

// GetAll returns all registered exporters.
func (r *ResourceExporterRegistry) GetAll() map[string]declarativeresource.ResourceExporter {
	return r.exporters
}
