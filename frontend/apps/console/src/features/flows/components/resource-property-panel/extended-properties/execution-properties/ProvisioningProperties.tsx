// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Checkbox, FormControlLabel, FormHelperText, FormLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {useNodes} from '@xyflow/react';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {FEDERATED_EXECUTORS} from './constants';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import type {StepData} from '@/features/flows/models/steps';

function ProvisioningProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const nodes = useNodes();
  const hasFederatedExecutor = useMemo(
    () => nodes.some((node) => FEDERATED_EXECUTORS.has((node.data as StepData)?.action?.executor?.name ?? '')),
    [nodes],
  );

  const handleBooleanPropertyChange = useCallback(
    (propertyName: string, value: boolean): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  const handleStringPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
    },
    [resource, onChange],
  );

  const handleNumberPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, Number(value), resource);
    },
    [resource, onChange],
  );

  const maxPerPromptValue = useMemo(() => {
    const rawValue = properties.maxPerPrompt;
    const numericValue = typeof rawValue === 'number' ? rawValue : Number(rawValue ?? 0);

    return Number.isFinite(numericValue) ? numericValue : 0;
  }, [properties.maxPerPrompt]);

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
            checked={!!properties.includeOptional}
            onChange={(e) => handleBooleanPropertyChange('includeOptional', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.provisioning.includeOptional.label')}
      />
      <FormHelperText>{t('flows:core.executions.provisioning.includeOptional.hint')}</FormHelperText>

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
        <FormLabel htmlFor="max-per-prompt">{t('flows:core.executions.provisioning.maxPerPrompt.label')}</FormLabel>
        <DraftTextField
          id="max-per-prompt"
          type="number"
          value={String(maxPerPromptValue)}
          onCommit={(value) => handleNumberPropertyChange('maxPerPrompt', value)}
          normalize={(raw) => clampToInteger(raw, 0)}
          placeholder={t('flows:core.executions.provisioning.maxPerPrompt.placeholder')}
          fullWidth
          size="small"
          slotProps={{
            htmlInput: {
              min: 0,
            },
          }}
        />
        <FormHelperText>{t('flows:core.executions.provisioning.maxPerPrompt.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="assign-group">{t('flows:core.executions.provisioning.assignGroup.label')}</FormLabel>
        <DraftTextField
          id="assign-group"
          value={(properties.assignGroup as string) || ''}
          onCommit={(value) => handleStringPropertyChange('assignGroup', value)}
          placeholder={t('flows:core.executions.provisioning.assignGroup.placeholder')}
          fullWidth
          size="small"
        />
      </div>

      <div>
        <FormLabel htmlFor="assign-role">{t('flows:core.executions.provisioning.assignRole.label')}</FormLabel>
        <DraftTextField
          id="assign-role"
          value={(properties.assignRole as string) || ''}
          onCommit={(value) => handleStringPropertyChange('assignRole', value)}
          placeholder={t('flows:core.executions.provisioning.assignRole.placeholder')}
          fullWidth
          size="small"
        />
      </div>

      {hasFederatedExecutor && (!!properties.assignGroup || !!properties.assignRole) && (
        <Alert severity="warning">
          {t(
            'flows:core.executions.provisioning.federatedOverrideWarning',
            "This flow has a federated sign-in. If its connection syncs groups and roles with Override all, the groups and roles assigned here are removed on the next sign-in unless the connection's authorization mapping also resolves them.",
          )}
        </Alert>
      )}
    </Stack>
  );
}

export default ProvisioningProperties;
