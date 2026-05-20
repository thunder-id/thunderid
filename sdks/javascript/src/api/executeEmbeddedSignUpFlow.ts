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
import {EmbeddedFlowType, EmbeddedFlowExecuteResponse, EmbeddedFlowExecuteRequestConfig} from '../models/embedded-flow';

/**
 * Executes an embedded signup flow by sending a request to the specified flow execution endpoint.
 *
 * @param requestConfig - Request configuration object containing URL and payload.
 * @returns A promise that resolves with the flow execution response.
 * @throws ThunderIDAPIError when the request fails or URL is invalid.
 *
 * @example
 * ```typescript
 * try {
 *   const embeddedSignUpResponse = await executeEmbeddedSignUpFlow({
 *     url: "https://api.asgardeo.io/t/<ORGANIZATION>/api/server/v1/flow/execute",
 *     payload: {
 *       flowType: "REGISTRATION"
 *     }
 *   });
 *   console.log(embeddedSignUpResponse);
 * } catch (error) {
 *   if (error instanceof ThunderIDAPIError) {
 *     console.error('Embedded SignUp flow execution failed:', error.message);
 *   }
 * }
 * ```
 */
const executeEmbeddedSignUpFlow = async ({
  url,
  baseUrl,
  payload,
  ...requestConfig
}: EmbeddedFlowExecuteRequestConfig): Promise<EmbeddedFlowExecuteResponse> => {
  if (!baseUrl && !url) {
    throw new ThunderIDAPIError(
      'Embedded SignUp flow execution failed: Base URL or URL is not provided.',
      'javascript-executeEmbeddedSignUpFlow-ValidationError-001',
      'javascript',
      400,
      'At least one of the baseUrl or url must be provided to execute the embedded sign up flow.',
    );
  }

  try {
    // eslint-disable-next-line no-new
    new URL(url ?? baseUrl);
  } catch (error) {
    throw new ThunderIDAPIError(
      `Invalid URL provided. ${error?.toString()}`,
      'executeEmbeddedSignUpFlow-ValidationError-001',
      'javascript',
      400,
      'The provided `url` or `baseUrl` path does not adhere to the URL schema.',
    );
  }

  try {
    const response: Response = await fetch(url ?? `${baseUrl}/api/server/v1/flow/execute`, {
      ...requestConfig,
      body: JSON.stringify({
        ...(payload ?? {}),
        flowType: EmbeddedFlowType.Registration,
      }),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        ...requestConfig.headers,
      },
      method: requestConfig.method || 'POST',
    });

    if (!response.ok) {
      const errorText: string = await response.text();

      throw new ThunderIDAPIError(
        errorText,
        'javascript-executeEmbeddedSignUpFlow-ResponseError-100',
        'javascript',
        response.status,
        response.statusText,
        'Embedded SignUp flow execution failed',
      );
    }

    return (await response.json()) as EmbeddedFlowExecuteResponse;
  } catch (error) {
    if (error instanceof ThunderIDAPIError) {
      throw error;
    }

    throw new ThunderIDAPIError(
      `Network or parsing error: ${error instanceof Error ? error.message : 'Unknown error'}`,
      'executeEmbeddedSignUpFlow-NetworkError-001',
      'javascript',
      0,
      'Network Error',
    );
  }
};

export default executeEmbeddedSignUpFlow;
