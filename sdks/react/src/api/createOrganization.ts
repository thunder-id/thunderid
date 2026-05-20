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

import {
  Organization,
  HttpResponse,
  FetchHttpClient,
  HttpRequestConfig,
  createOrganization as baseCreateOrganization,
  CreateOrganizationConfig as BaseCreateOrganizationConfig,
} from '@thunderid/browser';

/**
 * Configuration for the createOrganization request (React-specific)
 */
export interface CreateOrganizationConfig extends Omit<BaseCreateOrganizationConfig, 'fetcher'> {
  /**
   * Optional custom fetcher function. If not provided, the ThunderID SPA client's httpClient will be used
   * which is a wrapper around axios http.request
   */
  fetcher?: (url: string, config: RequestInit) => Promise<Response>;
  /**
   * Optional instance ID for multi-instance support. Defaults to 0.
   */
  instanceId?: number;
}

/**
 * Creates a new organization.
 * This function uses the ThunderID SPA client's httpClient by default, but allows for custom fetchers.
 *
 * @param config - Configuration object containing baseUrl, payload and optional request config.
 * @returns A promise that resolves with the created organization information.
 * @example
 * ```typescript
 * // Using default ThunderID SPA client httpClient
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
 * // Using custom fetcher
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
 *     fetcher: customFetchFunction
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
  fetcher,
  instanceId = 0,
  ...requestConfig
}: CreateOrganizationConfig): Promise<Organization> => {
  const defaultFetcher = async (url: string, config: RequestInit): Promise<Response> => {
    const httpClient: FetchHttpClient = FetchHttpClient.getInstance(instanceId);
    const response: HttpResponse<any> = await httpClient.request({
      data: config.body ? JSON.parse(config.body as string) : undefined,
      headers: config.headers as Record<string, string>,
      method: config.method || 'POST',
      url,
    } as HttpRequestConfig);

    return {
      json: () => Promise.resolve(response.data),
      ok: response.status >= 200 && response.status < 300,
      status: response.status,
      statusText: response.statusText || '',
      text: () => Promise.resolve(typeof response.data === 'string' ? response.data : JSON.stringify(response.data)),
    } as Response;
  };

  return baseCreateOrganization({
    ...requestConfig,
    fetcher: fetcher || defaultFetcher,
  });
};

export default createOrganization;
