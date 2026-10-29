// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useSaveSharingPolicy from '../useSaveSharingPolicy';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockShowToast = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

const collection = 'https://localhost:8090/applications/app-1/sharing-policies';

describe('useSaveSharingPolicy', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
    mockHttpRequest.mockResolvedValue({data: {}});
  });

  it('creates a policy with POST', async () => {
    const {result} = renderHook(() => useSaveSharingPolicy('applications/app-1'));

    result.current.mutate({type: 'create', sections: {childUnits: {mode: 'all'}}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: collection, method: 'POST', data: {childUnits: {mode: 'all'}}}),
    );
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('replaces a policy with PUT carrying the version it read', async () => {
    const {result} = renderHook(() => useSaveSharingPolicy('applications/app-1'));

    result.current.mutate({type: 'update', policyId: 'p1', version: 3, sections: {allUnits: {}}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${collection}/p1`,
        method: 'PUT',
        data: {allUnits: {}, version: 3},
      }),
    );
  });

  it('deletes a policy with DELETE', async () => {
    const {result} = renderHook(() => useSaveSharingPolicy('applications/app-1'));

    result.current.mutate({type: 'delete', policyId: 'p1'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: `${collection}/p1`, method: 'DELETE'}));
  });

  it('replaces by deleting before creating', async () => {
    const {result} = renderHook(() => useSaveSharingPolicy('applications/app-1'));

    result.current.mutate({
      type: 'replace',
      policyId: 'p1',
      sections: {childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit'}]}},
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest.mock.calls.map(([request]) => (request as {method: string}).method)).toEqual([
      'DELETE',
      'POST',
    ]);
  });

  it('sends nothing for an empty plan', async () => {
    const {result} = renderHook(() => useSaveSharingPolicy('applications/app-1'));

    result.current.mutate({type: 'none'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it("words the success toast in the resource's own wording", async () => {
    mockHttpRequest.mockResolvedValue({data: {}});
    const {result} = renderHook(() => useSaveSharingPolicy('users/u-1', 'users'));

    result.current.mutate({type: 'create', sections: {childUnits: {mode: 'all'}}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockShowToast).toHaveBeenCalledWith('Affiliations updated.', 'success');
  });

  it('reports a failure without a success toast', async () => {
    mockHttpRequest.mockRejectedValue(new Error('refused'));
    const {result} = renderHook(() => useSaveSharingPolicy('applications/app-1'));

    result.current.mutate({type: 'create', sections: {childUnits: {mode: 'all'}}});

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toBe('refused');
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
