// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Stack, Typography, Button} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link EditAdvancedSettings} component.
 *
 * @public
 */
export interface EditAdvancedSettingsProps {
  /**
   * Invoked when the delete button is clicked. The danger zone is hidden when it is not given.
   */
  onDeleteClick?: () => void;
}

/**
 * Advanced settings tab content for the agent type edit page.
 * Displays the Danger Zone section.
 *
 * @public
 */
export default function EditAdvancedSettings({onDeleteClick = undefined}: EditAdvancedSettingsProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <Stack spacing={3}>
      {onDeleteClick && (
        <SettingsCard
          title={t('agentTypes:edit.general.dangerZone.title', 'Danger zone')}
          description={t('agentTypes:edit.general.dangerZone.description', 'Irreversible actions for this agent type.')}
        >
          <Typography variant="h6" gutterBottom color="error">
            {t('agentTypes:edit.general.dangerZone.deleteAgentType', 'Delete agent type')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {t(
              'agentTypes:edit.general.dangerZone.deleteAgentTypeDescription',
              'Permanently delete this agent type and all associated schema definitions. You can delete it only when no agent is of this type and no application, agent, or flow lists it in its allowed agent types. This action cannot be undone.',
            )}
          </Typography>
          <Button variant="contained" color="error" onClick={onDeleteClick}>
            {t('common:actions.delete', 'Delete')}
          </Button>
        </SettingsCard>
      )}
    </Stack>
  );
}
