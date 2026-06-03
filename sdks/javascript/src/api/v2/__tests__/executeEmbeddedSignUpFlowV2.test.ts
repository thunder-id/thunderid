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

import {beforeEach, describe, expect, it, vi} from 'vitest';
import {EmbeddedSignUpFlowResponse, EmbeddedSignUpFlowStatus} from '../../../models/v2/embedded-signup-flow-v2';
import executeEmbeddedSignUpFlowV2 from '../executeEmbeddedSignUpFlowV2';

const URL = 'https://localhost:8090/flow/execute';

const mockFlowResponse = (overrides: Partial<EmbeddedSignUpFlowResponse> = {}): EmbeddedSignUpFlowResponse =>
  ({
    flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
    ...overrides,
  }) as EmbeddedSignUpFlowResponse;

const captureRequestBody = (): Record<string, unknown> => {
  const calls = (fetch as ReturnType<typeof vi.fn>).mock.calls;
  const requestInit = calls[calls.length - 1][1] as RequestInit;
  return JSON.parse(requestInit.body as string) as Record<string, unknown>;
};

describe('executeEmbeddedSignUpFlowV2', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockFlowResponse()),
      ok: true,
    });
  });

  describe('verbose: true injection', (): void => {
    it('injects verbose:true for a new flow start with applicationId and flowType', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {applicationId: 'app-1', flowType: 'REGISTRATION'},
        url: URL,
      });

      expect(captureRequestBody()).toMatchObject({verbose: true});
    });

    it('injects verbose:true for a new flow start that also includes scopes', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {applicationId: 'app-1', flowType: 'REGISTRATION', scopes: ['openid', 'profile']},
        url: URL,
      });

      const body = captureRequestBody();
      expect(body).toMatchObject({verbose: true, inputs: {requested_permissions: 'openid profile'}});
      expect(body).not.toHaveProperty('scopes');
    });

    it('injects verbose:true for a bare flow resumption (executionId only)', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {executionId: 'exec-abc'},
        url: URL,
      });

      expect(captureRequestBody()).toMatchObject({verbose: true});
    });

    it('does NOT inject verbose:true for a step submission (executionId + inputs)', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {action: 'submit', executionId: 'exec-abc', inputs: {email: 'user@example.com'}},
        url: URL,
      });

      expect(captureRequestBody()).not.toHaveProperty('verbose');
    });

    it('strips a user-supplied verbose before applying internal logic', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {action: 'submit', executionId: 'exec-abc', inputs: {}, verbose: false},
        url: URL,
      });

      expect(captureRequestBody()).not.toHaveProperty('verbose');
    });

    it('strips user-supplied verbose:true from step submissions', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {action: 'submit', executionId: 'exec-abc', inputs: {}, verbose: true},
        url: URL,
      });

      expect(captureRequestBody()).not.toHaveProperty('verbose');
    });
  });

  describe('scopes → inputs.requested_permissions translation', (): void => {
    it('translates scopes to a space-separated inputs.requested_permissions string', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {applicationId: 'app-1', flowType: 'REGISTRATION', scopes: ['openid', 'profile', 'email']},
        url: URL,
      });

      const body = captureRequestBody();
      expect(body).toMatchObject({inputs: {requested_permissions: 'openid profile email'}});
      expect(body).not.toHaveProperty('scopes');
    });

    it('does not add requested_permissions when scopes is absent', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {applicationId: 'app-1', flowType: 'REGISTRATION'},
        url: URL,
      });

      expect(captureRequestBody()).not.toHaveProperty('inputs');
    });

    it('does not add requested_permissions when scopes is an empty array', async (): Promise<void> => {
      await executeEmbeddedSignUpFlowV2({
        payload: {applicationId: 'app-1', flowType: 'REGISTRATION', scopes: []},
        url: URL,
      });

      const body = captureRequestBody();
      expect(body).not.toHaveProperty('scopes');
      expect(body).not.toHaveProperty('inputs');
    });
  });

  it('throws when payload is missing', async (): Promise<void> => {
    await expect(executeEmbeddedSignUpFlowV2({url: URL})).rejects.toThrow('Registration payload is required');
  });

  it('uses baseUrl to construct the endpoint when url is not provided', async (): Promise<void> => {
    await executeEmbeddedSignUpFlowV2({
      baseUrl: 'https://localhost:8090',
      payload: {applicationId: 'app-1', flowType: 'REGISTRATION'},
    });

    expect(fetch).toHaveBeenCalledWith('https://localhost:8090/flow/execute', expect.any(Object));
  });
});
