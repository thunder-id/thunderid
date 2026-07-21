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
import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CredentialResetDialog from './CredentialResetDialog';

export interface CredentialFieldInfo {
  fieldName: string;
  label: string;
}

interface CredentialsTabPanelProps {
  userId: string;
  credentialFields: CredentialFieldInfo[];
}

export default function CredentialsTabPanel({userId, credentialFields}: CredentialsTabPanelProps): JSX.Element {
  const {t} = useTranslation();

  const [activeField, setActiveField] = useState<CredentialFieldInfo | null>(null);

  return (
    <>
      <SettingsCard
        title={t('users:manageUser.sections.credentials.resetCredentialsTitle', 'Reset Credentials')}
        description={t(
          'users:manageUser.sections.credentials.resetCredentialsDescription',
          'Reset user credentials. These actions are irreversible.',
        )}
      >
        <Stack spacing={3}>
          {credentialFields.map((field, index) => (
            <Box key={field.fieldName} sx={{mt: index > 0 ? 5 : 0}}>
              <Typography variant="subtitle2" sx={{mb: 0.5}}>
                {field.label}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                {t(
                  'users:manageUser.sections.credentials.resetHint',
                  'Resetting will immediately invalidate the current {{label}}.',
                  {
                    label: field.label.toLowerCase(),
                  },
                )}
              </Typography>
              <Button variant="outlined" color="error" onClick={() => setActiveField(field)}>
                {t('users:manageUser.sections.credentials.resetButton', 'Reset {{label}}', {label: field.label})}
              </Button>
            </Box>
          ))}
        </Stack>
      </SettingsCard>

      <CredentialResetDialog
        open={activeField !== null}
        field={activeField}
        userId={userId}
        onClose={() => setActiveField(null)}
      />
    </>
  );
}
