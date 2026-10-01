// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AllowedOriginRow,
  AllowedOriginRowIssueFallbacks,
  AllowedOriginTypes,
  createRow,
  validateAllowedOriginRows,
  type AllowedOriginDraftRow,
  type AllowedOriginRowError,
  type AllowedOriginRowWarning,
} from '@thunderid/configure-settings';
import {Box, Button, FormControl, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';

interface CorsOriginsEditorProps {
  rows: AllowedOriginDraftRow[];
  onRowsChange: (rows: AllowedOriginDraftRow[]) => void;
}

/**
 * The Configuration step's CORS Allowed Origins editor. Each row states whether it is an exact
 * origin or a regular expression, so nothing is reclassified from its text on the way to the
 * deployment's allow-list.
 *
 * Validation messages are resolved from the `settings` namespace, which the Settings page's CORS
 * card uses too, so both surfaces phrase the same rule identically.
 */
export default function CorsOriginsEditor({rows, onRowsChange}: CorsOriginsEditorProps): JSX.Element {
  const {t} = useTranslation();
  // Messages stay hidden until a row has been blurred, so a half-typed origin doesn't shout.
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  // An empty list renders as a bare "Add Origin" button with no field to type into, which reads as
  // broken rather than "nothing added yet". Show one empty row instead; editing it commits the real
  // (currently empty) list upward. The placeholder row is created once, so typing into it doesn't
  // remount the field on every keystroke.
  const placeholderRow = useMemo(() => createRow(AllowedOriginTypes.ORIGIN), []);
  const displayRows = rows.length > 0 ? rows : [placeholderRow];

  const issues = validateAllowedOriginRows(displayRows);

  /**
   * Resolves a row's issue message, or `undefined` while the row has no issue or has not been
   * blurred yet.
   *
   * @param codes - The issue codes for every row, keyed by row id
   * @param id - The row to resolve a message for
   * @returns The localized message, or `undefined` when the row has nothing to say
   */
  const messageFor = (
    codes: Record<string, AllowedOriginRowError | AllowedOriginRowWarning>,
    id: string,
  ): string | undefined => {
    const code = codes[id];
    if (!touched[id] || !code) {
      return undefined;
    }
    return t(`settings:cors.validation.${code}`, AllowedOriginRowIssueFallbacks[code]);
  };

  /**
   * Commits a change to one row upward, leaving every other row as it is. Editing the placeholder row
   * is what turns it into the first real entry.
   *
   * @param id - The row to change
   * @param patch - The fields to overwrite on that row
   */
  const updateRow = (id: string, patch: Partial<AllowedOriginDraftRow>): void => {
    onRowsChange(displayRows.map((row) => (row.id === id ? {...row, ...patch} : row)));
  };

  return (
    <FormControl fullWidth>
      <FormLabel>{t('applications:onboarding.configure.details.corsOrigins.title', 'CORS Allowed Origins')}</FormLabel>
      <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
        {t(
          'applications:onboarding.configure.details.corsOrigins.description',
          'Origins allowed to make cross-origin requests to the token and userinfo endpoints. Each entry is either an exact origin or a regular expression.',
        )}
      </Typography>

      <Stack spacing={2}>
        {displayRows.map((row) => (
          <AllowedOriginRow
            key={row.id}
            testId="application-cors-origin-row"
            type={row.type}
            value={row.value}
            error={messageFor(issues.errors, row.id)}
            warning={messageFor(issues.warnings, row.id)}
            originPlaceholder={t(
              'applications:onboarding.configure.details.corsOrigins.placeholder',
              'https://example.com',
            )}
            regexPlaceholder={t(
              'applications:onboarding.configure.details.corsOrigins.regexPlaceholder',
              '^https://[a-z0-9-]+\\.example\\.com$',
            )}
            typeLabel={t('settings:cors.type.label', 'Entry type')}
            originOptionLabel={t('settings:cors.type.origin', 'Origin')}
            regexOptionLabel={t('settings:cors.type.regex', 'Regex')}
            removeLabel={t('applications:onboarding.configure.details.corsOrigins.removeOrigin', 'Remove Origin')}
            onTypeChange={(type) => {
              setTouched((prev) => ({...prev, [row.id]: true}));
              updateRow(row.id, {type});
            }}
            onChange={(value) => updateRow(row.id, {value})}
            onBlur={() => setTouched((prev) => ({...prev, [row.id]: true}))}
            onRemove={() => onRowsChange(displayRows.filter((candidate) => candidate.id !== row.id))}
          />
        ))}

        <Box>
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<Plus />}
            onClick={() => onRowsChange([...displayRows, createRow(AllowedOriginTypes.ORIGIN)])}
          >
            {t('applications:onboarding.configure.details.corsOrigins.addOrigin', 'Add Origin')}
          </Button>
        </Box>
      </Stack>
    </FormControl>
  );
}
