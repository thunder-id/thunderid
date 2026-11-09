// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, PageLoadingAnimation, QueryErrorNotice, UnsavedChangesBar} from '@thunderid/components';
import {getBreakingSchemaChanges} from '@thunderid/configure-user-types';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {useResolveDisplayName} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {
  getContactTypesMissingRegex,
  getErrorMessage,
  isEqualIgnoringEmpty,
  resolveAttributeRegex,
} from '@thunderid/utils';
import {
  Box,
  Stack,
  Typography,
  Button,
  Alert,
  Tabs,
  Tab,
  PageContent,
  PageTitle,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from '@wso2/oxygen-ui';
import {ArrowLeft} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode, SyntheticEvent, JSX} from 'react';
import {useState, useMemo, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetAgentType from '../api/useGetAgentType';
import useGetAllAgentTypes from '../api/useGetAllAgentTypes';
import useUpdateAgentType from '../api/useUpdateAgentType';
import EditAdvancedSettings from '../components/edit-agent-type/advanced-settings/EditAdvancedSettings';
import AgentTypeDeleteDialog from '../components/edit-agent-type/AgentTypeDeleteDialog';
import EditSchemaSettings from '../components/edit-agent-type/schema-settings/EditSchemaSettings';
import useAgentTypeRoutes from '../hooks/useAgentTypeRoutes';
import type {AgentTypeDefinition, SchemaPropertyInput} from '../models/property-definition';
import convertPropertiesToSchema from '../utils/convertPropertiesToSchema';
import isDisplayAttributeAvailable from '../utils/isDisplayAttributeAvailable';

interface TabPanelProps {
  children?: ReactNode;
  index: number;
  value: number;
}

function TabPanel({children = null, value, index, ...other}: TabPanelProps): JSX.Element {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`agenttype-tabpanel-${index}`}
      aria-labelledby={`agenttype-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

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
    regex: resolveAttributeRegex(value.type, 'regex' in value ? value.regex : undefined),
    ...('items' in value ? {items: value.items} : {}),
    ...('properties' in value ? {properties: value.properties} : {}),
  }));
}

export default function ViewAgentTypePage(): JSX.Element {
  const navigate = useNavigate();
  const routes = useAgentTypeRoutes();
  const {t} = useTranslation();
  const {can} = useCapabilitiesContext();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const logger = useLogger('ViewAgentTypePage');
  const {id} = useParams<{id: string}>();
  const listUrl = routes.list();

  const {data: agentType, isLoading, error: fetchError, refetch} = useGetAgentType(id);
  const updateAgentTypeMutation = useUpdateAgentType();

  // The detail response has no read-only flag, so it is read from the agent type list. Editing and
  // deletion stay off until the list shows the type is editable, and a list that fails offers a retry.
  const {data: agentTypes, error: agentTypesError, refetch: refetchAgentTypes} = useGetAllAgentTypes();
  const listedType = agentTypes?.find((candidate) => candidate.id === id);
  const isReadOnly = listedType?.isReadOnly === true;
  const isEditable = listedType !== undefined && !isReadOnly;

  // Resolves an error through the `agentTypes` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agentTypes:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `agentTypes:${key}`, options),
    [t],
  );

  // Tab state
  const [activeTab, setActiveTab] = useState(0);

  // Edited schema properties (null = no changes, non-null = user edited)
  const [editedProperties, setEditedProperties] = useState<SchemaPropertyInput[] | null>(null);

  // Validation error from the last save attempt. Takes precedence over the mutation's own error.
  const [validationError, setValidationError] = useState<string | null>(null);

  // Delete dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

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

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
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
      // Agents are listed by the property the display attribute names. This page has no control for
      // it, so the server's value is kept while the edited schema still has that property, and is
      // dropped otherwise: the API rejects a display attribute that names a missing or ineligible
      // property, which would leave a renamed or removed property impossible to save.
      const display = agentType.systemAttributes?.display;
      const preservedSystemAttributes =
        display && isDisplayAttributeAvailable(display, effectiveProperties) ? {systemAttributes: {display}} : {};
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

    const missingRegexTypes = getContactTypesMissingRegex(effectiveProperties);
    if (missingRegexTypes.length > 0) {
      setValidationError(
        missingRegexTypes
          .map((type) =>
            type === 'email'
              ? t('agentTypes:validationErrors.emailRegexRequired', 'Enter a regular expression for email attributes.')
              : t(
                  'agentTypes:validationErrors.mobileRegexRequired',
                  'Enter a regular expression for mobile attributes.',
                ),
          )
          .join(' '),
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

  const handleDeleteSuccess = (): void => {
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate after deleting agent type', {error});
    });
  };

  const resourceTypeTitle = t('navigation:pages.agentTypes', 'Agent types');

  // Loading state
  if (isLoading) {
    return (
      <>
        <DocumentTitle title={resourceTypeTitle} />
        <PageLoadingAnimation />
      </>
    );
  }

  // Error state
  if (fetchError) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
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
              {t('agentTypes:edit.back', 'Back to agent types')}
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
        <DocumentTitle title={resourceTypeTitle} />
        <Alert severity="warning" sx={{mb: 2}}>
          {t('agentTypes:edit.notFound', 'Agent type not found')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('agentTypes:edit.back', 'Back to agent types')}
        </Button>
      </PageContent>
    );
  }

  // A display name can be a translation template, so it is resolved here as in the agent types list.
  const agentTypeName = resolveDisplayName(agentType.displayName) || agentType.handle;

  return (
    <PageContent>
      <DocumentTitle title={agentTypeName} />
      {isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      {agentTypesError && !listedType && (
        <Box sx={{mb: 2}}>
          <QueryErrorNotice
            variant="inline"
            error={agentTypesError}
            t={tForErrors}
            fallbackKey="agentTypes:edit.editableCheckError"
            fallbackDefaultValue="Could not check whether this agent type can be edited, so editing and deleting are off."
            onRetry={() => void refetchAgentTypes()}
          />
        </Box>
      )}
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('agentTypes:edit.back', 'Back to agent types')}
        </PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            <Typography variant="h3">{agentTypeName}</Typography>
          </Stack>
        </PageTitle.Header>
      </PageTitle>

      {/* Tabs */}
      <Tabs
        value={activeTab}
        onChange={handleTabChange}
        aria-label={t('agentTypes:edit.tabs.ariaLabel', 'Agent type settings tabs')}
      >
        <Tab
          label={t('agentTypes:edit.tabs.schema', 'Schema')}
          id="agenttype-tab-0"
          aria-controls="agenttype-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
        <Tab
          label={t('agentTypes:edit.tabs.advanced', 'Advanced')}
          id="agenttype-tab-1"
          aria-controls="agenttype-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Tab Panels */}
      <TabPanel value={activeTab} index={0}>
        <EditSchemaSettings
          properties={effectiveProperties}
          onPropertiesChange={handlePropertiesChange}
          agentTypeHandle={agentTypeHandle}
          disabled={!isEditable}
          isEditMode
        />
      </TabPanel>

      <TabPanel value={activeTab} index={1}>
        <EditAdvancedSettings
          onDeleteClick={isEditable && can('agenttype', 'delete') ? () => setDeleteDialogOpen(true) : undefined}
        />
      </TabPanel>

      {/* Delete Dialog */}
      <AgentTypeDeleteDialog
        open={deleteDialogOpen}
        agentTypeId={id ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />

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
          saveDisabled={!isEditable || !can('agenttype', 'update')}
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
