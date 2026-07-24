/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {Alert, Box, Button, PageContent, Skeleton, Stack, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, type ReactNode, type SyntheticEvent, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useConnection from '../api/useConnection';
import useConnectionInstances from '../api/useConnectionInstances';
import useDeleteConnection from '../api/useDeleteConnection';
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
  };

  const formDirty: boolean = JSON.stringify(values) !== JSON.stringify(baseline) || secretReplacing;
  const attrDirty: boolean = editedAttr !== null && canonicalAttr(editedAttr) !== canonicalAttr(baselineAttr);
  const dirty: boolean = formDirty || attrDirty;
  const valid: boolean = Object.keys(validateConnectionForm(values, fields, 'edit')).length === 0 && attrValid;

  const handleSave = (): void => {
    if (!valid || !resolvedId) {
      return;
    }
    const payload = {
      ...formValuesToRequest(values, fields, {mode: 'edit', secretReplaced: secretReplacing}),
      ...(supportsAttributes ? {attributeConfiguration: editedAttr ?? baselineAttr} : {}),
    };
    updateMutation
      .mutateAsync(payload)
      .then(() => connectionQuery.refetch())
      .then(() => resetEdits())
      .catch(() => {
        // Errors (including the 409 duplicate-name) are surfaced by the mutation hook.
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
      ) : notFound || connectionQuery.isError ? (
        <Alert severity="error">{t('error.loadFailed')}</Alert>
      ) : (
        <>
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
                  showNameField={isCustom}
                  onFieldChange={(name, value) => setEditedValues((prev) => ({...prev, [name]: value}))}
                  onSecretReplacingChange={setSecretReplacing}
                />
              </SettingsCard>

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
                  onClick={() => setDeleteOpen(true)}
                  data-testid="connection-delete-button"
                >
                  {t('form.actions.delete')}
                </Button>
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

          {dirty && (
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
            onConfirm={handleDelete}
            onClose={() => setDeleteOpen(false)}
          />
        </>
      )}
    </PageContent>
  );
}
