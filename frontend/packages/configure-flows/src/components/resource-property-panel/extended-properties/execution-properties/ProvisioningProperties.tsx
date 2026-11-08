// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {PROVISIONING_OU_RESOLVED, PROVISIONING_OU_USER_TYPE} from './constants';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger} from './utils';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES, NUMBER_VARIABLES, TEXT_VARIABLES} from './variableBindingTypes';
import type {StepData} from '../../../../models/steps';

function ProvisioningProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const handleStringPropertyChange = useCallback(
    (propertyName: string, value: string): void => {
      onChange(`data.properties.${propertyName}`, value, resource);
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

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="delegated"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.provisioning.delegated.label', 'Delegated mode')}
      >
        {({commit, label, value: stored}) => (
          <>
            <FormControlLabel
              control={<Checkbox checked={stored === true} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>
              {t(
                'flows:core.executions.provisioning.delegated.hint',
                'When enabled, agents created by this node act on behalf of a signed-in user. A value collected by the flow takes precedence.',
              )}
            </FormHelperText>
          </>
        )}
      </VariableBinding>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="allowCrossOUProvisioning"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.federation.allowCrossOUProvisioning.label')}
      >
        {({commit, label, value: stored}) => (
          <>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.federation.allowCrossOUProvisioning.hint')}</FormHelperText>
          </>
        )}
      </VariableBinding>

      <div>
        <FormLabel id="provisioning-ou-label" htmlFor="provisioning-ou-select">
          {t('flows:core.executions.provisioning.provisioningOU.label', 'Create the user in')}
        </FormLabel>
        <Select
          id="provisioning-ou-select"
          labelId="provisioning-ou-label"
          value={(properties['provisioningOU'] as string) || PROVISIONING_OU_RESOLVED}
          onChange={(e) => handleStringPropertyChange('provisioningOU', e.target.value)}
          fullWidth
        >
          <MenuItem value={PROVISIONING_OU_RESOLVED}>
            {t('flows:core.executions.provisioning.provisioningOU.resolved', 'The resolved organization unit')}
          </MenuItem>
          <MenuItem value={PROVISIONING_OU_USER_TYPE}>
            {t('flows:core.executions.provisioning.provisioningOU.userType', "The user type's organization unit")}
          </MenuItem>
        </Select>
        <FormHelperText>
          {t(
            'flows:core.executions.provisioning.provisioningOU.hint',
            "The resolved organization unit is the one an earlier step resolved, or else the user type's.",
          )}
        </FormHelperText>
      </div>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="includeOptional"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.provisioning.includeOptional.label')}
      >
        {({commit, label, value: stored}) => (
          <>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.provisioning.includeOptional.hint')}</FormHelperText>
          </>
        )}
      </VariableBinding>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="includeOptionalCredentials"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.provisioning.includeOptionalCredentials.label')}
      >
        {({commit, label, value: stored}) => (
          <>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.provisioning.includeOptionalCredentials.hint')}</FormHelperText>
          </>
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="maxPerPrompt"
        accepts={NUMBER_VARIABLES}
        label={t('flows:core.executions.provisioning.maxPerPrompt.label')}
      >
        {({commit, label}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            helperText={t('flows:core.executions.provisioning.maxPerPrompt.hint')}
            id="max-per-prompt"
            type="number"
            value={String(maxPerPromptValue)}
            onCommit={(value) => commit(Number(value))}
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
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="assignGroup"
        accepts={TEXT_VARIABLES}
        label={t('flows:core.executions.provisioning.assignGroup.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            id="assign-group"
            value={(stored as string) || ''}
            onCommit={commit}
            placeholder={t('flows:core.executions.provisioning.assignGroup.placeholder')}
            fullWidth
            size="small"
          />
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="assignRole"
        accepts={TEXT_VARIABLES}
        label={t('flows:core.executions.provisioning.assignRole.label')}
      >
        {({commit, label, value: stored}) => (
          <DraftTextField
            labelPlacement="top"
            label={label}
            id="assign-role"
            value={(stored as string) || ''}
            onCommit={commit}
            placeholder={t('flows:core.executions.provisioning.assignRole.placeholder')}
            fullWidth
            size="small"
          />
        )}
      </VariableBinding>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="seedGroupsFromMapping"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.provisioning.seedGroupsFromMapping.label')}
      >
        {({commit, label, value: stored}) => (
          <>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.provisioning.seedGroupsFromMapping.hint')}</FormHelperText>
          </>
        )}
      </VariableBinding>

      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="seedRolesFromMapping"
        accepts={BOOLEAN_VARIABLES}
        label={t('flows:core.executions.provisioning.seedRolesFromMapping.label')}
      >
        {({commit, label, value: stored}) => (
          <>
            <FormControlLabel
              control={<Checkbox checked={!!stored} onChange={(e) => commit(e.target.checked)} size="small" />}
              label={label}
            />
            <FormHelperText>{t('flows:core.executions.provisioning.seedRolesFromMapping.hint')}</FormHelperText>
          </>
        )}
      </VariableBinding>
    </Stack>
  );
}

export default ProvisioningProperties;
