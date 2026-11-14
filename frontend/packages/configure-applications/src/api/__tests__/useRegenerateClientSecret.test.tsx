// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import ApplicationQueryKeys from '../../constants/application-query-keys';
import useRegenerateClientSecret from '../useRegenerateClientSecret';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn(),
    useAdministrationOperation: () => administrationOperation,
  };
});

// How the console says this operation runs. Every mode but a supplied function calls the API.
let administrationOperation: unknown = AdministrationModes.FLOW;

const {useThunderID} = await import('@thunderid/react');
const {useConfig, useToast} = await import('@thunderid/contexts');

const NEW_SECRET = 'KdJL7PrfDTh-XwT8kGtg1nMydcDQg_L5an25vV52Ej8';

/**
 * The request shape the hook issues, as far as these tests inspect it.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * The call signature of the mocked client, so an implementation returning a promise type-checks.
 */
type HttpRequest = (config: unknown) => Promise<{data?: unknown}>;

type HttpRequestMock = ReturnType<typeof vi.fn<HttpRequest>>;

describe('useRegenerateClientSecret', () => {
  let mockHttpRequest: HttpRequestMock;
  let mockShowToast: ReturnType<typeof vi.fn>;

  const applicationId = '550e8400-e29b-41d4-a716-446655440000';
  const regenerateUrl = `https://api.test.com/applications/${applicationId}/client-secret/regenerate`;

  beforeEach(() => {
    mockHttpRequest = vi.fn<HttpRequest>();
    mockShowToast = vi.fn();
    administrationOperation = AdministrationModes.FLOW;

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: vi.fn().mockReturnValue('https://api.test.com'),
    } as unknown as ReturnType<typeof useConfig>);

    vi.mocked(useToast).mockReturnValue({
      showToast: mockShowToast,
    } as unknown as ReturnType<typeof useToast>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  /**
   * The requests the hook issued.
   */
  function requests(): RecordedRequest[] {
    return mockHttpRequest.mock.calls.map(([config]: [unknown]) => config as RecordedRequest);
  }

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useRegenerateClientSecret());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  // The server generates the secret and revokes the tokens issued under the old one, so the returned
  // value is the server's rather than one the browser invented, and no flow is looked up.
  it('should regenerate through the API and return the secret the server generated', async () => {
    mockHttpRequest.mockResolvedValue({data: {clientSecret: NEW_SECRET}});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual({clientSecret: NEW_SECRET, revocationIncomplete: false});
    expect(requests()).toHaveLength(1);
    expect(requests()[0]).toMatchObject({method: 'POST', url: regenerateUrl});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  // A console that holds configuration only calls the same API: the server writes the new secret and
  // carries it to the gateways with the next configuration version.
  it('should warn when the server reports the revocation incomplete', async () => {
    mockHttpRequest.mockResolvedValue({data: {clientSecret: NEW_SECRET, revocationIncomplete: true}});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({clientSecret: NEW_SECRET, revocationIncomplete: true});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'warning');
    expect(mockShowToast).not.toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should regenerate through the API when the console says native', async () => {
    administrationOperation = AdministrationModes.NATIVE;
    mockHttpRequest.mockResolvedValue({data: {clientSecret: NEW_SECRET}});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.clientSecret).toBe(NEW_SECRET);
    expect(requests()).toHaveLength(1);
    expect(requests()[0]).toMatchObject({method: 'POST', url: regenerateUrl});
  });

  it('should fail when the server returns no secret', async () => {
    mockHttpRequest.mockResolvedValue({data: {}});

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toContain('without returning a secret');
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should surface a refused regeneration and show no toast', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Application has no client secret'));

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('Application has no client secret');
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should invalidate queries on successful regeneration', async () => {
    mockHttpRequest.mockResolvedValue({data: {clientSecret: NEW_SECRET}});

    const {result, queryClient} = renderHook(() => useRegenerateClientSecret());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [ApplicationQueryKeys.APPLICATION, applicationId],
      }),
    );
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [ApplicationQueryKeys.APPLICATIONS],
      }),
    );
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValue({data: {clientSecret: NEW_SECRET}});

    const {result, queryClient} = renderHook(() => useRegenerateClientSecret());

    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValue(new Error('Invalidation failed'));

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.clientSecret).toBe(NEW_SECRET);
  });

  // A distributor can replace one operation without forking this package.
  it('calls the function the console supplied', async () => {
    const rotated: string[] = [];
    administrationOperation = (id: string): Promise<{clientSecret: string}> => {
      rotated.push(id);
      return Promise.resolve({clientSecret: 'supplied-secret'});
    };

    const {result} = renderHook(() => useRegenerateClientSecret());

    result.current.mutate({applicationId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(rotated).toEqual([applicationId]);
    expect(result.current.data?.clientSecret).toBe('supplied-secret');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
