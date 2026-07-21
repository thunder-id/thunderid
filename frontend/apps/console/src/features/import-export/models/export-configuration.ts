/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Request body for exporting resources
 *
 * @public
 */
export interface ExportRequest {
  /**
   * List of application IDs to export. Use `["*"]` to export all applications.
   */
  applications?: string[];
  /**
   * List of connection (identity provider and notification sender) IDs to export. Use `["*"]` to export all.
   */
  connections?: string[];
  /**
   * List of user type IDs to export. Use `["*"]` to export all.
   */
  userTypes?: string[];
  /**
   * List of organization unit IDs to export. Use `["*"]` to export all.
   */
  organizationUnits?: string[];
  /**
   * List of user IDs to export. Use `["*"]` to export all.
   */
  users?: string[];
  /**
   * List of flow IDs to export. Use `["*"]` to export all.
   */
  flows?: string[];
  /**
   * List of translation IDs to export. Use `["*"]` to export all.
   */
  translations?: string[];
  /**
   * List of layout IDs to export. Use `["*"]` to export all.
   */
  layouts?: string[];
  /**
   * List of theme IDs to export. Use `["*"]` to export all.
   */
  themes?: string[];
  /**
   * List of resource server IDs to export. Use `["*"]` to export all.
   */
  resourceServers?: string[];
  /**
   * List of role IDs to export. Use `["*"]` to export all.
   */
  roles?: string[];
  /**
   * List of group IDs to export. Use `["*"]` to export all.
   */
  groups?: string[];
  /**
   * List of agent IDs to export. Use `["*"]` to export all.
   */
  agents?: string[];
  /**
   * List of server config names to export. Use `["*"]` to export all.
   */
  serverConfigs?: string[];
  /**
   * Optional configuration for export behavior
   */
  options?: ExportOptions;
}

/**
 * Optional configuration for export behavior
 *
 * @public
 */
export interface ExportOptions {
  /**
   * Include additional metadata in exported files (creation dates, IDs, etc.)
   */
  includeMetadata?: boolean;
  /**
   * Automatically export related resources and dependencies
   */
  includeDependencies?: boolean;
  /**
   * Output format for individual files
   */
  format?: 'yaml';
  /**
   * Configuration for how files are organized in exports
   */
  folderStructure?: FolderStructureOptions;
  /**
   * Pagination configuration for bulk exports
   */
  pagination?: PaginationOptions;
}

/**
 * Configuration for how files are organized in exports
 *
 * @public
 */
export interface FolderStructureOptions {
  /**
   * Create separate folders for each resource type
   */
  groupByType?: boolean;
  /**
   * Define custom folder paths for different resource types
   */
  customStructure?: Record<string, string>;
  /**
   * Pattern for file naming using template variables
   */
  fileNamingPattern?: string;
}

/**
 * Pagination configuration for bulk exports
 *
 * @public
 */
export interface PaginationOptions {
  /**
   * Page number (1-based)
   */
  page?: number;
  /**
   * Number of resources per page
   */
  limit?: number;
}

/**
 * Response containing exported files and summary
 *
 * @public
 */
export interface ExportResponse {
  /**
   * Array of exported configuration files
   */
  files: ExportFile[];
  /**
   * Summary information about the export operation
   */
  summary: ExportSummary;
}

/**
 * Individual exported configuration file
 *
 * @public
 */
export interface ExportFile {
  /**
   * Name of the file
   */
  fileName: string;
  /**
   * File content (YAML or JSON)
   */
  content: string;
  /**
   * Relative path within the export
   */
  folderPath: string;
  /**
   * Type of resource (application, connection, etc.)
   */
  resourceType: string;
  /**
   * ID of the exported resource
   */
  resourceId: string;
  /**
   * File size in bytes
   */
  size: number;
}

/**
 * Summary information about the export operation
 *
 * @public
 */
export interface ExportSummary {
  /**
   * Total number of files exported
   */
  totalFiles: number;
  /**
   * Total size of all exported files in bytes
   */
  totalSizeBytes: number;
  /**
   * Timestamp when the export was performed (ISO 8601 format)
   */
  exportedAt: string;
  /**
   * Count of resources by type
   */
  resourceTypes: Record<string, number>;
  /**
   * Errors encountered during export (partial success possible)
   */
  errors: ExportError[];
  /**
   * Pagination metadata for export results
   */
  pagination?: PaginationInfo;
}

/**
 * Error information for a failed resource export
 *
 * @public
 */
export interface ExportError {
  /**
   * Type of resource that failed to export
   */
  resourceType: string;
  /**
   * ID of the resource that failed
   */
  resourceId: string;
  /**
   * Error message
   */
  error: string;
  /**
   * Error code
   */
  code: string;
}

/**
 * Pagination metadata for export results
 *
 * @public
 */
export interface PaginationInfo {
  /**
   * Current page number
   */
  page: number;
  /**
   * Number of items per page
   */
  limit: number;
  /**
   * Total number of pages available
   */
  total_pages: number;
  /**
   * Whether more pages are available
   */
  has_more: boolean;
}

/**
 * JSON response from the /export endpoint with combined YAML and environment variables
 *
 * @public
 */
export interface JSONExportResponse {
  /**
   * Combined YAML content for all exported resources
   */
  resources: string;
  /**
   * Generated .env file content (empty when no template variables present)
   */
  environment_variables: string;
}
