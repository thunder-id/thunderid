// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The scope key for the top-level directives, which apply to every CSP path. Path scopes use their
 * `location` as the key, and a location always starts with `/`, so the two never collide.
 *
 * @public
 */
export const CSP_DEFAULT_SCOPE = 'all';

/**
 * A directive the admin set in one scope. Its sources replace the directive's default value.
 *
 * @public
 */
export interface CspDirectiveDraftRow {
  /** Stable client-only key for React and per-row error lookup. Never sent to the server. */
  id: string;
  /** The directive name, for example `img-src`. */
  directive: string;
  /** The directive's full list of sources. */
  sources: string[];
  /** Whether the directive is in the saved writable layer. */
  saved?: boolean;
}

/**
 * A directive as shown for one scope.
 *
 * @public
 */
export interface CspScopeRow {
  directive: string;
  /** The default value: from the baseline, the read-only layer, or the Shared scope. Empty for a new directive. */
  base: string[];
  /** The sources the admin set in this scope, replacing `base`. */
  override?: string[];
  /** Id of the draft row holding `override`. */
  draftId?: string;
  /** Whether the override is in the saved writable layer. */
  saved?: boolean;
}

/**
 * Editable rows per scope, keyed by {@link CSP_DEFAULT_SCOPE} or a path `location`.
 *
 * @public
 */
export type CspDraftScopes = Record<string, CspDirectiveDraftRow[]>;
