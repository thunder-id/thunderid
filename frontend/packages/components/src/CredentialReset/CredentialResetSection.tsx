// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CredentialResetDialog, {
  type CredentialResetField,
  type CredentialResetDialogProps,
} from './CredentialResetDialog';
import SettingsCard from '../lab/components/SettingsCard';

export interface CredentialResetSectionProps extends Pick<CredentialResetDialogProps, 'onReset' | 'getErrorMessage'> {
  /** The credentials the schema declares. None shows nothing. */
  fields: CredentialResetField[];
  disabled?: boolean;
}

/**
 * Lists the credentials a schema declares, each with a button that opens a reset dialog. The caller supplies the
 * update call, so the same section serves a user and an agent.
 */
export default function CredentialResetSection({
  fields,
  onReset,
  getErrorMessage,
  disabled = false,
}: CredentialResetSectionProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const [activeField, setActiveField] = useState<CredentialResetField | null>(null);

  if (fields.length === 0) return null;

  return (
    <>
      <SettingsCard
        title={t('credentialReset.section.title', 'Reset credentials')}
        description={t('credentialReset.section.description', 'These actions are irreversible.')}
      >
        <Stack spacing={3}>
          {fields.map((field, index) => (
            <Box key={field.fieldName} sx={{mt: index > 0 ? 5 : 0}}>
              <Typography variant="subtitle2" sx={{mb: 0.5}}>
                {field.label}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                {t('credentialReset.section.hint', 'Resetting will immediately invalidate the current {{label}}.', {
                  label: field.label.toLowerCase(),
                })}
              </Typography>
              <Button variant="outlined" color="error" onClick={() => setActiveField(field)} disabled={disabled}>
                {t('credentialReset.section.button', 'Reset {{label}}', {label: field.label})}
              </Button>
            </Box>
          ))}
        </Stack>
      </SettingsCard>

      <CredentialResetDialog
        open={activeField !== null}
        field={activeField}
        onReset={onReset}
        getErrorMessage={getErrorMessage}
        onClose={() => setActiveField(null)}
      />
    </>
  );
}
