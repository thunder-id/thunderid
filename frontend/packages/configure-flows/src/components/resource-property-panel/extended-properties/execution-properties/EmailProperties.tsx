// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Autocomplete,
  FormHelperText,
  FormLabel,
  Stack,
  TextField,
  Typography,
  type AutocompleteRenderInputParams,
} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {buildTemplateLabels, buildTemplateOptions} from './utils';
import useGetNotificationTemplates from '../../../../api/useGetNotificationTemplates';
import type {StepData} from '../../../../models/steps';

function EmailProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const {data: templates, isLoading, isSuccess, isError} = useGetNotificationTemplates('email');

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const emailTemplate = (properties['emailTemplate'] as string) || '';

  const options = useMemo((): string[] => buildTemplateOptions(templates, emailTemplate), [templates, emailTemplate]);

  const labelByHandle = useMemo((): Record<string, string> => buildTemplateLabels(templates), [templates]);

  const hasTemplates = (templates?.length ?? 0) > 0;

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
      </div>

      {isError && <Alert severity="error">{t('flows:core.executions.email.emailTemplate.loadError')}</Alert>}

      {isSuccess && !hasTemplates && (
        <Alert severity="warning">{t('flows:core.executions.email.emailTemplate.noTemplates')}</Alert>
      )}
    </Stack>
  );
}

export default EmailProperties;
