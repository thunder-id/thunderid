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

import {Mock, beforeEach, describe, expect, it, vi} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import {
  EmbeddedFlowExecuteResponse,
  EmbeddedFlowResponseType,
  EmbeddedFlowStatus,
  EmbeddedFlowType,
} from '../../models/embedded-flow';
import executeEmbeddedSignUpFlow from '../executeEmbeddedSignUpFlow';

describe('executeEmbeddedSignUpFlow', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should execute successfully with explicit url', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/api/server/v1/flow/execute';
    const payload: Record<string, string> = {foo: 'bar'};

    const result: EmbeddedFlowExecuteResponse = await executeEmbeddedSignUpFlow({payload, url});

    expect(fetch).toHaveBeenCalledWith(
      url,
      expect.objectContaining({
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        method: 'POST',
      }),
    );

    const callArgs: [string, RequestInit] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const parsedBody: Record<string, unknown> = JSON.parse(callArgs[1].body as string);
    expect(parsedBody).toEqual({
      flowType: EmbeddedFlowType.Registration,
      foo: 'bar',
    });
    expect(result).toEqual(mockResponse);
  });

  it('should fall back to baseUrl when url is not provided', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, number> = {a: 1};

    const result: EmbeddedFlowExecuteResponse = await executeEmbeddedSignUpFlow({baseUrl, payload});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/api/server/v1/flow/execute`, {
      body: JSON.stringify({
        a: 1,
        flowType: EmbeddedFlowType.Registration,
      }),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'POST',
    });
    expect(result).toEqual(mockResponse);
  });

  it('should prefer url over baseUrl when both are provided', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/api/server/v1/flow/execute';
    const baseUrl = 'https://api.asgardeo.io/t/ignored';

    await executeEmbeddedSignUpFlow({baseUrl, payload: {x: 1}, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.any(Object));
  });

  it('should respect method override from requestConfig', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';

    await executeEmbeddedSignUpFlow({
      baseUrl,
      method: 'PUT' as any,
      payload: {y: 1},
    });

    expect(fetch).toHaveBeenCalledWith(
      `${baseUrl}/api/server/v1/flow/execute`,
      expect.objectContaining({method: 'PUT'}),
    );
  });

  it('should enforce flowType=Registration even if provided differently', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, unknown> = {flowType: 'SOMETHING_ELSE', p: 1} as any;

    await executeEmbeddedSignUpFlow({baseUrl, payload});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toEqual({
      flowType: EmbeddedFlowType.Registration,
      p: 1,
    });
  });

  it('should send only flowType when payload is omitted', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await executeEmbeddedSignUpFlow({baseUrl});

    const [, init]: [string, RequestInit] = (fetch as unknown as Mock).mock.calls[0];
    expect(JSON.parse(init.body as string)).toEqual({
      flowType: EmbeddedFlowType.Registration,
    });
  });

  it('should throw ThunderIDAPIError when both url and baseUrl are missing', async (): Promise<void> => {
    await expect(executeEmbeddedSignUpFlow({payload: {a: 1}} as any)).rejects.toThrow(ThunderIDAPIError);

    await expect(executeEmbeddedSignUpFlow({payload: {a: 1}} as any)).rejects.toThrow(
      'Base URL or URL is not provided',
    );
  });

  it('should throw ThunderIDAPIError for invalid URL', async (): Promise<void> => {
    await expect(executeEmbeddedSignUpFlow({url: 'invalid-url' as any})).rejects.toThrow(ThunderIDAPIError);

    await expect(executeEmbeddedSignUpFlow({url: 'invalid-url' as any})).rejects.toThrow('Invalid URL provided.');
  });

  it('should throw ThunderIDAPIError for undefined URL and baseUrl', async (): Promise<void> => {
    await expect(
      executeEmbeddedSignUpFlow({baseUrl: undefined, payload: {a: 1}, url: undefined} as any),
    ).rejects.toThrow(ThunderIDAPIError);
    await expect(
      executeEmbeddedSignUpFlow({baseUrl: undefined, payload: {a: 1}, url: undefined} as any),
    ).rejects.toThrow('Base URL or URL is not provided');
  });

  it('should throw ThunderIDAPIError for empty string URL and baseUrl', async (): Promise<void> => {
    await expect(executeEmbeddedSignUpFlow({baseUrl: '', payload: {a: 1}, url: ''})).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignUpFlow({baseUrl: '', payload: {a: 1}, url: ''})).rejects.toThrow(
      'Base URL or URL is not provided',
    );
  });

  it('should handle HTTP error responses', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      text: () => Promise.resolve('Bad payload'),
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {a: 1}})).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {a: 1}})).rejects.toThrow(
      'Embedded SignUp flow execution failed: Bad payload',
    );
  });

  it('should handle network or parsing errors', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {a: 1}})).rejects.toThrow(ThunderIDAPIError);
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {a: 1}})).rejects.toThrow(
      'Network or parsing error: Network error',
    );
  });

  it('should handle non-Error rejections', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue('boom');

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await expect(executeEmbeddedSignUpFlow({baseUrl, payload: {a: 1}})).rejects.toThrow(
      'Network or parsing error: Unknown error',
    );
  });

  it('should include custom headers when provided', async (): Promise<void> => {
    const mockResponse: EmbeddedFlowExecuteResponse = {
      data: {},
      flowId: 'flow-123',
      flowStatus: EmbeddedFlowStatus.Complete,
      type: EmbeddedFlowResponseType.View,
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResponse),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const headers: Record<string, string> = {
      Authorization: 'Bearer token',
      'X-Custom-Header': 'custom',
    };

    await executeEmbeddedSignUpFlow({
      baseUrl,
      headers,
      payload: {a: 1},
    });

    expect(fetch).toHaveBeenCalledWith(
      `${baseUrl}/api/server/v1/flow/execute`,
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
