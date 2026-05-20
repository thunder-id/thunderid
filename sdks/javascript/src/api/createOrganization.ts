/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import ThunderIDAPIError from '../errors/ThunderIDAPIError';
import {Organization} from '../models/organization';

/**
 * Interface for organization creation payload.
 */
export interface CreateOrganizationPayload {
  /**
   * Organization description.
   */
  description: string;
  /**
   * Organization name.
   */
  name: string;
  /**
   * Organization handle/slug.
   */
  orgHandle?: string;
  /**
   * Parent organization ID.
   */
  parentId: string;
  /**
   * Organization type.
   */
  type: 'TENANT';
}

/**
 * Configuration for the createOrganization request
 */
export interface CreateOrganizationConfig extends Omit<RequestInit, 'method' | 'body'> {
  /**
   * The base URL for the API endpoint.
   */
  baseUrl: string;
  /**
   * Optional custom fetcher function.
   * If not provided, native fetch will be used
   */
  fetcher?: (url: string, config: RequestInit) => Promise<Response>;
  /**
   * Organization creation payload
   */
  payload: CreateOrganizationPayload;
}

/**
 * Creates a new organization.
 *
 * @param config - Configuration object containing baseUrl, payload and optional request config.
 * @returns A promise that resolves with the created organization information.
 * @example
 * ```typescript
 * // Using default fetch
 * try {
 *   const organization = await createOrganization({
 *     baseUrl: "https://api.asgardeo.io/t/<ORGANIZATION>",
 *     payload: {
 *       description: "Share your screens",
 *       name: "Team Viewer",
 *       orgHandle: "team-viewer",
 *       parentId: "f4825104-4948-40d9-ab65-a960eee3e3d5",
 *       type: "TENANT"
 *     }
 *   });
 *   console.log(organization);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to create organization:', error.message);
 *   }
 * }
 * ```
 *
 * @example
 * ```typescript
 * // Using custom fetcher (e.g., axios-based httpClient)
 * try {
 *   const organization = await createOrganization({
 *     baseUrl: "https://api.asgardeo.io/t/<ORGANIZATION>",
 *     payload: {
 *       description: "Share your screens",
 *       name: "Team Viewer",
 *       orgHandle: "team-viewer",
 *       parentId: "f4825104-4948-40d9-ab65-a960eee3e3d5",
 *       type: "TENANT"
 *     },
 *     fetcher: async (url, config) => {
 *       const response = await httpClient({
 *         url,
 *         method: config.method,
 *         headers: config.headers,
 *         data: config.body,
 *         ...config
 *       });
 *       // Convert axios-like response to fetch-like Response
 *       return {
 *         ok: response.status >= 200 && response.status < 300,
 *         status: response.status,
 *         statusText: response.statusText,
 *         json: () => Promise.resolve(response.data),
 *         text: () => Promise.resolve(typeof response.data === 'string' ? response.data : JSON.stringify(response.data))
 *       } as Response;
 *     }
 *   });
 *   console.log(organization);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to create organization:', error.message);
 *   }
 * }
 * ```
 */
const createOrganization = async ({
  baseUrl,
  payload,
  fetcher,
  ...requestConfig
}: CreateOrganizationConfig): Promise<Organization> => {
  try {
    // eslint-disable-next-line no-new
    new URL(baseUrl);
  } catch (error) {
    throw new ThunderIDAPIError(
      `Invalid base URL provided. ${error?.toString()}`,
      'createOrganization-ValidationError-001',
      'javascript',
      400,
      'The provided `baseUrl` does not adhere to the URL schema.',
    );
  }

  if (!payload) {
    throw new ThunderIDAPIError(
      'Organization payload is required',
      'createOrganization-ValidationError-002',
      'javascript',
      400,
      'Invalid Request',
    );
  }

  // Always set type to TENANT for now
  const organizationPayload: CreateOrganizationPayload = {
    ...payload,
    type: 'TENANT' as const,
  };

  const fetchFn: typeof fetch = fetcher || fetch;
  const resolvedUrl = `${baseUrl}/api/server/v1/organizations`;

  const requestInit: RequestInit = {
    ...requestConfig,
    body: JSON.stringify(organizationPayload),
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...requestConfig.headers,
    },
    method: 'POST',
  };

  try {
    const response: Response = await fetchFn(resolvedUrl, requestInit);

    if (!response?.ok) {
      const errorText: string = await response.text();

      throw new ThunderIDAPIError(
        errorText,
        'createOrganization-ResponseError-001',
        'javascript',
        response.status,
        response.statusText,
        'Failed to create organization',
      );
    }

    return (await response.json()) as Organization;
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }

    throw new ThunderIDAPIError(
      `Network or parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      'createOrganization-NetworkError-001',
      'javascript',
      0,
      'Network Error',
    );
  }
};

export default createOrganization;
