// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import DesignQueryKeys from '../../constants/design-query-keys';
import type {LayoutListResponse} from '../../models/responses';
import useGetLayouts from '../useGetLayouts';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const layoutList: LayoutListResponse = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  layouts: [{id: 'layout-1', handle: 'centered', displayName: 'Centered', projectId: 'project-1', layout: null}],
  links: [],
};

describe('useGetLayouts', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as unknown as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('lists the layouts a page at a time', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: layoutList});

    const {result} = renderHook(() => useGetLayouts({limit: 10, offset: 20}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(layoutList);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toBe('https://api.test.com/design/layouts?limit=10&offset=20');
  });

  it("lists only the given project's layouts", async () => {
    mockHttpRequest.mockResolvedValueOnce({data: layoutList});

    const {result, queryClient} = renderHook(() => useGetLayouts({projectId: 'project-1'}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('projectId=project-1');
    expect(
      queryClient.getQueryCache().findAll({queryKey: [DesignQueryKeys.LAYOUTS, {projectId: 'project-1'}]}),
    ).toHaveLength(1);
  });

  it('lists every layout without a project', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: layoutList});

    renderHook(() => useGetLayouts());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).not.toContain('projectId');
  });
});
