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
import {renderSchemaField} from '@thunderid/configure-users';
import {useResolveDisplayName} from '@thunderid/hooks';
import {Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import {useEffect, useRef, type JSX} from 'react';
import {useForm, useWatch} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import AttributesSummarySection from './AttributesSummarySection';
import type {Agent} from '../../../models/agent';

interface EditAgentAttributesProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

type AttributeFormData = Record<string, unknown>;

const filterAttributes = (data: AttributeFormData): AttributeFormData =>
  Object.fromEntries(Object.entries(data).filter(([, v]) => v !== '' && v !== undefined && v !== null));

// Order-independent equality check — the watched form values and the original attributes can
// have their keys in different orders, which would make a plain JSON.stringify comparison
// report a false difference even when nothing actually changed.
const areAttributesEqual = (a: AttributeFormData, b: AttributeFormData): boolean => {
  const aKeys = Object.keys(a);
  const bKeys = Object.keys(b);
  if (aKeys.length !== bKeys.length) return false;
  return aKeys.every((key) => JSON.stringify(a[key]) === JSON.stringify(b[key]));
};

/**
 * Every field edit stages directly into the page's shared editedAgent state via onFieldChange —
 * the page-level Save/Reset bar is the only thing that ever persists it, same as every other
 * tab. The parent remounts this component (via a `key` bumped on Save/Reset) so its local
 * react-hook-form state always starts fresh from the current attributes.
 */
export default function EditAgentAttributes({
  agent,
  editedAgent,
  onFieldChange,
}: EditAgentAttributesProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const {data: agentTypesData} = useGetAgentTypes();
  const matchedSchema = agentTypesData?.types?.find((s) => s.name === agent.type);
  const {data: schemaDetails, isLoading} = useGetAgentType(matchedSchema?.id);

  const attributes = (editedAgent.attributes ?? agent.attributes ?? {}) as AttributeFormData;

  const {
    control,
    formState: {errors},
  } = useForm<AttributeFormData>({
    defaultValues: attributes,
    mode: 'onChange',
  });

  const watchedValues = useWatch({control});
  // Frozen at mount (the parent remounts this component via a `key` on Save/Reset) — the
  // baseline every subsequent watched value is compared against to detect a real edit.
  const baselineRef = useRef(filterAttributes(attributes));

  useEffect(() => {
    const filtered = filterAttributes(watchedValues);
    // react-hook-form's useWatch fires again shortly after mount as each dynamically-rendered
    // field registers, even without any user interaction — only propagate once the values
    // actually diverge from the baseline, or the Save/Reset bar would show up unprompted.
    if (areAttributesEqual(filtered, baselineRef.current)) return;
    onFieldChange('attributes', filtered);
  }, [watchedValues, onFieldChange]);

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  // A read-only agent can't be edited at all, so there's nothing for a form to do here — fall
  // back to the same summary shown on the General tab.
  if (agent.isReadOnly) {
    return <AttributesSummarySection agent={agent} />;
  }

  const schemaFields = schemaDetails?.schema
    ? Object.entries(schemaDetails.schema).filter(
        ([, fieldDef]) => !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential),
      )
    : [];

  return (
    <SettingsCard
      title={t('agents:edit.attributes.title', 'Attributes')}
      description={t('agents:edit.attributes.description', 'View and manage agent attribute values.')}
    >
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
        {schemaFields.length > 0 ? (
          schemaFields.map(([fieldName, fieldDef]) =>
            renderSchemaField(fieldName, fieldDef, control, errors, resolveDisplayName),
          )
        ) : (
          <Typography variant="body2" color="text.secondary">
            {t('agents:edit.attributes.noSchema', 'No schema available for editing')}
          </Typography>
        )}
      </Box>
    </SettingsCard>
  );
}
