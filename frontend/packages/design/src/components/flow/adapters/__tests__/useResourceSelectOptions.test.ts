// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {act, cleanup, renderHook, waitFor} from '@testing-library/react';
import {afterEach, describe, it, expect, vi} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import {pageOf, pagedLoader, type FetchPage, type PagedPage} from '../../../../test/pagedOptions';
import useResourceSelectOptions, {
  DEFAULT_PAGE_SIZE,
  MAX_PAGE_SIZE,
  type UseResourceSelectOptionsArgs,
} from '../useResourceSelectOptions';

afterEach(() => {
  cleanup();
});

const component: FlowComponent = {id: 'owner_input', type: 'USER_SELECT', ref: 'owner'};

const setup = (args: Partial<UseResourceSelectOptionsArgs> & {fetchPage: FetchPage}) =>
  renderHook((props: UseResourceSelectOptionsArgs) => useResourceSelectOptions(props), {
    initialProps: {component, ...args},
  });

describe('useResourceSelectOptions', () => {
  describe('loading', () => {
    it('requests nothing until it is asked to', () => {
      const fetchPage = pagedLoader(50);
      setup({fetchPage});

      expect(fetchPage).not.toHaveBeenCalled();
    });

    it('loads the first page from offset 0 with the default page size', async () => {
      const fetchPage = pagedLoader(50);
      const {result} = setup({fetchPage});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(DEFAULT_PAGE_SIZE));
      expect(fetchPage).toHaveBeenCalledTimes(1);
      expect(fetchPage).toHaveBeenCalledWith(
        expect.objectContaining({component, limit: DEFAULT_PAGE_SIZE, offset: 0, signal: expect.any(AbortSignal)}),
      );
    });

    it('passes the filter to the loader', async () => {
      const fetchPage = pagedLoader(5);
      const {result} = setup({fetchPage, filter: 'alice'});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(5));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({filter: 'alice'}));
    });

    it('loads the next page from the previous nextOffset', async () => {
      const fetchPage = pagedLoader(50);
      const {result} = setup({fetchPage});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(result.current.options).toHaveLength(30));

      act(() => result.current.onLoadMore());

      await waitFor(() => expect(result.current.options).toHaveLength(50));
      expect(fetchPage).toHaveBeenLastCalledWith(expect.objectContaining({offset: 30}));
      expect(result.current.options[49]).toEqual({label: 'User 50', value: 'user-50'});
    });

    it('stops after the last page', async () => {
      const fetchPage = pagedLoader(40);
      const {result} = setup({fetchPage});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(result.current.options).toHaveLength(30));
      act(() => result.current.onLoadMore());
      await waitFor(() => expect(result.current.hasMore).toBe(false));

      act(() => result.current.onLoadMore());

      expect(fetchPage).toHaveBeenCalledTimes(2);
      expect(result.current.options).toHaveLength(40);
    });

    it('reports a single short page as complete', async () => {
      const {result} = setup({fetchPage: pagedLoader(3)});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.hasMore).toBe(false));
      expect(result.current.options).toHaveLength(3);
    });

    it('reports loading while a page is in flight', async () => {
      let resolvePage: (page: PagedPage) => void = () => undefined;
      const fetchPage = vi.fn<FetchPage>(
        () =>
          new Promise<PagedPage>((resolve) => {
            resolvePage = resolve;
          }),
      );
      const {result} = setup({fetchPage});

      act(() => result.current.onOpen?.());
      expect(result.current.loading).toBe(true);

      act(() => resolvePage(pageOf(0, 30, 3)));

      await waitFor(() => expect(result.current.loading).toBe(false));
    });
  });

  describe('onOpen', () => {
    it('is not offered once options have loaded', async () => {
      const {result} = setup({fetchPage: pagedLoader(50)});
      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(30));

      expect(result.current.onOpen).toBeUndefined();
    });

    it('asks for the first page again after a failed load', async () => {
      const fetchPage = vi
        .fn<FetchPage>()
        .mockRejectedValueOnce(new Error('unavailable'))
        .mockImplementation(({limit, offset}) => Promise.resolve(pageOf(offset, limit, 3)));
      const onError = vi.fn();
      const {result} = setup({fetchPage, onError});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
      await waitFor(() => expect(result.current.loading).toBe(false));

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(3));
      expect(fetchPage).toHaveBeenLastCalledWith(expect.objectContaining({offset: 0}));
    });
  });

  describe('errors', () => {
    it('reports the failure and keeps the list empty', async () => {
      const failure = new Error('forbidden');
      const onError = vi.fn();
      const {result} = setup({fetchPage: vi.fn<FetchPage>().mockRejectedValue(failure), onError});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(onError).toHaveBeenCalledWith(failure));
      expect(result.current.options).toEqual([]);
      expect(result.current.hasMore).toBe(true);
    });

    it('keeps the loaded options when a later page fails', async () => {
      const fetchPage = vi
        .fn<FetchPage>()
        .mockResolvedValueOnce(pageOf(0, 30, 50))
        .mockRejectedValue(new Error('unavailable'));
      const onError = vi.fn();
      const {result} = setup({fetchPage, onError});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(result.current.options).toHaveLength(30));

      act(() => result.current.onLoadMore());

      await waitFor(() => expect(onError).toHaveBeenCalledTimes(1));
      expect(result.current.options).toHaveLength(30);
    });
  });

  describe('page size', () => {
    it('uses the given page size', async () => {
      const fetchPage = pagedLoader(50);
      const {result} = setup({fetchPage, pageSize: 10});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(10));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: 10}));
    });

    it('never asks for more than the API serves', async () => {
      const fetchPage = pagedLoader(500);
      const {result} = setup({fetchPage, pageSize: 500});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(MAX_PAGE_SIZE));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: MAX_PAGE_SIZE}));
    });

    it.each([Number.NaN, Number.POSITIVE_INFINITY])('falls back to the default page size for %s', async (pageSize) => {
      const fetchPage = pagedLoader(50);
      const {result} = setup({fetchPage, pageSize});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(DEFAULT_PAGE_SIZE));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: DEFAULT_PAGE_SIZE}));
    });

    it('asks for a whole number of options, rounding a fractional size down', async () => {
      const fetchPage = pagedLoader(5);
      const {result} = setup({fetchPage, pageSize: 2.5});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(2));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: 2}));
    });

    it('never asks for fewer than one, even for a size between zero and one', async () => {
      const fetchPage = pagedLoader(5);
      const {result} = setup({fetchPage, pageSize: 0.4});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(1));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: 1}));
    });

    it('never asks for fewer than one', async () => {
      const fetchPage = pagedLoader(5);
      const {result} = setup({fetchPage, pageSize: 0});

      act(() => result.current.onOpen?.());

      await waitFor(() => expect(result.current.options).toHaveLength(1));
      expect(fetchPage).toHaveBeenCalledWith(expect.objectContaining({limit: 1}));
    });
  });

  describe('restarting', () => {
    it('does not restart when it renders again with the same inputs', async () => {
      const fetchPage = pagedLoader(50);
      const {result, rerender} = setup({fetchPage});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(result.current.options).toHaveLength(30));

      rerender({component, fetchPage});

      expect(fetchPage).toHaveBeenCalledTimes(1);
      expect(result.current.options).toHaveLength(30);
    });

    it('starts again from the first page when the loader changes', async () => {
      const first = pagedLoader(50);
      const second = vi.fn<FetchPage>(({limit, offset}) => Promise.resolve(pageOf(offset, limit, 4)));
      const {result, rerender} = setup({fetchPage: first});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(result.current.options).toHaveLength(30));

      rerender({component, fetchPage: second});

      await waitFor(() => expect(result.current.options).toHaveLength(4));
      expect(second).toHaveBeenCalledWith(expect.objectContaining({offset: 0}));
    });

    it('starts again from the first page when the filter changes', async () => {
      const fetchPage = pagedLoader(50);
      const {result, rerender} = setup({fetchPage, filter: 'a'});
      act(() => result.current.onOpen?.());
      await waitFor(() => expect(result.current.options).toHaveLength(30));

      rerender({component, fetchPage, filter: 'b'});

      await waitFor(() => expect(fetchPage).toHaveBeenCalledTimes(2));
      expect(fetchPage).toHaveBeenLastCalledWith(expect.objectContaining({filter: 'b', offset: 0}));
    });

    it('does not load anything for a new loader before the list was ever opened', () => {
      const first = pagedLoader(50);
      const second = pagedLoader(50);
      const {rerender} = setup({fetchPage: first});

      rerender({component, fetchPage: second});

      expect(first).not.toHaveBeenCalled();
      expect(second).not.toHaveBeenCalled();
    });
  });
});
