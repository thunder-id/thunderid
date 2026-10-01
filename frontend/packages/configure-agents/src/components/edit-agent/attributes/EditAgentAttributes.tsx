// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {useGetAgentType, useGetAgentTypes} from '@thunderid/configure-agent-types';
import {renderSchemaField} from '@thunderid/configure-users';
import {useResolveDisplayName} from '@thunderid/hooks';
import {Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import {useCallback, useEffect, useRef, type JSX} from 'react';
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
  const {data: schemaDetails, isLoading, error, refetch} = useGetAgentType(matchedSchema?.id);

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

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

  // Staging re-renders the page, which can recreate onFieldChange. Keying the effect on the
  // callback would restage and loop, so keep it keyed on the watched values only.
  const onFieldChangeRef = useRef(onFieldChange);
  useEffect(() => {
    onFieldChangeRef.current = onFieldChange;
  }, [onFieldChange]);

  useEffect(() => {
    const filtered = filterAttributes(watchedValues);
    // react-hook-form's useWatch fires again shortly after mount as each dynamically-rendered
    // field registers, even without any user interaction — only propagate once the values
    // actually diverge from the baseline, or the Save/Reset bar would show up unprompted.
    if (areAttributesEqual(filtered, baselineRef.current)) return;
    onFieldChangeRef.current('attributes', filtered);
  }, [watchedValues]);

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  if (error) {
    return <QueryErrorNotice error={error} t={tForErrors} variant="inline" onRetry={() => void refetch()} />;
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
