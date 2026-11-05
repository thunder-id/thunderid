// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion} from '@thunderid/components';
import {Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {type ChangeEvent, type JSX, useEffect} from 'react';
import {useTranslation} from 'react-i18next';

export interface ConfigureThemeNameProps {
  themeName: string;
  onThemeNameChange: (name: string) => void;
  onReadyChange?: (isReady: boolean) => void;
}

export default function ConfigureThemeName({
  themeName,
  onThemeNameChange,
  onReadyChange = () => null,
}: ConfigureThemeNameProps): JSX.Element {
  const {t} = useTranslation('design');

  useEffect(() => {
    onReadyChange?.(themeName.trim().length > 0);
  }, [themeName, onReadyChange]);

  return (
    <Stack direction="column" spacing={4}>
      <Typography variant="h1">
        {t('themes.forms.configure_name.title', "Let's collect some details about your theme")}
      </Typography>

      <Stack>
        <TextField
          labelPlacement="top"
          label={t('themes.forms.configure_name.fieldLabel', 'Theme name')}
          required
          fullWidth
          id="theme-name-input"
          value={themeName}
          onChange={(e: ChangeEvent<HTMLInputElement>) => onThemeNameChange(e.target.value)}
          placeholder={t('themes.forms.configure_name.placeholder', 'e.g. Solarized Light')}
        />

        <NameSuggestion onSelect={onThemeNameChange} />
      </Stack>
    </Stack>
  );
}
