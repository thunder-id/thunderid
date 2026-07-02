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

import {Alert, FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {SMS_OTP_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import useValidationStatus from '@/features/flows/hooks/useValidationStatus';
import type {StepData} from '@/features/flows/models/steps';
import useNotificationSenders from '@/features/notification-senders/api/useNotificationSenders';

function SmsOtpProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {selectedNotification} = useValidationStatus();
  const {data: notificationSenders, isLoading: isLoadingSenders} = useNotificationSenders('message');

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const currentSenderId = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties as {senderId?: string})?.senderId ?? '';
  }, [resource]);

  const senderErrorMessage: string = useMemo(() => {
    const key = `${resource?.id}_data.properties.senderId`;

    if (selectedNotification?.hasResourceFieldNotification(key)) {
      return selectedNotification?.getResourceFieldNotification(key);
    }

    return '';
  }, [resource, selectedNotification]);

  const handleModeChange = (selectedMode: string): void => {
    const modeConfig = SMS_OTP_MODES.find((mode) => mode.value === selectedMode);

    const updatedData = {
      ...((resource?.data as StepData) ?? {}),
      action: {
        ...((resource?.data as StepData)?.action ?? {}),
        executor: {
          ...((resource?.data as StepData)?.action?.executor ?? {}),
          mode: selectedMode,
        },
      },
      display: {
        ...((resource?.data as StepData)?.display ?? {}),
        label: modeConfig?.displayLabel ?? 'SMS OTP',
      },
    };

    onChange('data', updatedData, resource);
  };

  const handleSenderChange = (selectedSenderId: string): void => {
    onChange('data.properties.senderId', selectedSenderId, resource);
  };

  const hasSenders = (notificationSenders?.length ?? 0) > 0;
  const isSenderPlaceholder = currentSenderId === '{{SENDER_ID}}' || currentSenderId === '';
  const showSenderError = isSenderPlaceholder || !!senderErrorMessage;

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.smsOtp.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="mode-select">{t('flows:core.executions.smsOtp.mode.label')}</FormLabel>
        <Select
          id="mode-select"
          value={currentMode}
          onChange={(e) => handleModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.smsOtp.mode.placeholder')}
          </MenuItem>
          {SMS_OTP_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey)}
            </MenuItem>
          ))}
        </Select>
      </div>

      <div>
        <FormLabel htmlFor="sender-select">{t('flows:core.executions.smsOtp.sender.label')}</FormLabel>
        <Select
          id="sender-select"
          value={isSenderPlaceholder ? '' : currentSenderId}
          onChange={(e) => handleSenderChange(e.target.value)}
          displayEmpty
          fullWidth
          error={showSenderError}
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
        {showSenderError && (
          <FormHelperText error>
            {senderErrorMessage || t('flows:core.executions.smsOtp.sender.required')}
          </FormHelperText>
        )}
      </div>

      {!isLoadingSenders && !hasSenders && (
        <Alert severity="warning">{t('flows:core.executions.smsOtp.sender.noSenders')}</Alert>
      )}
    </Stack>
  );
}

export default SmsOtpProperties;
