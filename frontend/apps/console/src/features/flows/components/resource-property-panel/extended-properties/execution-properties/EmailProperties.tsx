// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEmailProviders} from '@thunderid/configure-connections';
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
import type {StepData} from '@/features/flows/models/steps';

function EmailProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {data: emailProviders, isLoading: isLoadingEmailProviders} = useEmailProviders();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const hasSenders = (emailProviders?.length ?? 0) > 0;
  const emailSenderId = (properties.senderId as string) || '';
  const isSenderPlaceholder = emailSenderId === '' || emailSenderId === '{{SENDER_ID}}';

  const emailTemplate = (properties.emailTemplate as string) || '';

  const options = useMemo((): string[] => getTemplateScenarioOptions(emailTemplate), [emailTemplate]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.email.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="email-template">{t('flows:core.executions.email.emailTemplate.label')}</FormLabel>
        <Autocomplete
          id="email-template"
          options={options}
          value={emailTemplate || null}
          getOptionLabel={(option: string) => getTemplateScenarioLabel(option, t)}
          onChange={(_event: SyntheticEvent, newValue: string | null) =>
            onChange('data.properties.emailTemplate', newValue ?? '', resource)
          }
          renderInput={(params: AutocompleteRenderInputParams) => (
            <TextField
              {...params}
              placeholder={t('flows:core.executions.email.emailTemplate.placeholder', 'Select an email template')}
              size="small"
            />
          )}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.email.emailTemplate.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="email-sender-select">{t('flows:core.executions.email.sender.label')}</FormLabel>
        <Select
          id="email-sender-select"
          value={isSenderPlaceholder ? '' : emailSenderId}
          onChange={(e) => onChange('data.properties.senderId', e.target.value, resource)}
          displayEmpty
          fullWidth
          disabled={isLoadingEmailProviders || !hasSenders}
        >
          <MenuItem value="" disabled>
            {isLoadingEmailProviders ? t('common:status.loading') : t('flows:core.executions.email.sender.placeholder')}
          </MenuItem>
          {emailProviders?.map((sender) => (
            <MenuItem key={sender.id} value={sender.id}>
              {sender.name}
            </MenuItem>
          ))}
        </Select>
        <FormHelperText>{t('flows:core.executions.email.sender.hint')}</FormHelperText>
      </div>

      {!isLoadingEmailProviders && !hasSenders && (
        <Alert severity="warning">{t('flows:core.executions.email.sender.noSenders')}</Alert>
      )}
    </Stack>
  );
}

export default EmailProperties;
