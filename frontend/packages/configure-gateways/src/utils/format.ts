// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The first seven characters of a version's hash, which is how a version is named. */
export const shortHash = (hash: string | undefined): string => (hash ?? '').slice(0, 7);

/** A version's label: "v14" by its number, or its short hash when it has none. */
export function versionLabel(hash: string | undefined, ordinal?: number): string {
  if (ordinal !== undefined) return `v${ordinal}`;
  return shortHash(hash);
}
