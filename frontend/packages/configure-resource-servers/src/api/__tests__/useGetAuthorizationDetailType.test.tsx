// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {AuthorizationDetailType} from '../../models/authorization-detail-type';
import useGetAuthorizationDetailType from '../useGetAuthorizationDetailType';
import useUpdateAuthorizationDetailType from '../useUpdateAuthorizationDetailType';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  };
});

describe('useGetAuthorizationDetailType', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('fetches the type by its id', async () => {
    const data: AuthorizationDetailType = {
      id: 'type-1',
      type: 'payment_initiation',
      name: 'Payment initiation',
      version: 1,
      schema: {type: 'object', properties: {}},
      reuse: false,
    };
    mockHttpRequest.mockResolvedValue({data});

    const {result} = renderHook(() => useGetAuthorizationDetailType('type-1'));

    await waitFor(() => expect(result.current.data).toEqual(data));
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/authorization-detail-types/type-1',
      method: 'GET',
    });
  });

  it('keeps the order of the fields a save or a refresh returns, even when only the order changed', async () => {
    const type = (properties: Record<string, unknown>): AuthorizationDetailType => ({
      id: 'type-1',
      type: 'payment_initiation',
      name: 'Payment initiation',
      version: 1,
      schema: {type: 'object', properties},
      reuse: false,
    });
    const before = type({amount: {type: 'number'}, payee: {type: 'string'}});
    const after = type({payee: {type: 'string'}, amount: {type: 'number'}});
    mockHttpRequest.mockResolvedValue({data: before});

    const {result} = renderHook(() => ({
      query: useGetAuthorizationDetailType('type-1'),
      update: useUpdateAuthorizationDetailType(),
    }));
    const order = (): string[] =>
      Object.keys((result.current.query.data?.schema as {properties: Record<string, unknown>}).properties);
    await waitFor(() => expect(result.current.query.data).toBeDefined());
    expect(order()).toEqual(['amount', 'payee']);

    mockHttpRequest.mockResolvedValue({data: after});
    await act(async () => {
      await result.current.update.mutateAsync({
        id: 'type-1',
        data: {type: 'payment_initiation', name: 'Payment initiation', schema: after.schema, reuse: false},
      });
    });
    expect(order(), 'the saved type as the server returned it').toEqual(['payee', 'amount']);
    expect(mockHttpRequest, 'the saved type is not fetched again').toHaveBeenCalledTimes(2);

    mockHttpRequest.mockResolvedValue({data: before});
    await act(async () => {
      await result.current.query.refetch();
    });
    expect(order()).toEqual(['amount', 'payee']);
  });

  it('does not fetch without an id', () => {
    const {result} = renderHook(() => useGetAuthorizationDetailType(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
