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

import {Alert, FormHelperText, FormLabel, MenuItem, Select, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '@/features/flows/models/steps';
import useNotificationSenders from '@/features/notification-senders/api/useNotificationSenders';

function EmailProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {data: notificationSenders, isLoading: isLoadingSenders} = useNotificationSenders('email');

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties ?? {}) as Record<string, unknown>;
  }, [resource]);

  const hasSenders = (notificationSenders?.length ?? 0) > 0;
  const emailSenderId = (properties.senderId as string) || '';
  const isSenderPlaceholder = emailSenderId === '' || emailSenderId === '{{SENDER_ID}}';

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

      <div>
        <FormLabel htmlFor="email-sender-select">{t('flows:core.executions.smsOtp.sender.label')}</FormLabel>
        <Select
          id="email-sender-select"
          value={isSenderPlaceholder ? '' : emailSenderId}
          onChange={(e) => onChange('data.properties.senderId', e.target.value, resource)}
          displayEmpty
          fullWidth
          disabled={isLoadingSenders || !hasSenders}
        >
          <MenuItem value="" disabled>
            {isLoadingSenders ? t('common:status.loading') : t('flows:core.executions.smsOtp.sender.placeholder')}
          </MenuItem>
          {notificationSenders?.map((sender) => (
            <MenuItem key={sender.id} value={sender.id}>
              {sender.name}
            </MenuItem>
          ))}
        </Select>
      </div>

      {!isLoadingSenders && !hasSenders && (
        <Alert severity="warning">{t('flows:core.executions.smsOtp.sender.noSenders')}</Alert>
      )}
    </Stack>
  );
}

export default EmailProperties;
