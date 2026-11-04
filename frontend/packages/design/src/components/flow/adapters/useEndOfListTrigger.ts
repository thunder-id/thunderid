// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useRef} from 'react';

/** How far below the visible list, in pixels, the end still counts as reached, so loading starts a little early. */
export const END_OF_LIST_MARGIN_PX = 64;

export interface EndOfListTriggerArgs {
  /** Nothing is observed unless this is `true`, for example while the list is closed. */
  enabled: boolean;
  /** Number of items in the list. A different count means the list grew or was replaced. */
  itemCount: number;
  /** While `true`, reaching the end does not call `onLoadMore`. */
  loading: boolean;
  onLoadMore?: () => void;
  /** The scrolling element that holds the list. */
  root: HTMLElement | null;
  /** An element at the end of the list, such as its last item. */
  sentinel: HTMLElement | null;
}

interface RequestedList {
  itemCount: number;
  root: HTMLElement;
}

/**
 * Calls `onLoadMore` when the end of a scrolling list comes into view, at most once for each list length. Scrolling
 * away from the end and back, opening the list again, or the list growing allows another call. It never calls while
 * `loading`, and it also fires for a list too short to scroll, since it watches visibility rather than scroll events.
 */
export default function useEndOfListTrigger({
  enabled,
  itemCount,
  loading,
  onLoadMore = undefined,
  root,
  sentinel,
}: EndOfListTriggerArgs): void {
  const requestedRef = useRef<RequestedList | null>(null);
  const latestRef = useRef({loading, onLoadMore});

  useEffect(() => {
    latestRef.current = {loading, onLoadMore};
  });

  // `loading` is a dependency so the end is looked at again, with a fresh observation, whenever a load finishes.
  useEffect(() => {
    if (!enabled || !root || !sentinel) return undefined;

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting) {
          requestedRef.current = null;
          return;
        }

        const {loading: isLoading, onLoadMore: loadMore} = latestRef.current;
        const requested = requestedRef.current;
        const alreadyRequested = requested?.root === root && requested.itemCount === itemCount;

        if (alreadyRequested || isLoading || !loadMore) return;

        requestedRef.current = {itemCount, root};
        loadMore();
      },
      {root, rootMargin: `0px 0px ${END_OF_LIST_MARGIN_PX}px 0px`},
    );

    observer.observe(sentinel);

    return () => observer.disconnect();
  }, [enabled, itemCount, loading, root, sentinel]);
}
