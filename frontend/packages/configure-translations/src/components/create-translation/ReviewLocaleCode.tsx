// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getDisplayNameForCode, toFlagEmoji} from '@thunderid/i18n';
import type {LocaleOption} from '@thunderid/i18n';
import {Box, Chip, Stack, TextField, Typography, useTheme} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link ReviewLocaleCode} component.
 *
 * @public
 */
export interface ReviewLocaleCodeProps {
  /** The locale code derived from the previous steps — used as the default value. */
  derivedLocale: LocaleOption;
  /** The current override value entered by the user (controlled). */
  localeCode: string;
  /** Callback invoked when the user edits the locale code input. */
  onLocaleCodeChange: (code: string) => void;
  /** Callback invoked whenever the step readiness changes (e.g. input becomes non-empty). */
  onReadyChange?: (isReady: boolean) => void;
}

/**
 * Step component in the language creation wizard that allows the user to review
 * and optionally override the BCP 47 locale code derived from the country and
 * language selections.
 *
 * Shows a preview of the flag emoji and resolved display name for the effective
 * locale code, along with a helper tip about the BCP 47 format.
 *
 * @param props - The component props
 * @param props.derivedLocale - Locale derived from the previous wizard steps, used as the default
 * @param props.localeCode - Current user-entered override value (controlled)
 * @param props.onLocaleCodeChange - Callback invoked when the locale code input changes
 * @param props.onReadyChange - Callback invoked when step readiness changes
 *
 * @returns JSX element rendering the locale code review step
 *
 * @example
 * ```tsx
 * import ReviewLocaleCode from './ReviewLocaleCode';
 *
 * function Wizard() {
 *   const [code, setCode] = useState('');
 *   return (
 *     <ReviewLocaleCode
 *       derivedLocale={{code: 'fr-FR', displayName: 'French (France)', flag: '🇫🇷'}}
 *       localeCode={code}
 *       onLocaleCodeChange={setCode}
 *       onReadyChange={(ready) => setStepReady(ready)}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ReviewLocaleCode({
  derivedLocale,
  localeCode,
  onLocaleCodeChange,
  onReadyChange = undefined,
}: ReviewLocaleCodeProps): JSX.Element {
  const theme = useTheme();
  const {t} = useTranslation('translations');

  const effectiveCode = localeCode.trim() || derivedLocale.code;
  const isLocaleCodeValid = /^[A-Za-z]{2,3}(?:-[A-Za-z]{4})?(?:-(?:[A-Za-z]{2}|\d{3}))?$/.test(effectiveCode);

  const resolvedName = useMemo(() => getDisplayNameForCode(effectiveCode), [effectiveCode]);

  const previewFlag = toFlagEmoji(effectiveCode.split('-')[1]?.toUpperCase() ?? '');

  useEffect(() => {
    onReadyChange?.(isLocaleCodeValid);
  }, [isLocaleCodeValid, onReadyChange]);

  return (
    <Stack spacing={4}>
      <Box>
        <Typography variant="h1" gutterBottom>
          {t('language.create.localeCode.title')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t('language.create.localeCode.subtitle')}
        </Typography>
      </Box>

      <Box>
        <TextField
          labelPlacement="top"
          label={t('language.add.code.label')}
          required
          id="locale-code-input"
          placeholder={derivedLocale.code}
          value={localeCode}
          onChange={(e) => onLocaleCodeChange(e.target.value)}
          fullWidth
        />

        {effectiveCode && (
          <Stack direction="row" alignItems="center" spacing={1} sx={{mt: 1.5}}>
            <Typography sx={{fontSize: '1.1rem', lineHeight: 1}}>{previewFlag}</Typography>
            {resolvedName && (
              <Typography variant="body2" color="text.secondary">
                {resolvedName}
              </Typography>
            )}
            <Chip
              label={effectiveCode}
              size="small"
              variant="outlined"
              sx={{fontFamily: 'monospace', fontSize: '0.7rem'}}
            />
          </Stack>
        )}
      </Box>

      <Stack direction="row" alignItems="center" spacing={1}>
        <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
        <Typography variant="body2" color="text.secondary">
          {t('language.add.code.helperText')}
        </Typography>
      </Stack>
    </Stack>
  );
}
