// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useInfiniteOptions, type FetchSelectPage, type SelectOption} from '@thunderid/react';
import {useEffect, useRef} from 'react';
import type {FlowComponent} from '../../../models/flow';

/** Options requested per page. */
export const PAGE_SIZE = 30;

export interface UseResourceSelectOptionsArgs {
  component: FlowComponent;
  /** Loads one page. Keep the reference stable: a new one restarts the list. */
  fetchPage: FetchSelectPage;
  onError?: (error: Error) => void;
}

export interface UseResourceSelectOptionsResult {
  hasMore: boolean;
  /** The last load failed. Cleared by the next successful one. */
  loadFailed: boolean;
  loading: boolean;
  onLoadMore: () => void;
  /** Loads the first page. Not set once there are options, since the end of the list asks for the rest. */
  onOpen: (() => void) | undefined;
  options: SelectOption[];
}

/**
 * Keeps the options of a select backed by a paged management API: the offset follows each page's
 * `nextOffset`, and the list restarts when the loader changes.
 */
export default function useResourceSelectOptions({
  component,
  fetchPage,
  onError = undefined,
}: UseResourceSelectOptionsArgs): UseResourceSelectOptionsResult {
  const {error, hasMore, loading, onLoadMore, options, reset} = useInfiniteOptions<number>({
    load: ({cursor, signal}) =>
      fetchPage({component, limit: PAGE_SIZE, offset: cursor ?? 0, signal}).then((page) => ({
        nextCursor: page.nextOffset,
        options: page.options ?? [],
      })),
    onError,
  });

  const loadedWith = useRef(fetchPage);

  useEffect(() => {
    if (loadedWith.current === fetchPage) return;

    loadedWith.current = fetchPage;
    reset();
  }, [fetchPage, reset]);

  return {
    hasMore,
    loadFailed: error !== null,
    loading,
    onLoadMore,
    onOpen: options.length === 0 ? onLoadMore : undefined,
    options,
  };
}
