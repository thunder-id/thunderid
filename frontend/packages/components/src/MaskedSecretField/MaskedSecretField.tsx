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
import {type JSX, useEffect, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';

export interface MaskedSecretFieldProps {
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
  showStoredControls?: boolean;
  /** Shows the secret without letting it be replaced or edited. */
  disabled?: boolean;
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
  showStoredControls = true,
  disabled = false,
}: MaskedSecretFieldProps): JSX.Element {
  const {t} = useTranslation('common');
  const [visible, setVisible] = useState(false);
  // The value before Update, restored by Cancel, and what to focus once either has rendered.
  const valueBeforeUpdate = useRef(value);
  const pendingFocus = useRef<'input' | 'update' | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const updateRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (pendingFocus.current === 'input') inputRef.current?.focus();
    if (pendingFocus.current === 'update') updateRef.current?.focus();
    pendingFocus.current = null;
  }, [replacing]);

  const startUpdate = (): void => {
    valueBeforeUpdate.current = value;
    pendingFocus.current = 'input';
    onReplacingChange(true);
  };

  const cancelUpdate = (): void => {
    onChange(valueBeforeUpdate.current);
    pendingFocus.current = 'update';
    onReplacingChange(false);
  };

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
          {showStoredControls && !disabled && (
            <Button
              variant="outlined"
              startIcon={<RotateCcw size={16} />}
              onClick={startUpdate}
              ref={updateRef}
              data-testid={`${id}-replace`}
            >
              {t('maskedSecretField.update', 'Update')}
            </Button>
          )}
        </Box>
        {showStoredControls && !disabled && (
          <FormHelperText>
            {t('maskedSecretField.keepHelp', 'Leave unchanged to keep the stored secret.')}
          </FormHelperText>
        )}
      </FormControl>
    );
  }

  const canCancel: boolean = hasStoredSecret && showStoredControls && !disabled;

  return (
    <Box sx={{display: 'flex', gap: 1, alignItems: 'flex-start'}}>
      <TextField
        labelPlacement="top"
        label={label}
        required={required}
        id={id}
        fullWidth
        type={visible ? 'text' : 'password'}
        value={value}
        disabled={disabled}
        inputRef={inputRef}
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
                  aria-label={
                    visible ? t('maskedSecretField.hide', 'Hide secret') : t('maskedSecretField.show', 'Show secret')
                  }
                >
                  {visible ? <EyeOff size={16} /> : <Eye size={16} />}
                </IconButton>
              </InputAdornment>
            ),
          },
        }}
      />
      {canCancel && (
        <Button variant="text" onClick={cancelUpdate} data-testid={`${id}-cancel`} sx={{mt: 3.5}}>
          {t('maskedSecretField.cancel', 'Cancel')}
        </Button>
      )}
    </Box>
  );
}
