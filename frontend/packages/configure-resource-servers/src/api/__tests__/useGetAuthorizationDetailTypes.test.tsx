// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {AuthorizationDetailTypeListResponse} from '../../models/authorization-detail-type';
import useGetAuthorizationDetailTypes from '../useGetAuthorizationDetailTypes';

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

describe('useGetAuthorizationDetailTypes', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('fetches the first page of the types', async () => {
    const data: AuthorizationDetailTypeListResponse = {
      totalResults: 1,
      startIndex: 0,
      count: 1,
      types: [
        {
          id: 'type-1',
          type: 'payment_initiation',
          name: 'Payment initiation',
          version: 1,
          schema: {type: 'object', properties: {}},
          reuse: true,
        },
      ],
      links: [],
    };
    mockHttpRequest.mockResolvedValue({data});

    const {result} = renderHook(() => useGetAuthorizationDetailTypes());

    await waitFor(() => expect(result.current.data).toEqual(data));
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/authorization-detail-types?limit=100&offset=0',
      method: 'GET',
    });
  });
});
