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

import {FormHelperText, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {parseCommaSeparated} from './utils';
import type {StepData} from '@/features/flows/models/steps';

function UserTypeResolverProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const allowedUserTypes = (properties.allowedUserTypes as string[]) || [];
  const typesString = allowedUserTypes.join(', ');

  // Local state for the raw input — avoids eager parsing that collapses trailing separators
  const [localValue, setLocalValue] = useState(typesString);

  // Sync local state when the persisted value changes externally
  useEffect(() => {
    setLocalValue(typesString);
  }, [typesString]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.userTypeResolver.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="allowed-user-types">
          {t('flows:core.executions.userTypeResolver.allowedUserTypes.label')}
        </FormLabel>
        <TextField
          id="allowed-user-types"
          value={localValue}
          onChange={(e) => setLocalValue(e.target.value)}
          onBlur={() => onChange('data.properties.allowedUserTypes', parseCommaSeparated(localValue), resource)}
          placeholder={t('flows:core.executions.userTypeResolver.allowedUserTypes.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.userTypeResolver.allowedUserTypes.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default UserTypeResolverProperties;
