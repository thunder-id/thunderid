// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient} from '@tanstack/react-query';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationQueryKeys from '../../constants/application-query-keys';
import useGetOpenIDConfiguration from '../useGetOpenIDConfiguration';

const {mockUseRuntimeUrl, mockFetch} = vi.hoisted(() => ({mockUseRuntimeUrl: vi.fn(), mockFetch: vi.fn()}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useRuntimeUrl: mockUseRuntimeUrl,
  };
});

const RUNTIME_URL = 'https://runtime.example.com';
const OPENID_CONFIGURATION = {
  grant_types_supported: ['authorization_code', 'refresh_token', 'client_credentials'],
  response_types_supported: ['code'],
  token_endpoint_auth_methods_supported: ['client_secret_basic', 'none'],
};

const okResponse = (body: unknown): Response =>
  ({ok: true, status: 200, json: () => Promise.resolve(body)}) as Response;

describe('useGetOpenIDConfiguration', () => {
  beforeEach(() => {
    mockUseRuntimeUrl.mockReturnValue(RUNTIME_URL);
    mockFetch.mockResolvedValue(okResponse(OPENID_CONFIGURATION));
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it('should fetch the OpenID configuration from the runtime URL without credentials', async () => {
    const {result} = renderHook(() => useGetOpenIDConfiguration());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(`${RUNTIME_URL}/.well-known/openid-configuration`);
    expect(result.current.data).toEqual(OPENID_CONFIGURATION);
  });

  it('should read from the runtime the applications call, which can differ from the sign-in server', async () => {
    mockUseRuntimeUrl.mockReturnValue('https://gateway.example.com');

    const {result} = renderHook(() => useGetOpenIDConfiguration());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledWith('https://gateway.example.com/.well-known/openid-configuration');
  });

  it('should fail with no data when the runtime returns a non-OK status', async () => {
    mockFetch.mockResolvedValue({ok: false, status: 404, json: () => Promise.resolve({})} as Response);

    const {result} = renderHook(() => useGetOpenIDConfiguration());

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.data).toBeUndefined();
  });

  it('should cache the document under the runtime URL', async () => {
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});

    const {result} = renderHook(() => useGetOpenIDConfiguration(), {queryClient});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData([ApplicationQueryKeys.OPENID_CONFIGURATION, {runtimeUrl: RUNTIME_URL}])).toEqual(
      OPENID_CONFIGURATION,
    );
  });
});
