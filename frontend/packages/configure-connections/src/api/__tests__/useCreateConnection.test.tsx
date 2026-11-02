// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

describe('useCreateConnection', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue({data: {id: 'c1', type: 'oidc', name: 'Acme'}});
    showToast.mockReset();
  });

  afterEach(() => vi.clearAllMocks());

  it('POSTs to /connections/{type} and toasts success', async () => {
    const {result} = renderHook(() => useCreateConnection('oidc'));
    result.current.mutate({
      ouId: 'ou-1',
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
    expect(mockHttpRequest.mock.calls[0]?.[0]).toMatchObject({data: {ouId: 'ou-1', name: 'Acme'}});
    expect(showToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('does NOT toast on a 409 conflict (handled inline by the caller)', async () => {
    mockHttpRequest.mockRejectedValue({response: {status: 409}});
    const {result} = renderHook(() => useCreateConnection('oidc'));
    result.current.mutate({name: 'dup', clientId: 'x', redirectUri: 'https://r'} as never);

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(showToast).not.toHaveBeenCalled();
  });

  it('does NOT toast on a non-conflict failure (handled inline by the caller)', async () => {
    mockHttpRequest.mockRejectedValue({response: {status: 500}});
    const {result} = renderHook(() => useCreateConnection('oidc'));
    result.current.mutate({name: 'x', clientId: 'x', redirectUri: 'https://r'} as never);

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(showToast).not.toHaveBeenCalled();
  });
});
