// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Divider, MenuItem, Stack, Switch, TextField, Typography} from '@wso2/oxygen-ui';
import {useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {AccountLockCategories, type AccountLockCategory} from '../../models/accountAccess';
import type {AccountLockForm} from '../../utils/accountLockPayload';
import {formatDurationShort} from '../../utils/describePolicy';

/** The update intervals offered, in seconds. `0` writes on every sign-in. */
const RESOLUTION_PRESETS: readonly number[] = [0, 300, 900, 3600, 86400];

/** Fallback copy for the per-category keys, matching en-US. */
const CATEGORY_FALLBACKS: Record<AccountLockCategory, {title: string; description: string}> = {
  user: {title: 'Users', description: 'Record when each user last signed in.'},
  agent: {title: 'Agents', description: 'Record when each agent last authenticated.'},
};

/**
 * Props for {@link SignInActivitySettings}.
 *
 * @public
 */
export interface SignInActivitySettingsProps {
  /** The current values, by category. */
  forms: Record<AccountLockCategory, AccountLockForm>;
  /** Called with the changed fields of one category. */
  onChange: (category: AccountLockCategory, patch: Partial<AccountLockForm>) => void;
  /** Whether the settings are read-only, as when they are managed declaratively. */
  disabled?: boolean;
}

/**
 * Whether each category records its last sign-in, and how often the recorded time is refreshed.
 *
 * A sibling of the lock cards rather than part of them, so switching locking off never switches
 * recording off. The interval is a field of each category, so each switch has its own. A stored
 * interval that is not a preset is listed as it is, so a save never rewrites it.
 *
 * @param props - The values, the change handler, and whether they are read-only
 * @returns The rendered settings
 *
 * @public
 */
export default function SignInActivitySettings({
  forms,
  onChange,
  disabled = false,
}: SignInActivitySettingsProps): JSX.Element {
  const {t} = useTranslation();

  // Resolves a bare key in the settings catalog, which is what the duration formatter expects.
  const tSettings = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(`settings:${key}`, options ?? {}),
    [t],
  );

  const resolutionLabel = (seconds: number): string =>
    seconds === 0
      ? t('settings:signInActivity.resolution.every', 'On every sign-in')
      : t('settings:signInActivity.resolution.atMost', 'At most every {{duration}}', {
          duration: formatDurationShort(seconds, tSettings),
        });

  return (
    <Stack spacing={2} divider={<Divider />}>
      {AccountLockCategories.map((category) => {
        const form: AccountLockForm = forms[category];
        const title: string = t(`settings:signInActivity.${category}.title`, CATEGORY_FALLBACKS[category].title);
        const options: number[] = RESOLUTION_PRESETS.includes(form.lastLoginResolutionSeconds)
          ? [...RESOLUTION_PRESETS]
          : [...RESOLUTION_PRESETS, form.lastLoginResolutionSeconds].sort((a, b) => a - b);
        const selectLabel: string = t('settings:signInActivity.resolution.label', 'Update the recorded time');

        return (
          <Box key={category} component="section" aria-label={title} data-testid={`sign-in-activity-${category}`}>
            <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={2}>
              <Box>
                <Typography variant="subtitle2">{title}</Typography>
                <Typography variant="body2" color="text.secondary">
                  {t(`settings:signInActivity.${category}.description`, CATEGORY_FALLBACKS[category].description)}
                </Typography>
              </Box>
              <Switch
                checked={form.recordLastLogin}
                disabled={disabled}
                onChange={(event) => onChange(category, {recordLastLogin: event.target.checked})}
                // Overriding slotProps.input drops MUI's default switch role, so restate it.
                slotProps={{
                  input: {
                    role: 'switch',
                    'aria-label': t(
                      'settings:signInActivity.toggle.ariaLabel',
                      'Record sign-in activity for {{title}}',
                      {
                        title,
                      },
                    ),
                  },
                }}
              />
            </Stack>
            {form.recordLastLogin && (
              <Box sx={{mt: 2, maxWidth: 400}}>
                <TextField
                  select
                  fullWidth
                  size="small"
                  label={selectLabel}
                  value={String(form.lastLoginResolutionSeconds)}
                  disabled={disabled}
                  helperText={t(
                    'settings:signInActivity.resolution.hint',
                    'Updating less often saves writes, and the recorded time can be behind by up to this long.',
                  )}
                  data-testid={`sign-in-activity-resolution-${category}`}
                  onChange={(event) => onChange(category, {lastLoginResolutionSeconds: Number(event.target.value)})}
                >
                  {options.map((seconds) => (
                    <MenuItem key={seconds} value={String(seconds)}>
                      {resolutionLabel(seconds)}
                    </MenuItem>
                  ))}
                </TextField>
              </Box>
            )}
          </Box>
        );
      })}
    </Stack>
  );
}
