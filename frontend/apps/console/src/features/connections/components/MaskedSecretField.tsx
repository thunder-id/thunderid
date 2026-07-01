/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
