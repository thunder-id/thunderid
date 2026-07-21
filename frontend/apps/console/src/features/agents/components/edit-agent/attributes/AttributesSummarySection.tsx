/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {SettingsCard} from '@thunderid/components';
import {useGetAgentType, useGetAgentTypes} from '@thunderid/configure-agent-types';
import {useResolveDisplayName} from '@thunderid/hooks';
import {Box, Chip, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Tag} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent} from '../../../models/agent';
import EmptyState from '../shared/EmptyState';

interface AttributesSummarySectionProps {
  agent: Agent;
}

const formatValue = (value: unknown): string => {
  if (value === null || value === undefined) return '-';
  if (Array.isArray(value)) return value.join(', ');
  if (typeof value === 'object') return JSON.stringify(value);
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  return '-';
};

/**
 * Read-only preview of this agent's attribute values, with no edit affordance — used on the
 * General tab. The Attributes tab is where values are actually edited.
 */
export default function AttributesSummarySection({agent}: AttributesSummarySectionProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const {data: agentTypesData} = useGetAgentTypes();
  const matchedSchema = agentTypesData?.types?.find((s) => s.name === agent.type);
  const {data: schemaDetails, isLoading} = useGetAgentType(matchedSchema?.id);

  const attributes = agent.attributes ?? {};

  const labelFor = (key: string): string => {
    const fieldDef = schemaDetails?.schema?.[key];
    if (fieldDef?.displayName) {
      return resolveDisplayName(fieldDef.displayName) || key;
    }
    return key;
  };

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.attributes.title', 'Attributes')}
      description={t(
        'agents:edit.general.sections.attributes.description',
        "A preview of this agent's attribute values. Manage them from the Attributes tab.",
      )}
    >
      {isLoading ? (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
          <CircularProgress size={32} />
        </Box>
      ) : Object.keys(attributes).length > 0 ? (
        <Stack spacing={2}>
          {Object.entries(attributes).map(([key, value]) => (
            <Box key={key}>
              <Typography variant="caption" color="text.secondary">
                {labelFor(key)}
              </Typography>
              {typeof value === 'boolean' ? (
                <Box>
                  <Chip
                    label={value ? t('common:actions.yes', 'Yes') : t('common:actions.no', 'No')}
                    size="small"
                    color={value ? 'success' : 'default'}
                    variant="outlined"
                  />
                </Box>
              ) : (
                <Typography variant="body1">{formatValue(value)}</Typography>
              )}
            </Box>
          ))}
        </Stack>
      ) : (
        <EmptyState icon={<Tag size={28} />} message={t('agents:edit.attributes.empty', 'No attributes available.')} />
      )}
    </SettingsCard>
  );
}
