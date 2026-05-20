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

/**
 * Retrieves the user information from the specified OIDC userinfo endpoint.
 *
 * @param requestConfig - Request configuration object.
 * @returns A promise that resolves with the user information.
 * @throw
 *   const userInfo = await getUserInfo({
 *     url: "https://api.asgardeo.io/t/<ORGANIZATION>/oauth2/userinfo",
 *   });
 *   console.log(userInfo);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Failed to get user info:', error.message);
 *   }
 * }
 * ```
 */
const getUserInfo = async ({url, ...requestConfig}: Partial<Request>): Promise<User> => {
  try {
    // eslint-disable-next-line no-new
    new URL(url);
  } catch (error) {
    throw new ThunderIDAPIError(
      'Invalid endpoint URL provided',
      'getUserInfo-ValidationError-001',
      'javascript',
      400,
      'Invalid Request',
    );
  }

  try {
    const response: Response = await fetch(url, {
      ...requestConfig,
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        ...requestConfig.headers,
      },
      method: 'GET',
    });

    if (!response.ok) {
      const errorText: string = await response.text();

      throw new ThunderIDAPIError(
        errorText,
        'getUserInfo-ResponseError-001',
        'javascript',
        response.status,
        response.statusText,
        'Failed to fetch user info',
      );
    }

    return (await response.json()) as User;
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }
    throw new ThunderIDAPIError(
      `Network or parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      'getUserInfo-NetworkError-001',
      'javascript',
      0,
      'Network Error',
    );
  }
};

export default getUserInfo;
