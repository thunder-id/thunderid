// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NotificationTemplatePreview, type TemplateWarning} from '@thunderid/configure-design';
import {useGetThemes, useGetTheme, type Theme} from '@thunderid/design';
import {getDisplayNameForCode, I18nDefaultConstants, toFlagEmoji, useGetLanguages} from '@thunderid/i18n';
import {Box, MenuItem, TextField, Typography} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {CONTEXT_TOKEN_KEYS} from '../../constants/template-tokens';
import type {NotificationChannel, TemplateColorScheme} from '../../models/notification-template';

/**
 * Props for the {@link TemplatePreviewPanel} component.
 *
 * @public
 */
export interface TemplatePreviewPanelProps {
  channel: NotificationChannel;
  /** Current subject (email only), placeholders intact. */
  subject?: string;
  /** Current body, placeholders intact. */
  body: string;
  /** Color scheme the email body is authored against. */
  colorScheme: TemplateColorScheme;
  /** Receives the preview's token warnings for the selected language and theme. */
  onWarningsChange?: (warnings: TemplateWarning[]) => void;
}

/**
 * Right-hand preview panel: a Language selector (`/i18n/languages`), a Design-theme selector
 * (`/design/themes`), and a live preview rendered for the selected locale + theme + color scheme.
 *
 * @public
 */
export default function TemplatePreviewPanel({
  channel,
  subject = undefined,
  body,
  colorScheme,
  onWarningsChange = undefined,
}: TemplatePreviewPanelProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const isEmail = channel === 'email';

  const {data: languagesData, isLoading: languagesLoading} = useGetLanguages();
  const {data: themesData, isLoading: themesLoading} = useGetThemes();

  const themes = themesData?.themes ?? [];

  // The default option resolves the base locale; drop it from the list so it does not appear twice.
  const defaultLocale = I18nDefaultConstants.FALLBACK_LANGUAGE;
  const languages = useMemo(
    () => (languagesData?.languages ?? []).filter((code) => code !== defaultLocale),
    [languagesData, defaultLocale],
  );

  const [language, setLanguage] = useState('');
  const [themeId, setThemeId] = useState('');

  // Selected theme drives the preview styling. `undefined` → default theme; `null` → spinner while
  // the chosen theme loads. On error or a theme with no data, fall back to the default theme so the
  // preview renders rather than spinning forever.
  const {data: selectedTheme, isLoading: selectedThemeLoading} = useGetTheme(themeId);
  let previewTheme: Theme | null | undefined;
  if (!themeId) {
    previewTheme = undefined;
  } else if (selectedThemeLoading) {
    previewTheme = null;
  } else {
    previewTheme = selectedTheme?.theme ?? undefined;
  }

  return (
    <Box sx={{position: 'sticky', top: 0}}>
      <Typography variant="subtitle2" sx={{fontWeight: 600, mb: 1.75}}>
        {t('editor.preview.title', 'Preview')}
      </Typography>

      {/* Design theme only applies to email; SMS has no design, so just the language selector. */}
      <Box sx={{display: 'grid', gridTemplateColumns: isEmail ? '1fr 1fr' : '1fr', gap: 1.5, mb: 1.75}}>
        <Box>
          <Typography variant="body2" sx={{mb: 0.75, color: 'text.secondary', fontWeight: 500}}>
            {t('editor.preview.language', 'Language')}
          </Typography>
          <TextField
            select
            fullWidth
            size="small"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            disabled={languagesLoading}
            SelectProps={{displayEmpty: true}}
          >
            <MenuItem value="">{t('editor.preview.languageDefault', 'English (US)')}</MenuItem>
            {languages.map((code) => (
              <MenuItem key={code} value={code}>
                {`${toFlagEmoji(code)} ${getDisplayNameForCode(code)} (${code})`}
              </MenuItem>
            ))}
          </TextField>
        </Box>

        {isEmail && (
          <Box>
            <Typography variant="body2" sx={{mb: 0.75, color: 'text.secondary', fontWeight: 500}}>
              {t('editor.preview.designTheme', 'Design theme')}
            </Typography>
            <TextField
              select
              fullWidth
              size="small"
              value={themeId}
              onChange={(e) => setThemeId(e.target.value)}
              disabled={themesLoading}
              SelectProps={{displayEmpty: true}}
            >
              <MenuItem value="">{t('editor.preview.designThemeDefault', 'Default design theme')}</MenuItem>
              {themes.map((theme) => (
                <MenuItem key={theme.id} value={theme.id}>
                  {theme.displayName}
                </MenuItem>
              ))}
            </TextField>
          </Box>
        )}
      </Box>

      <NotificationTemplatePreview
        channel={channel}
        subject={subject}
        body={body}
        locale={language || defaultLocale}
        theme={previewTheme}
        colorScheme={colorScheme}
        contextKeys={CONTEXT_TOKEN_KEYS}
        onWarningsChange={onWarningsChange}
      />
    </Box>
  );
}
