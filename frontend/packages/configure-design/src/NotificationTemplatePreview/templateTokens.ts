// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Notification-template token grammar, mirroring the backend
// (internal/notificationtemplate/placeholder.go): only {{t(key)}}, {{design(key)}}, {{ctx(key)}},
// no inner spaces, key = [A-Za-z0-9_.-]+; design is email-body only.

export type NotificationChannel = 'email' | 'sms';
export type TemplateField = 'subject' | 'body';
export type TokenKind = 't' | 'design' | 'ctx';

/** A parsed {{...}} block; `kind: null` means malformed/unsupported. */
export interface ParsedToken {
  raw: string;
  kind: TokenKind | null;
  key?: string;
}

/** A token the preview cannot render faithfully. */
export interface TemplateWarning {
  severity: 'error' | 'warning';
  code: 'unsupported' | 'designNotAllowed' | 'designUnresolved' | 'missingTranslation' | 'unknownContext';
  token: string;
}

/** Namespace that notification-template {{t(key)}} tokens resolve under (matches the backend). */
export const NOTIFICATION_NAMESPACE = 'notification';

const TOKEN_BLOCK_REGEX = /\{\{([\s\S]*?)\}\}/g;
const VALID_PLACEHOLDER_REGEX = /^(ctx|t|design)\(([A-Za-z0-9_.-]+)\)$/;

/**
 * Resolves a design token key (e.g. `palette.primary.main`) against a resolved theme, matching the
 * backend's `{{design(key)}}` resolution (`backend/internal/design/resolve/utils.go`): the applicable
 * color scheme — the requested one, or the theme's `defaultColorScheme` when the requested one is
 * absent — is promoted onto the theme root, so palette.* and root-level tokens (typography.*, shape.*)
 * share one dot-path namespace. Returns the string value, or undefined when the path is missing or not
 * a primitive (so the caller can warn).
 */
export function resolveDesignToken(
  key: string,
  theme: Record<string, unknown> | null | undefined,
  colorScheme: 'light' | 'dark',
): string | undefined {
  if (theme == null) {
    return undefined;
  }
  const schemes = theme['colorSchemes'] as Record<string, unknown> | undefined;
  let selected = schemes?.[colorScheme] as Record<string, unknown> | undefined;
  if (selected === undefined) {
    const defaultScheme = theme['defaultColorScheme'];
    if (typeof defaultScheme === 'string') {
      selected = schemes?.[defaultScheme] as Record<string, unknown> | undefined;
    }
  }
  // Shallow-promote the scheme onto the root (scheme wins), dropping the scheme-selection inputs.
  const root: Record<string, unknown> = {...theme, ...(selected ?? {})};
  delete root['colorSchemes'];
  delete root['defaultColorScheme'];

  let node: unknown = root;
  for (const segment of key.split('.')) {
    if (node == null || typeof node !== 'object') {
      return undefined;
    }
    node = (node as Record<string, unknown>)[segment];
  }
  return typeof node === 'string' || typeof node === 'number' || typeof node === 'boolean' ? String(node) : undefined;
}

/** Parses and classifies every {{...}} block in `text`. */
export function parseTokens(text: string): ParsedToken[] {
  const tokens: ParsedToken[] = [];
  for (const match of text.matchAll(TOKEN_BLOCK_REGEX)) {
    const raw = match[0];
    const inner = match[1];
    const parts = VALID_PLACEHOLDER_REGEX.exec(inner);
    if (parts) {
      tokens.push({raw, kind: parts[1] as TokenKind, key: parts[2]});
    } else {
      tokens.push({raw, kind: null});
    }
  }
  return tokens;
}

/** Options shared by preview resolution and warning collection. */
export interface PreviewResolveOptions {
  translations: Record<string, string>;
  theme?: Record<string, unknown> | null;
  colorScheme?: 'light' | 'dark';
  /**
   * The ctx keys the runtime can actually inject (the catalog the editor offers, e.g.
   * `CONTEXT_TOKENS`). When provided, a {{ctx(key)}} outside this set is flagged like a missing
   * translation; when omitted, ctx tokens are left unchecked (callers that don't surface a catalog).
   */
  contextKeys?: ReadonlySet<string>;
  /**
   * The design token keys the editor offers (e.g. `DESIGN_TOKENS`). When provided, a {{design(key)}}
   * outside this set is flagged as unrecognized, independent of the selected theme; when omitted,
   * design tokens are left unchecked.
   */
  designKeys?: ReadonlySet<string>;
}

// escapeHtml mirrors the backend's html.EscapeString (Go) character for character, so a resolved design
// value renders in the preview exactly as it does in the sent email.
function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/'/g, '&#39;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&#34;');
}

