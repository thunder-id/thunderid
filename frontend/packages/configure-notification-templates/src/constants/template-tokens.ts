// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A placeholder token a user can insert into a template's subject or body.
 *
 * @public
 */
export interface TemplateToken {
  /**
   * i18n key for the catalog label, resolved with {@link label} as the fallback. Omitted for
   * dynamic tokens whose label is a user-defined key (e.g. translation tokens).
   */
  labelKey?: string;
  /** Human-readable label shown in the catalog (and i18n fallback), e.g. `Primary color`. */
  label: string;
  /** The exact placeholder inserted into the editor, e.g. `{{design(palette.primary.main)}}`. */
  insert: string;
}

/**
 * Design tokens resolved from the theme at render time. Email body only.
 *
 * Uses the backend `{{design(key)}}` grammar; hardcoded until a design-token catalog API exists.
 *
 * @public
 */
export const DESIGN_TOKENS: readonly TemplateToken[] = [
  {labelKey: 'editor.tokens.labels.design.primary', label: 'Primary color', insert: '{{design(palette.primary.main)}}'},
  {
    labelKey: 'editor.tokens.labels.design.primaryContrast',
    label: 'Primary contrast text',
    insert: '{{design(palette.primary.contrastText)}}',
  },
  {
    labelKey: 'editor.tokens.labels.design.secondary',
    label: 'Secondary color',
    insert: '{{design(palette.secondary.main)}}',
  },
  {
    labelKey: 'editor.tokens.labels.design.textPrimary',
    label: 'Text primary',
    insert: '{{design(palette.text.primary)}}',
  },
  {
    labelKey: 'editor.tokens.labels.design.backgroundDefault',
    label: 'Background default',
    insert: '{{design(palette.background.default)}}',
  },
  {
    labelKey: 'editor.tokens.labels.design.backgroundPaper',
    label: 'Background paper',
    insert: '{{design(palette.background.paper)}}',
  },
];

/**
 * Context tokens substituted with runtime values when a notification is sent. Both channels.
 *
 * Keys mirror the runtime data injected by the backend flow executors (OTP, magic-link, invite),
 * the authn service and the account-lock email; see `backend/internal/flow/executor/*`,
 * `backend/internal/authn` and `backend/internal/identitygovernance`.
 * Hardcoded here until a context-placeholder catalog API exists; keep in sync with those keys so a
 * token the catalog offers actually resolves on send.
 *
 * @public
 */
export const CONTEXT_TOKENS: readonly TemplateToken[] = [
  {labelKey: 'editor.tokens.labels.context.appName', label: 'App name', insert: '{{ctx(appName)}}'},
  {labelKey: 'editor.tokens.labels.context.otpCode', label: 'OTP code', insert: '{{ctx(otpCode)}}'},
  {labelKey: 'editor.tokens.labels.context.expiryTime', label: 'Expiry time', insert: '{{ctx(expiryTime)}}'},
  {labelKey: 'editor.tokens.labels.context.magicLink', label: 'Magic link', insert: '{{ctx(magicLink)}}'},
  {labelKey: 'editor.tokens.labels.context.inviteLink', label: 'Invite link', insert: '{{ctx(inviteLink)}}'},
  {
    labelKey: 'editor.tokens.labels.context.lockedAuthenticationMethod',
    label: 'Locked sign-in method',
    insert: '{{ctx(lockedAuthenticationMethod)}}',
  },
  {labelKey: 'editor.tokens.labels.context.lockDuration', label: 'Lock duration', insert: '{{ctx(lockDuration)}}'},
  {labelKey: 'editor.tokens.labels.context.unlockTime', label: 'Unlock time', insert: '{{ctx(unlockTime)}}'},
];

/**
 * The ctx keys {@link CONTEXT_TOKENS} offers, derived from their `insert` strings — the set the editor
 * preview validates {{ctx(key)}} against, so an unoffered key is flagged. Single source: the catalog
 * above. Empty when an entry's insert is not a `{{ctx(key)}}` placeholder (not expected).
 */
export const CONTEXT_TOKEN_KEYS: ReadonlySet<string> = new Set(
  CONTEXT_TOKENS.map((token) => /^\{\{ctx\(([A-Za-z0-9_.-]+)\)\}\}$/.exec(token.insert)?.[1]).filter(
    (key): key is string => key !== undefined,
  ),
);

/**
 * The design token keys {@link DESIGN_TOKENS} offers, derived from their `insert` strings. The preview
 * validates {{design(key)}} against this catalog (not the selected theme), so a key outside the list is
 * flagged while a recognized key is accepted regardless of which theme is previewed.
 */
export const DESIGN_TOKEN_KEYS: ReadonlySet<string> = new Set(
  DESIGN_TOKENS.map((token) => /^\{\{design\(([A-Za-z0-9_.-]+)\)\}\}$/.exec(token.insert)?.[1]).filter(
    (key): key is string => key !== undefined,
  ),
);
