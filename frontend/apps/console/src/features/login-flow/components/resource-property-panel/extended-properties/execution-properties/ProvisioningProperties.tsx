/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Checkbox, FormControlLabel, FormHelperText, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function ProvisioningProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties ?? {}) as Record<string, unknown>;
  }, [resource]);

  const handleBooleanPropertyChange = useCallback(
    (propertyName: string, value: boolean): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  const handleStringPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, value, resource, true);
    },
    [resource, onChange],
  );

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.provisioning.description')}
      </Typography>

      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties.allowCrossOUProvisioning}
            onChange={(e) => handleBooleanPropertyChange('allowCrossOUProvisioning', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.federation.allowCrossOUProvisioning.label')}
      />
      <FormHelperText>{t('flows:core.executions.federation.allowCrossOUProvisioning.hint')}</FormHelperText>

      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties.includeOptionalCredentials}
            onChange={(e) => handleBooleanPropertyChange('includeOptionalCredentials', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.provisioning.includeOptionalCredentials.label')}
      />
      <FormHelperText>{t('flows:core.executions.provisioning.includeOptionalCredentials.hint')}</FormHelperText>

      <div>
        <FormLabel htmlFor="assign-group">{t('flows:core.executions.provisioning.assignGroup.label')}</FormLabel>
        <TextField
          id="assign-group"
          value={(properties.assignGroup as string) || ''}
          onChange={(e) => handleStringPropertyChange('assignGroup', e.target.value)}
          placeholder={t('flows:core.executions.provisioning.assignGroup.placeholder')}
          fullWidth
          size="small"
        />
      </div>

      <div>
        <FormLabel htmlFor="assign-role">{t('flows:core.executions.provisioning.assignRole.label')}</FormLabel>
        <TextField
          id="assign-role"
          value={(properties.assignRole as string) || ''}
          onChange={(e) => handleStringPropertyChange('assignRole', e.target.value)}
          placeholder={t('flows:core.executions.provisioning.assignRole.placeholder')}
          fullWidth
          size="small"
        />
      </div>
    </Stack>
  );
}

export default ProvisioningProperties;
