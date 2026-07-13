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

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useCreateConnection from '../useCreateConnection';

const mockHttpRequest = vi.fn();
const showToast = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
    useToast: () => ({showToast}),
  };
});
vi.mock('@thunderid/utils', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/utils')>();
  return {...actual, getErrorMessage: vi.fn().mockReturnValue('generic error')};
});

describe('useCreateConnection', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue({data: {id: 'c1', type: 'oidc', name: 'Acme'}});
    showToast.mockReset();
  });

  afterEach(() => vi.clearAllMocks());

  it('POSTs to /connections/{type} and toasts success', async () => {
    const {result} = renderHook(() => useCreateConnection('oidc'));
    result.current.mutate({
      name: 'Acme',
      clientId: 'x',
      redirectUri: 'https://r',
      clientSecret: 's',
      authorizationEndpoint: 'https://a',
      tokenEndpoint: 'https://t',
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/connections/oidc', method: 'POST'}),
    );
    expect(showToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('does NOT toast on a 409 conflict (handled inline by the caller)', async () => {
    mockHttpRequest.mockRejectedValue({response: {status: 409}});
    const {result} = renderHook(() => useCreateConnection('oidc'));
    result.current.mutate({name: 'dup', clientId: 'x', redirectUri: 'https://r'} as never);

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(showToast).not.toHaveBeenCalled();
  });

  it('toasts a generic error for non-conflict failures', async () => {
    mockHttpRequest.mockRejectedValue({response: {status: 500}});
    const {result} = renderHook(() => useCreateConnection('oidc'));
    result.current.mutate({name: 'x', clientId: 'x', redirectUri: 'https://r'} as never);

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(showToast).toHaveBeenCalledWith('generic error', 'error');
  });
});
