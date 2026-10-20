// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useInfiniteOptions, type FetchSelectPage, type SelectOption} from '@thunderid/react';
import {useEffect, useRef} from 'react';
import type {FlowComponent} from '../../../models/flow';

export const DEFAULT_PAGE_SIZE = 30;

/** The most the management API serves in one page. */
export const MAX_PAGE_SIZE = 100;

export interface UseResourceSelectOptionsArgs {
  component: FlowComponent;
  /** Loads one page. Keep the reference stable: a new one restarts the list. */
  fetchPage: FetchSelectPage;
  /** A new filter restarts the list. */
  filter?: string;
  onError?: (error: Error) => void;
  pageSize?: number;
}

export interface UseResourceSelectOptionsResult {
  hasMore: boolean;
  loading: boolean;
  onLoadMore: () => void;
  /** Loads the first page. Not set once there are options, since the end of the list asks for the rest. */
  onOpen: (() => void) | undefined;
  options: SelectOption[];
}

/**
 * Keeps the options of a select backed by a paged management API: the offset follows each page's
 * `nextOffset`, and the list restarts when the loader, filter or page size changes.
 */
export default function useResourceSelectOptions({
  component,
  fetchPage,
  filter = undefined,
  onError = undefined,
  pageSize = DEFAULT_PAGE_SIZE,
}: UseResourceSelectOptionsArgs): UseResourceSelectOptionsResult {
  const limit = Number.isFinite(pageSize)
    ? Math.max(1, Math.min(Math.floor(pageSize), MAX_PAGE_SIZE))
    : DEFAULT_PAGE_SIZE;

  const {hasMore, loading, onLoadMore, options, reset} = useInfiniteOptions<number>({
    load: ({cursor, signal}) =>
      fetchPage({component, filter, limit, offset: cursor ?? 0, signal}).then((page) => ({
        nextCursor: page.nextOffset,
        options: page.options ?? [],
      })),
    onError,
  });

  const loadedWith = useRef({fetchPage, filter, limit});

  useEffect(() => {
    const previous = loadedWith.current;

    if (previous.fetchPage === fetchPage && previous.filter === filter && previous.limit === limit) return;

    loadedWith.current = {fetchPage, filter, limit};
    reset();
  }, [fetchPage, filter, limit, reset]);

  return {hasMore, loading, onLoadMore, onOpen: options.length === 0 ? onLoadMore : undefined, options};
}
