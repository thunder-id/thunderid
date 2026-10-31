// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient} from '@tanstack/react-query';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionQueryKeys from '../../constants/query-keys';
import useAuthorizationServerMetadata from '../useAuthorizationServerMetadata';

const {mockUseRuntimeUrl, mockFetch} = vi.hoisted(() => ({mockUseRuntimeUrl: vi.fn(), mockFetch: vi.fn()}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useRuntimeUrl: mockUseRuntimeUrl,
  };
});

const RUNTIME_URL = 'https://runtime.example.com';
const METADATA = {
  grant_types_supported: ['authorization_code', 'urn:ietf:params:oauth:grant-type:token-exchange'],
  authorization_grant_profiles_supported: ['urn:ietf:params:oauth:grant-profile:id-jag'],
};

const okResponse = (body: unknown): Response =>
  ({ok: true, status: 200, json: () => Promise.resolve(body)}) as Response;

describe('useAuthorizationServerMetadata', () => {
  beforeEach(() => {
    mockUseRuntimeUrl.mockReturnValue(RUNTIME_URL);
    mockFetch.mockResolvedValue(okResponse(METADATA));
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it('should fetch the authorization server metadata from the runtime URL without credentials', async () => {
    const {result} = renderHook(() => useAuthorizationServerMetadata());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(`${RUNTIME_URL}/.well-known/oauth-authorization-server`);
    expect(result.current.data).toEqual(METADATA);
  });

  it('should read from the runtime the applications call, which can differ from the sign-in server', async () => {
    mockUseRuntimeUrl.mockReturnValue('https://gateway.example.com');

    const {result} = renderHook(() => useAuthorizationServerMetadata());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockFetch).toHaveBeenCalledWith('https://gateway.example.com/.well-known/oauth-authorization-server');
  });

  it('should have no data while the request is in flight', () => {
    mockFetch.mockReturnValue(new Promise(() => undefined));

    const {result} = renderHook(() => useAuthorizationServerMetadata());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it('should fail with no data when the runtime returns a non-OK status', async () => {
    mockFetch.mockResolvedValue({ok: false, status: 404, json: () => Promise.resolve({})} as Response);

    const {result} = renderHook(() => useAuthorizationServerMetadata());

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.data).toBeUndefined();
  });

  it('should fail with no data when the request cannot reach the runtime', async () => {
    mockFetch.mockRejectedValue(new TypeError('Failed to fetch'));

    const {result} = renderHook(() => useAuthorizationServerMetadata());

    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(result.current.data).toBeUndefined();
  });

  it('should cache the metadata under the runtime URL', async () => {
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});

    const {result} = renderHook(() => useAuthorizationServerMetadata(), {queryClient});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(
      queryClient.getQueryData([ConnectionQueryKeys.AUTHORIZATION_SERVER_METADATA, {runtimeUrl: RUNTIME_URL}]),
    ).toEqual(METADATA);
  });

  it('should serve a second consumer from the cache without fetching again', async () => {
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false, staleTime: Infinity}}});

    const first = renderHook(() => useAuthorizationServerMetadata(), {queryClient});
    await waitFor(() => expect(first.result.current.isSuccess).toBe(true));

    const second = renderHook(() => useAuthorizationServerMetadata(), {queryClient});

    expect(second.result.current.data).toEqual(METADATA);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});
