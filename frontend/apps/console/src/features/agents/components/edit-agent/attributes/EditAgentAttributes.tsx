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
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Save, X} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useForm} from 'react-hook-form';
import {useTranslation} from 'react-i18next';
import useUpdateAgent from '../../../api/useUpdateAgent';
import type {Agent} from '../../../models/agent';

interface EditAgentAttributesProps {
  agent: Agent;
  onSaved?: () => void;
}

type AttributeFormData = Record<string, unknown>;

export default function EditAgentAttributes({agent, onSaved = undefined}: EditAgentAttributesProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});

  const {data: agentTypesData} = useGetAgentTypes();
  const matchedSchema = agentTypesData?.types?.find((s) => s.name === agent.type);
  const {data: schemaDetails, isLoading} = useGetAgentType(matchedSchema?.id);

  const updateAgent = useUpdateAgent();

  const [isEditMode, setIsEditMode] = useState(false);

  const {
    control,
    handleSubmit,
    reset,
    formState: {errors, isSubmitting},
  } = useForm<AttributeFormData>({
    defaultValues: (agent.attributes as AttributeFormData) ?? {},
    mode: 'onChange',
  });

  if (isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  // Credential fields are only collected on create; hide them from the edit page
  // (matches UserEditPage behaviour — credentials need a dedicated change flow).
  const schemaFields = schemaDetails?.schema
    ? Object.entries(schemaDetails.schema).filter(
        ([, fieldDef]) => !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential),
      )
    : [];

  const hasEditableFields = schemaFields.length > 0;

  const attributes = agent.attributes ?? {};

  const formatValue = (value: unknown): string => {
    if (value === null || value === undefined) return '-';
    if (typeof value === 'boolean') return value ? t('common:actions.yes', 'Yes') : t('common:actions.no', 'No');
    if (Array.isArray(value)) return value.join(', ');
    if (typeof value === 'object') return JSON.stringify(value);
    if (typeof value === 'string' || typeof value === 'number') return String(value);
    return '-';
  };

  const labelFor = (key: string): string => {
    const fieldDef = schemaDetails?.schema?.[key];
    if (fieldDef?.displayName) {
      const resolved = resolveDisplayName(fieldDef.displayName);
      if (resolved) return resolved;
    }
    return key;
  };

  const onSubmit = async (data: AttributeFormData): Promise<void> => {
    const filtered = Object.fromEntries(
      Object.entries(data).filter(([, v]) => v !== '' && v !== undefined && v !== null),
    );
    try {
      await updateAgent.mutateAsync({
        agentId: agent.id,
        data: {...agent, attributes: filtered},
      });
      setIsEditMode(false);
      onSaved?.();
    } catch {
      // surfaced via mutation error below
    }
  };

  const handleCancel = (): void => {
    reset((agent.attributes as AttributeFormData) ?? {});
    setIsEditMode(false);
    updateAgent.reset();
  };

  return (
    <SettingsCard
      title={t('agents:edit.attributes.title', 'Attributes')}
      description={t('agents:edit.attributes.description', 'View and manage agent attribute values.')}
      headerAction={
        !isEditMode && hasEditableFields ? (
          <Button variant="outlined" size="small" onClick={() => setIsEditMode(true)}>
            {t('common:actions.edit', 'Edit')}
          </Button>
        ) : undefined
      }
    >
      {!isEditMode ? (
        <Stack spacing={2}>
          {Object.keys(attributes).length > 0 ? (
            Object.entries(attributes).map(([key, value]) => (
              <Box key={key}>
                <Typography variant="caption" color="text.secondary">
                  {labelFor(key)}
                </Typography>
                <Typography variant="body1">{formatValue(value)}</Typography>
              </Box>
            ))
          ) : (
            <Typography variant="body2" color="text.secondary">
              {t('agents:edit.attributes.empty', 'No attributes available.')}
            </Typography>
          )}
        </Stack>
      ) : (
        <Box
          component="form"
          onSubmit={(event) => {
            handleSubmit(onSubmit)(event).catch(() => null);
          }}
          noValidate
          sx={{display: 'flex', flexDirection: 'column', gap: 2}}
        >
          {hasEditableFields ? (
            schemaFields.map(([fieldName, fieldDef]) =>
              renderSchemaField(fieldName, fieldDef, control, errors, resolveDisplayName),
            )
          ) : (
            <Typography variant="body2" color="text.secondary">
              {t('agents:edit.attributes.noEditable', 'No editable attributes available.')}
            </Typography>
          )}

          {updateAgent.error && (
            <Alert severity="error" sx={{mt: 2}}>
              <Typography variant="body2" sx={{fontWeight: 'bold', mb: 0.5}}>
                {updateAgent.error.message}
              </Typography>
            </Alert>
          )}

          <Stack direction="row" spacing={2} justifyContent="flex-end" sx={{mt: 2}}>
            <Button
              variant="outlined"
              onClick={handleCancel}
              disabled={isSubmitting || updateAgent.isPending}
              startIcon={<X size={16} />}
            >
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button
              type="submit"
              variant="contained"
              startIcon={isSubmitting || updateAgent.isPending ? null : <Save size={16} />}
              disabled={isSubmitting || updateAgent.isPending}
            >
              {isSubmitting || updateAgent.isPending
                ? t('common:status.saving', 'Saving...')
                : t('common:actions.save', 'Save Changes')}
            </Button>
          </Stack>
        </Box>
      )}
    </SettingsCard>
  );
}
