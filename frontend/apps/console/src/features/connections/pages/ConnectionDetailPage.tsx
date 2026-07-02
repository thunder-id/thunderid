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
import {Alert, Box, Button, Chip, PageContent, Skeleton, Stack, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
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
import ConnectionForm, {type ConnectionFormSnapshot} from '../components/ConnectionForm';
import ReadOnlyCopyField from '../components/ReadOnlyCopyField';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import type {AttributeConfiguration, ConnectionType} from '../models/connection';
import {formValuesToRequest, responseToFormValues} from '../utils/connectionFormMapping';

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
  const mappings = config?.userTypeAttributeMappings?.[0]?.attributes ?? [];
  return JSON.stringify({
    default: config?.userTypeResolution?.default ?? '',
    maps: mappings.map((m) => `${m.externalAttribute}=${m.localAttribute}`).sort(),
  });
}

export default function ConnectionDetailPage(): JSX.Element | null {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const {getServerUrl} = useConfig();
  const {type, id} = useParams<{type: string; id?: string}>();

  const connectionType = type as ConnectionType;
  const meta = VENDOR_META_BY_TYPE[connectionType];
  const isCustom: boolean = meta?.presentation === 'custom';

  // Branded vendors are singletons and route without an id — resolve the single instance.
  const instancesQuery = useConnectionInstances(connectionType, {enabled: Boolean(meta) && !id});
  const resolvedId: string | undefined = id ?? instancesQuery.data?.[0]?.id;
  const connectionQuery = useConnection(connectionType, resolvedId);

  const [activeTab, setActiveTab] = useState(0);
  const [snapshot, setSnapshot] = useState<ConnectionFormSnapshot | null>(null);
  const [attrConfig, setAttrConfig] = useState<AttributeConfiguration | undefined>(undefined);
  const [attrValid, setAttrValid] = useState(true);
  const [formVersion, setFormVersion] = useState(0);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const updateMutation = useUpdateConnection(connectionType, resolvedId ?? '');
  const deleteMutation = useDeleteConnection(connectionType);

  useEffect(() => {
    if (!meta) {
      void navigate('/connections');
    }
  }, [meta, navigate]);

  const fields = useMemo(() => (meta ? CONNECTION_FORM_FIELDS[connectionType] : []), [meta, connectionType]);
  const redirectUri = `${getServerUrl()}/oauth2/callback`;
  const data = connectionQuery.data;

  const initialValues = useMemo(
    () => (data ? responseToFormValues(data, fields, redirectUri) : {}),
    [data, fields, redirectUri],
  );
  const initialAttr: AttributeConfiguration | undefined = data?.attributeConfiguration;

  if (!meta) {
    return null;
  }

  const isResolving: boolean = (!id && instancesQuery.isLoading) || connectionQuery.isLoading;
  const notFound: boolean = !isResolving && !data;

  const formDirty: boolean = snapshot ? JSON.stringify(snapshot.values) !== JSON.stringify(initialValues) : false;
  const attrDirty: boolean = canonicalAttr(attrConfig) !== canonicalAttr(initialAttr);
  const dirty: boolean = formDirty || attrDirty;
  const valid: boolean = (snapshot?.valid ?? true) && attrValid;

  const handleSave = (): void => {
    if (!snapshot || !valid || !resolvedId) {
      return;
    }
    const payload = {
      ...formValuesToRequest(snapshot.values, fields, {mode: 'edit', secretReplaced: snapshot.secretReplacing}),
      attributeConfiguration: attrConfig,
    };
    updateMutation.mutate(payload, {onSuccess: () => setFormVersion((v) => v + 1)});
  };

  const handleDelete = (): void => {
    if (!resolvedId) {
      return;
    }
    deleteMutation.mutate(resolvedId, {
      onSuccess: () => {
        setDeleteOpen(false);
        void navigate('/connections');
      },
    });
  };

  return (
    <PageContent>
      <Button
        variant="text"
        startIcon={<ChevronLeft size={16} />}
        onClick={() => void navigate('/connections')}
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
              <Stack direction="row" spacing={1.5} alignItems="center" flexWrap="wrap" useFlexGap>
                <Typography variant="h5" fontWeight={700}>
                  {data?.name ?? meta.displayName}
                </Typography>
                <Chip size="small" color="success" label={t('card.configured')} />
              </Stack>
              <Typography variant="body2" color="text.secondary">
                {t('detail.subtitle', {name: data?.name ?? meta.displayName})}
              </Typography>
            </Stack>
          </Stack>

          <Tabs
            value={activeTab}
            onChange={(_e: SyntheticEvent, v: number) => setActiveTab(v)}
            aria-label="connection settings tabs"
          >
            <Tab label={t('detail.tabs.general')} sx={{textTransform: 'none'}} data-testid="connection-tab-general" />
            <Tab
              label={t('detail.tabs.attributeMapping')}
              sx={{textTransform: 'none'}}
              data-testid="connection-tab-attributes"
            />
          </Tabs>

          <TabPanel value={activeTab} index={0}>
            <Stack direction="column" spacing={4}>
              <SettingsCard title={t('detail.quickCopy.title')} description={t('detail.quickCopy.description')}>
                <Stack direction="column" spacing={3}>
                  <ReadOnlyCopyField
                    id="connection-id"
                    label={t('detail.connectionId')}
                    value={data?.id ?? ''}
                    helperText={t('detail.connectionId.hint')}
                  />
                  <ReadOnlyCopyField
                    id="connection-redirect-uri"
                    label={t('form.fields.redirectUri.label')}
                    value={data?.redirectUri ?? redirectUri}
                    helperText={t('form.fields.redirectUri.help', {vendor: meta.displayName})}
                  />
                </Stack>
              </SettingsCard>

              <SettingsCard title={t('detail.credentials.title')} description={t('detail.credentials.description')}>
                <ConnectionForm
                  key={`general-${resolvedId}-${formVersion}`}
                  type={connectionType}
                  mode="edit"
                  initialValues={initialValues}
                  hasStoredSecret
                  vendorDisplayName={meta.displayName}
                  showNameField={isCustom}
                  showRedirectUri={false}
                  onChange={setSnapshot}
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

          <TabPanel value={activeTab} index={1}>
            <SettingsCard title={t('detail.provisioning.title')} description={t('detail.provisioning.description')}>
              <AttributeMappingSection
                key={`attrs-${resolvedId}-${formVersion}`}
                initialConfig={initialAttr}
                onChange={(config, isValid) => {
                  setAttrConfig(config);
                  setAttrValid(isValid);
                }}
              />
            </SettingsCard>
          </TabPanel>

          {dirty && (
            <UnsavedChangesBar
              message={t('detail.saveBar.unsaved')}
              resetLabel={t('detail.saveBar.discard')}
              saveLabel={t('detail.saveBar.save')}
              savingLabel={t('detail.saveBar.saving')}
              isSaving={updateMutation.isPending}
              saveDisabled={!valid}
              onReset={() => setFormVersion((v) => v + 1)}
              onSave={handleSave}
            />
          )}

          <ConnectionDeleteDialog
            open={deleteOpen}
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
