// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Checkbox, FormControlLabel, FormHelperText, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import type {StepData} from '../../../../models/steps';

function ProvisioningProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

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

  const rawMaxPerPrompt = properties['maxPerPrompt'];

  const maxPerPromptValue = useMemo(() => {
    const numericValue = typeof rawMaxPerPrompt === 'number' ? rawMaxPerPrompt : Number(rawMaxPerPrompt ?? 0);

    return Number.isFinite(numericValue) ? numericValue : 0;
  }, [rawMaxPerPrompt]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.provisioning.description')}
      </Typography>

      <FormControlLabel
        control={
          <Checkbox
            checked={properties['delegated'] === true}
            onChange={(e) => handleBooleanPropertyChange('delegated', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.provisioning.delegated.label', 'Delegated mode')}
      />
      <FormHelperText>
        {t(
          'flows:core.executions.provisioning.delegated.hint',
          'When enabled, agents created by this node act on behalf of a signed-in user. A value collected by the flow takes precedence.',
        )}
      </FormHelperText>

      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties['allowCrossOUProvisioning']}
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
            checked={!!properties['includeOptional']}
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
            checked={!!properties['includeOptionalCredentials']}
            onChange={(e) => handleBooleanPropertyChange('includeOptionalCredentials', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.provisioning.includeOptionalCredentials.label')}
      />
      <FormHelperText>{t('flows:core.executions.provisioning.includeOptionalCredentials.hint')}</FormHelperText>

      <DraftTextField
        labelPlacement="top"
        label={t('flows:core.executions.provisioning.maxPerPrompt.label')}
        helperText={t('flows:core.executions.provisioning.maxPerPrompt.hint')}
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

      <DraftTextField
        labelPlacement="top"
        label={t('flows:core.executions.provisioning.assignGroup.label')}
        id="assign-group"
        value={(properties['assignGroup'] as string) || ''}
        onCommit={(value) => handleStringPropertyChange('assignGroup', value)}
        placeholder={t('flows:core.executions.provisioning.assignGroup.placeholder')}
        fullWidth
        size="small"
      />

      <DraftTextField
        labelPlacement="top"
        label={t('flows:core.executions.provisioning.assignRole.label')}
        id="assign-role"
        value={(properties['assignRole'] as string) || ''}
        onCommit={(value) => handleStringPropertyChange('assignRole', value)}
        placeholder={t('flows:core.executions.provisioning.assignRole.placeholder')}
        fullWidth
        size="small"
      />

      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties['seedGroupsFromMapping']}
            onChange={(e) => handleBooleanPropertyChange('seedGroupsFromMapping', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.provisioning.seedGroupsFromMapping.label')}
      />
      <FormHelperText>{t('flows:core.executions.provisioning.seedGroupsFromMapping.hint')}</FormHelperText>

      <FormControlLabel
        control={
          <Checkbox
            checked={!!properties['seedRolesFromMapping']}
            onChange={(e) => handleBooleanPropertyChange('seedRolesFromMapping', e.target.checked)}
            size="small"
          />
        }
        label={t('flows:core.executions.provisioning.seedRolesFromMapping.label')}
      />
      <FormHelperText>{t('flows:core.executions.provisioning.seedRolesFromMapping.hint')}</FormHelperText>
    </Stack>
  );
}

export default ProvisioningProperties;
