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

import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import {GetOrganizationUnitChildrenConfig, OrganizationUnitListResponse} from '../../models/v2/organization-unit';

/**
 * Retrieves the child organization units of a given parent OU.
 *
 * @param config - Request configuration including `baseUrl`/`url`, `organizationUnitId`,
 *                 and optional `limit`/`offset` pagination parameters.
 * @returns A promise that resolves with the paginated list of child organization units.
 *
 * @throws {ThunderIDAPIError} When the server returns a non-OK response.
 *
 * @example
 * ```typescript
 * const children = await getOrganizationUnitChildren({
 *   baseUrl: 'https://localhost:8090',
 *   organizationUnitId: '0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1',
 *   limit: 10,
 *   offset: 0,
 * });
 * console.log(children.organizationUnits);
 * ```
 *
 * @experimental This function targets the ThunderID V2 platform API
 */
const getOrganizationUnitChildren = async ({
  url,
  baseUrl,
  organizationUnitId,
  limit = 10,
  offset = 0,
  ...requestConfig
}: GetOrganizationUnitChildrenConfig): Promise<OrganizationUnitListResponse> => {
  if (!organizationUnitId) {
    throw new ThunderIDAPIError(
      'Organization Unit ID is required',
      'getOrganizationUnitChildren-ValidationError-001',
      'javascript',
      400,
      'If an organization unit ID is not provided, the request cannot be constructed correctly.',
    );
  }

  const queryParams: URLSearchParams = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });

  const endpoint: string = url ?? `${baseUrl}/organization-units/${organizationUnitId}/ous?${queryParams.toString()}`;

  const response: Response = await fetch(endpoint, {
    ...requestConfig,
    headers: {
      Accept: 'application/json',
      ...requestConfig.headers,
    },
    method: 'GET',
  });

  if (!response.ok) {
    const errorText: string = await response.text();

    throw new ThunderIDAPIError(
      errorText,
      'getOrganizationUnitChildren-ResponseError-001',
      'javascript',
      response.status,
      response.statusText,
      'Failed to fetch organization unit children',
    );
  }

  const listResponse: OrganizationUnitListResponse = await response.json();

  return listResponse;
};

export default getOrganizationUnitChildren;
