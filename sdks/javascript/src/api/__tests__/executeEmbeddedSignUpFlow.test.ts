/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {Mock, beforeEach, describe, expect, it, vi} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import {EmbeddedFlowType} from '../../models/embedded-flow';
import {
  EmbeddedSignUpFlowResponse,
  EmbeddedSignUpFlowStatus,
  EmbeddedSignUpFlowType,
} from '../../models/embedded-signup-flow';
import executeEmbeddedSignUpFlow from '../executeEmbeddedSignUpFlow';

describe('executeEmbeddedSignUpFlow', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should execute successfully with explicit url', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-123',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const url = 'https://localhost:8090/flow/execute';
    const payload = {executionId: 'exec-123', action: 'submit', inputs: {username: 'user@example.com'}};

    const result: EmbeddedSignUpFlowResponse = await executeEmbeddedSignUpFlow({payload, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.objectContaining({method: 'POST'}));
    expect(result).toEqual(mockResponse);
  });

  it('should fall back to baseUrl when url is not provided', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-456',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://localhost:8090';
    const payload = {executionId: 'exec-456', action: 'submit', inputs: {username: 'user'}};

    const result: EmbeddedSignUpFlowResponse = await executeEmbeddedSignUpFlow({baseUrl, payload});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/flow/execute`, expect.any(Object));
    expect(result).toEqual(mockResponse);
  });

  it('should prefer url over baseUrl when both are provided', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-789',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const url = 'https://localhost:8090/flow/execute';
    const baseUrl = 'https://localhost:8090';

    await executeEmbeddedSignUpFlow({baseUrl, payload: {executionId: 'exec-789'}, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.any(Object));
  });

  it('should respect method override from requestConfig', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-1',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://localhost:8090';

    await executeEmbeddedSignUpFlow({
      baseUrl,
      method: 'PUT' as any,
      payload: {executionId: 'exec-1'},
    });

    expect(fetch).toHaveBeenCalledWith(
      `${baseUrl}/flow/execute`,
      expect.objectContaining({method: 'PUT'}),
    );
  });

  it('should throw ThunderIDAPIError when payload is missing', async (): Promise<void> => {
    const baseUrl = 'https://localhost:8090';

    await expect(executeEmbeddedSignUpFlow({baseUrl} as any)).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignUpFlow({baseUrl} as any)).rejects.toThrow('Registration payload is required');
  });

  it('should add verbose=true for new flow start (applicationId + flowType in payload)', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-new',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    const payload = {applicationId: 'app-123', flowType: EmbeddedFlowType.Registration};
    await executeEmbeddedSignUpFlow({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toMatchObject({applicationId: 'app-123', verbose: true});
  });

  it('should strip user-provided verbose before adding it', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-2',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    const payload = {applicationId: 'app-123', flowType: EmbeddedFlowType.Registration, verbose: false};
    await executeEmbeddedSignUpFlow({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toMatchObject({verbose: true});
  });

  it('should send payload without verbose for mid-flow step', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-3',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    const payload = {executionId: 'exec-3', action: 'submit', inputs: {email: 'user@example.com'}};
    await executeEmbeddedSignUpFlow({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toEqual(payload);
  });

  it('should throw ThunderIDAPIError when HTTP response is not ok', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      text: () => Promise.resolve('Bad payload'),
    });

    const baseUrl = 'https://localhost:8090';
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {executionId: 'exec-err'}})).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {executionId: 'exec-err'}})).rejects.toThrow(
      'Bad payload',
    );
  });

  it('should include custom headers when provided', async (): Promise<void> => {
    const mockResponse: EmbeddedSignUpFlowResponse = {
      data: {},
      executionId: 'exec-4',
      flowStatus: EmbeddedSignUpFlowStatus.Incomplete,
      type: EmbeddedSignUpFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://localhost:8090';
    const headers: Record<string, string> = {
      Authorization: 'Bearer token',
      'X-Custom-Header': 'custom',
    };

    await executeEmbeddedSignUpFlow({
      baseUrl,
      headers,
      payload: {executionId: 'exec-4'},
    });

    expect(fetch).toHaveBeenCalledWith(
      `${baseUrl}/flow/execute`,
      expect.objectContaining({
        headers: {
          Accept: 'application/json',
          Authorization: 'Bearer token',
          'Content-Type': 'application/json',
          'X-Custom-Header': 'custom',
        },
      }),
    );
  });
});
