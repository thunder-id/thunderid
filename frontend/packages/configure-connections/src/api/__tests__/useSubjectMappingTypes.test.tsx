// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useSubjectMappingTypes from '../useSubjectMappingTypes';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

describe('useSubjectMappingTypes', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  it('loads every user-type and agent-type page', async () => {
    mockHttpRequest.mockImplementation((request: {url: string}) => {
      const url = new URL(request.url);
      const offset = url.searchParams.get('offset');
      const path = url.pathname;
      if (path === '/user-types' && offset === '0') {
        return Promise.resolve({data: {totalResults: 101, types: [{id: 'user-1', name: 'employee'}]}});
      }
      if (path === '/user-types' && offset === '100') {
        return Promise.resolve({data: {totalResults: 101, types: [{id: 'user-2', name: 'customer'}]}});
      }
      if (path === '/agent-types' && offset === '0') {
        return Promise.resolve({data: {totalResults: 101, types: [{id: 'agent-1', name: 'assistant'}]}});
      }
      if (path === '/agent-types' && offset === '100') {
        return Promise.resolve({data: {totalResults: 101, types: [{id: 'agent-2', name: 'default'}]}});
      }
      throw new Error(`Unexpected subject type request: ${request.url}`);
    });

    const {result} = renderHook(() => useSubjectMappingTypes());

    await waitFor(() => {
      expect(result.current.data).toEqual({
        userTypes: [
          {id: 'user-1', name: 'employee', category: 'user'},
          {id: 'user-2', name: 'customer', category: 'user'},
        ],
        agentTypes: [
          {id: 'agent-1', name: 'assistant', category: 'agent'},
          {id: 'agent-2', name: 'default', category: 'agent'},
        ],
      });
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(4);
  });
});
