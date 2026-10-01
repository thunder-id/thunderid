// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Typography, Button} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link DangerZoneSection} component.
 */
interface DangerZoneSectionProps {
  /**
   * Callback function to open the delete application confirmation dialog
   */
  onDeleteClick: () => void;
}

/**
 * Section component displaying the danger zone with destructive actions.
 *
 * Displays a delete application button. This action is irreversible.
 *
 * @param props - Component props
 * @returns Danger zone UI within a SettingsCard
 */
export default function DangerZoneSection({onDeleteClick}: DangerZoneSectionProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('applications:edit.general.sections.dangerZone.title', 'Danger Zone')}
      description={t(
        'applications:edit.general.sections.dangerZone.description',
        'Actions in this section are irreversible. Proceed with caution.',
      )}
    >
      <Typography variant="h6" gutterBottom color="error">
        {t('applications:edit.general.sections.dangerZone.deleteApplication.title', 'Delete Application')}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
        {t(
          'applications:edit.general.sections.dangerZone.deleteApplication.description',
          'Permanently delete this application and all associated data. This action cannot be undone.',
        )}
      </Typography>
      <Button data-testid="delete-application-button" variant="contained" color="error" onClick={onDeleteClick}>
        {t('applications:edit.general.sections.dangerZone.deleteApplication.button', 'Delete Application')}
      </Button>
    </SettingsCard>
  );
}
