// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {ThemeListResponse} from '../../models/responses';
import useGetThemes from '../useGetThemes';

const {mockHttpRequest} = vi.hoisted(() => ({mockHttpRequest: vi.fn()}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
  };
});

describe('useGetThemes', () => {
  const themeList: ThemeListResponse = {
    totalResults: 1,
    startIndex: 1,
    count: 1,
    themes: [{id: 'theme-1', handle: 'classic', displayName: 'Classic', projectId: 'project-1'}],
    links: [],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockHttpRequest.mockResolvedValue({data: themeList});
  });

  it('should list only the themes of the given project', async () => {
    const {result} = renderHook(() => useGetThemes({projectId: 'project-1'}));

    await waitFor(() => {
      expect(result.current.data).toEqual(themeList);
    });
    expect((mockHttpRequest.mock.calls[0][0] as {url: string}).url).toBe(
      'https://api.test.com/design/themes?limit=30&offset=0&projectId=project-1',
    );
  });

  it('should list every theme without a project', async () => {
    const {result} = renderHook(() => useGetThemes());

    await waitFor(() => {
      expect(result.current.data).toEqual(themeList);
    });
    expect((mockHttpRequest.mock.calls[0][0] as {url: string}).url).not.toContain('projectId');
  });
});
