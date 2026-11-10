// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The server's maximum page size (MaxPageSize in backend/internal/system/constants). A list endpoint caps its `limit`
 * at this value, so a full list is read by paging through it with `offset`.
 */
export const MAX_PAGE_SIZE = 100;

/**
 * One page of a list endpoint.
 */
export interface Page<T> {
  /**
   * The items of this page.
   */
  items: T[];

  /**
   * The total number of items the server reports, when it does.
   */
  totalResults?: number;
}

/**
 * Reads every item of a paged list endpoint, requesting {@link MAX_PAGE_SIZE} items at a time until the list ends.
 * A short page is always the last. After a full page, the reported total ends the loop; when the server reports none,
 * paging continues until a short or empty page arrives.
 *
 * @param fetchPage - Reads one page, given the `limit` and `offset` to request.
 * @returns Every item of the list.
 */
export default async function fetchAllPages<T>(
  fetchPage: (limit: number, offset: number) => Promise<Page<T>>,
): Promise<T[]> {
  const items: T[] = [];

  for (;;) {
    const page: Page<T> = await fetchPage(MAX_PAGE_SIZE, items.length);
    items.push(...page.items);

    if (page.items.length < MAX_PAGE_SIZE || (page.totalResults !== undefined && items.length >= page.totalResults)) {
      return items;
    }
  }
}
