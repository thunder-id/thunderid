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
import {
  EmbeddedSignInFlowResponse,
  EmbeddedSignInFlowStatus,
  EmbeddedSignInFlowType,
} from '../../models/embedded-signin-flow';
import executeEmbeddedSignInFlow from '../executeEmbeddedSignInFlow';

describe('executeEmbeddedSignInFlow', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should execute successfully with explicit url', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-123',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const url = 'https://localhost:8090/flow/execute';
    const payload = {executionId: 'exec-123', action: 'submit', inputs: {username: 'user@example.com'}};

    const result: EmbeddedSignInFlowResponse = await executeEmbeddedSignInFlow({payload, url});

    expect(fetch).toHaveBeenCalledWith(url, {
      body: JSON.stringify(payload),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'POST',
    });
    expect(result).toEqual(mockResponse);
  });

  it('should fall back to baseUrl when url is not provided', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-456',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://localhost:8090';
    const payload = {executionId: 'exec-456', action: 'submit', inputs: {username: 'user'}};

    const result: EmbeddedSignInFlowResponse = await executeEmbeddedSignInFlow({baseUrl, payload});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/flow/execute`, {
      body: JSON.stringify(payload),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'POST',
    });
    expect(result).toEqual(mockResponse);
  });

  it('should prefer url over baseUrl when both are provided', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-789',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const url = 'https://localhost:8090/flow/execute';
    const baseUrl = 'https://localhost:8090';
    await executeEmbeddedSignInFlow({baseUrl, payload: {executionId: 'exec-789'}, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.any(Object));
  });

  it('should respect method override from requestConfig', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-1',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    await executeEmbeddedSignInFlow({baseUrl, method: 'PUT' as any, payload: {executionId: 'exec-1'}});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/flow/execute`, expect.objectContaining({method: 'PUT'}));
  });

  it('should throw ThunderIDAPIError when payload is missing', async (): Promise<void> => {
    const baseUrl = 'https://localhost:8090';

    await expect(executeEmbeddedSignInFlow({baseUrl} as any)).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignInFlow({baseUrl} as any)).rejects.toThrow('Authorization payload is required');
  });

  it('should add verbose=true for new flow start (applicationId + flowType in payload)', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-new',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    const payload = {applicationId: 'app-123', flowType: 'AUTHENTICATION'};
    await executeEmbeddedSignInFlow({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toMatchObject({applicationId: 'app-123', flowType: 'AUTHENTICATION', verbose: true});
  });

  it('should strip user-provided verbose before adding it', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-2',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    const payload = {applicationId: 'app-123', flowType: 'AUTHENTICATION', verbose: false};
    await executeEmbeddedSignInFlow({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toMatchObject({verbose: true});
  });

  it('should add verbose=true when payload contains only executionId', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-3',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };
    global.fetch = vi.fn().mockResolvedValue({json: () => Promise.resolve(mockResponse), ok: true});

    const baseUrl = 'https://localhost:8090';
    await executeEmbeddedSignInFlow({baseUrl, payload: {executionId: 'exec-3'}});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toEqual({executionId: 'exec-3', verbose: true});
  });

  it('should handle HTTP error responses', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      text: () => Promise.resolve('Invalid credentials'),
    });

    const payload = {executionId: 'exec-4', inputs: {password: 'wrong'}};
    const baseUrl = 'https://localhost:8090';

    await expect(executeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow('Invalid credentials');
  });

  it('should extract error message from structured error response body', async (): Promise<void> => {
    const structuredError: string = JSON.stringify({
      code: 'SSE-5000',
      description: {
        defaultValue: 'An unexpected error occurred while processing the request',
        key: 'error.internal_server_error_description',
      },
      message: {defaultValue: 'Internal server error', key: 'error.internal_server_error'},
    });

    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      text: () => Promise.resolve(structuredError),
    });

    const payload = {executionId: 'exec-5', inputs: {password: 'pass'}};
    const baseUrl = 'https://localhost:8090';

    await expect(executeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(
      'An unexpected error occurred while processing the request',
    );
  });

  it('should include custom headers when provided', async (): Promise<void> => {
    const mockResponse: EmbeddedSignInFlowResponse = {
      data: {},
      executionId: 'exec-6',
      flowStatus: EmbeddedSignInFlowStatus.Incomplete,
      type: EmbeddedSignInFlowType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const payload = {executionId: 'exec-6', inputs: {username: 'user'}};
    const baseUrl = 'https://localhost:8090';
    const customHeaders: Record<string, string> = {
      Authorization: 'Bearer token',
      'X-Custom-Header': 'custom-value',
    };

    await executeEmbeddedSignInFlow({
      baseUrl,
      headers: customHeaders,
      payload,
    });

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/flow/execute`, {
      body: JSON.stringify(payload),
      headers: {
        Accept: 'application/json',
        Authorization: 'Bearer token',
        'Content-Type': 'application/json',
        'X-Custom-Header': 'custom-value',
      },
      method: 'POST',
    });
  });
});
