// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type Theme} from '@thunderid/design';
import {useGetTranslations} from '@thunderid/i18n';
import {Alert, Box, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {
  collectWarnings,
  NOTIFICATION_NAMESPACE,
  resolveForPreview,
  type NotificationChannel,
  type TemplateWarning,
} from './templateTokens';

export type {NotificationChannel} from './templateTokens';

// Appended to the email preview so links/buttons are inert (the iframe is a static preview, not a
// live email) — disables pointer interaction on top of the iframe's sandbox.
const PREVIEW_INERT_STYLE =
  '<style>a,button,input[type=submit],input[type=button],[role=button]{pointer-events:none!important;cursor:default!important;}</style>';

// Chrome around the rendered template, keyed by color scheme so the preview adapts to dark mode
// (mirrors GatePreview's NEUTRAL_CANVAS_BACKGROUND). `canvas` is the area behind the message,
// `surface` the message background (email frame / SMS bubble), `text` the SMS bubble text.
const PREVIEW_CHROME: Record<'light' | 'dark', {canvas: string; surface: string; text: string}> = {
  light: {canvas: '#f4f6f9', surface: '#ffffff', text: '#2b3648'},
  dark: {canvas: '#18181b', surface: '#27272a', text: '#e4e4e7'},
};

export interface NotificationTemplatePreviewProps {
  channel: NotificationChannel;
  /** Raw subject (email only), placeholders intact. */
  subject?: string;
  /** Raw body — HTML (email) or text (SMS), placeholders intact. */
  body: string;
  /** Locale resolving {{t(key)}}; unset → not resolved. */
  locale?: string;
  /** Resolved theme (caller-provided). `undefined` → no theme, design tokens render empty; `null` → spinner. */
  theme?: Theme | null;
  colorScheme?: 'light' | 'dark';
  /**
   * ctx keys the runtime can inject (the catalog the editor offers). When set, a {{ctx(key)}} outside
   * it is flagged; when omitted, ctx tokens are left unchecked.
   */
  contextKeys?: ReadonlySet<string>;
  /**
   * design keys the editor offers (the catalog). When set, a {{design(key)}} outside it is flagged,
   * independent of the selected theme; when omitted, design tokens are left unchecked.
   */
  designKeys?: ReadonlySet<string>;
}

/**
 * Presentational live preview of a notification template (email/SMS) for a given locale,
 * theme and color scheme, rendered via GatePreview. Resolves {{t(key)}}; leaves {{ctx}}
 * and {{design}} literal; warns on tokens that won't render (see `collectWarnings`).
 * Reused by the flow step preview and the template editor.
 */
export default function NotificationTemplatePreview({
  channel,
  subject = undefined,
  body,
  locale = undefined,
  theme = undefined,
  colorScheme = undefined,
  contextKeys = undefined,
  designKeys = undefined,
}: NotificationTemplatePreviewProps): JSX.Element {
  const {t} = useTranslation('design');

  // Translations for the selected locale, notification namespace; disabled until a locale is set.
  const {data: translationsData, isSuccess: translationsLoaded} = useGetTranslations({
    language: locale ?? '',
    namespace: NOTIFICATION_NAMESPACE,
    enabled: Boolean(locale),
  });
  const translations = useMemo(
    () => translationsData?.translations?.[NOTIFICATION_NAMESPACE] ?? {},
    [translationsData],
  );

  // The theme {{design(...)}} tokens resolve against. No theme selected (undefined) means no design:
  // tokens resolve to empty, matching delivery for an app with no theme. null = the caller is still
  // loading the selected theme (spinner below); don't warn on tokens we can't resolve yet.
  const effectiveTheme = (theme ?? null) as Record<string, unknown> | null;
  const scheme = colorScheme ?? 'light';
  const chrome = PREVIEW_CHROME[scheme];
  const themeLoading = theme === null;

  // Until translations have loaded (or when no locale is set), every {{t(key)}} would look missing;
  // suppress those warnings so the preview doesn't flash false errors on load or language switch.
  const skipMissingTranslation = !translationsLoaded;

  const warnings = useMemo(
    () =>
      themeLoading
        ? []
        : collectWarnings({
            subject,
            body,
            channel,
            translations,
            contextKeys,
            designKeys,
            skipMissingTranslation,
          }),
    [themeLoading, subject, body, channel, translations, contextKeys, designKeys, skipMissingTranslation],
  );

  const opts = useMemo(
    () => ({translations, theme: effectiveTheme, colorScheme: scheme}),
    [translations, effectiveTheme, scheme],
  );
  const resolvedSubject = useMemo(
    () => (subject ? resolveForPreview(subject, opts, 'subject', channel) : ''),
    [subject, opts, channel],
  );
  const resolvedBody = useMemo(() => resolveForPreview(body, opts, 'body', channel), [body, opts, channel]);

  const renderCardBody = (): JSX.Element => {
    if (themeLoading) {
      return (
        <Box sx={{flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <CircularProgress size={28} />
        </Box>
      );
    }
    if (channel === 'email') {
      return (
        <>
          {subject && (
            <Box sx={{px: 2, py: 1.25, borderBottom: '1px solid', borderColor: 'divider', flexShrink: 0}}>
              <Typography variant="caption" sx={{color: 'text.disabled'}}>
                {t('notificationTemplatePreview.subjectLabel', 'Subject')}
              </Typography>
              <Typography variant="body2" sx={{fontWeight: 600}}>
                {resolvedSubject}
              </Typography>
            </Box>
          )}
          <Box
            component="iframe"
            title={t('notificationTemplatePreview.frameTitle', 'Email preview')}
            srcDoc={resolvedBody + PREVIEW_INERT_STYLE}
            sandbox=""
            sx={{flex: 1, width: '100%', border: 0, bgcolor: chrome.surface}}
          />
        </>
      );
    }
    return (
      <Box sx={{flex: 1, p: 3, bgcolor: chrome.canvas, overflow: 'auto'}}>
        <Box
          sx={{
            maxWidth: 280,
            p: 1.5,
            borderRadius: 3,
            bgcolor: chrome.surface,
            boxShadow: 1,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            fontSize: '0.875rem',
            color: chrome.text,
          }}
        >
          {resolvedBody}
        </Box>
      </Box>
    );
  };

  return (
    <Stack spacing={1.5} sx={{width: '100%'}}>
      {warnings.length > 0 && (
        <Stack spacing={1}>
          {warnings.map((warning) => (
            <Alert key={`${warning.code}:${warning.token}`} severity={warning.severity} sx={{py: 0.25}}>
              {warningMessage(t, warning)}
            </Alert>
          ))}
        </Stack>
      )}

      {/* Floating card. Email renders the resolved HTML as a real document (iframe srcDoc) so
          {{design}}-driven styles, <style> blocks and body-level CSS apply like an email client;
          SMS renders the resolved text. */}
      <Box sx={{display: 'flex', justifyContent: 'center', width: '100%', p: 2, boxSizing: 'border-box'}}>
        <Box
          sx={{
            width: '100%',
            maxWidth: channel === 'sms' ? 360 : 640,
            height: channel === 'sms' ? 400 : 520,
            borderRadius: 2,
            boxShadow: 8,
            overflow: 'hidden',
            bgcolor: 'background.paper',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {renderCardBody()}
        </Box>
      </Box>
    </Stack>
  );
}

/** Maps a warning code to a localized message. */
function warningMessage(
  t: (key: string, defaultValue: string, options?: Record<string, unknown>) => string,
  warning: TemplateWarning,
): string {
  const {code, token} = warning;
  switch (code) {
    case 'unsupported':
      return t(
        'notificationTemplatePreview.warnings.unsupported',
        '"{{token}}" is not a supported token and will cause the template to fail to send.',
        {token},
      );
    case 'designNotAllowed':
      return t(
        'notificationTemplatePreview.warnings.designNotAllowed',
        'Design token "{{token}}" is only allowed in the email body, not here.',
        {token},
      );
    case 'designUnresolved':
      return t(
        'notificationTemplatePreview.warnings.designUnresolved',
        'Design token "{{token}}" is not a recognized design token; it may not render as expected.',
        {token},
      );
    case 'missingTranslation':
      return t(
        'notificationTemplatePreview.warnings.missingTranslation',
        'No translation for "{{token}}" in any language; the template will fail to send until it is added.',
        {token},
      );
    case 'unknownContext':
      return t(
        'notificationTemplatePreview.warnings.unknownContext',
        '"{{token}}" is not a recognized context value and will not be substituted when the template is sent.',
        {token},
      );
    default:
      return token;
  }
}
