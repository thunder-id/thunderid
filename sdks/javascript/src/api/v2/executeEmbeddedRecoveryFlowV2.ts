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
import {EmbeddedFlowExecuteRequestConfig as EmbeddedFlowExecuteRequestConfigV2} from '../../models/v2/embedded-flow-v2';
import {EmbeddedRecoveryFlowResponse} from '../../models/v2/embedded-recovery-flow-v2';

/**
 * Executes an embedded recovery flow by sending a request to the flow execution endpoint.
 *
 * This function handles password-recovery and account-recovery flows driven by the
 * ThunderID server. The server returns UI components for each step (e.g. username
 * collection, OTP verification, password reset) and this function forwards the
 * user's responses back to the server.
 *
 * @param requestConfig - Request configuration containing URL, payload, and optional headers.
 * @returns A promise that resolves with the flow execution response.
 * @throws ThunderIDAPIError when the request fails or a payload is missing.
 *
 * @example
 * ```typescript
 * // Initiate recovery flow
 * const response = await executeEmbeddedRecoveryFlowV2({
 *   baseUrl: 'https://api.asgardeo.io/t/myorg',
 *   payload: {
 *     flowType: 'RECOVERY',
 *     applicationId: 'my-app-id',
 *   },
 * });
 *
 * // Continue recovery flow with user input
 * const nextResponse = await executeEmbeddedRecoveryFlowV2({
 *   baseUrl: 'https://api.asgardeo.io/t/myorg',
 *   payload: {
 *     executionId: response.executionId,
 *     action: 'submit',
 *     inputs: { username: 'user@example.com' },
 *     challengeToken: response.challengeToken,
 *   },
 * });
 * ```
 */
const executeEmbeddedRecoveryFlowV2 = async ({
  url,
  baseUrl,
  payload,
  ...requestConfig
}: EmbeddedFlowExecuteRequestConfigV2): Promise<EmbeddedRecoveryFlowResponse> => {
  if (!payload) {
    throw new ThunderIDAPIError(
      'Recovery payload is required',
      'executeEmbeddedRecoveryFlow-ValidationError-002',
      'javascript',
      400,
      'If a recovery payload is not provided, the request cannot be constructed correctly.',
    );
  }

  const endpoint: string = url ?? `${baseUrl}/flow/execute`;

  // Strip any user-provided 'verbose' parameter as it should only be used internally
  const cleanPayload: typeof payload =
    typeof payload === 'object' && payload !== null
      ? Object.fromEntries(Object.entries(payload).filter(([key]: [string, unknown]) => key !== 'verbose'))
      : payload;

  // `verbose: true` is required to get the `meta` field in the response that includes component details.
  // Add verbose:true if:
  // 1. payload contains only applicationId and flowType (initial request)
  // 2. payload contains only executionId (flow continuation without inputs)
  const hasOnlyAppIdAndFlowType: boolean =
    typeof cleanPayload === 'object' &&
    cleanPayload !== null &&
    'applicationId' in cleanPayload &&
    'flowType' in cleanPayload &&
    Object.keys(cleanPayload).length === 2;
  const hasOnlyFlowId: boolean =
    typeof cleanPayload === 'object' &&
    cleanPayload !== null &&
    'executionId' in cleanPayload &&
    Object.keys(cleanPayload).length === 1;

  const requestPayload: Record<string, unknown> =
    hasOnlyAppIdAndFlowType || hasOnlyFlowId
      ? {
          ...cleanPayload,
          verbose: true,
        }
      : cleanPayload;

  const response: Response = await fetch(endpoint, {
    ...requestConfig,
    body: JSON.stringify(requestPayload),
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
      `Recovery request failed: ${errorText}`,
      'executeEmbeddedRecoveryFlow-ResponseError-001',
      'javascript',
      response.status,
      response.statusText,
    );
  }

  const flowResponse: EmbeddedRecoveryFlowResponse = await response.json();

  return flowResponse;
};

export default executeEmbeddedRecoveryFlowV2;
