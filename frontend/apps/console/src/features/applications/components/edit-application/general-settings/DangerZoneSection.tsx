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

import {SettingsCard} from '@thunderid/components';
import {Typography, Button, Divider} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link DangerZoneSection} component.
 */
interface DangerZoneSectionProps {
  /**
   * Callback function to open the regenerate client secret confirmation dialog
   */
  onRegenerateClick?: () => void;
  /**
   * Whether to show the regenerate client secret section (only for confidential clients)
   */
  showRegenerateSecret?: boolean;
  /**
   * Callback function to open the regenerate App Secret confirmation dialog
   */
  onRegenerateAppSecretClick?: () => void;
  /**
   * Whether to show the regenerate App Secret section (only for backend / non-public clients)
   */
  showRegenerateAppSecret?: boolean;
  /**
   * Callback function to open the delete application confirmation dialog
   */
  onDeleteClick: () => void;
}

/**
 * Section component displaying the danger zone with destructive actions.
 *
 * Displays a regenerate client secret button (for confidential clients) and a
 * delete application button. These actions are irreversible.
 *
 * @param props - Component props
 * @returns Danger zone UI within a SettingsCard
 */
export default function DangerZoneSection({
  onRegenerateClick = undefined,
  showRegenerateSecret = false,
  onRegenerateAppSecretClick = undefined,
  showRegenerateAppSecret = false,
  onDeleteClick,
}: DangerZoneSectionProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('applications:edit.general.sections.dangerZone.title', 'Danger Zone')}
      description={t(
        'applications:edit.general.sections.dangerZone.description',
        'Actions in this section are irreversible. Proceed with caution.',
      )}
    >
      {showRegenerateSecret && (
        <>
          <Typography variant="h6" gutterBottom color="error">
            {t('applications:edit.general.sections.dangerZone.regenerateSecret.title', 'Regenerate Client Secret')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {t(
              'applications:edit.general.sections.dangerZone.regenerateSecret.description',
              'Regenerating the client secret will immediately invalidate the current client secret and cannot be undone.',
            )}
          </Typography>
          <Button variant="contained" color="error" onClick={onRegenerateClick}>
            {t('applications:edit.general.sections.dangerZone.regenerateSecret.button', 'Regenerate Client Secret')}
          </Button>
          <Divider sx={{my: 3}} />
        </>
      )}
      {showRegenerateAppSecret && (
        <>
          <Typography variant="h6" gutterBottom color="error">
            {t('applications:edit.general.sections.dangerZone.regenerateAppSecret.title', 'Regenerate App Secret')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {t(
              'applications:edit.general.sections.dangerZone.regenerateAppSecret.description',
              'Regenerating the App Secret immediately invalidates the current one. Server-side flow initiation will fail until the new secret is deployed.',
            )}
          </Typography>
          <Button variant="contained" color="error" onClick={onRegenerateAppSecretClick}>
            {t('applications:edit.general.sections.dangerZone.regenerateAppSecret.button', 'Regenerate App Secret')}
          </Button>
          <Divider sx={{my: 3}} />
        </>
      )}
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
