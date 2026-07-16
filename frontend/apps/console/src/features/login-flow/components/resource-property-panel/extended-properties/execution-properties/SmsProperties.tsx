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

import {useSMSProviders} from '@thunderid/configure-connections';
import {Alert, FormHelperText, FormLabel, MenuItem, Select, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';

function SmsProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {data: smsProviders, isLoading: isLoadingSMSProviders} = useSMSProviders();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties ?? {}) as Record<string, unknown>;
  }, [resource]);

  const hasSenders = (smsProviders?.length ?? 0) > 0;
  const smsSenderId = (properties.senderId as string) || '';
  const isSenderPlaceholder = smsSenderId === '' || smsSenderId === '{{SENDER_ID}}';

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.sms.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="sms-template">{t('flows:core.executions.sms.smsTemplate.label')}</FormLabel>
        <TextField
          id="sms-template"
          value={(properties.smsTemplate as string) || ''}
          onChange={(e) => onChange('data.properties.smsTemplate', e.target.value, resource, true)}
          placeholder={t('flows:core.executions.sms.smsTemplate.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.sms.smsTemplate.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="sms-sender-select">{t('flows:core.executions.smsOtp.sender.label')}</FormLabel>
        <Select
          id="sms-sender-select"
          value={isSenderPlaceholder ? '' : smsSenderId}
          onChange={(e) => onChange('data.properties.senderId', e.target.value, resource)}
          displayEmpty
          fullWidth
          disabled={isLoadingSMSProviders || !hasSenders}
        >
          <MenuItem value="" disabled>
            {isLoadingSMSProviders ? t('common:status.loading') : t('flows:core.executions.smsOtp.sender.placeholder')}
          </MenuItem>
          {smsProviders?.map((sender) => (
            <MenuItem key={sender.id} value={sender.id}>
              {sender.name}
            </MenuItem>
          ))}
        </Select>
      </div>

      {!isLoadingSMSProviders && !hasSenders && (
        <Alert severity="warning">{t('flows:core.executions.smsOtp.sender.noSenders')}</Alert>
      )}
    </Stack>
  );
}

export default SmsProperties;
