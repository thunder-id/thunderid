// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccountLinking} from '../models/connection';

/** One account-linking attribute row, keyed for stable React list rendering. */
export interface KeyedLink {
  key: number;
  value: string;
}

let keySeq = 0;
const nextKey = (): number => {
  keySeq += 1;
  return keySeq;
};

/** A fresh, empty account-linking row, for appending to the list. */
export function newLinkRow(): KeyedLink {
  return {key: nextKey(), value: ''};
}

/**
 * Build the API `accountLinking` from the form rows. Empty and whitespace-only rows are dropped.
 * Returns `undefined` when nothing is left, so an untouched section omits the field entirely.
 */
export function toAccountLinking(rows: KeyedLink[]): AccountLinking | undefined {
  const attributes = rows.map((row) => row.value.trim()).filter((value) => value !== '');
  return attributes.length > 0 ? {attributes} : undefined;
}

/**
 * Convert the API shape into form state, keying every row for stable React list rendering. Always
 * returns at least one (empty) row, so the form has a starter input to type into without the admin
 * having to click "Add" first.
 */
export function fromAccountLinking(config: AccountLinking | undefined): KeyedLink[] {
  const attributes = config?.attributes ?? [];
  return attributes.length > 0 ? attributes.map((value) => ({key: nextKey(), value})) : [newLinkRow()];
}
