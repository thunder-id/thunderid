// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion} from '@thunderid/components';
import {Stack, TextField, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ConnectionNameStepProps {
  name: string;
  onNameChange: (name: string) => void;
  /** External error, e.g. a duplicate-name 409 bounced back from a later step. */
  nameError?: string | null;
}

/**
 * The name step of the "Add custom connection" wizard: a text field for the connection name plus
 * random name suggestions, mirroring the application creation wizard's name step.
 */
export default function ConnectionNameStep({
  name,
  onNameChange,
  nameError = null,
}: ConnectionNameStepProps): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <Stack direction="column" spacing={4} data-testid="connection-name-step">
      <Typography variant="h1" gutterBottom>
        {t('wizard.name.title', "Let's collect some details about your connection")}
      </Typography>

      <Stack>
        <TextField
          labelPlacement="top"
          label={t('wizard.name.fieldLabel', 'Connection name')}
          required
          fullWidth
          id="connection-name-input"
          value={name}
          error={Boolean(nameError)}
          helperText={nameError ?? undefined}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder={t('wizard.name.placeholder', 'Enter your connection name')}
          inputProps={{'data-testid': 'connection-name-input'}}
        />

        <NameSuggestion onSelect={onNameChange} />
      </Stack>
    </Stack>
  );
}
