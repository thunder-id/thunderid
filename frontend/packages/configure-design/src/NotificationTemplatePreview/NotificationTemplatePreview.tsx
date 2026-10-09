// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DefaultTheme, type Theme} from '@thunderid/design';
import {useGetTranslations} from '@thunderid/i18n';
import {Alert, Box, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {
  collectWarnings,
  resolveForPreview,
  type NotificationChannel,
  type TemplateWarning,
} from './templateTokens';

export type {NotificationChannel} from './templateTokens';

// Namespace {{t(key)}} tokens resolve under (fixed, matches the backend).
const NOTIFICATION_NAMESPACE = 'notification';

// Appended to the email preview so links/buttons are inert (the iframe is a static preview, not a
// live email) — disables pointer interaction on top of the iframe's sandbox.
const PREVIEW_INERT_STYLE =
  '<style>a,button,input[type=submit],input[type=button],[role=button]{pointer-events:none!important;cursor:default!important;}</style>';

export interface NotificationTemplatePreviewProps {
  channel: NotificationChannel;
  /** Raw subject (email only), placeholders intact. */
  subject?: string;
  /** Raw body — HTML (email) or text (SMS), placeholders intact. */
  body: string;
  /** Locale resolving {{t(key)}}; unset → not resolved. */
  locale?: string;
  /** Resolved theme (caller-provided). `undefined` → default theme; `null` → spinner. */
  theme?: Theme | null;
  colorScheme?: 'light' | 'dark';
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
}: NotificationTemplatePreviewProps): JSX.Element {
  const {t} = useTranslation('design');

  // Translations for the selected locale, notification namespace; disabled until a locale is set.
  const {data: translationsData} = useGetTranslations({
    language: locale ?? '',
    namespace: NOTIFICATION_NAMESPACE,
    enabled: Boolean(locale),
  });
  const translations = useMemo(
    () => translationsData?.translations?.[NOTIFICATION_NAMESPACE] ?? {},
    [translationsData],
  );

  // The theme GatePreview actually renders with (undefined falls back to the default theme), so
  // {{design(...)}} resolves against the same theme shown in the preview.
  const effectiveTheme = (theme === undefined ? DefaultTheme : theme) as Record<string, unknown> | null;
  const scheme = colorScheme ?? 'light';
  // null theme = caller is still loading the selected theme; don't warn on tokens we can't resolve yet.
  const themeLoading = theme === null;

  const warnings = useMemo(
    () =>
      themeLoading
        ? []
        : collectWarnings({subject, body, channel, translations, theme: effectiveTheme, colorScheme: scheme}),
    [themeLoading, subject, body, channel, translations, effectiveTheme, scheme],
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
            sx={{flex: 1, width: '100%', border: 0, bgcolor: '#fff'}}
          />
        </>
      );
    }
    return (
      <Box sx={{flex: 1, p: 3, bgcolor: '#f4f6f9', overflow: 'auto'}}>
        <Box
          sx={{
            maxWidth: 280,
            p: 1.5,
            borderRadius: 3,
            bgcolor: '#fff',
            boxShadow: 1,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            fontSize: '0.875rem',
            color: '#2b3648',
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
        'Design token "{{token}}" does not match any value in the selected theme; it is shown as-is and may not render.',
        {token},
      );
    case 'missingTranslation':
      return t(
        'notificationTemplatePreview.warnings.missingTranslation',
        'No translation for "{{token}}" in the selected language; it is shown as-is and may not render.',
        {token},
      );
    default:
      return token;
  }
}
