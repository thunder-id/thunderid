// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, ListingTable, PageContent, Skeleton, Stack, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
import {AlertCircle, ChevronLeft, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type ReactNode, type SyntheticEvent, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useConnection from '../api/useConnection';
import useConnectionInstances from '../api/useConnectionInstances';
import useDeleteConnection from '../api/useDeleteConnection';
import useIsManagedConnection from '../api/useIsManagedConnection';
import useUpdateConnection from '../api/useUpdateConnection';
import AttributeMappingSection from '../components/AttributeMappingSection';
import ConnectionDeleteDialog from '../components/ConnectionDeleteDialog';
import ConnectionForm from '../components/ConnectionForm';
import ReadOnlyCopyField from '../components/ReadOnlyCopyField';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import type {AttributeConfiguration, ConnectionType} from '../models/connection';
import {
  type ConnectionFormValues,
  formValuesToRequest,
  responseToFormValues,
  validateConnectionForm,
} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';

interface TabPanelProps {
  children: ReactNode;
  index: number;
  value: number;
}

function TabPanel({children, value, index}: TabPanelProps): JSX.Element {
  return (
    <div role="tabpanel" hidden={value !== index} id={`connection-tabpanel-${index}`}>
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

/** Canonical serialization of an attribute configuration for dirty-checking (order-independent). */
function canonicalAttr(config: AttributeConfiguration | undefined): string {
  const resolution = config?.userTypeResolution;
  const valueMapping = Object.entries(resolution?.valueMapping ?? {})
    .map(([value, userType]) => `${value}=${userType}`)
    .sort();
  const groups = (config?.userTypeAttributeMappings ?? [])
    .map((group) => ({
      userType: group.userType,
      maps: group.attributes.map((m) => `${m.externalAttribute}=${m.localAttribute}`).sort(),
    }))
    .sort((a, b) => a.userType.localeCompare(b.userType));
  return JSON.stringify({
    default: resolution?.default ?? '',
    externalAttribute: resolution?.externalAttribute ?? '',
    valueMapping,
    groups,
    linking: [...(config?.accountLinking?.attributes ?? [])].sort(),
  });
}

export default function ConnectionDetailPage(): JSX.Element | null {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const routes = useConnectionRoutes();
  const {getGateCallbackUrl} = useConfig();
  const {type, id} = useParams<{type: string; id?: string}>();

  const connectionType = type as ConnectionType;
  const meta = VENDOR_META_BY_TYPE[connectionType];
  const isCustom: boolean = meta?.presentation === 'custom';
  const supportsAttributes: boolean = meta?.supportsAttributeMapping ?? false;

  // Branded vendors are singletons and route without an id — resolve the single instance.
  const instancesQuery = useConnectionInstances(connectionType, {enabled: Boolean(meta) && !id});
  const resolvedId: string | undefined = id ?? instancesQuery.data?.[0]?.id;
  const connectionQuery = useConnection(connectionType, resolvedId);

  const [activeTab, setActiveTab] = useState(0);
  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [secretReplacing, setSecretReplacing] = useState(false);
  const [editedAttr, setEditedAttr] = useState<AttributeConfiguration | undefined | null>(null);
  const [attrValid, setAttrValid] = useState(true);
  const [attrsKey, setAttrsKey] = useState(0);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const isManagedConnection = useIsManagedConnection();
  // Owned by the control plane: a change made here would be replaced by the next apply, and the
  // server refuses it with 403, so the controls are not offered at all.
  const isManaged: boolean = isManagedConnection(resolvedId);

  const updateMutation = useUpdateConnection(connectionType, resolvedId ?? '');
  const deleteMutation = useDeleteConnection(connectionType);

  useEffect(() => {
    if (!meta) {
      void navigate(routes.connections.list());
    }
  }, [meta, navigate, routes]);

  const fields = useMemo(() => (meta ? CONNECTION_FORM_FIELDS[connectionType] : []), [meta, connectionType]);
  const redirectUri = getGateCallbackUrl();
  const data = connectionQuery.data;

  const baseline = useMemo<ConnectionFormValues>(
    () => (data ? responseToFormValues(data, fields, redirectUri) : {}),
    [data, fields, redirectUri],
  );
  const baselineAttr: AttributeConfiguration | undefined = data?.attributeConfiguration;

  if (!meta) {
    return null;
  }

  const values: ConnectionFormValues = {...baseline, ...editedValues};

  const isResolving: boolean = (!id && instancesQuery.isLoading) || connectionQuery.isLoading;
  const notFound: boolean = !isResolving && !data;

  const resetEdits = (): void => {
    setEditedValues({});
    setSecretReplacing(false);
    setEditedAttr(null);
    setAttrValid(true);
    setAttrsKey((k) => k + 1);
    setNameError(null);
    setGeneralError(null);
  };

  const formDirty: boolean = JSON.stringify(values) !== JSON.stringify(baseline) || secretReplacing;
  const attrDirty: boolean = editedAttr !== null && canonicalAttr(editedAttr) !== canonicalAttr(baselineAttr);
  const dirty: boolean = formDirty || attrDirty;
  const valid: boolean = Object.keys(validateConnectionForm(values, fields, 'edit')).length === 0 && attrValid;

  // A save failure is stale once the user edits any field. Only reset the mutation once it has
  // actually failed: resetting while it's still pending would flip isPending back to false and
  // re-enable save before the in-flight request settles.
  const clearSaveError = (): void => {
    setNameError(null);
    setGeneralError(null);
    if (updateMutation.isError) {
      updateMutation.reset();
    }
  };

  const handleSave = (): void => {
    if (!valid || !resolvedId) {
      return;
    }
    setNameError(null);
    setGeneralError(null);
    const payload = {
      ...formValuesToRequest(values, fields, {mode: 'edit', secretReplaced: secretReplacing}),
      ...(supportsAttributes ? {attributeConfiguration: editedAttr ?? baselineAttr} : {}),
    };
    updateMutation
      .mutateAsync(payload)
      .then(() => connectionQuery.refetch())
      .then(() => resetEdits())
      .catch((error: unknown) => {
        if (isConflictError(error)) {
          setNameError(t('error.duplicateName', 'A connection with this name already exists.'));
        } else {
          setGeneralError(getErrorMessage(error as Error, t, 'update.error', 'Failed to update connection.'));
        }
      });
  };

  const handleDelete = (): void => {
    if (!resolvedId) {
      return;
    }
    deleteMutation.mutate(resolvedId, {
      onSuccess: () => {
        setDeleteOpen(false);
        void navigate(routes.connections.list());
      },
      onError: (error) => {
        setDeleteError(getErrorMessage(error, t, 'delete.error', 'Failed to delete connection.'));
      },
    });
  };

  return (
    <PageContent>
      <Button
        variant="text"
        startIcon={<ChevronLeft size={16} />}
        onClick={() => void navigate(routes.connections.list())}
        sx={{mb: 2, alignSelf: 'flex-start'}}
      >
        {t('detail.backToConnections')}
      </Button>

      {isResolving ? (
        <Skeleton variant="rounded" height={480} />
      ) : connectionQuery.error ? (
        <QueryErrorNotice
          error={connectionQuery.error}
          t={t}
          variant="block"
          title={t('detail.loadError.title', 'Failed to load connection')}
          onRetry={() => void connectionQuery.refetch()}
        />
      ) : notFound ? (
        <ListingTable.EmptyState
          illustration={<AlertCircle size={40} />}
          title={t('detail.notFound.title', 'Connection not found')}
          description={t(
            'detail.notFound.description',
            'This connection may have been deleted or the link is incorrect.',
          )}
          action={
            <Button variant="outlined" onClick={() => void navigate(routes.connections.list())}>
              {t('detail.backToConnections')}
            </Button>
          }
        />
      ) : (
        <>
          {isManaged && (
            <Alert severity="info" sx={{mb: 2}}>
              {t('common:managedResource.body', {
                defaultValue:
                  'This resource was applied from the control plane and is read only here. Change it there and apply again, otherwise the next apply would replace whatever was changed on this deployment.',
              })}
            </Alert>
          )}

          <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: 3}}>
            <Box
              sx={{
                width: 52,
                height: 52,
                borderRadius: 2,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                bgcolor: 'action.hover',
                flexShrink: 0,
              }}
            >
              {meta.logo}
            </Box>
            <Stack direction="column" spacing={0.5}>
              <Typography variant="h5" fontWeight={700}>
                {data?.name ?? meta.displayName}
              </Typography>
              <Stack direction="row" spacing={0.75} alignItems="center">
                <Box sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: 'success.main'}} />
                <Typography variant="body2" color="text.secondary">
                  {t('card.configured')}
                </Typography>
              </Stack>
            </Stack>
          </Stack>

          {generalError && (
            <Alert severity="error" onClose={clearSaveError} sx={{mb: 3}}>
              {generalError}
            </Alert>
          )}

          <Tabs
            value={activeTab}
            onChange={(_e: SyntheticEvent, v: number) => setActiveTab(v)}
            aria-label="connection settings tabs"
          >
            <Tab label={t('detail.tabs.general')} sx={{textTransform: 'none'}} data-testid="connection-tab-general" />
            {supportsAttributes && (
              <Tab
                label={t('detail.tabs.attributeMapping')}
                sx={{textTransform: 'none'}}
                data-testid="connection-tab-attributes"
              />
            )}
            <Tab
              label={t('detail.tabs.advanced', 'Advanced')}
              sx={{textTransform: 'none'}}
              data-testid="connection-tab-advanced"
            />
          </Tabs>

          <TabPanel value={activeTab} index={0}>
            <Stack direction="column" spacing={4}>
              <SettingsCard title={t('detail.quickCopy.title')} description={t('detail.quickCopy.description')}>
                <ReadOnlyCopyField
                  id="connection-id"
                  label={t('detail.connectionId')}
                  value={data?.id ?? ''}
                  helperText={t('detail.connectionId.hint')}
                />
              </SettingsCard>

              <SettingsCard title={t('detail.credentials.title')} description={t('detail.credentials.description')}>
                <ConnectionForm
                  type={connectionType}
                  mode="edit"
                  values={values}
                  secretReplacing={secretReplacing}
                  hasStoredSecret
                  vendorDisplayName={meta.displayName}
                  nameError={nameError}
                  showNameField={isCustom}
                  isReadOnly={isManaged}
                  onFieldChange={(name, value) => {
                    clearSaveError();
                    setEditedValues((prev) => ({...prev, [name]: value}));
                  }}
                  onSecretReplacingChange={setSecretReplacing}
                />
              </SettingsCard>
            </Stack>
          </TabPanel>

          {supportsAttributes && (
            <TabPanel value={activeTab} index={1}>
              <AttributeMappingSection
                key={`attrs-${resolvedId}-${attrsKey}`}
                initialConfig={baselineAttr}
                onChange={(config, isValid) => {
                  setEditedAttr(config);
                  setAttrValid(isValid);
                }}
              />
            </TabPanel>
          )}

          <TabPanel value={activeTab} index={supportsAttributes ? 2 : 1}>
            <Stack direction="column" spacing={4}>
              {!isManaged && (
                <SettingsCard title={t('detail.dangerZone.title')} description={t('detail.dangerZone.description')}>
                  <Typography variant="h6" gutterBottom color="error">
                    {t('detail.dangerZone.delete.title')}
                  </Typography>
                  <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
                    {t('detail.dangerZone.delete.description')}
                  </Typography>
                  <Button
                    variant="contained"
                    color="error"
                    startIcon={<Trash2 size={16} />}
                    onClick={() => {
                      setDeleteError(null);
                      setDeleteOpen(true);
                    }}
                    data-testid="connection-delete-button"
                  >
                    {t('form.actions.delete')}
                  </Button>
                </SettingsCard>
              )}
            </Stack>
          </TabPanel>

          {dirty && !isManaged && (
            <UnsavedChangesBar
              message={t('detail.saveBar.unsaved', 'You have unsaved changes.')}
              resetLabel={t('detail.saveBar.reset', 'Reset')}
              saveLabel={t('detail.saveBar.save', 'Save changes')}
              savingLabel={t('detail.saveBar.saving', 'Saving changes...')}
              isSaving={updateMutation.isPending}
              saveDisabled={!valid}
              onReset={resetEdits}
              onSave={handleSave}
            />
          )}

          <ConnectionDeleteDialog
            open={deleteOpen}
            connectionType={connectionType}
            connectionId={resolvedId ?? ''}
            connectionName={data?.name ?? ''}
            isPending={deleteMutation.isPending}
            error={deleteError}
            onConfirm={handleDelete}
            onClose={() => {
              setDeleteOpen(false);
              setDeleteError(null);
            }}
          />
        </>
      )}
    </PageContent>
  );
}
