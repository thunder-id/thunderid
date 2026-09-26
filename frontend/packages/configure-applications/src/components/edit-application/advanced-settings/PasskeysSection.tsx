// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Box, Button, FormControl, FormLabel, IconButton, Stack, TextField, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface PasskeysSectionProps {
  /**
   * The current list of allowed origins for passkey operations.
   */
  allowedOrigins?: string[];
  /**
   * Callback invoked when the list changes. Omit to render read-only.
   */
  onPasskeysChange?: (origins: string[]) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
  /**
   * Called whenever the section's validation state changes. `true` means at least one origin is
   * empty or not a valid URL, so the parent can block Save.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

const isValidURL = (value: string): boolean => {
  try {
    return Boolean(new URL(value));
  } catch {
    return false;
  }
};

export default function PasskeysSection({
  allowedOrigins = undefined,
  onPasskeysChange = undefined,
  disabled = false,
  onValidationChange = undefined,
}: PasskeysSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const [errors, setErrors] = useState<Record<number, string>>({});

  const origins = allowedOrigins ?? [];
  const isEditable = Boolean(onPasskeysChange) && !disabled;

  // An empty list renders as a bare "Add Origin" button with no field to type into, which reads as
  // broken rather than "nothing added yet". Show one empty row by default instead; typing into it
  // (or removing it) operates on the real (currently empty) `origins` array via the index above.
  const displayOrigins = origins.length > 0 ? origins : [''];

  const hasInvalidOrigins = origins.length > 0 && origins.some((o) => !o.trim() || !isValidURL(o));

  useEffect(() => {
    onValidationChange?.(hasInvalidOrigins);
  }, [hasInvalidOrigins, onValidationChange]);

  const commit = (next: string[]): void => {
    if (!isEditable) return;
    onPasskeysChange?.(next);
  };

  const handleChange = (index: number, value: string): void => {
    const next = [...origins];
    next[index] = value;
    if (value.trim()) {
      setErrors((prev) => {
        const copy = {...prev};
        delete copy[index];
        return copy;
      });
    }
    commit(next);
  };

  const handleBlur = (index: number): void => {
    const value = origins[index] ?? '';
    if (!value.trim()) {
      setErrors((prev) => ({
        ...prev,
        [index]: t('applications:edit.advanced.passkeys.allowedOrigins.error.empty', 'Origin cannot be empty'),
      }));
      return;
    }
    if (!isValidURL(value)) {
      setErrors((prev) => ({
        ...prev,
        [index]: t('applications:edit.advanced.passkeys.allowedOrigins.error.invalid', 'Enter a valid URL'),
      }));
    }
  };

  // Appends relative to what's on screen (displayOrigins), not the possibly-empty backing
  // `origins` array — otherwise the first click while the list is empty turns [] into [''],
  // which renders identically to the placeholder row already shown and looks like nothing happened.
  const handleAdd = (): void => {
    commit([...displayOrigins, '']);
  };

  const handleRemove = (index: number): void => {
    const next = origins.filter((_, i) => i !== index);
    setErrors((prev) => {
      const copy: Record<number, string> = {};
      Object.entries(prev).forEach(([key, value]) => {
        const i = parseInt(key, 10);
        if (i < index) copy[i] = value;
        else if (i > index) copy[i - 1] = value;
      });
      return copy;
    });
    commit(next);
  };

  return (
    <SettingsCard
      title={t('applications:edit.advanced.labels.passkeys', 'Passkeys')}
      description={t('applications:edit.advanced.passkeys.intro', 'Passkey settings for this application.')}
    >
      <FormControl fullWidth>
        <FormLabel>{t('applications:edit.advanced.passkeys.allowedOrigins.label', 'Allowed Origins')}</FormLabel>
        <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
          {t(
            'applications:edit.advanced.passkeys.allowedOrigins.hint',
            'Allowed origins for passkey operations initiated through this application.',
          )}
        </Typography>
        <Stack spacing={2} sx={{mt: 1}}>
          {displayOrigins.map((origin, index) => (
            // eslint-disable-next-line react/no-array-index-key
            <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
              <FormControl fullWidth sx={{flex: 1}}>
                <TextField
                  fullWidth
                  id={`app-passkey-origin-${index}`}
                  value={origin}
                  onChange={(e) => handleChange(index, e.target.value)}
                  onBlur={() => handleBlur(index)}
                  error={!!errors[index]}
                  helperText={errors[index]}
                  placeholder={t(
                    'applications:edit.advanced.passkeys.allowedOrigins.placeholder',
                    'https://app.example.com',
                  )}
                  disabled={!isEditable}
                />
              </FormControl>
              {isEditable && (
                <Tooltip title={t('common:actions.delete', 'Delete')}>
                  <IconButton onClick={() => handleRemove(index)} color="error" sx={{mt: 1}}>
                    <Trash size={20} />
                  </IconButton>
                </Tooltip>
              )}
            </Stack>
          ))}
          {isEditable && (
            <Box>
              <Button variant="text" color="primary" startIcon={<Plus />} onClick={handleAdd} size="small">
                {t('applications:edit.advanced.passkeys.allowedOrigins.addOrigin', 'Add Origin')}
              </Button>
            </Box>
          )}
        </Stack>
      </FormControl>
    </SettingsCard>
  );
}
