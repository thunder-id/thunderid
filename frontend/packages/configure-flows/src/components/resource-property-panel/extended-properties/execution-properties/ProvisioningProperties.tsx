// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {IdMultiSelect, type IdMultiSelectOption} from '@thunderid/components';
import {
  Alert,
  Checkbox,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useStore, type ReactFlowState} from '@xyflow/react';
import {useCallback, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {FEDERATED_EXECUTORS, PROVISIONING_OU_RESOLVED, PROVISIONING_OU_USER_TYPE} from './constants';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {clampToInteger, parseCommaSeparated} from './utils';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES, NUMBER_VARIABLES, TEXT_VARIABLES} from './variableBindingTypes';
import useGetAllGroups from '../../../../api/useGetAllGroups';
import useGetAllRoles from '../../../../api/useGetAllRoles';
import type {StepData} from '../../../../models/steps';

function selectHasFederatedExecutor(state: ReactFlowState): boolean {
  return state.nodes.some((node) => FEDERATED_EXECUTORS.has((node.data as StepData)?.action?.executor?.name ?? ''));
}

/**
 * Turns groups or roles into picker options. Names are unique only within an organization unit, so a name that more than
 * one resource has is shown with the handle of its organization unit to tell them apart.
 *
 * @param resources - The groups or roles to offer.
 * @returns The options, stored by ID and shown by name.
 */
const toOptions = (resources: {id: string; name: string; ouHandle?: string}[] = []): IdMultiSelectOption[] => {
  const nameCounts = new Map<string, number>();
  resources.forEach(({name}) => nameCounts.set(name, (nameCounts.get(name) ?? 0) + 1));

  return resources.map(({id, name, ouHandle}) => ({
    id,
    name: (nameCounts.get(name) ?? 0) > 1 && ouHandle ? `${name} (${ouHandle})` : name,
  }));
};

/**
 * Properties of a Provisioning step: where the new user is created, which attributes it is prompted for, and the
 * groups and roles it is assigned, picked by name from those that exist.
 *
 * @param props - Props injected to the component.
 * @returns The ProvisioningProperties component.
 */
function ProvisioningProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const groupsQuery = useGetAllGroups();
  const rolesQuery = useGetAllRoles();

  const groupOptions: IdMultiSelectOption[] = useMemo(() => toOptions(groupsQuery.data), [groupsQuery.data]);

  const roleOptions: IdMultiSelectOption[] = useMemo(() => toOptions(rolesQuery.data), [rolesQuery.data]);

  const unavailableLabel = useCallback(
    (id: string): string => t('flows:core.executions.provisioning.unavailable', 'Unavailable: {{id}}', {id}),
    [t],
  );

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const hasFederatedExecutor = useStore(selectHasFederatedExecutor);

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
        showLabel
        htmlFor="assign-group"
        label={t('flows:core.executions.provisioning.assignGroup.label', 'Assign group')}
      >
        {({commit, value: stored}) => (
          <>
            <IdMultiSelect
              id="assign-group"
              value={parseCommaSeparated(typeof stored === 'string' ? stored : '')}
              onChange={(ids) => commit(ids.join(','))}
              options={groupOptions}
              loading={groupsQuery.isLoading}
              error={groupsQuery.error}
              onRetry={() => void groupsQuery.refetch()}
              placeholder={t('flows:core.executions.provisioning.assignGroup.placeholder', 'Select groups to assign')}
              noOptionsText={t('flows:core.executions.provisioning.assignGroup.noOptions', 'No groups found')}
              unavailableLabel={unavailableLabel}
              errorFallback={{
                key: 'flows:core.executions.provisioning.assignGroup.loadError',
                defaultValue: 'Failed to load groups.',
              }}
            />
            {groupsQuery.isSuccess && groupOptions.length === 0 && (
              <Alert severity="warning" sx={{mt: 1}}>
                {t(
                  'flows:core.executions.provisioning.assignGroup.noneExist',
                  'No groups exist. Create one, then select it here.',
                )}
              </Alert>
            )}
          </>
        )}
      </VariableBinding>

      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="assignRole"
        accepts={TEXT_VARIABLES}
        showLabel
        htmlFor="assign-role"
        label={t('flows:core.executions.provisioning.assignRole.label', 'Assign role')}
      >
        {({commit, value: stored}) => (
          <>
            <IdMultiSelect
              id="assign-role"
              value={parseCommaSeparated(typeof stored === 'string' ? stored : '')}
              onChange={(ids) => commit(ids.join(','))}
              options={roleOptions}
              loading={rolesQuery.isLoading}
              error={rolesQuery.error}
              onRetry={() => void rolesQuery.refetch()}
              placeholder={t('flows:core.executions.provisioning.assignRole.placeholder', 'Select roles to assign')}
              noOptionsText={t('flows:core.executions.provisioning.assignRole.noOptions', 'No roles found')}
              unavailableLabel={unavailableLabel}
              errorFallback={{
                key: 'flows:core.executions.provisioning.assignRole.loadError',
                defaultValue: 'Failed to load roles.',
              }}
            />
            {rolesQuery.isSuccess && roleOptions.length === 0 && (
              <Alert severity="warning" sx={{mt: 1}}>
                {t(
                  'flows:core.executions.provisioning.assignRole.noneExist',
                  'No roles exist. Create one, then select it here.',
                )}
              </Alert>
            )}
          </>
        )}
      </VariableBinding>

      {hasFederatedExecutor && (!!properties['assignGroup'] || !!properties['assignRole']) && (
        <Alert severity="warning">
          {t(
            'flows:core.executions.provisioning.federatedOverrideWarning',
            "With Override all sync, a federated sign-in removes groups and roles assigned here that the connection's authorization mapping doesn't resolve.",
          )}
        </Alert>
      )}
    </Stack>
  );
}

export default ProvisioningProperties;
