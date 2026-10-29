// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useGetChildOrganizationUnits from '../useGetChildOrganizationUnits';
import useGetOrganizationUnits from '../useGetOrganizationUnits';
import useHasMultipleOUs from '../useHasMultipleOUs';

vi.mock('../useGetOrganizationUnits');
vi.mock('../useGetChildOrganizationUnits');

const rootQuery = (organizationUnits: {id: string}[], totalResults: number, isError = false) =>
  ({data: isError ? undefined : {organizationUnits, totalResults}, isLoading: false, isError}) as never;

const childQuery = (totalResults: number, isError = false) =>
  ({data: isError ? undefined : {totalResults}, isLoading: false, isError}) as never;

describe('useHasMultipleOUs', () => {
  beforeEach(() => {
    vi.mocked(useGetOrganizationUnits).mockReset();
    vi.mocked(useGetChildOrganizationUnits).mockReset();
  });

  it('reports several organization units when there is more than one root', () => {
    vi.mocked(useGetOrganizationUnits).mockReturnValue(rootQuery([{id: 'a'}, {id: 'b'}], 2));
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue(childQuery(0));

    const {result} = renderHook(() => useHasMultipleOUs());

    expect(result.current).toMatchObject({hasMultipleOUs: true, isError: false});
  });

  it('reports several organization units when the only root has children', () => {
    vi.mocked(useGetOrganizationUnits).mockReturnValue(rootQuery([{id: 'a'}], 1));
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue(childQuery(3));

    const {result} = renderHook(() => useHasMultipleOUs());

    expect(result.current).toMatchObject({hasMultipleOUs: true, isError: false});
  });

  it('reports a single organization unit when the only root has no children', () => {
    vi.mocked(useGetOrganizationUnits).mockReturnValue(rootQuery([{id: 'a'}], 1));
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue(childQuery(0));

    const {result} = renderHook(() => useHasMultipleOUs());

    expect(result.current).toMatchObject({hasMultipleOUs: false, isError: false});
  });

  it('reports an error when the only root has its children lookup fail', () => {
    vi.mocked(useGetOrganizationUnits).mockReturnValue(rootQuery([{id: 'a'}], 1));
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue(childQuery(0, true));

    const {result} = renderHook(() => useHasMultipleOUs());

    expect(result.current.isError).toBe(true);
  });

  it('reports an error when the root lookup fails', () => {
    vi.mocked(useGetOrganizationUnits).mockReturnValue(rootQuery([], 0, true));
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue(childQuery(0));

    const {result} = renderHook(() => useHasMultipleOUs());

    expect(result.current.isError).toBe(true);
  });
});
