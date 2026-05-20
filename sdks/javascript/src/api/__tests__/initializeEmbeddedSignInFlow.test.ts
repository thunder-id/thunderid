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

import {describe, it, expect, vi, beforeEach, Mock} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import type {EmbeddedSignInFlowInitiateResponse} from '../../models/embedded-signin-flow';
import initializeEmbeddedSignInFlow from '../initializeEmbeddedSignInFlow';

describe('initializeEmbeddedSignInFlow', (): void => {
  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should execute successfully with explicit url (default fetch)', async (): Promise<void> => {
    const mockResp: EmbeddedSignInFlowInitiateResponse = {
      flowId: 'fid-123',
      flowStatus: 'PENDING',
    } as any;

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResp),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/oauth2/authorize';
    const payload: Record<string, string> = {
      client_id: 'cid',
      code_challenge: 'abc',
      code_challenge_method: 'S256',
      redirect_uri: 'https://app/cb',
      response_type: 'code',
      scope: 'openid profile',
      state: 'xyz',
    };

    const result: EmbeddedSignInFlowInitiateResponse = await initializeEmbeddedSignInFlow({payload, url});

    expect(fetch).toHaveBeenCalledWith(url, {
      body: new URLSearchParams(payload).toString(),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      method: 'POST',
    });
    expect(result).toEqual(mockResp);
  });

  it('should fall back to baseUrl when url is not provided', async (): Promise<void> => {
    const mockResp: EmbeddedSignInFlowInitiateResponse = {
      flowId: 'fid-456',
      flowStatus: 'PENDING',
    } as any;

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResp),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};

    const result: EmbeddedSignInFlowInitiateResponse = await initializeEmbeddedSignInFlow({baseUrl, payload});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/oauth2/authorize`, {
      body: new URLSearchParams(payload).toString(),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      method: 'POST',
    });
    expect(result).toEqual(mockResp);
  });

  it('should use custom method from requestConfig when provided', async (): Promise<void> => {
    const mockResp: EmbeddedSignInFlowInitiateResponse = {
      flowId: 'fid-789',
      flowStatus: 'PENDING',
    } as any;

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResp),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};

    await initializeEmbeddedSignInFlow({baseUrl, method: 'PUT' as any, payload});

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/oauth2/authorize`, expect.objectContaining({method: 'PUT'}));
  });

  it('should prefer url over baseUrl when both are provided', async (): Promise<void> => {
    const mockResp: EmbeddedSignInFlowInitiateResponse = {
      flowId: 'fid-000',
      flowStatus: 'PENDING',
    } as any;

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResp),
      ok: true,
    });

    const url = 'https://api.asgardeo.io/t/demo/oauth2/authorize';
    const baseUrl = 'https://api.asgardeo.io/t/ignored';
    await initializeEmbeddedSignInFlow({baseUrl, payload: {response_type: 'code'}, url});

    expect(fetch).toHaveBeenCalledWith(url, expect.any(Object));
  });

  it('should throw ThunderIDAPIError for invalid URL/baseUrl', async (): Promise<void> => {
    await expect(initializeEmbeddedSignInFlow({payload: {a: 1} as any, url: 'invalid-url' as any})).rejects.toThrow(
      ThunderIDAPIError,
    );
    await expect(initializeEmbeddedSignInFlow({payload: {a: 1} as any, url: 'invalid-url' as any})).rejects.toThrow(
      'Invalid URL provided.',
    );
  });

  it('should throw ThunderIDAPIError when payload is missing', async (): Promise<void> => {
    const baseUrl = 'https://api.asgardeo.io/t/demo';
    await expect(initializeEmbeddedSignInFlow({baseUrl} as any)).rejects.toThrow(ThunderIDAPIError);
    await expect(initializeEmbeddedSignInFlow({baseUrl} as any)).rejects.toThrow('Authorization payload is required');
  });

  it('should handle HTTP error responses with plain-text body', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      text: () => Promise.resolve('invalid request'),
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};

    await expect(initializeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(ThunderIDAPIError);
    await expect(initializeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow('invalid request');
  });

  it('should extract description.defaultValue from a structured error response body', async (): Promise<void> => {
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

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};

    await expect(initializeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(
      'An unexpected error occurred while processing the request',
    );
  });

  it('should handle network or parsing errors', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network down'));

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};

    await expect(initializeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(ThunderIDAPIError);
    await expect(initializeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(
      'Network or parsing error: Network down',
    );
  });

  it('should handle non-Error rejections', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue('weird failure');

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};

    await expect(initializeEmbeddedSignInFlow({baseUrl, payload})).rejects.toThrow(
      'Network or parsing error: Unknown error',
    );
  });

  it('should pass through custom headers (and enforces content-type & accept)', async (): Promise<void> => {
    const mockResp: EmbeddedSignInFlowInitiateResponse = {
      flowId: 'fid-headers',
      flowStatus: 'PENDING',
    } as any;

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResp),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {client_id: 'cid', response_type: 'code'};
    const customHeaders: Record<string, string> = {
      Accept: 'text/plain',
      Authorization: 'Bearer token',
      'Content-Type': 'text/plain',
      'X-Custom-Header': 'custom-value',
    };

    await initializeEmbeddedSignInFlow({
      baseUrl,
      headers: customHeaders,
      payload,
    });

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/oauth2/authorize`, {
      body: new URLSearchParams(payload).toString(),
      headers: {
        Accept: 'application/json',
        Authorization: 'Bearer token',
        'Content-Type': 'application/x-www-form-urlencoded',
        'X-Custom-Header': 'custom-value',
      },
      method: 'POST',
    });
  });

  it('should encode payload as application/x-www-form-urlencoded', async (): Promise<void> => {
    const mockResp: EmbeddedSignInFlowInitiateResponse = {
      flowId: 'fid-enc',
      flowStatus: 'PENDING',
    } as any;

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockResp),
      ok: true,
    });

    const baseUrl = 'https://api.asgardeo.io/t/demo';
    const payload: Record<string, string> = {
      client_id: 'cid',
      redirect_uri: 'https://app.example.com/cb?x=1&y=2',
      response_type: 'code',
      scope: 'openid profile email',
      state: 'chars !@#$&=+,:;/?',
    };

    await initializeEmbeddedSignInFlow({baseUrl, payload});

    const [, init] = (fetch as unknown as Mock).mock.calls[0];
    expect(init.headers['Content-Type']).toBe('application/x-www-form-urlencoded');
    // ensure characters are url-encoded in body
    expect(init.body).toContain('scope=openid+profile+email');
    expect(init.body).toContain('redirect_uri=https%3A%2F%2Fapp.example.com%2Fcb%3Fx%3D1%26y%3D2');
    expect(init.body).toContain('state=chars+%21%40%23%24%26%3D%2B%2C%3A%3B%2F%3F');
  });
});
