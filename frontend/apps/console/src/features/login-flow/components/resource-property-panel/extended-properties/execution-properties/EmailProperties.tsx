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
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function EmailProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.email.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="email-template">{t('flows:core.executions.email.emailTemplate.label')}</FormLabel>
        <TextField
          id="email-template"
          value={(properties.emailTemplate as string) || ''}
          onChange={(e) => onChange('data.properties.emailTemplate', e.target.value, resource, true)}
          placeholder={t('flows:core.executions.email.emailTemplate.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.email.emailTemplate.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default EmailProperties;
