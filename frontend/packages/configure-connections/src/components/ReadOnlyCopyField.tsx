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

import {useToast} from '@thunderid/contexts';
import {Box, Button, FormControl, FormHelperText, FormLabel, TextField} from '@wso2/oxygen-ui';
import {Copy} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ReadOnlyCopyFieldProps {
  id: string;
  label: string;
  value: string;
  helperText?: string;
}

export default function ReadOnlyCopyField({
  id,
  label,
  value,
  helperText = undefined,
}: ReadOnlyCopyFieldProps): JSX.Element {
  const {t} = useTranslation('connections');
  const {showToast} = useToast();

  const handleCopy = (): void => {
    if (!navigator.clipboard?.writeText) {
      return;
    }

    navigator.clipboard
      .writeText(value)
      .then(() => showToast(t('form.copied'), 'success'))
      .catch(() => {
        // Clipboard write can fail silently (e.g. permissions); no user-facing error needed.
      });
  };

  return (
    <FormControl fullWidth>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <Box sx={{display: 'flex', gap: 1, alignItems: 'flex-start'}}>
        <TextField
          id={id}
          fullWidth
          value={value}
          slotProps={{input: {readOnly: true, sx: {fontFamily: 'monospace'}}}}
        />
        <Button variant="outlined" startIcon={<Copy size={16} />} onClick={handleCopy} data-testid={`${id}-copy`}>
          {t('form.copy')}
        </Button>
      </Box>
      {helperText && <FormHelperText>{helperText}</FormHelperText>}
    </FormControl>
  );
}
