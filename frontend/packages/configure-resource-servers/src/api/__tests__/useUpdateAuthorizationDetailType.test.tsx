// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import AuthorizationDetailTypeQueryKeys from '../../constants/authorization-detail-type-query-keys';
import type {AuthorizationDetailType, AuthorizationDetailTypeRequest} from '../../models/authorization-detail-type';
import useUpdateAuthorizationDetailType from '../useUpdateAuthorizationDetailType';

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

const data: AuthorizationDetailTypeRequest = {
  type: 'payment_initiation',
  name: 'Payment initiation',
  schema: {type: 'object', properties: {amount: {type: 'number'}}},
  reuse: true,
};

const saved: AuthorizationDetailType = {id: 'type-1', version: 2, ...data};

describe('useUpdateAuthorizationDetailType', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
  });

  it('saves the type and returns it', async () => {
    mockHttpRequest.mockResolvedValue({data: saved});
    const {result} = renderHook(() => useUpdateAuthorizationDetailType());

    act(() => {
      result.current.mutate({id: 'type-1', data});
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(saved);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/authorization-detail-types/type-1',
      method: 'PUT',
      data,
    });
  });

  it('stores the saved type, refreshes the list and confirms with a toast on success', async () => {
    mockHttpRequest.mockResolvedValue({data: saved});
    const {result, queryClient} = renderHook(() => useUpdateAuthorizationDetailType());
    const invalidateQueries = vi.spyOn(queryClient, 'invalidateQueries');

    act(() => {
      result.current.mutate({id: 'type-1', data});
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(queryClient.getQueryData([AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, 'type-1'])).toEqual(
      saved,
    );
    expect(invalidateQueries).toHaveBeenCalledTimes(1);
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES],
    });
    expect(mockShowToast).toHaveBeenCalledWith('Intent updated successfully.', 'success');
  });

  it('leaves a failure to the caller, without a toast or a change to the stored type', async () => {
    const error = new Error('Bad request');
    mockHttpRequest.mockRejectedValue(error);
    const {result, queryClient} = renderHook(() => useUpdateAuthorizationDetailType());
    const setQueryData = vi.spyOn(queryClient, 'setQueryData');

    act(() => {
      result.current.mutate({id: 'type-1', data});
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBe(error);
    expect(setQueryData).not.toHaveBeenCalled();
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
