// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AuthorizationDetailTypeQueryKeys from '../../constants/authorization-detail-type-query-keys';
import useDeleteAuthorizationDetailType from '../useDeleteAuthorizationDetailType';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockShowToast = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

describe('useDeleteAuthorizationDetailType', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('deletes the type', async () => {
    mockHttpRequest.mockResolvedValue({data: undefined});
    const {result} = renderHook(() => useDeleteAuthorizationDetailType());

    act(() => {
      result.current.mutate('type-1');
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/authorization-detail-types/type-1',
      method: 'DELETE',
    });
  });

  it('drops the deleted type, refreshes the list and confirms with a toast on success', async () => {
    mockHttpRequest.mockResolvedValue({data: undefined});
    const {result, queryClient} = renderHook(() => useDeleteAuthorizationDetailType());
    queryClient.setQueryData([AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, 'type-1'], {id: 'type-1'});
    const removeQueries = vi.spyOn(queryClient, 'removeQueries');
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    act(() => {
      result.current.mutate('type-1');
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(removeQueries).toHaveBeenCalledWith({
      queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, 'type-1'],
    });
    expect(queryClient.getQueryData([AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, 'type-1'])).toBe(
      undefined,
    );
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES],
    });
    expect(mockShowToast).toHaveBeenCalledWith('Intent deleted successfully.', 'success');
  });

  it('refreshes the list on failure, keeping the type and leaving the message to the caller', async () => {
    const error = new Error('Not found');
    mockHttpRequest.mockRejectedValue(error);
    const {result, queryClient} = renderHook(() => useDeleteAuthorizationDetailType());
    const removeQueries = vi.spyOn(queryClient, 'removeQueries');
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    act(() => {
      result.current.mutate('type-1');
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(error);
    expect(removeQueries).not.toHaveBeenCalled();
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES],
    });
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
