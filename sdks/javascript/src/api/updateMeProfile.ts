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
import {User} from '../models/user';
import processUserUsername from '../utils/processUsername';

/**
 * Configuration for the updateMeProfile request
 */
export interface UpdateMeProfileConfig extends Omit<RequestInit, 'method' | 'body'> {
  /**
   * The base path of the API endpoint.
   */
  baseUrl?: string;
  /**
   * Optional custom fetcher function.
   * If not provided, native fetch will be used
   */
  fetcher?: (url: string, config: RequestInit) => Promise<Response>;
  /**
   * The value object to patch (SCIM2 PATCH value)
   */
  payload: any;
  /**
   * The absolute API endpoint.
   */
  url?: string;
}

/**
 * Updates the user profile information at the specified SCIM2 Me endpoint.
 *
 * @param config - Configuration object with URL, payload and optional request config.
 * @returns A promise that resolves with the updated user profile information.
 * @example
 * ```typescript
 * // Using default fetch
 * await updateMeProfile({
 *   url: "https://api.asgardeo.io/t/<ORG>/scim2/Me",
 *   payload: { "urn:scim:wso2:schema": { mobileNumbers: ["0777933830"] } }
 * });
 * ```
 *
 * @example
 * ```typescript
 * // Using custom fetcher (e.g., axios-based httpClient)
 * await updateMeProfile({
 *   url: "https://api.asgardeo.io/t/<ORG>/scim2/Me",
 *   payload: { "urn:scim:wso2:schema": { mobileNumbers: ["0777933830"] } },
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
const updateMeProfile = async ({
  url,
  baseUrl,
  payload,
  fetcher,
  ...requestConfig
}: UpdateMeProfileConfig): Promise<User> => {
  try {
    // eslint-disable-next-line no-new
    new URL(url ?? baseUrl);
  } catch (error) {
    throw new ThunderIDAPIError(
      `Invalid URL provided. ${error?.toString()}`,
      'updateMeProfile-ValidationError-001',
      'javascript',
      400,
      'The provided `url` or `baseUrl` path does not adhere to the URL schema.',
    );
  }

  const data: Record<string, unknown> = {
    Operations: [
      {
        op: 'replace',
        value: payload,
      },
    ],
    schemas: ['urn:ietf:params:scim:api:messages:2.0:PatchOp'],
  };

  const fetchFn: typeof fetch = fetcher || fetch;
  const resolvedUrl: string = url ?? `${baseUrl}/scim2/Me`;

  const requestInit: RequestInit = {
    method: 'PATCH',
    ...requestConfig,
    body: JSON.stringify(data),
    headers: {
      ...requestConfig.headers,
      Accept: 'application/json',
      'Content-Type': 'application/scim+json',
    },
  };

  try {
    const response: Response = await fetchFn(resolvedUrl, requestInit);

    if (!response?.ok) {
      const errorText: string = await response.text();

      throw new ThunderIDAPIError(
        errorText,
        'updateMeProfile-ResponseError-001',
        'javascript',
        response.status,
        response.statusText,
        'Failed to update user profile',
      );
    }

    // Match the read path (`getScim2Me`) — strip the userstore prefix
    // (e.g. "DEFAULT/") so consumers receive a clean `userName`. Without
    // this, the optimistic-update path would put the prefixed value into
    // local state and the UI would flip to "DEFAULT/<email>" after a save.
    return processUserUsername((await response.json()) as User);
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }

    throw new ThunderIDAPIError(
      error?.response?.data?.detail || 'An error occurred while updating the user profile. Please try again.',
      'updateMeProfile-NetworkError-001',
      'javascript',
      error?.data?.status,
      'Network Error',
    );
  }
};

export default updateMeProfile;
