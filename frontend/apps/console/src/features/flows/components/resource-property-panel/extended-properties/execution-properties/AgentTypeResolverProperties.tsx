// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, useMemo, useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {CommonResourcePropertiesPropsInterface} from './types';
import {parseCommaSeparated} from './utils';
import type {StepData} from '@/features/flows/models/steps';

function AgentTypeResolverProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  const allowedAgentTypes = (properties.allowedAgentTypes as string[]) || [];
  const typesString = allowedAgentTypes.join(', ');

  // Local state for the raw input — avoids eager parsing that collapses trailing separators
  const [localValue, setLocalValue] = useState(typesString);

  // Sync local state when the persisted value changes externally
  useEffect(() => {
    setLocalValue(typesString);
  }, [typesString]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.agentTypeResolver.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="allowed-agent-types">
          {t('flows:core.executions.agentTypeResolver.allowedAgentTypes.label')}
        </FormLabel>
        <TextField
          id="allowed-agent-types"
          value={localValue}
          onChange={(e) => setLocalValue(e.target.value)}
          onBlur={() => onChange('data.properties.allowedAgentTypes', parseCommaSeparated(localValue), resource)}
          placeholder={t('flows:core.executions.agentTypeResolver.allowedAgentTypes.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.executions.agentTypeResolver.allowedAgentTypes.hint')}</FormHelperText>
      </div>
    </Stack>
  );
}

export default AgentTypeResolverProperties;
