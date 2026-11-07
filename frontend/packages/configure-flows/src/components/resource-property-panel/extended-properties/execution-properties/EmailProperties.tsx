// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEmailProviders} from '@thunderid/configure-connections';
import {
  Alert,
  Autocomplete,
  FormHelperText,
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
import {buildTemplateLabels, buildTemplateOptions} from './utils';
import VariableBinding from './VariableBinding';
import useGetNotificationTemplates from '../../../../api/useGetNotificationTemplates';

interface EmailStepProperties {
  senderId?: string;
  emailTemplate?: string;
  [key: string]: unknown;
}

function EmailProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {data: templates, isLoading, isSuccess, isError} = useGetNotificationTemplates('email');
  const {
    data: emailProviders,
    isLoading: isLoadingEmailProviders,
    isError: isEmailProvidersError,
  } = useEmailProviders();

  const properties = useMemo<EmailStepProperties>(() => {
    const stepData = resource?.data as {properties?: EmailStepProperties} | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const hasSenders = (emailProviders?.length ?? 0) > 0;
  const emailSenderId = typeof properties.senderId === 'string' ? properties.senderId : '';
  const isSenderPlaceholder = emailSenderId === '' || emailSenderId === '{{SENDER_ID}}';

  const emailTemplate = typeof properties.emailTemplate === 'string' ? properties.emailTemplate : '';

  const options = useMemo((): string[] => buildTemplateOptions(templates, emailTemplate), [templates, emailTemplate]);

  const labelByHandle = useMemo((): Record<string, string> => buildTemplateLabels(templates), [templates]);

  const hasTemplates = (templates?.length ?? 0) > 0;

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.email.description')}
      </Typography>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="emailTemplate"
        htmlFor="email-template"
        label={t('flows:core.executions.email.emailTemplate.label')}
      >
        <Autocomplete
          id="email-template"
          options={options}
          value={emailTemplate || null}
          loading={isLoading}
          getOptionLabel={(option: string) => labelByHandle[option] ?? option}
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
      </VariableBinding>

      {isError && <Alert severity="error">{t('flows:core.executions.email.emailTemplate.loadError')}</Alert>}

      {isSuccess && !hasTemplates && (
        <Alert severity="warning">{t('flows:core.executions.email.emailTemplate.noTemplates')}</Alert>
      )}

      {isEmailProvidersError && <Alert severity="error">{t('flows:core.executions.email.sender.loadError')}</Alert>}

      {!isLoadingEmailProviders && !isEmailProvidersError && !hasSenders && (
        <Alert severity="warning">{t('flows:core.executions.email.sender.noSenders')}</Alert>
      )}

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="senderId"
        htmlFor="email-sender-select"
        label={t('flows:core.executions.email.sender.label')}
      >
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
      </VariableBinding>
    </Stack>
  );
}

export default EmailProperties;
