// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {parseCommaSeparated} from './utils';
import type {StepData} from '../../../../models/steps';

function PermissionValidatorProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const requiredScopes = (properties['requiredScopes'] as string[]) ?? [];
  const scopesString = requiredScopes.join(', ');

  // Local state for the raw input — avoids eager parsing that collapses trailing separators
  const [localValue, setLocalValue] = useState(scopesString);

  // Sync local state when the persisted value changes externally
  useEffect(() => {
    setLocalValue(scopesString);
  }, [scopesString]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.permissionValidator.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="required-scopes">
          {t('flows:core.executions.permissionValidator.requiredScopes.label')}
        </FormLabel>
        <TextField
          id="required-scopes"
          value={localValue}
          onChange={(e) => setLocalValue(e.target.value)}
          onBlur={() => onChange('data.properties.requiredScopes', parseCommaSeparated(localValue), resource)}
          placeholder={t('flows:core.executions.permissionValidator.requiredScopes.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.permissionValidator.requiredScopes.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default PermissionValidatorProperties;
