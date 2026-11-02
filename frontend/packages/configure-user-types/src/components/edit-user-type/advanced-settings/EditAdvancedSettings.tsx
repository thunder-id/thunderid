// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, Typography, Button} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface EditAdvancedSettingsProps {
  onDeleteClick?: () => void;
}

/**
 * Advanced settings tab content for the User Type edit page.
 * Displays the Danger Zone section.
 */
export default function EditAdvancedSettings({onDeleteClick = undefined}: EditAdvancedSettingsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3}>
      {onDeleteClick && (
        <SettingsCard
          title={t('userTypes:edit.general.dangerZone.title', 'Danger zone')}
          description={t('userTypes:edit.general.dangerZone.description', 'Irreversible actions for this user type.')}
        >
          <Typography variant="h6" gutterBottom color="error">
            {t('userTypes:edit.general.dangerZone.deleteUserType', 'Delete user type')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {t(
              'userTypes:edit.general.dangerZone.deleteUserTypeDescription',
              'Permanently delete this user type and all associated schema definitions. This action cannot be undone.',
            )}
          </Typography>
          <Button variant="contained" color="error" onClick={onDeleteClick}>
            {t('common:actions.delete')}
          </Button>
        </SettingsCard>
      )}
    </Stack>
  );
}
