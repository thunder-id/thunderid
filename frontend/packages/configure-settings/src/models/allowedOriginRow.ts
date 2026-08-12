// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The two shapes an allowed origin can take on the wire: a literal origin string or a `{regex}`
 * entry. The admin picks this per row; it is never inferred from the text.
 *
 * @public
 */
export const AllowedOriginTypes = {
  ORIGIN: 'origin',
  REGEX: 'regex',
} as const;

/**
 * The type of a single allowed-origin row.
 *
 * @public
 */
export type AllowedOriginType = (typeof AllowedOriginTypes)[keyof typeof AllowedOriginTypes];

/**
 * One editable allowed-origin row.
 *
 * @public
 */
export interface AllowedOriginDraftRow {
  /** Stable client-only key for React and per-row error lookup. Never sent to the server. */
  id: string;
  /** Whether this row is a literal origin or a regex pattern. */
  type: AllowedOriginType;
  /** The origin or pattern text, without the decorative `/` delimiters shown for regex rows. */
  value: string;
}
