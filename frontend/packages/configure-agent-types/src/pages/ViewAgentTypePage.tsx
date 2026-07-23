/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {PageLoadingAnimation, UnsavedChangesBar} from '@thunderid/components';
import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Stack, Typography, Button, Alert, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {ArrowLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState, useMemo, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetAgentType from '../api/useGetAgentType';
import useUpdateAgentType from '../api/useUpdateAgentType';
import EditSchemaSettings from '../components/edit-agent-type/schema-settings/EditSchemaSettings';
import useAgentTypeRoutes from '../hooks/useAgentTypeRoutes';
import type {
  AgentTypeDefinition,
  PropertyDefinition,
  PropertyType,
  SchemaPropertyInput,
} from '../models/property-definition';

/**
 * Convert API schema to editable property inputs.
 */
function convertSchemaToProperties(schema: AgentTypeDefinition): SchemaPropertyInput[] {
  return Object.entries(schema).map(([key, value], index) => ({
    id: `${index}`,
    name: key,
    displayName: 'displayName' in value ? (value.displayName ?? '') : '',
    type:
      value.type === 'string' && 'enum' in value && Array.isArray(value.enum) && value.enum.length > 0
        ? 'enum'
        : value.type,
    required: value.required ?? false,
    unique: 'unique' in value ? (value.unique ?? false) : false,
    credential: 'credential' in value ? (value.credential ?? false) : false,
    enum: 'enum' in value ? (value.enum ?? []) : [],
    regex: 'regex' in value ? (value.regex ?? '') : '',
    ...('items' in value ? {items: value.items} : {}),
    ...('properties' in value ? {properties: value.properties} : {}),
  }));
}

/**
 * Convert editable property inputs back to API schema format.
 */
function convertPropertiesToSchema(properties: SchemaPropertyInput[]): AgentTypeDefinition {
  const schema: AgentTypeDefinition = {};

  properties
    .filter((prop) => prop.name.trim())
    .forEach((prop) => {
      const actualType: PropertyType = prop.type === 'enum' ? 'string' : prop.type;

      const propDef: Partial<PropertyDefinition> = {
        type: actualType,
        required: prop.required,
        ...(prop.displayName.trim() ? {displayName: prop.displayName.trim()} : {}),
      };

      if (prop.unique) {
        (propDef as {unique?: boolean}).unique = true;
      }

      if ((prop.type === 'string' || prop.type === 'number' || prop.type === 'enum') && prop.credential) {
        (propDef as {credential?: boolean}).credential = true;
      }

      if (prop.type === 'string' || prop.type === 'enum') {
        if (prop.enum.length > 0) {
          (propDef as {enum?: string[]}).enum = prop.enum;
        }
        if (prop.regex.trim()) {
          (propDef as {regex?: string}).regex = prop.regex;
        }
      }

      if (prop.type === 'array') {
        (propDef as {items?: {type: string}}).items = prop.items ?? {type: 'string'};
      } else if (prop.type === 'object') {
        (propDef as {properties?: Record<string, PropertyDefinition>}).properties = prop.properties ?? {};
      }

      schema[prop.name.trim()] = propDef as PropertyDefinition;
    });

  return schema;
}

export default function ViewAgentTypePage(): JSX.Element {
  const navigate = useNavigate();
  const routes = useAgentTypeRoutes();
  const {t} = useTranslation();
  const logger = useLogger('ViewAgentTypePage');
  const {showToast} = useToast();
  const {id} = useParams<{id: string}>();
  // Agent types are restricted to a single `default` schema; there is no agent-types listing
  // page anymore, so the back button returns to the agent listing.
  const listUrl = routes.agents.list();

  const {data: agentType, isLoading, error: fetchError} = useGetAgentType(id);
  const updateAgentTypeMutation = useUpdateAgentType();

  // Edited schema properties (null = no changes, non-null = user edited)
  const [editedProperties, setEditedProperties] = useState<SchemaPropertyInput[] | null>(null);

  // Base properties from server data (useMemo so they're available synchronously)
  const baseProperties = useMemo(() => (agentType ? convertSchemaToProperties(agentType.schema) : []), [agentType]);

  // Effective properties (edited or from server)
  const effectiveProperties = editedProperties ?? baseProperties;

  // Effective name (locked to the server-side value)
  const effectiveName = agentType?.name ?? '';

  // Change detection
  const hasChanges = useMemo(() => editedProperties !== null, [editedProperties]);

  const handleBack = async (): Promise<void> => {
    await navigate(listUrl);
  };

  const handlePropertiesChange = useCallback((newProperties: SchemaPropertyInput[]): void => {
    setEditedProperties(newProperties);
  }, []);

  const handleReset = useCallback((): void => {
    setEditedProperties(null);
    updateAgentTypeMutation.reset();
  }, [updateAgentTypeMutation]);

  const handleSave = useCallback(async (): Promise<void> => {
    if (!id || !agentType) return;

    const name = agentType.name.trim();
    const ouId = agentType.ouId.trim();

    // Check for duplicate property names
    const trimmedNames = effectiveProperties.filter((p) => p.name.trim()).map((p) => p.name.trim());
    const duplicates = trimmedNames.filter((n, i) => trimmedNames.indexOf(n) !== i);
    if (duplicates.length > 0) {
      showToast(
        t('agentTypes:validationErrors.duplicateProperties', {duplicates: [...new Set(duplicates)].join(', ')}),
        'error',
      );
      return;
    }

    const schema = convertPropertiesToSchema(effectiveProperties);

    try {
      // The display attribute is preserved verbatim if the server returned one, but the agent UI
      // never consumes it (agents always render their `name` field), so we don't expose it as an
      // editable control.
      const preservedSystemAttributes = agentType.systemAttributes?.display
        ? {systemAttributes: {display: agentType.systemAttributes.display}}
        : {};
      await updateAgentTypeMutation.mutateAsync({
        agentTypeId: id,
        data: {
          name,
          ouId,
          ...preservedSystemAttributes,
          schema,
        },
      });
      setEditedProperties(null);
    } catch (err: unknown) {
      logger.error('Failed to update agent type', {error: err});
      const message = err instanceof Error ? err.message : t('agentTypes:edit.saveError', 'Failed to save agent type');
      showToast(message, 'error');
    }
  }, [id, agentType, effectiveProperties, updateAgentTypeMutation, logger, showToast, t]);

  // Loading state
  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  // Error state
  if (fetchError) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {fetchError.message ?? t('agentTypes:edit.loadError', 'Failed to load agent type information')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('agentTypes:edit.back', 'Back to Agents')}
        </Button>
      </PageContent>
    );
  }

  // Not found
  if (!agentType) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('agentTypes:edit.notFound', 'Agent type not found')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('agentTypes:edit.back', 'Back to Agents')}
        </Button>
      </PageContent>
    );
  }

  return (
    <PageContent>
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('agentTypes:edit.back', 'Back to Agents')}
        </PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            <Typography variant="h3">{t('agentTypes:edit.title', 'Agent Schema')}</Typography>
          </Stack>
        </PageTitle.Header>
      </PageTitle>

      <Stack spacing={3} mt={3}>
        <EditSchemaSettings
          properties={effectiveProperties}
          onPropertiesChange={handlePropertiesChange}
          agentTypeName={effectiveName}
        />
      </Stack>

      {/* Unsaved Changes Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={t('agentTypes:edit.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('common:actions.reset', 'Reset')}
          saveLabel={t('common:actions.save', 'Save')}
          savingLabel={t('common:status.saving', 'Saving...')}
          isSaving={updateAgentTypeMutation.isPending}
          onReset={handleReset}
          onSave={() => {
            handleSave().catch(() => null);
          }}
        />
      )}
    </PageContent>
  );
}
