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
