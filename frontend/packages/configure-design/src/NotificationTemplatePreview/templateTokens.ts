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

/** A token the preview cannot render faithfully, tagged with the field it appears in. */
export interface TemplateWarning {
  field: TemplateField;
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
  skipMissingTranslation: boolean,
): TemplateWarning[] {
  if (!text) {
    return [];
  }
  const scheme = opts.colorScheme ?? 'light';
  const warnings: TemplateWarning[] = [];
  for (const token of parseTokens(text)) {
    if (token.kind === null) {
      warnings.push({field, severity: 'error', code: 'unsupported', token: token.raw});
      continue;
    }
    if (token.kind === 'design') {
      if (field === 'subject' || channel === 'sms') {
        warnings.push({field, severity: 'error', code: 'designNotAllowed', token: token.raw});
      } else if (token.key !== undefined && resolveDesignToken(token.key, opts.theme, scheme) === undefined) {
        warnings.push({field, severity: 'warning', code: 'designUnresolved', token: token.raw});
      }
      continue;
    }
    if (token.kind === 'ctx') {
      // ctx is left literal for the runtime to substitute; but a key the runtime never injects (not in
      // the offered catalog) will render literally on send — surface it like a missing translation.
      // Only validate when the caller supplies the catalog; otherwise ctx is left unchecked.
      if (opts.contextKeys !== undefined && token.key !== undefined && !opts.contextKeys.has(token.key)) {
        warnings.push({field, severity: 'error', code: 'unknownContext', token: token.raw});
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
      warnings.push({field, severity: 'error', code: 'missingTranslation', token: token.raw});
    }
  }
  return warnings;
}

/** Warnings for subject and body, de-duplicated within each field. */
export function collectWarnings(input: {
  subject?: string;
  body: string;
  channel: NotificationChannel;
  translations: Record<string, string>;
  theme?: Record<string, unknown> | null;
  colorScheme?: 'light' | 'dark';
  /** ctx keys the runtime can inject; a {{ctx(key)}} outside this set is flagged. Unset → unchecked. */
  contextKeys?: ReadonlySet<string>;
  /** Suppress missing-translation warnings while translations are still loading / unavailable. */
  skipMissingTranslation?: boolean;
}): TemplateWarning[] {
  const opts: PreviewResolveOptions = {
    translations: input.translations,
    theme: input.theme,
    colorScheme: input.colorScheme,
    contextKeys: input.contextKeys,
  };
  const skipMissingTranslation = input.skipMissingTranslation ?? false;
  const all = [
    ...collectFieldWarnings(input.subject, 'subject', input.channel, opts, skipMissingTranslation),
    ...collectFieldWarnings(input.body, 'body', input.channel, opts, skipMissingTranslation),
  ];
  const seen = new Set<string>();
  const deduped: TemplateWarning[] = [];
  for (const warning of all) {
    const dedupeKey = `${warning.field}:${warning.code}:${warning.token}`;
    if (seen.has(dedupeKey)) {
      continue;
    }
    seen.add(dedupeKey);
    deduped.push(warning);
  }
  return deduped;
}

/** Maps a warning code to a localized message; keys carry the `design:` namespace so any `t` resolves them. */
export function templateWarningMessage(
  t: (key: string, defaultValue: string, options?: Record<string, unknown>) => string,
  warning: TemplateWarning,
): string {
  const {code, token} = warning;
  switch (code) {
    case 'unsupported':
      return t(
        'design:notificationTemplatePreview.warnings.unsupported',
        '"{{token}}" is not a supported token and will cause the template to fail to send.',
        {token},
      );
    case 'designNotAllowed':
      return t(
        'design:notificationTemplatePreview.warnings.designNotAllowed',
        'Design token "{{token}}" is only allowed in the email body, not here.',
        {token},
      );
    case 'designUnresolved':
      return t(
        'design:notificationTemplatePreview.warnings.designUnresolved',
        'Design token "{{token}}" does not match any value in the selected theme; it is shown as-is and may not render.',
        {token},
      );
    case 'missingTranslation':
      return t(
        'design:notificationTemplatePreview.warnings.missingTranslation',
        'No translation for "{{token}}" in any language; the template will fail to send until it is added.',
        {token},
      );
    case 'unknownContext':
      return t(
        'design:notificationTemplatePreview.warnings.unknownContext',
        '"{{token}}" is not a recognized context value and will not be substituted when the template is sent.',
        {token},
      );
    default:
      return token;
  }
}
