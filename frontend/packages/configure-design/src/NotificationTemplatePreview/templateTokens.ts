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
  code: 'unsupported' | 'designNotAllowed' | 'designUnresolved' | 'missingTranslation';
  token: string;
}

const TOKEN_BLOCK_REGEX = /\{\{([\s\S]*?)\}\}/g;
const VALID_PLACEHOLDER_REGEX = /^(ctx|t|design)\(([A-Za-z0-9_.-]+)\)$/;

/**
 * Resolves a design token key (e.g. `palette.primary.main`) against a resolved theme's color
 * scheme, matching the backend's `{{design(key)}}` resolution. Returns the string value, or
 * undefined when the path is missing or not a primitive (so the caller can warn).
 */
export function resolveDesignToken(
  key: string,
  theme: Record<string, unknown> | null | undefined,
  colorScheme: 'light' | 'dark',
): string | undefined {
  const schemes = theme?.['colorSchemes'] as Record<string, unknown> | undefined;
  const scheme = schemes?.[colorScheme] as Record<string, unknown> | undefined;
  let node: unknown = scheme;
  for (const segment of key.split('.')) {
    if (node == null || typeof node !== 'object') {
      return undefined;
    }
    node = (node as Record<string, unknown>)[segment];
  }
  return typeof node === 'string' || typeof node === 'number' ? String(node) : undefined;
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
}

/**
 * Resolves a field for preview: substitutes {{t(key)}} with its translation and {{design(key)}}
 * with the theme value (email body only); leaves {{ctx}} and anything unresolvable literal.
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
      return resolveDesignToken(parts[2], opts.theme, scheme) ?? raw;
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
): TemplateWarning[] {
  if (!text) {
    return [];
  }
  const scheme = opts.colorScheme ?? 'light';
  const warnings: TemplateWarning[] = [];
  for (const token of parseTokens(text)) {
    if (token.kind === null) {
      warnings.push({severity: 'error', code: 'unsupported', token: token.raw});
      continue;
    }
    if (token.kind === 'design') {
      if (field === 'subject' || channel === 'sms') {
        warnings.push({severity: 'error', code: 'designNotAllowed', token: token.raw});
      } else if (token.key !== undefined && resolveDesignToken(token.key, opts.theme, scheme) === undefined) {
        warnings.push({severity: 'warning', code: 'designUnresolved', token: token.raw});
      }
      continue;
    }
    if (token.kind === 't' && token.key !== undefined && opts.translations[token.key] === undefined) {
      warnings.push({severity: 'warning', code: 'missingTranslation', token: token.raw});
    }
    // ctx is the expected runtime case — left literal, not warned.
  }
  return warnings;
}

/** De-duplicated warnings across subject and body. */
export function collectWarnings(input: {
  subject?: string;
  body: string;
  channel: NotificationChannel;
  translations: Record<string, string>;
  theme?: Record<string, unknown> | null;
  colorScheme?: 'light' | 'dark';
}): TemplateWarning[] {
  const opts: PreviewResolveOptions = {
    translations: input.translations,
    theme: input.theme,
    colorScheme: input.colorScheme,
  };
  const all = [
    ...collectFieldWarnings(input.subject, 'subject', input.channel, opts),
    ...collectFieldWarnings(input.body, 'body', input.channel, opts),
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
