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
import useUpdateConnection from '../api/useUpdateConnection';
import AccountLinkingSection from '../components/AccountLinkingSection';
import AttributeMappingSection from '../components/AttributeMappingSection';
import AuthorizationMappingSection from '../components/AuthorizationMappingSection';
import ConnectionDeleteDialog from '../components/ConnectionDeleteDialog';
import ConnectionForm from '../components/ConnectionForm';
import ReadOnlyCopyField from '../components/ReadOnlyCopyField';
import ServiceAuthenticationMethods from '../components/ServiceAuthenticationMethods';
import SubjectMappingSection from '../components/SubjectMappingSection';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import {
  AUTHENTICATION_METHOD_FIELD_NAMES,
  AuthenticationMethods,
  type AuthenticationMethod,
} from '../models/authentication-methods';
import type {
  AccountLinking,
  AttributeConfiguration,
  AuthorizationDirectMapping,
  AuthorizationRuleMapping,
  ConnectionType,
  SubjectMappingValues,
} from '../models/connection';
import {
  type ConnectionFormValues,
  formValuesToRequest,
  outboundAuthenticationFromFormValues,
  responseToFormValues,
  validateConnectionForm,
} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';
import {hasDuplicateSubjectAttributes, normalizeSubjectAttributeMappings} from '../utils/subjectMapping';

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
  const supportsSubjectMapping: boolean = meta?.supportsSubjectMapping ?? false;
  const supportsAuthentication: boolean = meta?.supportsAuthentication ?? false;
  const generalSettingsCardCopy = meta?.generalSettingsCardCopy ?? 'credentials';

  // Branded vendors are singletons and route without an id — resolve the single instance.
  const instancesQuery = useConnectionInstances(connectionType, {enabled: Boolean(meta) && !id});
  const resolvedId: string | undefined = id ?? instancesQuery.data?.[0]?.id;
  const connectionQuery = useConnection(connectionType, resolvedId);

  const [activeTab, setActiveTab] = useState(0);
  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [editedSubjectMapping, setEditedSubjectMapping] = useState<Partial<SubjectMappingValues>>({});
  const [secretReplacing, setSecretReplacing] = useState(false);
  const [headersReplacing, setHeadersReplacing] = useState(false);
  const [editedAttr, setEditedAttr] = useState<AttributeConfiguration | undefined | null>(null);
  const [attrValid, setAttrValid] = useState(true);
  const [attrsKey, setAttrsKey] = useState(0);
  const [editedAuthzMappings, setEditedAuthzMappings] = useState<AuthorizationRuleMapping[] | undefined | null>(null);
  const [authzMappingsValid, setAuthzMappingsValid] = useState(true);
  const [editedDirectMappings, setEditedDirectMappings] = useState<AuthorizationDirectMapping[] | undefined | null>(
    null,
  );
  const [directMappingsValid, setDirectMappingsValid] = useState(true);
  const [editedLinking, setEditedLinking] = useState<AccountLinking | undefined | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

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
  const baselineAuthzMappings: AuthorizationRuleMapping[] | undefined =
    data?.attributeConfiguration?.authorizationMapping?.rules;
  const baselineDirectMappings: AuthorizationDirectMapping[] | undefined =
    data?.attributeConfiguration?.authorizationMapping?.direct;
  const baselineLinking: AccountLinking | undefined = data?.attributeConfiguration?.accountLinking;
  const baselineSubjectMapping: SubjectMappingValues = {
    subjectAttributeMappings: normalizeSubjectAttributeMappings(data?.subjectAttributeMappings),
  };

  if (!meta) {
    return null;
  }

  const values: ConnectionFormValues = {...baseline, ...editedValues};

  const isResolving: boolean = (!id && instancesQuery.isLoading) || connectionQuery.isLoading;
  const notFound: boolean = !isResolving && !data;

  const resetEdits = (): void => {
    setEditedValues({});
    setEditedSubjectMapping({});
    setSecretReplacing(false);
    setHeadersReplacing(false);
    setEditedAttr(null);
    setAttrValid(true);
    setAttrsKey((k) => k + 1);
    setEditedAuthzMappings(null);
    setAuthzMappingsValid(true);
    setEditedDirectMappings(null);
    setDirectMappingsValid(true);
    setEditedLinking(null);
    setNameError(null);
    setGeneralError(null);
  };

  const formDirty: boolean = JSON.stringify(values) !== JSON.stringify(baseline) || secretReplacing || headersReplacing;
  const attrDirty: boolean = editedAttr !== null && canonicalAttr(editedAttr) !== canonicalAttr(baselineAttr);
  const subjectMappingValues: SubjectMappingValues = {...baselineSubjectMapping, ...editedSubjectMapping};
  const subjectMappingDirty: boolean =
    supportsSubjectMapping && JSON.stringify(subjectMappingValues) !== JSON.stringify(baselineSubjectMapping);
  const subjectMappingValid = !hasDuplicateSubjectAttributes(subjectMappingValues.subjectAttributeMappings);
  const authzMappingsDirty: boolean =
    editedAuthzMappings !== null &&
    JSON.stringify(editedAuthzMappings ?? []) !== JSON.stringify(baselineAuthzMappings ?? []);
  const directMappingsDirty: boolean =
    editedDirectMappings !== null &&
    JSON.stringify(editedDirectMappings ?? []) !== JSON.stringify(baselineDirectMappings ?? []);
  const linkingDirty: boolean =
    editedLinking !== null &&
    JSON.stringify(editedLinking?.attributes ?? []) !== JSON.stringify(baselineLinking?.attributes ?? []);
  const dirty: boolean =
    formDirty ||
    attrDirty ||
    subjectMappingDirty ||
    authzMappingsDirty ||
    directMappingsDirty ||
    linkingDirty ||
    !authzMappingsValid ||
    !directMappingsValid;
  const selectedAuthenticationScheme =
    (values['authenticationScheme'] as AuthenticationMethod | undefined) ?? AuthenticationMethods.NONE;
  const persistedAuthenticationScheme = data?.authentication?.scheme ?? AuthenticationMethods.NONE;
  const hasStoredCredentialForSelectedScheme =
    selectedAuthenticationScheme === persistedAuthenticationScheme &&
    (selectedAuthenticationScheme === AuthenticationMethods.BEARER
      ? Boolean(data?.authentication?.bearer?.token)
      : selectedAuthenticationScheme === AuthenticationMethods.BASIC
        ? Boolean(data?.authentication?.basic?.username) && Boolean(data?.authentication?.basic?.password)
        : false);
  const hasStoredHTTPHeaders =
    selectedAuthenticationScheme === AuthenticationMethods.API_KEY &&
    selectedAuthenticationScheme === persistedAuthenticationScheme &&
    Boolean(data?.authentication?.apiKey?.headers?.length);
  const requiredCredentialError = t('validation.required', 'This field is required.');
  const authenticationErrors = {
    bearerToken:
      selectedAuthenticationScheme === AuthenticationMethods.BEARER &&
      (secretReplacing || !hasStoredCredentialForSelectedScheme) &&
      (values['bearerToken'] ?? '').trim() === ''
        ? requiredCredentialError
        : undefined,
    basicUsername:
      selectedAuthenticationScheme === AuthenticationMethods.BASIC &&
      (secretReplacing || !hasStoredCredentialForSelectedScheme) &&
      (values['basicUsername'] ?? '').trim() === ''
        ? requiredCredentialError
        : undefined,
    basicPassword:
      selectedAuthenticationScheme === AuthenticationMethods.BASIC &&
      (secretReplacing || !hasStoredCredentialForSelectedScheme) &&
      (values['basicPassword'] ?? '').trim() === ''
        ? requiredCredentialError
        : undefined,
    httpHeaders:
      selectedAuthenticationScheme === AuthenticationMethods.API_KEY &&
      (headersReplacing || !hasStoredHTTPHeaders) &&
      (values['httpHeaders'] ?? '').trim() === ''
        ? requiredCredentialError
        : undefined,
  };
  const hasRequiredAuthenticationCredential = Object.values(authenticationErrors).every((error) => !error);
  const hasEnteredAuthenticationCredential =
    selectedAuthenticationScheme === AuthenticationMethods.BEARER
      ? (values['bearerToken'] ?? '').trim() !== ''
      : selectedAuthenticationScheme === AuthenticationMethods.BASIC
        ? (values['basicUsername'] ?? '').trim() !== '' && (values['basicPassword'] ?? '').trim() !== ''
        : selectedAuthenticationScheme === AuthenticationMethods.API_KEY
          ? (values['httpHeaders'] ?? '').trim() !== ''
          : false;
  const valid: boolean =
    Object.keys(validateConnectionForm(values, fields, 'edit')).length === 0 &&
    attrValid &&
    authzMappingsValid &&
    directMappingsValid &&
    subjectMappingValid &&
    hasRequiredAuthenticationCredential;
  const authenticationTabIndex = 1;
  const attributeMappingTabIndex = 1 + (supportsAuthentication ? 1 : 0);
  const subjectMappingTabIndex = attributeMappingTabIndex + (supportsAttributes ? 1 : 0);
  const advancedTabIndex = subjectMappingTabIndex + (supportsSubjectMapping ? 1 : 0);

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
    // null means untouched (fall back to the server's last-fetched value); undefined means the user
    // explicitly cleared the section, which must be respected rather than falling back too — `??`
    // cannot tell those apart, since it treats undefined the same as null.
    const effectiveAttr = editedAttr !== null ? editedAttr : baselineAttr;
    const effectiveAuthzMappings = editedAuthzMappings !== null ? editedAuthzMappings : baselineAuthzMappings;
    const effectiveDirectMappings = editedDirectMappings !== null ? editedDirectMappings : baselineDirectMappings;
    const effectiveLinking = editedLinking !== null ? editedLinking : baselineLinking;
    const hasAuthzMappings = Boolean(effectiveAuthzMappings && effectiveAuthzMappings.length > 0);
    const hasDirectMappings = Boolean(effectiveDirectMappings && effectiveDirectMappings.length > 0);
    const hasLinking = Boolean(effectiveLinking && effectiveLinking.attributes.length > 0);
    // userTypeResolution is required by the wire type, but a connection may carry only authorization
    // mappings or only account linking (e.g. a token-exchange-only connection with no attribute mapping
    // configured at all). The empty default is inert: GetMappedUserType treats it identically to no
    // userTypeResolution. authorizationMapping/accountLinking are set explicitly (not spread
    // conditionally) so clearing either drops it from the payload instead of leaking effectiveAttr's
    // stale value (baselineAttr, when unedited, still carries whatever the server last returned).
    const mergedAttributeConfiguration: AttributeConfiguration | undefined =
      effectiveAttr || hasAuthzMappings || hasDirectMappings || hasLinking
        ? {
            userTypeResolution: {default: ''},
            ...effectiveAttr,
            authorizationMapping:
              hasAuthzMappings || hasDirectMappings
                ? {rules: effectiveAuthzMappings, direct: effectiveDirectMappings}
                : undefined,
            accountLinking: hasLinking ? effectiveLinking : undefined,
          }
        : undefined;
    const authenticationChanged =
      selectedAuthenticationScheme !== persistedAuthenticationScheme ||
      ((selectedAuthenticationScheme === AuthenticationMethods.BEARER ||
        selectedAuthenticationScheme === AuthenticationMethods.BASIC) &&
        (secretReplacing || hasEnteredAuthenticationCredential)) ||
      (selectedAuthenticationScheme === AuthenticationMethods.API_KEY &&
        (headersReplacing || hasEnteredAuthenticationCredential));
    const requestFields = supportsAuthentication
      ? fields.filter((field) => !AUTHENTICATION_METHOD_FIELD_NAMES.has(field.name))
      : fields;
    const payload = {
      ...formValuesToRequest(values, requestFields, {mode: 'edit', secretReplaced: secretReplacing}),
      ...(supportsAuthentication && authenticationChanged
        ? {authentication: outboundAuthenticationFromFormValues(values)}
        : {}),
      ...(supportsAttributes ? {attributeConfiguration: mergedAttributeConfiguration} : {}),
      ...(supportsSubjectMapping ? subjectMappingValues : {}),
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
            {supportsAuthentication && (
              <Tab
                label={t('detail.tabs.authentication', 'Authentication')}
                aria-invalid={!hasRequiredAuthenticationCredential}
                sx={{textTransform: 'none', color: hasRequiredAuthenticationCredential ? undefined : 'error.main'}}
                data-testid="connection-tab-authentication"
              />
            )}
            {supportsAttributes && (
              <Tab
                label={t('detail.tabs.attributeMapping')}
                sx={{textTransform: 'none'}}
                data-testid="connection-tab-attributes"
              />
            )}
            {supportsSubjectMapping && (
              <Tab
                label={t('detail.tabs.subjectMapping', 'Attribute Configuration')}
                sx={{textTransform: 'none'}}
                data-testid="connection-tab-subject-mapping"
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

              <SettingsCard
                title={
                  generalSettingsCardCopy === 'configuration'
                    ? t('detail.configuration.title', 'Connection Configuration')
                    : t('detail.credentials.title', 'Credentials')
                }
                description={
                  generalSettingsCardCopy === 'configuration'
                    ? t(
                        'detail.configuration.description',
                        'Provide the details ThunderID needs to connect to this service.',
                      )
                    : t(
                        'detail.credentials.description',
                        'Credentials and endpoints for this connection. Secrets are stored write-only.',
                      )
                }
              >
                <ConnectionForm
                  type={connectionType}
                  mode="edit"
                  values={values}
                  secretReplacing={secretReplacing}
                  hasStoredSecret
                  vendorDisplayName={meta.displayName}
                  nameError={nameError}
                  showNameField={isCustom}
                  excludeFieldNames={supportsAuthentication ? AUTHENTICATION_METHOD_FIELD_NAMES : undefined}
                  onFieldChange={(name, value) => {
                    clearSaveError();
                    setEditedValues((prev) => ({...prev, [name]: value}));
                  }}
                  onSecretReplacingChange={setSecretReplacing}
                />
              </SettingsCard>
            </Stack>
          </TabPanel>

          {supportsAuthentication && (
            <TabPanel value={activeTab} index={authenticationTabIndex}>
              <SettingsCard
                title={t('detail.authentication.title', 'Authentication')}
                description={t(
                  'detail.authentication.description',
                  'Configure how ThunderID authenticates requests sent to this policy decision point.',
                )}
              >
                <ServiceAuthenticationMethods
                  method={selectedAuthenticationScheme}
                  bearerToken={values['bearerToken'] ?? ''}
                  basicUsername={values['basicUsername'] ?? ''}
                  basicPassword={values['basicPassword'] ?? ''}
                  httpHeaders={values['httpHeaders'] ?? ''}
                  hasStoredHTTPHeaders={hasStoredHTTPHeaders}
                  headersReplacing={headersReplacing}
                  hasStoredSecret={hasStoredCredentialForSelectedScheme}
                  replacing={secretReplacing}
                  errors={authenticationErrors}
                  onChange={(name, value) => {
                    clearSaveError();
                    if (name === 'authenticationScheme') {
                      setSecretReplacing(false);
                      setHeadersReplacing(false);
                      setEditedValues((prev) => ({
                        ...prev,
                        authenticationScheme: value,
                        bearerToken: '',
                        basicUsername: '',
                        basicPassword: '',
                        httpHeaders: '',
                      }));
                      return;
                    }
                    setEditedValues((prev) => ({...prev, [name]: value}));
                  }}
                  onHeadersReplacingChange={setHeadersReplacing}
                  onReplacingChange={setSecretReplacing}
                />
              </SettingsCard>
            </TabPanel>
          )}

          {supportsAttributes && (
            <TabPanel value={activeTab} index={attributeMappingTabIndex}>
              <Stack direction="column" spacing={4}>
                <AttributeMappingSection
                  key={`attrs-${resolvedId}-${attrsKey}`}
                  initialConfig={baselineAttr}
                  onChange={(config, isValid) => {
                    setEditedAttr(config);
                    setAttrValid(isValid);
                  }}
                />
                <AuthorizationMappingSection
                  key={`authz-${resolvedId}-${attrsKey}`}
                  initialRuleConfig={baselineAuthzMappings}
                  initialDirectConfig={baselineDirectMappings}
                  onRuleChange={(mappings, isValid) => {
                    setEditedAuthzMappings(mappings);
                    setAuthzMappingsValid(isValid);
                  }}
                  onDirectChange={(mappings, isValid) => {
                    setEditedDirectMappings(mappings);
                    setDirectMappingsValid(isValid);
                  }}
                />
                <AccountLinkingSection
                  key={`linking-${resolvedId}-${attrsKey}`}
                  initialConfig={baselineLinking}
                  onChange={(linking) => setEditedLinking(linking)}
                />
              </Stack>
            </TabPanel>
          )}

          {supportsSubjectMapping && (
            <TabPanel value={activeTab} index={subjectMappingTabIndex}>
              <SubjectMappingSection
                values={subjectMappingValues}
                onChange={(field, value) => {
                  clearSaveError();
                  setEditedSubjectMapping((prev) => ({...prev, [field]: value}));
                }}
              />
            </TabPanel>
          )}

          <TabPanel value={activeTab} index={advancedTabIndex}>
            <Stack direction="column" spacing={4}>
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
            </Stack>
          </TabPanel>

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
