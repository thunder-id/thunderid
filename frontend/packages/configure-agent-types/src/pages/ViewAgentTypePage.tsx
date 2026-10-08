// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation, QueryErrorNotice, UnsavedChangesBar} from '@thunderid/components';
import {getBreakingSchemaChanges} from '@thunderid/configure-user-types';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage, isEqualIgnoringEmpty} from '@thunderid/utils';
import {
  Stack,
  Typography,
  Button,
  Alert,
  PageContent,
  PageTitle,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from '@wso2/oxygen-ui';
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
  const {id} = useParams<{id: string}>();
  // Agent types are restricted to a single `default` schema; there is no agent-types listing
  // page anymore, so the back button returns to the agent listing.
  const listUrl = routes.agents.list();

  const {data: agentType, isLoading, error: fetchError, refetch} = useGetAgentType(id);
  const updateAgentTypeMutation = useUpdateAgentType();

  // Resolves an error through the `agentTypes` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agentTypes:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `agentTypes:${key}`, options),
    [t],
  );

  // Edited schema properties (null = no changes, non-null = user edited)
  const [editedProperties, setEditedProperties] = useState<SchemaPropertyInput[] | null>(null);

  // Validation error from the last save attempt. Takes precedence over the mutation's own error.
  const [validationError, setValidationError] = useState<string | null>(null);

  // Schema-change warning confirmation
  const [showSchemaWarning, setShowSchemaWarning] = useState(false);
  const [breakingAttributes, setBreakingAttributes] = useState<string[]>([]);

  // Base properties from server data (useMemo so they're available synchronously)
  const baseProperties = useMemo(() => (agentType ? convertSchemaToProperties(agentType.schema) : []), [agentType]);

  // Effective properties (edited or from server)
  const effectiveProperties = editedProperties ?? baseProperties;

  // Handle used as the i18n key prefix (immutable on the server)
  const agentTypeHandle = agentType?.handle ?? '';

  // Whether there are unsaved changes (deep compare edited vs base)
  const hasChanges = useMemo(
    () => editedProperties !== null && !isEqualIgnoringEmpty(editedProperties, baseProperties),
    [editedProperties, baseProperties],
  );

  const handleBack = async (): Promise<void> => {
    await navigate(listUrl);
  };

  const handlePropertiesChange = useCallback(
    (newProperties: SchemaPropertyInput[]): void => {
      // A previous save error is stale once the schema changes again.
      if (updateAgentTypeMutation.isError) {
        updateAgentTypeMutation.reset();
      }
      setValidationError(null);
      setEditedProperties(newProperties);
    },
    [updateAgentTypeMutation],
  );

  const handleReset = useCallback((): void => {
    setEditedProperties(null);
    setValidationError(null);
    if (updateAgentTypeMutation.isError) {
      updateAgentTypeMutation.reset();
    }
  }, [updateAgentTypeMutation]);

  const performSave = useCallback(async (): Promise<void> => {
    if (!id || !agentType) return;

    const {displayName} = agentType;
    const ouId = agentType.ouId.trim();
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
          displayName,
          ouId,
          ...preservedSystemAttributes,
          schema,
        },
      });
      setEditedProperties(null);
    } catch (err: unknown) {
      logger.error('Failed to update agent type', {error: err});
    }
  }, [id, agentType, effectiveProperties, updateAgentTypeMutation, logger]);

  const handleSave = useCallback(async (): Promise<void> => {
    if (!id || !agentType) return;

    setValidationError(null);

    // Check for duplicate property names
    const trimmedNames = effectiveProperties.filter((p) => p.name.trim()).map((p) => p.name.trim());
    const duplicates = trimmedNames.filter((n, i) => trimmedNames.indexOf(n) !== i);
    if (duplicates.length > 0) {
      setValidationError(
        t('agentTypes:validationErrors.duplicateProperties', {
          duplicates: [...new Set(duplicates)].join(', '),
          defaultValue: 'Duplicate property names found: {{duplicates}}',
        }),
      );
      return;
    }

    // Warn only when a schema change could strand existing agents (removed/newly-required/tightened attribute).
    const breaking = getBreakingSchemaChanges(
      convertPropertiesToSchema(baseProperties),
      convertPropertiesToSchema(effectiveProperties),
    );

    if (breaking.length > 0) {
      setBreakingAttributes(breaking);
      setShowSchemaWarning(true);
      return;
    }

    await performSave();
  }, [id, agentType, effectiveProperties, baseProperties, t, performSave]);

  const handleConfirmSchemaChange = useCallback((): void => {
    setShowSchemaWarning(false);
    void performSave();
  }, [performSave]);

  // Loading state
  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  // Error state
  if (fetchError) {
    return (
      <PageContent>
        <QueryErrorNotice
          error={fetchError}
          t={tForErrors}
          variant="block"
          title={t('agentTypes:edit.loadErrorTitle', 'Failed to load agent type')}
          fallbackKey="agentTypes:edit.loadError"
          fallbackDefaultValue="Failed to load agent type information"
          onRetry={() => void refetch()}
          action={
            <Button
              onClick={() => {
                handleBack().catch(() => null);
              }}
              startIcon={<ArrowLeft size={16} />}
            >
              {t('agentTypes:edit.back', 'Back to Agents')}
            </Button>
          }
        />
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
          agentTypeHandle={agentTypeHandle}
        />
      </Stack>

      {/* Schema-change warning */}
      <Dialog open={showSchemaWarning} onClose={() => setShowSchemaWarning(false)}>
        <DialogTitle>{t('agentTypes:schemaChangeWarning.title', 'Confirm schema changes')}</DialogTitle>
        <DialogContent>
          <Alert severity="warning">
            {t(
              'agentTypes:schemaChangeWarning.description',
              'Existing agents may require updates if their attributes no longer match the revised schema. Applications that return removed attributes in tokens or userinfo must also be updated.',
            )}
            <Typography variant="body2" sx={{mt: 1}}>
              {t('agentTypes:schemaChangeWarning.affected', 'Affected attributes:')}
            </Typography>
            <Stack component="ul" sx={{mt: 0.5, mb: 0, pl: 2.5}}>
              {breakingAttributes.map((name) => (
                <Typography component="li" variant="body2" key={name}>
                  {name}
                </Typography>
              ))}
            </Stack>
            <Typography variant="body2" sx={{mt: 1}}>
              {t('agentTypes:schemaChangeWarning.areYouSure', 'Do you want to continue?')}
            </Typography>
          </Alert>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowSchemaWarning(false)}>{t('common:actions.cancel', 'Cancel')}</Button>
          <Button color="warning" variant="contained" onClick={handleConfirmSchemaChange}>
            {t('agentTypes:schemaChangeWarning.confirm', 'Continue')}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Unsaved Changes Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={t('agentTypes:edit.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('common:actions.reset', 'Reset')}
          saveLabel={t('common:actions.save', 'Save')}
          savingLabel={t('common:status.saving', 'Saving...')}
          isSaving={updateAgentTypeMutation.isPending}
          error={
            validationError ??
            (updateAgentTypeMutation.error
              ? getErrorMessage(
                  updateAgentTypeMutation.error,
                  tForErrors,
                  'update.error',
                  'Failed to update agent type. Please try again.',
                )
              : undefined)
          }
          onReset={handleReset}
          onSave={() => {
            handleSave().catch(() => null);
          }}
        />
      )}
    </PageContent>
  );
}
