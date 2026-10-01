// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Typography, Button} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface DangerZoneSectionProps {
  onDeleteClick: () => void;
}

export default function DangerZoneSection({onDeleteClick}: DangerZoneSectionProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.dangerZone.title', 'Danger Zone')}
      description={t(
        'agents:edit.general.sections.dangerZone.description',
        'Actions here are permanent. Make sure before you proceed.',
      )}
    >
      <Typography variant="h6" gutterBottom color="error">
        {t('agents:edit.general.dangerZone.deleteAgent.title', 'Delete Agent')}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
        {t(
          'agents:edit.general.dangerZone.deleteAgent.description',
          'Permanently deletes this agent and immediately invalidates any tokens it has issued. This action cannot be undone.',
        )}
      </Typography>
      <Button data-testid="delete-agent-button" variant="contained" color="error" onClick={onDeleteClick}>
        {t('agents:edit.general.dangerZone.deleteAgent.button', 'Delete Agent')}
      </Button>
    </SettingsCard>
  );
}
