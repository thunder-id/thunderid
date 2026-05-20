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

// ExportRequest represents the request structure for exporting resources.
type ExportRequest struct {
	Applications        []string `json:"applications,omitempty"`
	IdentityProviders   []string `json:"identityProviders,omitempty"`
	NotificationSenders []string `json:"notificationSenders,omitempty"`
	UserTypes           []string `json:"userTypes,omitempty"`
	OrganizationUnits   []string `json:"organizationUnits,omitempty"`
	Users               []string `json:"users,omitempty"`
	Groups              []string `json:"groups,omitempty"`
	ResourceServers     []string `json:"resourceServers,omitempty"`
	Roles               []string `json:"roles,omitempty"`
	Flows               []string `json:"flows,omitempty"`
	Translations        []string `json:"translations,omitempty"`
	Layouts             []string `json:"layouts,omitempty"`
	Themes              []string `json:"themes,omitempty"`

	Options *ExportOptions `json:"options,omitempty"`
}

// ExportOptions provides configuration for export behavior.
type ExportOptions struct {
	// IncludeMetadata determines whether to include metadata (creation dates, IDs, etc.)
	IncludeMetadata bool `json:"includeMetadata,omitempty"`

	// IncludeDependencies automatically exports related resources
	IncludeDependencies bool `json:"includeDependencies,omitempty"`

	// Format specifies the output format for individual files (yaml, json)
	Format string `json:"format,omitempty"` // Default: "yaml"

	// Folder structure options
	FolderStructure *FolderStructureOptions `json:"folderStructure,omitempty"`

	// Pagination for bulk exports
	Pagination *PaginationOptions `json:"pagination,omitempty"`
}

// FolderStructureOptions configures how files are organized in exports.
type FolderStructureOptions struct {
	// GroupByType creates separate folders for each resource type
	GroupByType bool `json:"groupByType,omitempty"`

	// CustomStructure allows defining custom folder paths
	CustomStructure map[string]string `json:"customStructure,omitempty"`

	// FileNamingPattern defines how files should be named
	FileNamingPattern string `json:"fileNamingPattern,omitempty"` // e.g., "${name}_${id}", "${type}_${name}"
}

// PaginationOptions configures pagination for bulk exports.
type PaginationOptions struct {
	// Page number (1-based)
	Page int `json:"page,omitempty"`

	// Number of resources per page
	Limit int `json:"limit,omitempty"`
}

// ExportResponse represents the response structure for exporting resources.
type ExportResponse struct {
	Files   []ExportFile     `json:"files"`
	EnvFile *EnvironmentFile `json:"envFile,omitempty"`

	// Summary information about the export
	Summary *ExportSummary `json:"summary,omitempty"`
}

// JSONExportResponse represents the JSON payload returned by the export endpoints.
type JSONExportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

// EnvironmentFile represents a generated .env file containing template variables.
type EnvironmentFile struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
	Size     int64  `json:"size,omitempty"`
}

// ExportSummary provides metadata about the export operation.
type ExportSummary struct {
	TotalFiles    int                               `json:"totalFiles"`
	TotalSize     int64                             `json:"totalSizeBytes,omitempty"`
	ExportedAt    string                            `json:"exportedAt,omitempty"`
	ResourceTypes map[string]int                    `json:"resourceTypes,omitempty"` // Type -> count
	Errors        []declarativeresource.ExportError `json:"errors,omitempty"`
	Pagination    *PaginationInfo                   `json:"pagination,omitempty"`
}

// ExportError represents errors that occurred during export.
// Deprecated: Use declarativeresource.ExportError instead.
type ExportError = declarativeresource.ExportError

// PaginationInfo provides pagination metadata.
type PaginationInfo struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalPages int  `json:"totalPages,omitempty"`
	HasMore    bool `json:"hasMore"`
}

// ExportFile represents a single YAML file in the export response.
type ExportFile struct {
	FileName     string `json:"fileName"`
	Content      string `json:"content"`
	FolderPath   string `json:"folderPath,omitempty"`   // Relative path within the export
	ResourceType string `json:"resourceType,omitempty"` // application, group, user, idp
	ResourceID   string `json:"resourceId,omitempty"`   // ID of the exported resource
	Size         int64  `json:"size,omitempty"`         // File size in bytes
}
