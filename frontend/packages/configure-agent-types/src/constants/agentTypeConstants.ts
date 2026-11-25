// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Page size used when every agent type is fetched, one page at a time. It matches the largest
 * `limit` the API accepts, so the fewest requests are made.
 */
export const AGENT_TYPES_PAGE_SIZE = 100;
