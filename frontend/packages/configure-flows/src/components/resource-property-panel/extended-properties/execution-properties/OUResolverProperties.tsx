// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationUnitTreePicker} from '@thunderid/configure-organization-units';
import {useProjectScope} from '@thunderid/contexts';
import {FormControl, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, useContext, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import {OU_RESOLVE_FROM_OPTIONS} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowOrganizationUnitContext from '../../../../context/FlowOrganizationUnitContext';
import FlowTypeContext from '../../../../context/FlowTypeContext';
import {FlowType} from '../../../../models/flows';
import type {StepData} from '../../../../models/steps';
import {PARENT_OU_ID_PLACEHOLDER} from '../../../../utils/parentOrganizationUnit';

function OUResolverProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const flowType = useContext(FlowTypeContext);
  const flowOuId = useContext(FlowOrganizationUnitContext);
  const {projectId} = useProjectScope();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const currentResolveFrom = (properties['resolveFrom'] as string) || 'caller';
  // A flow in a project without organization units is created with the placeholder until the console sets its
  // system organization unit.
  const configuredParentOuId = (properties['parentOuId'] as string) ?? '';
  const parentOuId = configuredParentOuId === PARENT_OU_ID_PLACEHOLDER ? '' : configuredParentOuId;
  const isDirectChildrenOnly = Boolean(properties['directChildrenOnly']);

  const handleResolveFromChange = useCallback(
    (value: string) => {
      onChange('data.properties.resolveFrom', value, resource);
      if (value === 'prompt') {
        return;
      }
      // The panel's change handler turns undefined into '', which is not a valid directChildrenOnly, so reset each
      // property to a value that means "not set" and survives that conversion.
      if (properties['parentOuId']) {
        onChange('data.properties.parentOuId', '', resource);
      }
      if (properties['directChildrenOnly']) {
        onChange('data.properties.directChildrenOnly', false, resource);
      }
    },
    [onChange, properties, resource],
  );

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.ouResolver.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="resolve-from-select">{t('flows:core.executions.ouResolver.resolveFrom.label')}</FormLabel>
        <Select
          id="resolve-from-select"
          value={currentResolveFrom}
          onChange={(e) => handleResolveFromChange(e.target.value)}
          fullWidth
        >
          {OU_RESOLVE_FROM_OPTIONS.map((option) => (
            <MenuItem key={option.value} value={option.value}>
              {t(option.translationKey)}
            </MenuItem>
          ))}
        </Select>
      </div>

      {currentResolveFrom === 'prompt' && flowType === FlowType.AUTHENTICATION && (
        <FormControl fullWidth required error={parentOuId === ''}>
          <FormLabel htmlFor="parent-ou-id">
            {t('flows:core.executions.ouResolver.parentOuId.label', 'Parent OU')}
          </FormLabel>
          <OrganizationUnitTreePicker
            id="parent-ou-id"
            value={parentOuId}
            onChange={(ouId: string) => onChange('data.properties.parentOuId', ouId, resource)}
            error={parentOuId === ''}
            helperText={
              parentOuId === ''
                ? t('flows:core.executions.ouResolver.parentOuId.required', 'Select the parent OU.')
                : t('flows:core.executions.ouResolver.parentOuId.hint', 'Users select an OU below this one.')
            }
            maxHeight={240}
            projectId={projectId}
            withinOuId={flowOuId}
            rootOuId={projectId ? undefined : flowOuId}
          />
        </FormControl>
      )}

      {currentResolveFrom === 'prompt' && (
        <CheckboxWithHint
          checked={isDirectChildrenOnly}
          onChange={(checked) => onChange('data.properties.directChildrenOnly', checked, resource)}
          label={t('flows:core.executions.ouResolver.directChildrenOnly.label', 'Limit to direct children')}
          hint={t(
            'flows:core.executions.ouResolver.directChildrenOnly.hint',
            'Only direct children of the parent OU can be selected. Declare an ouHandle input below to select by handle.',
          )}
        />
      )}
    </Stack>
  );
}

export default OUResolverProperties;
