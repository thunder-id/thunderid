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

import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import {EmbeddedFlowExecuteRequestConfig as EmbeddedFlowExecuteRequestConfigV2} from '../../models/v2/embedded-flow-v2';
import {
  EmbeddedSignInFlowResponse as EmbeddedSignInFlowResponseV2,
  EmbeddedSignInFlowStatus as EmbeddedSignInFlowStatusV2,
} from '../../models/v2/embedded-signin-flow-v2';

const executeEmbeddedSignInFlowV2 = async ({
  url,
  baseUrl,
  payload,
  authId,
  ...requestConfig
}: EmbeddedFlowExecuteRequestConfigV2): Promise<EmbeddedSignInFlowResponseV2> => {
  if (!payload) {
    throw new ThunderIDAPIError(
      'Authorization payload is required',
      'executeEmbeddedSignInFlow-ValidationError-002',
      'javascript',
      400,
      'If an authorization payload is not provided, the request cannot be constructed correctly.',
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
  // 1. payload contains only applicationId and flowType
  // 2. payload contains only executionId
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
    hasOnlyAppIdAndFlowType || hasOnlyFlowId ? {...cleanPayload, verbose: true} : cleanPayload;

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
      errorText,
      'executeEmbeddedSignInFlow-ResponseError-001',
      'javascript',
      response.status,
      response.statusText,
      'Authorization request failed',
    );
  }

  const flowResponse: EmbeddedSignInFlowResponseV2 = await response.json();

  // IMPORTANT: Only applicable for ThunderID V2 platform.
  // Check if the flow is complete and has an assertion and authId is provided, then call OAuth2 auth callback.
  if (flowResponse.flowStatus === EmbeddedSignInFlowStatusV2.Complete && flowResponse.assertion && authId) {
    try {
      const oauth2Response: Response = await fetch(`${baseUrl}/oauth2/auth/callback`, {
        body: JSON.stringify({
          assertion: flowResponse.assertion,
          authId,
        }),
        credentials: 'include',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
          ...requestConfig.headers,
        },
        method: 'POST',
      });

      if (!oauth2Response.ok) {
        const oauth2ErrorText: string = await oauth2Response.text();

        throw new ThunderIDAPIError(
          `OAuth2 authorization failed: ${oauth2ErrorText}`,
          'executeEmbeddedSignInFlow-OAuth2Error-002',
          'javascript',
          oauth2Response.status,
          oauth2Response.statusText,
        );
      }

      const oauth2Result: Record<string, unknown> = await oauth2Response.json();

      return {
        flowStatus: flowResponse.flowStatus,
        redirectUrl: oauth2Result['redirect_uri'],
      } as any;
    } catch (authError) {
      throw new ThunderIDAPIError(
        `OAuth2 authorization failed: ${authError instanceof Error ? authError.message : 'Unknown error'}`,
        'executeEmbeddedSignInFlow-OAuth2Error-001',
        'javascript',
        500,
        'Failed to complete OAuth2 authorization after successful embedded sign-in flow.',
      );
    }
  }

  return flowResponse;
};

export default executeEmbeddedSignInFlowV2;
