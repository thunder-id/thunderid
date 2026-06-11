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

import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {FormControl, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ConfigureOrgUnitProps {
  selectedOuId: string;
  onOuIdChange: (ouId: string) => void;
  onReadyChange?: (isReady: boolean) => void;
}

export default function ConfigureOrgUnit({
  selectedOuId,
  onOuIdChange,
  onReadyChange = undefined,
}: ConfigureOrgUnitProps): JSX.Element {
  const {t} = useTranslation();

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(selectedOuId.length > 0);
    }
  }, [selectedOuId, onReadyChange]);

  return (
    <Stack direction="column" spacing={4}>
      <Typography variant="h1" gutterBottom>
        {t('resourceServers:create.orgUnit.title', 'Choose an organization unit')}
      </Typography>
      <Typography variant="body1" color="text.secondary">
        {t(
          'resourceServers:create.orgUnit.subtitle',
          'Select which organization unit this resource server belongs to.',
        )}
      </Typography>

      <FormControl fullWidth required>
        <FormLabel>{t('resourceServers:create.orgUnit.fieldLabel', 'Organization Unit')}</FormLabel>
        <OrganizationUnitTreePicker
          id="resource-server-create-ou-picker"
          value={selectedOuId}
          onChange={onOuIdChange}
          maxHeight={400}
        />
      </FormControl>
    </Stack>
  );
}
