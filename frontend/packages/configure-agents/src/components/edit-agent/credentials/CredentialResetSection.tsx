// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CredentialResetDialog, {type AgentCredentialField} from './CredentialResetDialog';

interface CredentialResetSectionProps {
  agentId: string;
  fields: AgentCredentialField[];
  disabled?: boolean;
}

/**
 * Lists the credentials the agent type declares, such as a password, each with a button that resets it. A
 * reset takes the new value from the administrator and ends the tokens and sessions the agent holds.
 */
export default function CredentialResetSection({
  agentId,
  fields,
  disabled = false,
}: CredentialResetSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const {can} = useCapabilitiesContext();
  const [activeField, setActiveField] = useState<AgentCredentialField | null>(null);

  if (fields.length === 0) return null;

  return (
    <SettingsCard
      title={t('agents:edit.credentials.sections.reset.title', 'Reset credentials')}
      description={t(
        'agents:edit.credentials.sections.reset.description',
        'Reset the credentials of this agent. These actions are irreversible.',
      )}
    >
      <Stack spacing={3}>
        {fields.map((field, index) => (
          <Box key={field.fieldName} sx={{mt: index > 0 ? 5 : 0}}>
            <Typography variant="subtitle2" sx={{mb: 0.5}}>
              {field.label}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
              {t(
                'agents:edit.credentials.sections.reset.hint',
                'Resetting will immediately invalidate the current {{label}} and end the tokens and sessions this agent holds.',
                {label: field.label.toLowerCase()},
              )}
            </Typography>
            <Button
              variant="outlined"
              color="error"
              onClick={() => setActiveField(field)}
              disabled={disabled || !can('agent', 'update')}
            >
              {t('agents:edit.credentials.sections.reset.button', 'Reset {{label}}', {label: field.label})}
            </Button>
          </Box>
        ))}
      </Stack>

      <CredentialResetDialog
        open={activeField !== null}
        field={activeField}
        agentId={agentId}
        onClose={() => setActiveField(null)}
      />
    </SettingsCard>
  );
}
