// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useContext, useMemo, type ReactNode} from 'react';
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
          onChange={(e) => onChange('data.properties.resolveFrom', e.target.value, resource)}
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
          <FormLabel htmlFor="parent-ou-id">
            {t('flows:core.executions.ouResolver.parentOuId.label', 'Parent organization unit')}
          </FormLabel>
          <DraftTextField
            id="parent-ou-id"
            value={(properties['parentOuId'] as string) ?? ''}
            onCommit={(value) => onChange('data.properties.parentOuId', value.trim(), resource)}
            placeholder={t('flows:core.executions.ouResolver.parentOuId.placeholder', 'Organization unit ID')}
            fullWidth
            size="small"
          />
          <FormHelperText>
            {t(
              'flows:core.executions.ouResolver.parentOuId.hint',
              'Organizations are selected below this one. Leave empty to use the unit set by the User Type Resolver.',
            )}
          </FormHelperText>
        </div>
      )}

      {currentResolveFrom === 'prompt' && (
        <CheckboxWithHint
          checked={Boolean(properties['directChildrenOnly'])}
          onChange={(checked) => onChange('data.properties.directChildrenOnly', checked, resource)}
          label={t('flows:core.executions.ouResolver.directChildrenOnly.label', 'Limit to direct children')}
          hint={t(
            'flows:core.executions.ouResolver.directChildrenOnly.hint',
            'Only direct children of the parent organization unit can be selected, by handle or ID.',
          )}
        />
      )}
    </Stack>
  );
}

export default OUResolverProperties;
