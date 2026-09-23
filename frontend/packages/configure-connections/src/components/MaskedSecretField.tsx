// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  InputAdornment,
  TextField,
} from '@wso2/oxygen-ui';
import {Eye, EyeOff, Lock, RotateCcw} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useState} from 'react';
import {useTranslation} from 'react-i18next';

interface MaskedSecretFieldProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** True when editing a connection whose secret is already stored (masked on the API). */
  hasStoredSecret: boolean;
  /** Whether the user has chosen to replace the stored secret. */
  replacing: boolean;
  onReplacingChange: (replacing: boolean) => void;
  error?: string;
  hint?: string;
  required?: boolean;
  /**
   * Renders a textarea instead of a single-line input, for secrets that carry newlines (a PEM
   * private key, for example). A single-line input strips the line breaks on paste, which
   * corrupts the value. Multiline secrets are not masked while being entered, because a textarea
   * cannot be a password field; they are still write-only once stored.
   */
  multiline?: boolean;
}

export default function MaskedSecretField({
  id,
  label,
  value,
  onChange,
  hasStoredSecret,
  replacing,
  onReplacingChange,
  error = undefined,
  hint = undefined,
  required = false,
  multiline = false,
}: MaskedSecretFieldProps): JSX.Element {
  const {t} = useTranslation('connections');
  const [visible, setVisible] = useState(false);

  // Stored secret that the user has not chosen to replace yet: show a locked, read-only field.
  if (hasStoredSecret && !replacing) {
    return (
      <FormControl fullWidth>
        <FormLabel htmlFor={id}>{label}</FormLabel>
        <Box sx={{display: 'flex', gap: 1, alignItems: 'flex-start'}}>
          <TextField
            id={id}
            fullWidth
            disabled
            value="••••••••••••••••"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Lock size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
          <Button
            variant="outlined"
            startIcon={<RotateCcw size={16} />}
            onClick={() => onReplacingChange(true)}
            data-testid={`${id}-replace`}
          >
            {t('form.secret.update')}
          </Button>
        </Box>
        <FormHelperText>{t('form.secret.keepHelp')}</FormHelperText>
      </FormControl>
    );
  }

  if (multiline) {
    return (
      <FormControl fullWidth required={required} error={Boolean(error)}>
        <FormLabel htmlFor={id}>{label}</FormLabel>
        <TextField
          id={id}
          fullWidth
          multiline
          minRows={4}
          maxRows={12}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          error={Boolean(error)}
          helperText={error ?? hint}
          slotProps={{input: {autoComplete: 'off'}}}
        />
      </FormControl>
    );
  }

  return (
    <FormControl fullWidth required={required} error={Boolean(error)}>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <TextField
        id={id}
        fullWidth
        type={visible ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        error={Boolean(error)}
        helperText={error ?? hint}
        slotProps={{
          input: {
            endAdornment: (
              <InputAdornment position="end">
                <IconButton
                  onClick={() => setVisible((prev) => !prev)}
                  edge="end"
                  size="small"
                  aria-label="toggle secret visibility"
                >
                  {visible ? <EyeOff size={16} /> : <Eye size={16} />}
                </IconButton>
              </InputAdornment>
            ),
          },
        }}
      />
    </FormControl>
  );
}