/**
 * Resolves a field for preview: substitutes {{t(key)}} with its translation and {{design(key)}}
 * with the theme value (email body only, HTML-escaped to match delivery). A design token that does not
 * resolve (no theme, or a token the theme does not define) becomes an empty string, matching delivery;
 * {{ctx}} and any unresolvable {{t}} are left literal.
 */
export function resolveForPreview(
  text: string | undefined,
  opts: PreviewResolveOptions,
  field: TemplateField = 'body',
  channel: NotificationChannel = 'email',
): string {
  if (!text) {
    return '';
  }
  const scheme = opts.colorScheme ?? 'light';
  const designAllowed = field === 'body' && channel === 'email';
  return text.replace(TOKEN_BLOCK_REGEX, (raw, inner: string) => {
    const parts = VALID_PLACEHOLDER_REGEX.exec(inner);
    if (!parts) {
      return raw;
    }
    if (parts[1] === 't') {
      return opts.translations[parts[2]] ?? raw;
    }
    if (parts[1] === 'design' && designAllowed) {
      const value = resolveDesignToken(parts[2], opts.theme, scheme);
      return value === undefined ? '' : escapeHtml(value);
    }
    return raw;
  });
}

/** Collects warnings for one field. */
function collectFieldWarnings(
  text: string | undefined,
  field: TemplateField,
  channel: NotificationChannel,
  opts: PreviewResolveOptions,
  skipMissingTranslation: boolean,
): TemplateWarning[] {
  if (!text) {
    return [];
  }
  const warnings: TemplateWarning[] = [];
  for (const token of parseTokens(text)) {
    if (token.kind === null) {
      warnings.push({severity: 'error', code: 'unsupported', token: token.raw});
      continue;
    }
    if (token.kind === 'design') {
      if (field === 'subject' || channel === 'sms') {
        warnings.push({severity: 'error', code: 'designNotAllowed', token: token.raw});
      } else if (opts.designKeys !== undefined && token.key !== undefined && !opts.designKeys.has(token.key)) {
        // Flag a token outside the offered catalog (independent of the selected theme); a recognized key
        // is accepted even when the previewed theme does not define it.
        warnings.push({severity: 'warning', code: 'designUnresolved', token: token.raw});
      }
      continue;
    }
    if (token.kind === 'ctx') {
      // ctx is left literal for the runtime to substitute; but a key the runtime never injects (not in
      // the offered catalog) will render literally on send — surface it like a missing translation.
      // Only validate when the caller supplies the catalog; otherwise ctx is left unchecked.
      if (opts.contextKeys !== undefined && token.key !== undefined && !opts.contextKeys.has(token.key)) {
        warnings.push({severity: 'error', code: 'unknownContext', token: token.raw});
      }
      continue;
    }
    if (
      !skipMissingTranslation &&
      token.kind === 't' &&
      token.key !== undefined &&
      opts.translations[token.key] === undefined
    ) {
      // Preview translations are already the best match across languages, so a key missing here has
      // no value in any language and the renderer fails the send — surface it as an error, not a warning.
      warnings.push({severity: 'error', code: 'missingTranslation', token: token.raw});
    }
  }
  return warnings;
}

/**
 * De-duplicated warnings across subject and body. Warnings are theme-independent — a design token is
 * flagged against the offered catalog (designKeys), not the selected theme — so no theme/colorScheme.
 */
export function collectWarnings(input: {
  subject?: string;
  body: string;
  channel: NotificationChannel;
  translations: Record<string, string>;
  /** ctx keys the runtime can inject; a {{ctx(key)}} outside this set is flagged. Unset → unchecked. */
  contextKeys?: ReadonlySet<string>;
  /** design keys the editor offers; a {{design(key)}} outside this set is flagged. Unset → unchecked. */
  designKeys?: ReadonlySet<string>;
  /** Suppress missing-translation warnings while translations are still loading / unavailable. */
  skipMissingTranslation?: boolean;
}): TemplateWarning[] {
  const opts: PreviewResolveOptions = {
    translations: input.translations,
    contextKeys: input.contextKeys,
    designKeys: input.designKeys,
  };
  const skipMissingTranslation = input.skipMissingTranslation ?? false;
  const all = [
    ...collectFieldWarnings(input.subject, 'subject', input.channel, opts, skipMissingTranslation),
    ...collectFieldWarnings(input.body, 'body', input.channel, opts, skipMissingTranslation),
  ];
  const seen = new Set<string>();
  const deduped: TemplateWarning[] = [];
  for (const warning of all) {
    const dedupeKey = `${warning.code}:${warning.token}`;
    if (seen.has(dedupeKey)) {
      continue;
    }
    seen.add(dedupeKey);
    deduped.push(warning);
  }
  return deduped;
}
