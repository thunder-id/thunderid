// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {CapabilitiesResponse} from '../useCapabilities';

const mockHttpRequest = vi.fn();
let mockPermissions: string[] = ['system:app'];
let mockOuId = 'ou-1';
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
  useGrantedPermissions: () => ({permissions: mockPermissions, isLoading: false}),
  useUser: () => ({flattenedProfile: {ouId: mockOuId}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  };
});

const {default: useCapabilities} = await import('../useCapabilities');

describe('useCapabilities', () => {
  const response: CapabilitiesResponse = {
    scope: {type: 'ou', ouId: 'ou-1'},
    resources: {app: {view: true, create: false, update: true, delete: false}},
    root: {view: false, manage: false},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockPermissions = ['system:app'];
    mockOuId = 'ou-1';
  });

  it('answers from GET /capabilities', async () => {
    mockHttpRequest.mockResolvedValue({data: response});
    const {result} = renderHook(() => useCapabilities());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/capabilities', method: 'GET'}),
    );
    expect(result.current.can('app')).toBe(true);
    expect(result.current.can('app', 'update')).toBe(true);
    expect(result.current.can('app', 'create')).toBe(false);
    expect(result.current.can('flow')).toBe(false);
    expect(result.current.canManageRoot).toBe(false);
  });

  it('allows nothing while loading', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => undefined));
    const {result} = renderHook(() => useCapabilities());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.can('app')).toBe(false);
    expect(result.current.canManageRoot).toBe(false);
  });

  it('leaves the decision to the server when the capabilities cannot be read', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));
    const {result} = renderHook(() => useCapabilities());

    await waitFor(() => {
      expect(result.current.can('app')).toBe(true);
    });
    expect(result.current.canManageRoot).toBe(true);
  });

  it('reads the capabilities again for a token with other scopes or another organization unit', async () => {
    mockHttpRequest.mockResolvedValue({data: response});
    const {result, rerender} = renderHook(() => useCapabilities());
    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    mockPermissions = ['system:app', 'system:flow:view'];
    rerender();
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    });

    mockOuId = 'ou-2';
    rerender();
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(3);
    });
  });
});
