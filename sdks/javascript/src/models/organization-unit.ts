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

export interface OrganizationUnit {
  description?: string;
  handle: string;
  id: string;
  logoUrl?: string;
  name: string;
  parent?: {
    id: string;
    ref?: string;
  };
}

export interface OrganizationUnitListResponse {
  count: number;
  organizationUnits: OrganizationUnit[];
  startIndex: number;
  totalResults: number;
}

/**
 * Request configuration for fetching a single organization unit.
 *
 * @example
 * ```typescript
 * const config: GetOrganizationUnitConfig = {
 *   baseUrl: 'https://localhost:8090',
 *   organizationUnitId: '0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1',
 * };
 * ```
 *
 * @experimental This API may change in future versions
 */
export interface GetOrganizationUnitConfig extends Omit<Partial<RequestInit>, 'method' | 'body'> {
  /**
   * Base URL of the API server.
   * Either `baseUrl` or `url` must be provided.
   */
  baseUrl?: string;

  /**
   * The ID of the organization unit to retrieve.
   */
  organizationUnitId: string;

  /**
   * Fully qualified URL of the organization unit endpoint.
   * When provided, `baseUrl` is ignored.
   */
  url?: string;
}

/**
 * Request configuration for fetching child organization units.
 *
 * @example
 * ```typescript
 * const config: GetOrganizationUnitChildrenConfig = {
 *   baseUrl: 'https://localhost:8090',
 *   organizationUnitId: '0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1',
 *   limit: 10,
 *   offset: 0,
 * };
 * ```
 *
 * @experimental This API may change in future versions
 */
export interface GetOrganizationUnitChildrenConfig extends Omit<Partial<RequestInit>, 'method' | 'body'> {
  /**
   * Base URL of the API server.
   * Either `baseUrl` or `url` must be provided.
   */
  baseUrl?: string;

  /**
   * Maximum number of child OUs to return. Defaults to 10.
   */
  limit?: number;

  /**
   * Pagination offset. Defaults to 0.
   */
  offset?: number;

  /**
   * The ID of the parent organization unit.
   */
  organizationUnitId: string;

  /**
   * Fully qualified URL of the organization unit children endpoint.
   * When provided, `baseUrl` is ignored.
   */
  url?: string;
}
