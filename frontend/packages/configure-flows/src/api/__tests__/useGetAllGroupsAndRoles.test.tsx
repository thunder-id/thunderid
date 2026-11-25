// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseQueryResult} from '@tanstack/react-query';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useGetAllGroups from '../useGetAllGroups';
import useGetAllRoles from '../useGetAllRoles';

const mockHttpRequest = vi.fn();
const mockProjectScope = vi.fn<() => {projectId?: string}>(() => ({}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getServerUrl: () => 'https://api.example.com'}),
  useProjectScope: () => mockProjectScope(),
}));

vi.mock('@thunderid/react', () => ({
  GroupQueryKeys: {GROUPS: 'groups'},
  RoleQueryKeys: {ROLES: 'roles'},
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

interface Item {
  id: string;
  name: string;
  ouId: string;
}

/** Builds an item named after its index. */
const item = (prefix: string, index: number): Item => ({
  id: `${prefix}-${index}`,
  name: `${prefix} ${index}`,
  ouId: 'ou-1',
});

describe.each([
  {kind: 'group', path: 'groups', prefix: 'group', useHook: useGetAllGroups},
  {kind: 'role', path: 'roles', prefix: 'role', useHook: useGetAllRoles},
])(
  'useGetAll $path',
  ({
    kind,
    path,
    prefix,
    useHook,
  }: {
    kind: string;
    path: string;
    prefix: string;
    useHook: () => UseQueryResult<Item[]>;
  }) => {
    const url = (query: string): string => `https://api.example.com/${path}?${query}`;

    /** Builds the response of one page of the list, reporting totalResults items in all. */
    const page = (items: Item[], totalResults: number): {data: Record<string, unknown>} => ({
      data: {[path]: items, totalResults, count: items.length, startIndex: 1},
    });

    /** Builds a full page of items starting at the given index. */
    const fullPage = (from: number, size = 100): Item[] => Array.from({length: size}, (_, i) => item(prefix, from + i));

    beforeEach(() => {
      vi.clearAllMocks();
      mockProjectScope.mockReturnValue({});
    });

    it(`should return every ${kind} when they fit in one page`, async () => {
      const items = [item(prefix, 0), item(prefix, 1)];
      mockHttpRequest.mockResolvedValueOnce(page(items, 2));

      const {result} = renderHook(() => useHook());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
      expect(mockHttpRequest).toHaveBeenCalledWith({url: url('limit=100&offset=0&include=display'), method: 'GET'});
      expect(result.current.data).toEqual(items);
    });

    it(`should list only the ${path} of the project the console works in`, async () => {
      mockProjectScope.mockReturnValue({projectId: 'project-1'});
      mockHttpRequest.mockResolvedValueOnce(page([item(prefix, 0)], 1));

      const {result} = renderHook(() => useHook());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({url: url('limit=100&offset=0&include=display&projectId=project-1')}),
      );
    });

    it(`should page through with offset until every ${kind} is loaded`, async () => {
      mockHttpRequest
        .mockResolvedValueOnce(page(fullPage(0), 102))
        .mockResolvedValueOnce(page([item(prefix, 100), item(prefix, 101)], 102));

      const {result} = renderHook(() => useHook());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
      expect(mockHttpRequest).toHaveBeenNthCalledWith(
        2,
        expect.objectContaining({url: url('limit=100&offset=100&include=display')}),
      );
      expect(result.current.data).toHaveLength(102);
    });

    it('should stop after a full page that reaches the total', async () => {
      mockHttpRequest.mockResolvedValueOnce(page(fullPage(0), 100));

      const {result} = renderHook(() => useHook());

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    it('should report a failed request', async () => {
      mockHttpRequest.mockRejectedValueOnce(new Error('Network error'));

      const {result} = renderHook(() => useHook());

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });
    });
  },
);
