// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useRef, type RefObject} from 'react';

/**
 * Result of {@link useMoveRows}.
 *
 * @public
 */
export interface UseMoveRowsResult {
  /** Row elements by row ID. Attach each row's element here so its move buttons can get focus back. */
  rowRefs: RefObject<Record<string, HTMLDivElement | null>>;
  /** Swaps the row at `index` with its neighbour, up for -1 and down for 1. */
  moveRow: (index: number, offset: -1 | 1) => void;
}

/**
 * Moves rows of a list up and down, and keeps focus on the move button that was pressed. Move buttons
 * are found by their `data-move="up"` or `data-move="down"` attribute.
 *
 * @param rows - The rows, in their current order
 * @param onRowsChange - Called with the rows in their new order
 * @returns The row element refs and the move handler
 *
 * @public
 */
export default function useMoveRows<T extends {id: string}>(
  rows: T[],
  onRowsChange: (rows: T[]) => void,
): UseMoveRowsResult {
  const rowRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const pendingFocus = useRef<{id: string; direction: 'up' | 'down'} | null>(null);

  // A reorder moves the row's DOM node, so focus the move button again once it settles.
  useEffect(() => {
    if (!pendingFocus.current) return;
    const {id, direction} = pendingFocus.current;
    pendingFocus.current = null;
    const row = rowRefs.current[id];
    const button =
      row?.querySelector<HTMLButtonElement>(`[data-move="${direction}"]:not(:disabled)`) ??
      row?.querySelector<HTMLButtonElement>('[data-move]:not(:disabled)');
    button?.focus();
  }, [rows]);

  const moveRow = (index: number, offset: -1 | 1): void => {
    const target = index + offset;
    if (target < 0 || target >= rows.length) return;
    const reordered = [...rows];
    [reordered[index], reordered[target]] = [reordered[target], reordered[index]];
    pendingFocus.current = {id: rows[index].id, direction: offset < 0 ? 'up' : 'down'};
    onRowsChange(reordered);
  };

  return {rowRefs, moveRow};
}
