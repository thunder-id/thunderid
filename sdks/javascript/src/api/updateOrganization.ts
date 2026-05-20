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

import {OrganizationDetails} from './getOrganization';
import ThunderIDAPIError from '../errors/ThunderIDAPIError';
import isEmpty from '../utils/isEmpty';

/**
 * Configuration for the updateOrganization request
 */
export interface UpdateOrganizationConfig extends Omit<RequestInit, 'method' | 'body'> {
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
   * Array of patch operations to apply
   */
  operations: {
    operation: 'REPLACE' | 'ADD' | 'REMOVE';
    path: string;
    value?: any;
  }[];
  /**
   * The ID of the organization to update
   */
  organizationId: string;
}

/**
 * Updates the organization information using the Organizations Management API.
 *
 * @param config - Configuration object with baseUrl, organizationId, operations and optional request config.
 * @returns A promise that resolves with the updated organization information.
 * @example
 * ```typescript
 * // Using the helper function to create operations automatically
 * const operations = createPatchOperations({
 *   name: "Updated Organization Name",      // Will use REPLACE
 *   description: "",                        // Will use REMOVE (empty string)
 *   customField: "Some value"              // Will use REPLACE
 * });
 *
 * await updateOrganization({
 *   baseUrl: "https://api.asgardeo.io/t/<ORG>",
 *   organizationId: "0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1",
 *   operations
 * });
 *
 * // Or manually specify operations
 * await updateOrganization({
 *   baseUrl: "https://api.asgardeo.io/t/<ORG>",
 *   organizationId: "0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1",
 *   operations: [
 *     { operation: "REPLACE", path: "/name", value: "Updated Organization Name" },
 *     { operation: "REMOVE", path: "/description" }
 *   ]
 * });
 * ```
 *
 * @example
 * ```typescript
 * // Using custom fetcher (e.g., axios-based httpClient)
 * await updateOrganization({
 *   baseUrl: "https://api.asgardeo.io/t/<ORG>",
 *   organizationId: "0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1",
 *   operations: [
 *     { operation: "REPLACE", path: "/name", value: "Updated Organization Name" }
 *   ],
 *   fetcher: async (url, config) => {
 *     const response = await httpClient({
 *       url,
 *       method: config.method,
 *       headers: config.headers,
 *       data: config.body,
 *       ...config
 *     });
 *     // Convert axios-like response to fetch-like Response
 *     return {
 *       ok: response.status >= 200 && response.status < 300,
 *       status: response.status,
 *       statusText: response.statusText,
 *       json: () => Promise.resolve(response.data),
 *       text: () => Promise.resolve(typeof response.data === 'string' ? response.data : JSON.stringify(response.data))
 *     } as Response;
 *   }
 * });
 * ```
 */
const updateOrganization = async ({
  baseUrl,
  organizationId,
  operations,
  fetcher,
  ...requestConfig
}: UpdateOrganizationConfig): Promise<OrganizationDetails> => {
  try {
    // eslint-disable-next-line no-new
    new URL(baseUrl);
  } catch (error) {
    throw new ThunderIDAPIError(
      `Invalid base URL provided. ${error?.toString()}`,
      'updateOrganization-ValidationError-001',
      'javascript',
      400,
      'The provided `baseUrl` does not adhere to the URL schema.',
    );
  }

  if (!organizationId) {
    throw new ThunderIDAPIError(
      'Organization ID is required',
      'updateOrganization-ValidationError-002',
      'javascript',
      400,
      'Invalid Request',
    );
  }

  if (!operations || !Array.isArray(operations) || operations.length === 0) {
    throw new ThunderIDAPIError(
      'Operations array is required and cannot be empty',
      'updateOrganization-ValidationError-003',
      'javascript',
      400,
      'Invalid Request',
    );
  }

  const fetchFn: typeof fetch = fetcher || fetch;
  const resolvedUrl = `${baseUrl}/api/server/v1/organizations/${organizationId}`;

  const requestInit: RequestInit = {
    ...requestConfig,
    body: JSON.stringify(operations),
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...requestConfig.headers,
    },
    method: 'PATCH',
  };

  try {
    const response: Response = await fetchFn(resolvedUrl, requestInit);

    if (!response?.ok) {
      const errorText: string = await response.text();

      throw new ThunderIDAPIError(
        errorText,
        'updateOrganization-ResponseError-001',
        'javascript',
        response.status,
        response.statusText,
        'Failed to update organization',
      );
    }

    return (await response.json()) as OrganizationDetails;
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }

    throw new ThunderIDAPIError(
      `Network or parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      'updateOrganization-NetworkError-001',
      'javascript',
      0,
      'Network Error',
    );
  }
};

/**
 * Helper function to convert field updates to patch operations format.
 * Uses REMOVE operation when the value is empty, otherwise uses REPLACE.
 *
 * @param payload - Object containing field updates
 * @returns Array of patch operations
 */
export const createPatchOperations = (
  payload: Record<string, any>,
): {
  operation: 'REPLACE' | 'REMOVE';
  path: string;
  value?: any;
}[] =>
  Object.entries(payload).map(([key, value]: [string, any]) => {
    if (isEmpty(value)) {
      return {
        operation: 'REMOVE' as const,
        path: `/${key}`,
      };
    }

    return {
      operation: 'REPLACE' as const,
      path: `/${key}`,
      value,
    };
  });

export default updateOrganization;
