/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {generateRandomHumanReadableIdentifiers} from '@thunderid/utils';
import {Box, Typography, Stack, TextField, Chip, FormControl, FormLabel, useTheme} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX} from 'react';
import {useMemo, useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import deriveHandle from '@/lib/deriveHandle';

export interface ConfigureNameProps {
  name: string;
  handle: string;
  handleEdited: boolean;
  onNameChange: (name: string) => void;
  onHandleChange: (handle: string) => void;
  onHandleEditedChange: (edited: boolean) => void;
  onReadyChange?: (isReady: boolean) => void;
}

/**
 * Step 1 of the credential configuration creation wizard: configure the name
 * and the handle derived from it (editable, but no longer auto-derived once touched).
 */
export default function ConfigureName({
  name,
  handle,
  handleEdited,
  onNameChange,
  onHandleChange,
  onHandleEditedChange,
  onReadyChange = undefined,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation('verifiable-credentials');
  const theme = useTheme();

  const nameSuggestions: string[] = useMemo((): string[] => generateRandomHumanReadableIdentifiers(), []);

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(name.trim().length > 0 && handle.trim().length > 0);
    }
  }, [name, handle, onReadyChange]);

  const handleNameChange = (e: ChangeEvent<HTMLInputElement>): void => {
    const newName = e.target.value;
    onNameChange(newName);
    if (!handleEdited) {
      onHandleChange(deriveHandle(newName));
    }
  };

  const handleSuggestionClick = (suggestion: string): void => {
    onNameChange(suggestion);
    onHandleChange(deriveHandle(suggestion));
    onHandleEditedChange(false);
  };

  const handleHandleChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onHandleEditedChange(true);
    onHandleChange(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''));
  };

  return (
    <Stack direction="column" spacing={4} data-testid="configure-name">
      <FormControl fullWidth required>
        <FormLabel htmlFor="vc-name-input">{t('form.name.label')}</FormLabel>
        <TextField
          fullWidth
          id="vc-name-input"
          value={name}
          onChange={handleNameChange}
          placeholder={t('form.name.placeholder')}
          helperText={t('form.name.hint')}
        />
      </FormControl>

      <Stack direction="column" spacing={2}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
          <Typography variant="body2" color="text.secondary">
            {t('createWizard.name.suggestions.label')}
          </Typography>
        </Stack>
        <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1}}>
          {nameSuggestions.map(
            (suggestion: string): JSX.Element => (
              <Chip
                key={suggestion}
                label={suggestion}
                onClick={(): void => handleSuggestionClick(suggestion)}
                variant="outlined"
                clickable
                sx={{
                  '&:hover': {
                    bgcolor: 'primary.main',
                    color: 'text.primary',
                    borderColor: 'primary.main',
                  },
                }}
              />
            ),
          )}
        </Box>
      </Stack>

      <FormControl fullWidth required>
        <FormLabel htmlFor="vc-handle-input">{t('form.handle.label')}</FormLabel>
        <TextField
          fullWidth
          id="vc-handle-input"
          value={handle}
          onChange={handleHandleChange}
          placeholder="eudi-pid"
          helperText={t('form.handle.hint')}
        />
      </FormControl>
    </Stack>
  );
}
