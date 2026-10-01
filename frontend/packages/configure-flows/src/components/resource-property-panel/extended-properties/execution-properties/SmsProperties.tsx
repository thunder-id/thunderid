// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useSMSProviders} from '@thunderid/configure-connections';
import {
  Alert,
  Autocomplete,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
  type AutocompleteRenderInputParams,
} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {getTemplateScenarioLabel, getTemplateScenarioOptions} from './utils';
import type {StepData} from '../../../../models/steps';

function SmsProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {data: smsProviders, isLoading: isLoadingSMSProviders} = useSMSProviders();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const hasSenders = (smsProviders?.length ?? 0) > 0;
  const smsSenderId = (properties['senderId'] as string) || '';
  const isSenderPlaceholder = smsSenderId === '' || smsSenderId === '{{SENDER_ID}}';

  const smsTemplate = (properties['smsTemplate'] as string) || '';

  const templateOptions = useMemo((): string[] => getTemplateScenarioOptions(smsTemplate), [smsTemplate]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.sms.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="sms-template">{t('flows:core.executions.sms.smsTemplate.label')}</FormLabel>
        <Autocomplete
          id="sms-template"
          options={templateOptions}
          value={smsTemplate || null}
          getOptionLabel={(option: string) => getTemplateScenarioLabel(option, t)}
          onChange={(_event: SyntheticEvent, newValue: string | null) =>
            onChange('data.properties.smsTemplate', newValue ?? '', resource)
          }
          renderInput={(params: AutocompleteRenderInputParams) => (
            <TextField
              {...params}
              placeholder={t('flows:core.executions.sms.smsTemplate.placeholder', 'Select an SMS template')}
              size="small"
            />
          )}
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
