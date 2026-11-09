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

  it('hands back the failed lookup and retries only that one', () => {
    const rootError = new Error('Boom');
    const refetchRoots = vi.fn();
    const refetchChildren = vi.fn();
    vi.mocked(useGetOrganizationUnits).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: rootError,
      refetch: refetchRoots,
    } as never);
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue({
      ...(childQuery(0) as object),
      error: null,
      refetch: refetchChildren,
    } as never);

    const {result} = renderHook(() => useHasMultipleOUs());
    result.current.refetch();

    expect(result.current.error).toBe(rootError);
    expect(refetchRoots).toHaveBeenCalledTimes(1);
    expect(refetchChildren).not.toHaveBeenCalled();
  });

  it('hands back the failed children lookup of the only root and retries it', () => {
    const childError = new Error('Boom');
    const refetchRoots = vi.fn();
    const refetchChildren = vi.fn();
    vi.mocked(useGetOrganizationUnits).mockReturnValue({
      ...(rootQuery([{id: 'a'}], 1) as object),
      error: null,
      refetch: refetchRoots,
    } as never);
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: childError,
      refetch: refetchChildren,
    } as never);

    const {result} = renderHook(() => useHasMultipleOUs());
    result.current.refetch();

    expect(result.current.error).toBe(childError);
    expect(refetchChildren).toHaveBeenCalledTimes(1);
    expect(refetchRoots).not.toHaveBeenCalled();
  });

  it('ignores a children lookup error unless there is exactly one root', () => {
    const refetchRoots = vi.fn();
    const refetchChildren = vi.fn();
    vi.mocked(useGetOrganizationUnits).mockReturnValue({
      ...(rootQuery([{id: 'a'}, {id: 'b'}], 2) as object),
      error: null,
      refetch: refetchRoots,
    } as never);
    vi.mocked(useGetChildOrganizationUnits).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error('Boom'),
      refetch: refetchChildren,
    } as never);

    const {result} = renderHook(() => useHasMultipleOUs());
    result.current.refetch();

    expect(result.current).toMatchObject({hasMultipleOUs: true, isError: false, error: null});
    expect(refetchRoots).not.toHaveBeenCalled();
    expect(refetchChildren).not.toHaveBeenCalled();
  });
});
