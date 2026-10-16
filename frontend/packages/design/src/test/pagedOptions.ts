// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {vi, type Mock} from 'vitest';
import type {FlowComponent} from '../models/flow';

export interface PagedOption {
  label: string;
  value: string;
}

export interface PagedRequest {
  component: FlowComponent;
  filter?: string;
  limit: number;
  offset: number;
  signal: AbortSignal;
}

export interface PagedPage {
  nextOffset: number | null;
  options: PagedOption[];
  totalResults?: number;
}

export type FetchPage = (request: PagedRequest) => Promise<PagedPage>;

/** One page of `total` numbered users, `User 1` with value `user-1` and so on, starting at `offset`. */
export const pageOf = (offset: number, limit: number, total: number): PagedPage => {
  const count = Math.max(0, Math.min(limit, total - offset));

  return {
    nextOffset: offset + count < total ? offset + count : null,
    options: Array.from({length: count}, (_, index) => ({
      label: `User ${offset + index + 1}`,
      value: `user-${offset + index + 1}`,
    })),
    totalResults: total,
  };
};

/** A loader that serves `total` numbered users, `limit` at a time. */
export const pagedLoader = (total: number): Mock<FetchPage> =>
  vi.fn<FetchPage>(({limit, offset}) => Promise.resolve(pageOf(offset, limit, total)));
