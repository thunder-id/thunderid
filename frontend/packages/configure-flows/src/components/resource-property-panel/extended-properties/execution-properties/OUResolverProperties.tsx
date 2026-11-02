// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, useContext, useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import {OU_RESOLVE_FROM_OPTIONS} from './constants';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import FlowTypeContext from '../../../../context/FlowTypeContext';
import {FlowType} from '../../../../models/flows';
import type {StepData} from '../../../../models/steps';

function OUResolverProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();
  const flowType = useContext(FlowTypeContext);

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const currentResolveFrom = (properties['resolveFrom'] as string) || 'caller';
  const parentOuId = (properties['parentOuId'] as string) ?? '';
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
        <div>
          <FormLabel htmlFor="parent-ou-id" required>
            {t('flows:core.executions.ouResolver.parentOuId.label', 'Parent OU ID')}
          </FormLabel>
          <DraftTextField
            id="parent-ou-id"
            value={parentOuId}
            onCommit={(value) => onChange('data.properties.parentOuId', value.trim(), resource)}
            placeholder={t('flows:core.executions.ouResolver.parentOuId.placeholder', 'OU ID')}
            required
            error={parentOuId.trim() === ''}
            fullWidth
            size="small"
          />
          <FormHelperText>
            {t('flows:core.executions.ouResolver.parentOuId.hint', 'Users select an OU below this one.')}
          </FormHelperText>
        </div>
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
