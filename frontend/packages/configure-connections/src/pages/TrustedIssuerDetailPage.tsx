// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  DocumentTitle,
  QueryErrorNotice,
  ReadOnlyFieldset,
  ResourceAvatar,
  SettingsCard,
  UnsavedChangesBar,
} from '@thunderid/components';
import {useConfig, useRecordAccess} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  Button,
  ListingTable,
  PageContent,
  Skeleton,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {AlertCircle, ChevronLeft, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useMemo, useRef, useState, type JSX, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useDeleteConnection from '../api/useDeleteConnection';
import useTrustedIssuer from '../api/useTrustedIssuer';
import useUpdateTrustedIssuer from '../api/useUpdateTrustedIssuer';
import AuthorizationMappingSection from '../components/AuthorizationMappingSection';
import ConnectionDeleteDialog from '../components/ConnectionDeleteDialog';
import OrganizationUnitSection from '../components/OrganizationUnitSection';
import ConnectionConstants from '../constants/connection-constants';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import useIsIdJagAcceptanceUnavailable from '../hooks/useIsIdJagAcceptanceUnavailable';
import {
  ConnectionTypes,
  type AttributeConfiguration,
  type AuthorizationDirectMapping,
  type AuthorizationRuleMapping,
} from '../models/connection';
import type {TrustedIssuerFormData} from '../models/trusted-issuer';
import isConflictError from '../utils/isConflictError';
import isTrustedIssuerFormDirty from '../utils/isTrustedIssuerFormDirty';
import validateTrustedIssuerForm, {
  type TrustedIssuerFieldErrorKind,
  type TrustedIssuerFormErrors,
} from '../utils/validateTrustedIssuerForm';

interface TabPanelProps {
  children: ReactNode;
  index: number;
  value: number;
  keepMounted?: boolean;
}

function TabPanel({children, value, index, keepMounted = false}: TabPanelProps): JSX.Element {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`trusted-issuer-tabpanel-${index}`}
      aria-labelledby={`trusted-issuer-tab-${index}`}
    >
      {(keepMounted || value === index) && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

export default function TrustedIssuerDetailPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {id} = useParams<{id: string}>();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const routes = useConnectionRoutes();
  const trustedIssuerQuery = useTrustedIssuer(id);
  const {readOnly, canDelete} = useRecordAccess('connection', trustedIssuerQuery.data);
  const canUpdate = !readOnly;
  const updateMutation = useUpdateTrustedIssuer(id ?? '');
  const deleteMutation = useDeleteConnection(ConnectionTypes.OIDC);
  const isIdJagAcceptanceUnavailable = useIsIdJagAcceptanceUnavailable();

  const [activeTab, setActiveTab] = useState(0);
  const [editedValues, setEditedValues] = useState<Partial<TrustedIssuerFormData>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [nameError, setNameError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [editedAuthzMappings, setEditedAuthzMappings] = useState<AuthorizationRuleMapping[] | undefined | null>(null);
  const [authzMappingsValid, setAuthzMappingsValid] = useState(true);
  const [editedDirectMappings, setEditedDirectMappings] = useState<AuthorizationDirectMapping[] | undefined | null>(
    null,
  );
  const [directMappingsValid, setDirectMappingsValid] = useState(true);
  const [mappingsKey, setMappingsKey] = useState(0);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const data = trustedIssuerQuery.data;

  const baseline: TrustedIssuerFormData = useMemo(
    () => ({
      name: data?.name ?? '',
      issuer: data?.issuer ?? '',
      jwksEndpoint: data?.jwksEndpoint ?? '',
      idJagEnabled: data?.idJagEnabled ?? false,
      tokenExchangeEnabled: data?.tokenExchangeEnabled ?? false,
      trustedTokenAudience: data?.trustedTokenAudience ?? undefined,
    }),
    [data],
  );

  const values: TrustedIssuerFormData = useMemo(() => ({...baseline, ...editedValues}), [baseline, editedValues]);
  const baselineAttr: AttributeConfiguration | undefined = data?.attributeConfiguration;
  const baselineAuthzMappings: AuthorizationRuleMapping[] | undefined = baselineAttr?.authorizationMapping?.rules;
  const baselineDirectMappings: AuthorizationDirectMapping[] | undefined = baselineAttr?.authorizationMapping?.direct;
  const authzMappingsDirty: boolean =
    editedAuthzMappings !== null &&
    JSON.stringify(editedAuthzMappings ?? []) !== JSON.stringify(baselineAuthzMappings ?? []);
  const directMappingsDirty: boolean =
    editedDirectMappings !== null &&
    JSON.stringify(editedDirectMappings ?? []) !== JSON.stringify(baselineDirectMappings ?? []);
  // When a refetch brings a different stored mapping and the section holds no edits, remount it
  // so it shows the stored rows instead of the ones it was first given.
  const storedMapping = baselineAttr?.authorizationMapping;
  const [seededMapping, setSeededMapping] = useState(storedMapping);
  if (storedMapping !== seededMapping && editedAuthzMappings === null && editedDirectMappings === null) {
    setSeededMapping(storedMapping);
    setMappingsKey((prev) => prev + 1);
  }
  // An incomplete row is left out of the payload, so only its validity flag shows the change.
  const dirty: boolean =
    isTrustedIssuerFormDirty(values, baseline) ||
    authzMappingsDirty ||
    directMappingsDirty ||
    !authzMappingsValid ||
    !directMappingsValid;

  const errors: TrustedIssuerFormErrors = useMemo(() => validateTrustedIssuerForm(values), [values]);
  const valid: boolean = Object.keys(errors).length === 0 && authzMappingsValid && directMappingsValid;

  const fieldErrorMessage = (kind: TrustedIssuerFieldErrorKind | undefined): string | undefined => {
    if (kind === 'required') {
      return t('trustedIssuers:validation.required', 'This field is required.');
    }
    if (kind === 'url') {
      return t('trustedIssuers:validation.url', 'Enter a valid https:// URL.');
    }
    return undefined;
  };

  // Resolves an error through the `trustedIssuers` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `trustedIssuers:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `trustedIssuers:${key}`, options),
    [t],
  );

  // An update failure is stale once the user edits any field. Only reset the mutation once it
  // has actually failed: resetting while it's still pending would flip isPending back to false
  // and re-enable save before the in-flight request settles.
  const clearUpdateError = useCallback((): void => {
    setNameError(null);
    setGeneralError(null);
    if (updateMutation.isError) {
      updateMutation.reset();
    }
  }, [updateMutation]);

  const setField = <K extends keyof TrustedIssuerFormData>(field: K, value: TrustedIssuerFormData[K]): void => {
    clearUpdateError();
    setEditedValues((prev) => ({...prev, [field]: value}));
  };

  const setTouchedField = (field: string): void => setTouched((prev) => ({...prev, [field]: true}));

  // The current edits, read after the post-save refetch to tell whether any were made while it ran.
  const latestEdits = useRef({
    editedValues,
    editedAuthzMappings,
    editedDirectMappings,
    authzMappingsValid,
    directMappingsValid,
  });
  useEffect(() => {
    latestEdits.current = {
      editedValues,
      editedAuthzMappings,
      editedDirectMappings,
      authzMappingsValid,
      directMappingsValid,
    };
  }, [editedValues, editedAuthzMappings, editedDirectMappings, authzMappingsValid, directMappingsValid]);

  const resetEdits = (): void => {
    setEditedValues({});
    setEditedAuthzMappings(null);
    setAuthzMappingsValid(true);
    setEditedDirectMappings(null);
    setDirectMappingsValid(true);
    setMappingsKey((prev) => prev + 1);
    setTouched({});
    setNameError(null);
    setGeneralError(null);
  };

  const idJagTitle = t(
    'trustedIssuers:detail.consumption.title',
    'Identity Assertion JWT Authorization Grant (ID-JAG)',
  );
  const idJagUnavailableHint = t(
    'trustedIssuers:idJag.acceptanceUnavailable',
    'This server does not accept identity assertions from any issuer.',
  );
  // While the runtime does not accept identity assertions, ID-JAG can be turned off but not on, and
  // the card body says why whether it is on or off.
  const idJagSwitch = (
    <Switch
      checked={values.idJagEnabled}
      disabled={isIdJagAcceptanceUnavailable && !values.idJagEnabled}
      onChange={(e) => setField('idJagEnabled', e.target.checked)}
      slotProps={{input: {'aria-label': idJagTitle, role: 'switch'}}}
    />
  );

  const isLoading: boolean = trustedIssuerQuery.isLoading;
  const notFound: boolean = !isLoading && !data;

  const handleSave = (): void => {
    if (!valid || !id) return;

    setNameError(null);
    setGeneralError(null);
    const submitted = latestEdits.current;
    // null means untouched (keep the stored value); undefined means the admin cleared the mapping.
    const effectiveRules = editedAuthzMappings !== null ? editedAuthzMappings : baselineAuthzMappings;
    const effectiveDirect = editedDirectMappings !== null ? editedDirectMappings : baselineDirectMappings;
    const hasMapping = Boolean(effectiveRules?.length) || Boolean(effectiveDirect?.length);
    // Send the stored attribute configuration back so the update keeps it. The empty user type default is inert.
    const attributeConfiguration: AttributeConfiguration | undefined =
      baselineAttr || hasMapping
        ? {
            userTypeResolution: {default: ''},
            ...baselineAttr,
            authorizationMapping: hasMapping ? {rules: effectiveRules, direct: effectiveDirect} : undefined,
          }
        : undefined;
    updateMutation.mutate(attributeConfiguration ? {...values, attributeConfiguration} : values, {
      onSuccess: () => {
        // Reset after the refetch so the mapping section remounts from the saved values,
        // unless the admin edited again while it ran.
        void trustedIssuerQuery.refetch().then((result) => {
          // A failed refetch keeps the old values cached, so keep the saved edits instead.
          if (result.isError) return;
          const current = latestEdits.current;
          const unchanged = (Object.keys(current) as (keyof typeof current)[]).every(
            (key) => current[key] === submitted[key],
          );
          if (unchanged) resetEdits();
        });
      },
      onError: (error) => {
        if (isConflictError(error)) {
          // The name field is on General, so show it there.
          setActiveTab(0);
          setNameError(t('trustedIssuers:detail.duplicateName', 'A trusted issuer with this name already exists.'));
        } else {
          setGeneralError(
            getErrorMessage(error, tForErrors, 'update.error', 'Failed to update trusted issuer. Please try again.'),
          );
        }
      },
    });
  };

  return (
    <PageContent>
      <DocumentTitle title={data?.name ?? t('navigation:pages.connections', 'Connections')} />
      <Button
        variant="text"
        startIcon={<ChevronLeft size={16} />}
        onClick={() => void navigate(routes.connections.list())}
        sx={{mb: 2, alignSelf: 'flex-start'}}
      >
        {t('trustedIssuers:detail.back', 'Back to connections')}
      </Button>

      {isLoading ? (
        <Skeleton variant="rounded" height={480} />
      ) : trustedIssuerQuery.error && !data ? (
        <QueryErrorNotice
          error={trustedIssuerQuery.error}
          t={tForErrors}
          variant="block"
          title={t('trustedIssuers:detail.loadError', 'Failed to load trusted issuer.')}
          onRetry={() => void trustedIssuerQuery.refetch()}
        />
      ) : notFound ? (
        <ListingTable.EmptyState
          illustration={<AlertCircle size={40} />}
          title={t('trustedIssuers:detail.notFound.title', 'Trusted issuer not found')}
          description={t(
            'trustedIssuers:detail.notFound.description',
            'This trusted issuer may have been deleted or the link is incorrect.',
          )}
          action={
            <Button variant="outlined" onClick={() => void navigate(routes.connections.list())}>
              {t('trustedIssuers:detail.back', 'Back to connections')}
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
              <ResourceAvatar variant="rounded" size={55} fallback={ConnectionConstants.DEFAULT_TRUSTED_IDP_AVATAR} />
            </Box>
            <Stack direction="column" spacing={0.5}>
              <Typography variant="h5" fontWeight={700}>
                {data?.name}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {data?.issuer}
              </Typography>
            </Stack>
          </Stack>

          {/* A failed refresh keeps the page, and the edits on it, in place. */}
          {trustedIssuerQuery.error && (
            <Box sx={{mb: 3}}>
              <QueryErrorNotice
                error={trustedIssuerQuery.error}
                t={tForErrors}
                variant="inline"
                fallbackKey="trustedIssuers:detail.loadError"
                fallbackDefaultValue="Failed to load trusted issuer."
                onRetry={() => void trustedIssuerQuery.refetch()}
              />
            </Box>
          )}

          {generalError && (
            <Alert severity="error" onClose={clearUpdateError} sx={{mb: 3}}>
              {generalError}
            </Alert>
          )}

          <Tabs
            value={activeTab}
            onChange={(_e: SyntheticEvent, v: number) => setActiveTab(v)}
            aria-label={t('trustedIssuers:detail.tabs.ariaLabel', 'Trusted issuer settings')}
          >
            <Tab
              label={t('connections:detail.tabs.general', 'General')}
              sx={{textTransform: 'none'}}
              id="trusted-issuer-tab-0"
              aria-controls="trusted-issuer-tabpanel-0"
              data-testid="trusted-issuer-tab-general"
            />
            <Tab
              label={t('connections:detail.tabs.attributeMapping', 'Attribute configuration')}
              sx={{textTransform: 'none'}}
              id="trusted-issuer-tab-1"
              aria-controls="trusted-issuer-tabpanel-1"
              data-testid="trusted-issuer-tab-attributes"
            />
            {canDelete && (
              <Tab
                label={t('connections:detail.tabs.advanced', 'Advanced')}
                sx={{textTransform: 'none'}}
                id="trusted-issuer-tab-2"
                aria-controls="trusted-issuer-tabpanel-2"
                data-testid="trusted-issuer-tab-advanced"
              />
            )}
          </Tabs>

          <TabPanel value={activeTab} index={0}>
            <Stack direction="column" spacing={4}>
              <ReadOnlyFieldset readOnly={!canUpdate}>
                <SettingsCard
                  title={t('trustedIssuers:detail.general.title', 'General')}
                  description={t('trustedIssuers:detail.general.description', 'Core identity of this trusted issuer.')}
                >
                  <Stack direction="column" spacing={3}>
                    <TextField
                      labelPlacement="top"
                      label={t('trustedIssuers:create.form.name.label', 'Name')}
                      required
                      id="trusted-issuer-name"
                      fullWidth
                      value={values.name}
                      error={Boolean(nameError ?? (touched['name'] && errors.name))}
                      helperText={nameError ?? (touched['name'] ? fieldErrorMessage(errors.name) : undefined)}
                      onChange={(e) => setField('name', e.target.value)}
                      onBlur={() => setTouchedField('name')}
                    />

                    <TextField
                      labelPlacement="top"
                      label={t('trustedIssuers:create.form.issuer.label', 'Issuer URI')}
                      required
                      id="trusted-issuer-issuer"
                      fullWidth
                      value={values.issuer}
                      error={Boolean(touched['issuer'] && errors.issuer)}
                      helperText={
                        (touched['issuer'] ? fieldErrorMessage(errors.issuer) : undefined) ??
                        t(
                          'trustedIssuers:create.form.issuer.hint',
                          "The issuer URI from the external IdP's OpenID Connect discovery document.",
                        )
                      }
                      onChange={(e) => setField('issuer', e.target.value)}
                      onBlur={() => setTouchedField('issuer')}
                    />

                    <TextField
                      labelPlacement="top"
                      label={t('trustedIssuers:create.form.jwksEndpoint.label', 'JWKS endpoint')}
                      required
                      id="trusted-issuer-jwks-endpoint"
                      fullWidth
                      value={values.jwksEndpoint}
                      error={Boolean(touched['jwksEndpoint'] && errors.jwksEndpoint)}
                      helperText={
                        (touched['jwksEndpoint'] ? fieldErrorMessage(errors.jwksEndpoint) : undefined) ??
                        t(
                          'trustedIssuers:create.form.jwksEndpoint.hint',
                          'The JWKS endpoint used to validate the signature of incoming identity assertions.',
                        )
                      }
                      onChange={(e) => setField('jwksEndpoint', e.target.value)}
                      onBlur={() => setTouchedField('jwksEndpoint')}
                    />
                  </Stack>
                </SettingsCard>
              </ReadOnlyFieldset>

              {data?.ouId && <OrganizationUnitSection ouId={data.ouId} />}

              <ReadOnlyFieldset readOnly={!canUpdate}>
                <SettingsCard
                  title={t('trustedIssuers:detail.tokenExchange.title', 'Token exchange')}
                  description={t(
                    'trustedIssuers:detail.tokenExchange.description',
                    'Exchange subject tokens from this issuer for access tokens.',
                  )}
                  enabled={values.tokenExchangeEnabled}
                  onToggle={(checked) => setField('tokenExchangeEnabled', checked)}
                >
                  <TextField
                    labelPlacement="top"
                    label={t('trustedIssuers:detail.tokenExchange.audience.label', 'Trusted token audience')}
                    id="trusted-issuer-token-audience"
                    fullWidth
                    placeholder="api://thunderid"
                    value={values.trustedTokenAudience ?? ''}
                    helperText={t(
                      'trustedIssuers:detail.tokenExchange.audience.hint',
                      "An additional audience value {{productName}} will accept in subject tokens from this issuer. Tokens whose audience is {{productName}}'s own issuer URL are always accepted.",
                      {productName},
                    )}
                    onChange={(e) => setField('trustedTokenAudience', e.target.value || undefined)}
                  />
                </SettingsCard>
              </ReadOnlyFieldset>

              <ReadOnlyFieldset readOnly={!canUpdate}>
                <SettingsCard
                  title={idJagTitle}
                  description={t(
                    'trustedIssuers:detail.idJag.description',
                    'Accept and exchange signed identity assertions from this issuer for access tokens.',
                  )}
                  headerAction={
                    isIdJagAcceptanceUnavailable ? (
                      <Tooltip title={idJagUnavailableHint}>
                        <span>{idJagSwitch}</span>
                      </Tooltip>
                    ) : (
                      idJagSwitch
                    )
                  }
                >
                  {(values.idJagEnabled || isIdJagAcceptanceUnavailable) && (
                    <Typography variant="body2" color="text.secondary">
                      {isIdJagAcceptanceUnavailable
                        ? idJagUnavailableHint
                        : t(
                            'trustedIssuers:detail.idJag.enabledNote',
                            'Identity assertions from this issuer are accepted via the ID-JAG protocol.',
                          )}
                    </Typography>
                  )}
                </SettingsCard>
              </ReadOnlyFieldset>
            </Stack>
          </TabPanel>

          {/* Kept mounted: the section holds rows being edited in its own state. */}
          <TabPanel value={activeTab} index={1} keepMounted>
            <AuthorizationMappingSection
              key={`authz-${id}-${mappingsKey}`}
              ouId={data?.ouId}
              initialRuleConfig={baselineAuthzMappings}
              initialDirectConfig={baselineDirectMappings}
              readOnly={!canUpdate}
              description={t(
                'trustedIssuers:detail.authorizationMapping.description',
                'Map values in tokens from this issuer to local roles, groups, or permissions. ' +
                  'Tokens from this issuer carry only the permissions the mapping grants, and none without a mapping.',
              )}
              onRuleChange={(mappings, isValid) => {
                clearUpdateError();
                setEditedAuthzMappings(mappings);
                setAuthzMappingsValid(isValid);
              }}
              onDirectChange={(mappings, isValid) => {
                clearUpdateError();
                setEditedDirectMappings(mappings);
                setDirectMappingsValid(isValid);
              }}
            />
          </TabPanel>

          {canDelete && (
            <TabPanel value={activeTab} index={2}>
              <SettingsCard title={t('trustedIssuers:detail.dangerZone.title', 'Danger zone')}>
                <Typography variant="h6" gutterBottom color="error">
                  {t('trustedIssuers:detail.dangerZone.delete.title', 'Delete trusted issuer')}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
                  {t(
                    'trustedIssuers:detail.dangerZone.delete.description',
                    'Applications relying on assertions from this issuer will stop receiving tokens. This cannot be undone.',
                  )}
                </Typography>
                <Button
                  variant="contained"
                  color="error"
                  startIcon={<Trash2 size={16} />}
                  onClick={() => {
                    setDeleteError(null);
                    setDeleteOpen(true);
                  }}
                  data-testid="trusted-issuer-delete-button"
                >
                  {t('common:actions.delete')}
                </Button>
              </SettingsCard>
            </TabPanel>
          )}

          {dirty && canUpdate && (
            <UnsavedChangesBar
              message={t('trustedIssuers:detail.saveBar.unsaved', 'You have unsaved changes')}
              resetLabel={t('trustedIssuers:detail.saveBar.reset', 'Reset')}
              saveLabel={t('trustedIssuers:detail.saveBar.save', 'Save changes')}
              savingLabel={t('common:status.saving', 'Saving...')}
              isSaving={updateMutation.isPending}
              saveDisabled={!valid}
              onReset={resetEdits}
              onSave={handleSave}
            />
          )}

          <ConnectionDeleteDialog
            open={deleteOpen}
            connectionType={ConnectionTypes.OIDC}
            connectionId={id ?? ''}
            connectionName={data?.name ?? ''}
            isPending={deleteMutation.isPending}
            error={deleteError}
            onClose={() => {
              setDeleteOpen(false);
              setDeleteError(null);
            }}
            onConfirm={() => {
              if (!id) return;
              deleteMutation.mutate(id, {
                onSuccess: () => {
                  setDeleteOpen(false);
                  void navigate(routes.connections.list());
                },
                onError: (error) => {
                  setDeleteError(
                    getErrorMessage(
                      error,
                      tForErrors,
                      'delete.error',
                      'Failed to delete trusted issuer. Please try again.',
                    ),
                  );
                },
              });
            }}
          />
        </>
      )}
    </PageContent>
  );
}
