// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AccessStatusBanner,
  AccountStatusAction,
  DocumentTitle,
  PageLoadingAnimation,
  QueryErrorNotice,
  ResourceAvatar,
  UnsavedChangesBar,
} from '@thunderid/components';
import {useOrganizationAccess} from '@thunderid/configure-resource-sharing';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {isEqualIgnoringEmpty} from '@thunderid/utils';
import {
  Box,
  Stack,
  Typography,
  Button,
  Alert,
  IconButton,
  TextField,
  Chip,
  Tabs,
  Tab,
  PageContent,
  PageTitle,
  Dialog,
  DialogContent,
} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useEffect, useMemo, useRef, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useLocation, useNavigate, useParams} from 'react-router';
import useGetApplication from '../api/useGetApplication';
import useUpdateApplication from '../api/useUpdateApplication';
import SettingsLockNotice from '../components/common/SettingsLockNotice';
import ShowClientSecret from '../components/create-application/ShowClientSecret';
import EditAccessSettings from '../components/edit-application/access/EditAccessSettings';
import EditAdvancedSettings from '../components/edit-application/advanced-settings/EditAdvancedSettings';
import CimdDocumentSection from '../components/edit-application/cimd/CimdDocumentSection';
import EditCredentialsSettings from '../components/edit-application/credentials/EditCredentialsSettings';
import EditCustomizationSettings from '../components/edit-application/customization-settings/EditCustomizationSettings';
import EditFlowsSettings from '../components/edit-application/flows-settings/EditFlowsSettings';
import IntegrationGuides from '../components/edit-application/integration-guides/IntegrationGuides';
import McpConnectTab from '../components/edit-application/mcp/McpConnectTab';
import EditOrganizationAccessSettings from '../components/edit-application/organization-access/EditOrganizationAccessSettings';
import EditTokenSettings from '../components/edit-application/token-settings/EditTokenSettings';
import EditTokenSettingsTabs from '../components/edit-application/token-settings/EditTokenSettingsTabs';
import ApplicationConstants from '../constants/application-constants';
import ApplicationQueryKeys from '../constants/application-query-keys';
import TemplateConstants from '../constants/template-constants';
import useApplicationRoutes from '../hooks/useApplicationRoutes';
import type {Application} from '../models/application';
import type {OAuth2Config} from '../models/oauth';
import {OAuth2GrantTypes, TokenEndpointAuthMethods} from '../models/oauth';
import getApplicationErrorMessage from '../utils/getApplicationErrorMessage';
import getTemplateCapabilities from '../utils/getTemplateCapabilities';
import getTemplateFieldConstraints from '../utils/getTemplateFieldConstraints';
import getTemplateMetadata from '../utils/getTemplateMetadata';
import isSystemConsoleClient from '../utils/isSystemConsoleClient';
import isValidRedirectUriFormat from '../utils/isValidRedirectUriFormat';
import {hasClientAccess, hasUserAccess} from '../utils/oauth2Rules';
import validateBackchannelLogoutUri from '../utils/validateBackchannelLogoutUri';

interface TabConfig {
  key: string;
  label: string;
  panel: React.ReactNode;
  hidden?: boolean;
}

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

interface JustCreatedSecret {
  appName: string;
  clientId?: string;
  clientSecret?: string;
  flowSecret?: string;
}

function TabPanel({children = null, value, index, ...other}: TabPanelProps) {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`edit-tabpanel-${index}`}
      aria-labelledby={`edit-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{py: 3}}>{children}</Box>}
    </div>
  );
}

export default function ApplicationEditPage() {
  const routes = useApplicationRoutes();
  const logger = useLogger('ApplicationEditPage');
  const {t} = useTranslation();
  const {config} = useConfig();
  const navigate = useNavigate();
  const location = useLocation();
  const {applicationId} = useParams<{applicationId: string}>();

  const {data: application, isLoading, error, refetch} = useGetApplication(applicationId ?? '');
  const updateApplication = useUpdateApplication();
  const handleStatusChanged = useCallback((): void => {
    void refetch();
  }, [refetch]);
  // TODO: Offer Organization Access to every application once sharing an application that signs
  // users in is supported, by removing this check. Until then a granted unit can only request tokens
  // through the application, so only applications with the client credentials grant get the tab,
  // whatever other grants they also have. Read from the saved application, so the tab does not
  // appear or vanish while grant types are being edited.
  const offersOrganizationAccess = useMemo(
    () =>
      hasClientAccess(application?.inboundAuthConfig?.find((config) => config.type === 'oauth2')?.config.grantTypes),
    [application],
  );
  // Held here rather than in its tab, because inactive tabs unmount and the draft must survive them.
  const organizationAccess = useOrganizationAccess({
    resourcePath: `applications/${applicationId}`,
    enabled: Boolean(applicationId) && offersOrganizationAccess,
    // The tab's sentences say what access means for an application: `applications:organizationAccess.*`.
    wordingNamespace: 'applications',
  });

  // Resolves an error through the `applications` catalog. `t` defaults to the `common` namespace,
  // so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `applications:`,
  // per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `applications:${key}`, options),
    [t],
  );

  // The create wizard hands over the new secret in the navigation state. Keep it for this mount only
  // and drop it from the history entry, so reloading the page never shows the secret again.
  const navigationSecret = (location.state as {justCreatedSecret?: JustCreatedSecret} | null)?.justCreatedSecret;
  const [justCreatedSecret] = useState(navigationSecret);
  const [secretDialogOpen, setSecretDialogOpen] = useState(Boolean(justCreatedSecret));

  useEffect(() => {
    if (navigationSecret) {
      void navigate(location.pathname, {replace: true, state: null});
    }
  }, [navigationSecret, navigate, location.pathname]);

  const [activeTabKey, setActiveTabKey] = useState('overview');
  const [editedApp, setEditedApp] = useState<Partial<Application>>({});
  // Bumped on Save/Reset to force AccessSection/McpAccessSection/UrlsSection to remount with a
  // clean form — they keep local state (redirect URI list, react-hook-form defaults) that a
  // `setEditedApp({})` alone wouldn't reset.
  const [sectionResetKey, setSectionResetKey] = useState(0);
  const [isEditingName, setIsEditingName] = useState(false);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempName, setTempName] = useState('');
  const [tempDescription, setTempDescription] = useState('');
  const [hasValidationErrors, setHasValidationErrors] = useState(false);
  const [mcpAccessInvalid, setMcpAccessInvalid] = useState(false);
  const [advancedSettingsInvalid, setAdvancedSettingsInvalid] = useState(false);
  const [customizationSettingsInvalid, setCustomizationSettingsInvalid] = useState(false);
  const [accessSettingsInvalid, setAccessSettingsInvalid] = useState(false);
  const [credentialsSettingsInvalid, setCredentialsSettingsInvalid] = useState(false);

  const handleBack = async () => {
    await navigate(routes.applications.list());
  };

  const createTabChangeHandler = (tabs: TabConfig[]) => (_event: SyntheticEvent, newValue: number) => {
    const tab = tabs[newValue];
    if (tab) {
      setActiveTabKey(tab.key);
    }
  };

  const oauth2Constraints = useMemo(
    () => getTemplateFieldConstraints(application?.template)?.oauth2,
    [application?.template],
  );

  // Attestation is offered only for templates that declare the capability (e.g. mobile).
  const supportsAttestation = useMemo(
    () => Boolean(getTemplateCapabilities(application?.template)?.attestation),
    [application?.template],
  );

  const handleFieldChange = useCallback(
    (field: keyof Application, value: unknown) => {
      // A previous save error is stale the moment the form changes again.
      updateApplication.reset();
      setEditedApp((prev) => ({...prev, [field]: value}));
    },
    [updateApplication],
  );

  const commitName = useCallback(
    (value: string): void => {
      const trimmedName = value.trim();
      // The API rejects names outside these bounds, so an out of range rename is discarded here.
      if (
        trimmedName.length < ApplicationConstants.NAME_MIN_LENGTH ||
        trimmedName.length > ApplicationConstants.NAME_MAX_LENGTH
      ) {
        return;
      }
      handleFieldChange('name', trimmedName);
    },
    [handleFieldChange],
  );

  const hasApplicationChanges = useMemo(
    () =>
      Object.entries(editedApp).some(
        ([key, value]) => !isEqualIgnoringEmpty(value, application?.[key as keyof Application]),
      ),
    [editedApp, application],
  );
  const hasChanges = hasApplicationChanges || organizationAccess.hasChanges;

  // One flag over the whole save (application update, refetch, then access save), so the bar's Save and
  // Reset stay disabled between the steps. The ref also drops a second click before the re-render lands.
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const handleSave = useCallback(async () => {
    if (submittingRef.current || !application || !applicationId) return;

    const {certificate, ...updatedData} = {
      ...application,
      ...editedApp,
    };
    void certificate;

    // The application is saved first, so a refused organization access change leaves the
    // application's own settings saved and only the access draft waiting to be retried.
    submittingRef.current = true;
    setIsSubmitting(true);
    try {
      if (hasApplicationChanges) {
        await updateApplication.mutateAsync({
          applicationId,
          data: updatedData,
        });
        setEditedApp({});
        await refetch();
        // Bumped only after refetch resolves to prevent stale data being passed to the remounted sections.
        setSectionResetKey((key) => key + 1);
      }
      if (organizationAccess.hasChanges) {
        await organizationAccess.save();
      }
    } catch {
      logger.error('Failed to update application');
    } finally {
      submittingRef.current = false;
      setIsSubmitting(false);
    }
  }, [
    application,
    applicationId,
    editedApp,
    hasApplicationChanges,
    updateApplication,
    organizationAccess,
    refetch,
    logger,
  ]);

  const resourceTypeTitle = t('navigation:pages.applications', 'Applications');

  if (isLoading) {
    return (
      <>
        <DocumentTitle title={resourceTypeTitle} />
        <PageLoadingAnimation />
      </>
    );
  }

  if (error) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="block"
          title={t('applications:edit.page.error', 'Failed to load application information')}
          resolveErrorMessage={getApplicationErrorMessage}
          onRetry={() => void refetch()}
          action={
            <Button
              onClick={() => {
                handleBack().catch(() => null);
              }}
              startIcon={<ArrowLeft size={16} />}
            >
              {t('applications:edit.page.back', 'Back to applications')}
            </Button>
          }
        />
      </PageContent>
    );
  }

  if (!application) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
        <Alert severity="warning" sx={{mb: 2}}>
          {t('applications:edit.page.notFound')}
        </Alert>
        <Button
          onClick={() => {
            handleBack().catch(() => null);
          }}
          startIcon={<ArrowLeft size={16} />}
        >
          {t('applications:edit.page.back')}
        </Button>
      </PageContent>
    );
  }

  const oauth2Config: OAuth2Config | undefined = (editedApp.inboundAuthConfig ?? application.inboundAuthConfig)?.find(
    (config) => config.type === 'oauth2',
  )?.config;

  const isMcpClient = application.template === TemplateConstants.MCP_CLIENT_TEMPLATE_ID;
  // A metadata-document client's redirect URIs, authentication method, and keys come from its document.
  const isCimdClient = oauth2Config?.clientIdMetadataDocument === true;

  // Suspending the Console's own client would lock every operator out, with nowhere to unsuspend it.
  const isConsoleClient = isSystemConsoleClient(oauth2Config?.clientId, config?.client?.client_id);

  // User-facing tabs (Flows, Customization) and the general Access section only apply when the
  // client can act on behalf of a user. When it can't, they are frozen/hidden rather than removed.
  const userAccessUnlocked = !oauth2Config || hasUserAccess(oauth2Config.grantTypes);
  const userGatedApplication = userAccessUnlocked ? application : {...application, isReadOnly: true};
  const userAccessLockMessage = t(
    'applications:edit.userAccessLock.message',
    'These settings apply only to user-facing flows. Enable a user-facing grant (e.g. authorization code) in the Advanced tab to configure them.',
  );

  // Page-level required checks, gated on the grant types they apply to, surfaced by name in the
  // save bar. Computed from state (not reported by the tabs) since inactive tabs are unmounted.
  // MCP clients validate their redirect URIs via McpConnectTab, so that check is skipped there. The
  // certificate check applies to every client with a Credentials tab, which a CIMD MCP client lacks.
  const grantTypes = oauth2Config?.grantTypes ?? [];
  const hasAuthorizationCodeGrant = grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
  const hasValidRedirectUri = (oauth2Config?.redirectUris ?? []).some((uri) => isValidRedirectUriFormat(uri));
  const isMissingRedirectUri = !isMcpClient && hasAuthorizationCodeGrant && !hasValidRedirectUri;
  const isMissingCertificate =
    !(isMcpClient && isCimdClient) &&
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.PRIVATE_KEY_JWT &&
    !oauth2Config?.certificate?.value;
  // The field shows only with a user-facing grant, so a hidden value never blocks saving. The field
  // reports only its typed draft; the stored value is checked here, since the tab may be unmounted.
  const isInvalidBackchannelLogoutUri =
    hasUserAccess(grantTypes) &&
    !validateBackchannelLogoutUri(oauth2Config?.backchannelLogoutUri, oauth2Config?.publicClient ?? false).valid;

  // Each issue is a full, standalone sentence so the message needs no grammar assembly and stays
  // translatable; multiple issues read as consecutive sentences.
  const validationIssues: string[] = [];
  if (isMissingRedirectUri) {
    validationIssues.push(t('applications:edit.page.validation.missingRedirectUri', 'A redirect URI is required.'));
  }
  if (isMissingCertificate) {
    validationIssues.push(t('applications:edit.page.validation.missingCertificate', 'A certificate is required.'));
  }
  if (isInvalidBackchannelLogoutUri) {
    validationIssues.push(
      t('applications:edit.page.validation.invalidBackchannelLogoutUri', 'The back-channel logout URI is not valid.'),
    );
  }

  const unsavedChangesMessage =
    validationIssues.length > 0 ? validationIssues.join(' ') : t('applications:edit.page.unsavedChanges');

  const baseMcpTabs: TabConfig[] = isMcpClient
    ? (
        [
          {
            key: 'general',
            label: t('applications:edit.page.tabs.general'),
            panel: (
              <McpConnectTab
                application={application}
                oauth2Config={oauth2Config}
                onFieldChange={handleFieldChange}
                isReadOnly={application.isReadOnly === true}
                onValidationChange={setMcpAccessInvalid}
                sectionResetKey={sectionResetKey}
                onGoToCredentials={isCimdClient ? undefined : () => setActiveTabKey('credentials')}
              />
            ),
          },
          {
            key: 'credentials',
            label: t('applications:edit.page.tabs.credentials', 'Credentials'),
            panel: (
              <EditCredentialsSettings
                application={application}
                editedApp={editedApp}
                oauth2Config={oauth2Config}
                onFieldChange={handleFieldChange}
                showAttestation={false}
                onValidationChange={setCredentialsSettingsInvalid}
              />
            ),
            // A metadata-document client's keys come from its document.
            hidden: isCimdClient,
          },
          {
            key: 'flows',
            label: t('applications:edit.page.tabs.flows'),
            panel: (
              <EditFlowsSettings application={application} editedApp={editedApp} onFieldChange={handleFieldChange} />
            ),
            hidden: !userAccessUnlocked,
          },
          {
            key: 'customization',
            label: t('applications:edit.page.tabs.customization'),
            panel: (
              <EditCustomizationSettings
                application={application}
                editedApp={editedApp}
                onFieldChange={handleFieldChange}
                onValidationChange={setCustomizationSettingsInvalid}
                sectionResetKey={sectionResetKey}
              />
            ),
            hidden: !userAccessUnlocked,
          },
          {
            key: 'token',
            label: t('applications:edit.page.tabs.token'),
            panel: (
              <EditTokenSettings
                sectionResetKey={sectionResetKey}
                application={application}
                editedApp={editedApp}
                oauth2Config={oauth2Config}
                onFieldChange={handleFieldChange}
                onValidationChange={setHasValidationErrors}
              />
            ),
          },
          {
            key: 'organizationAccess',
            label: t('applications:edit.page.tabs.organizationAccess', 'Organization Access'),
            panel: <EditOrganizationAccessSettings application={application} access={organizationAccess} />,
            hidden: !offersOrganizationAccess,
          },
          {
            key: 'advanced',
            label: t('applications:edit.page.tabs.advanced'),
            panel: (
              <EditAdvancedSettings
                application={application}
                editedApp={editedApp}
                oauth2Config={oauth2Config}
                // The backend rejects pkceRequired: true without the authorization_code grant, so
                // the template's PKCE lock only applies while that grant is present. A client without
                // it (machine-to-machine or JWT bearer only) is stored with pkceRequired: false.
                oauth2Constraints={hasAuthorizationCodeGrant ? oauth2Constraints : undefined}
                onFieldChange={handleFieldChange}
                allowedGrantTypes={[...TemplateConstants.MCP_CLIENT_ALLOWED_GRANT_TYPES]}
                // MCP clients manage their redirect URIs on their own Connect tab.
                showRedirectUris={false}
                sectionResetKey={sectionResetKey}
                onValidationChange={setAdvancedSettingsInvalid}
                onDeleteSuccess={() => {
                  handleBack().catch(() => null);
                }}
              />
            ),
          },
        ] satisfies TabConfig[]
      ).filter((tab) => !tab.hidden)
    : [];

  const mcpFlowsTabIndex = baseMcpTabs.findIndex((tab) => tab.key === 'flows');
  const mcpCustomizationTabIndex = baseMcpTabs.findIndex((tab) => tab.key === 'customization');

  const mcpTabs: TabConfig[] = isMcpClient
    ? [
        {
          key: 'overview',
          label: t('applications:edit.page.tabs.overview'),
          panel: (
            <IntegrationGuides
              application={application}
              oauth2Config={oauth2Config}
              onGoToFlows={mcpFlowsTabIndex >= 0 ? () => setActiveTabKey('flows') : undefined}
              onGoToCustomization={mcpCustomizationTabIndex >= 0 ? () => setActiveTabKey('customization') : undefined}
            />
          ),
        },
        ...baseMcpTabs,
      ]
    : [];

  const standardTabs: TabConfig[] = !isMcpClient
    ? [
        {
          key: 'overview',
          label: t('applications:edit.page.tabs.overview'),
          panel: (
            <IntegrationGuides
              application={application}
              oauth2Config={oauth2Config}
              onGoToFlows={() => setActiveTabKey('flows')}
              onGoToCustomization={() => setActiveTabKey('customization')}
            />
          ),
        },
        // An MCP client shows its document on the Connect tab instead.
        ...(isCimdClient && !isMcpClient && oauth2Config
          ? [
              {
                key: 'metadata-document',
                label: t('applications:edit.page.tabs.metadataDocument', 'Metadata document'),
                panel: (
                  <CimdDocumentSection
                    application={application}
                    oauth2Config={oauth2Config}
                    onFieldChange={handleFieldChange}
                    isReadOnly={application.isReadOnly === true}
                  />
                ),
              },
            ]
          : []),
        {
          key: 'access',
          label: t('applications:edit.page.tabs.access', 'Access'),
          panel: (
            <EditAccessSettings
              application={application}
              editedApp={editedApp}
              onFieldChange={handleFieldChange}
              onValidationChange={setAccessSettingsInvalid}
              showUserAccessConfig={userAccessUnlocked}
              sectionResetKey={sectionResetKey}
            />
          ),
        },
        ...(isCimdClient
          ? []
          : [
              {
                key: 'credentials',
                label: t('applications:edit.page.tabs.credentials', 'Credentials'),
                panel: (
                  <EditCredentialsSettings
                    application={application}
                    editedApp={editedApp}
                    oauth2Config={oauth2Config}
                    onFieldChange={handleFieldChange}
                    showAttestation={supportsAttestation}
                    onValidationChange={setCredentialsSettingsInvalid}
                  />
                ),
              },
            ]),
        {
          key: 'flows',
          label: t('applications:edit.page.tabs.flows'),
          panel: (
            <SettingsLockNotice isUnlocked={userAccessUnlocked} message={userAccessLockMessage}>
              <EditFlowsSettings
                application={userGatedApplication}
                editedApp={editedApp}
                onFieldChange={handleFieldChange}
              />
            </SettingsLockNotice>
          ),
        },
        {
          key: 'customization',
          label: t('applications:edit.page.tabs.customization'),
          panel: (
            <SettingsLockNotice isUnlocked={userAccessUnlocked} message={userAccessLockMessage}>
              <EditCustomizationSettings
                application={userGatedApplication}
                editedApp={editedApp}
                onFieldChange={handleFieldChange}
                onValidationChange={setCustomizationSettingsInvalid}
                sectionResetKey={sectionResetKey}
              />
            </SettingsLockNotice>
          ),
        },
        {
          key: 'token',
          label: t('applications:edit.page.tabs.token'),
          panel: (
            <EditTokenSettingsTabs
              sectionResetKey={sectionResetKey}
              application={application}
              editedApp={editedApp}
              oauth2Config={oauth2Config}
              onFieldChange={handleFieldChange}
              onValidationChange={setHasValidationErrors}
            />
          ),
        },
        {
          key: 'organizationAccess',
          label: t('applications:edit.page.tabs.organizationAccess', 'Organization Access'),
          panel: <EditOrganizationAccessSettings application={application} access={organizationAccess} />,
          hidden: !offersOrganizationAccess,
        },
        {
          key: 'advanced',
          label: t('applications:edit.page.tabs.advanced'),
          panel: (
            <EditAdvancedSettings
              application={application}
              editedApp={editedApp}
              oauth2Config={oauth2Config}
              oauth2Constraints={oauth2Constraints}
              onFieldChange={handleFieldChange}
              showRedirectUris={userAccessUnlocked && !isCimdClient}
              sectionResetKey={sectionResetKey}
              onValidationChange={setAdvancedSettingsInvalid}
              onDeleteSuccess={() => {
                handleBack().catch(() => null);
              }}
            />
          ),
        },
      ].filter((tab) => !tab.hidden)
    : [];

  const activeTabs = isMcpClient ? mcpTabs : standardTabs;
  const activeTabIndex = Math.max(
    0,
    activeTabs.findIndex((tab) => tab.key === activeTabKey),
  );

  return (
    <PageContent>
      <DocumentTitle title={application.name} />
      {application.isReadOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}
      <AccessStatusBanner status={application.status} category="application" sx={{mb: 2}} />
      {/* Header */}
      <PageTitle>
        <PageTitle.BackButton component={<Link to={routes.applications.list()} />}>
          {t('applications:edit.page.back')}
        </PageTitle.BackButton>
        <PageTitle.Avatar variant="rounded" sx={{overflow: 'visible'}}>
          <ResourceAvatar
            size={55}
            variant="rounded"
            supportedShapes={['rounded']}
            editable={!application.isReadOnly}
            value={editedApp.logoUrl ?? application.logoUrl}
            fallback={ApplicationConstants.DEFAULT_AVATAR}
            editAriaLabel={t('applications:edit.page.logoUpdate.label', 'Update logo')}
            onSelect={(newLogoUrl: string) => {
              // Can't go through handleFieldChange: reverting to the original logo drops the key
              // rather than setting it, so the stale save error has to be cleared here too.
              updateApplication.reset();
              setEditedApp((prev) => {
                if (newLogoUrl === application.logoUrl) {
                  const {logoUrl, ...rest} = prev;
                  void logoUrl;
                  return rest;
                }
                return {...prev, logoUrl: newLogoUrl};
              });
            }}
            onSave={handleSave}
          />
        </PageTitle.Avatar>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  commitName(tempName);
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    commitName(tempName);
                    setIsEditingName(false);
                  } else if (e.key === 'Escape') {
                    setIsEditingName(false);
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{editedApp.name ?? application.name}</Typography>
                {!application.isReadOnly && (
                  <IconButton
                    size="small"
                    onClick={() => {
                      setTempName(editedApp.name ?? application.name);
                      setIsEditingName(true);
                    }}
                    sx={{
                      opacity: 0.6,
                      '&:hover': {opacity: 1},
                    }}
                  >
                    <Edit size={16} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <Stack direction="row" alignItems="flex-start" spacing={1}>
            {isEditingDescription ? (
              <TextField
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={() => {
                  const trimmedDescription = tempDescription.trim();
                  const currentValue = editedApp.description ?? application.description ?? '';
                  if (trimmedDescription !== currentValue) {
                    handleFieldChange('description', trimmedDescription);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmedDescription = tempDescription.trim();
                    const currentValue = editedApp.description ?? application.description ?? '';
                    if (trimmedDescription !== currentValue) {
                      handleFieldChange('description', trimmedDescription);
                    }
                    setIsEditingDescription(false);
                  } else if (e.key === 'Escape') {
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('applications:edit.page.description.placeholder')}
                sx={{
                  maxWidth: '600px',
                  '& .MuiInputBase-root': {
                    fontSize: '0.875rem',
                  },
                }}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {editedApp.description ?? application.description ?? t('applications:edit.page.description.empty')}
                </Typography>
                {!application.isReadOnly && (
                  <IconButton
                    size="small"
                    onClick={() => {
                      setTempDescription(editedApp.description ?? application.description ?? '');
                      setIsEditingDescription(true);
                    }}
                    sx={{
                      opacity: 0.6,
                      '&:hover': {opacity: 1},
                      mt: -0.5,
                    }}
                  >
                    <Edit size={14} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
          {(editedApp.template ?? application.template) &&
            (() => {
              const templateMetadata = getTemplateMetadata(editedApp.template ?? application.template);
              return templateMetadata ? (
                <Box sx={{mt: 1}}>
                  <Chip
                    label={templateMetadata.displayName}
                    size="small"
                    color="primary"
                    variant="outlined"
                    sx={{fontSize: '0.7rem'}}
                  />
                </Box>
              ) : null;
            })()}
        </PageTitle.SubHeader>
        {/* Read-only records too: the flows change only runtime state. */}
        {!isConsoleClient && (
          <PageTitle.Actions>
            <AccountStatusAction
              category="application"
              subject={application.id}
              status={application.status}
              listQueryKey={[ApplicationQueryKeys.APPLICATIONS]}
              onChanged={handleStatusChanged}
            />
          </PageTitle.Actions>
        )}
      </PageTitle>

      {/* Tabs */}
      <Tabs value={activeTabIndex} onChange={createTabChangeHandler(activeTabs)} aria-label="application settings tabs">
        {activeTabs.map((tab, index) => (
          <Tab
            key={tab.key}
            label={tab.label}
            id={`edit-tab-${index}`}
            aria-controls={`edit-tabpanel-${index}`}
            sx={{textTransform: 'none'}}
          />
        ))}
      </Tabs>

      {/* Tab Panels */}
      <>
        {activeTabs.map((tab, index) => (
          <TabPanel key={tab.key} value={activeTabIndex} index={index}>
            {tab.panel}
          </TabPanel>
        ))}
      </>

      {/* Floating Action Bar */}
      {hasChanges && (
        <UnsavedChangesBar
          message={unsavedChangesMessage}
          resetLabel={t('applications:edit.page.reset')}
          saveLabel={t('applications:edit.page.save')}
          savingLabel={t('applications:edit.page.saving')}
          isSaving={isSubmitting || updateApplication.isPending || organizationAccess.isSaving}
          resetDisabled={isSubmitting || updateApplication.isPending || organizationAccess.isSaving}
          saveDisabled={
            hasValidationErrors ||
            mcpAccessInvalid ||
            customizationSettingsInvalid ||
            advancedSettingsInvalid ||
            accessSettingsInvalid ||
            credentialsSettingsInvalid ||
            isMissingRedirectUri ||
            isInvalidBackchannelLogoutUri ||
            isMissingCertificate ||
            application.isReadOnly === true
          }
          error={
            (updateApplication.error
              ? getApplicationErrorMessage(
                  updateApplication.error,
                  tForErrors,
                  'update.error',
                  'Failed to update application. Please try again.',
                )
              : undefined) ??
            (organizationAccess.saveError
              ? getApplicationErrorMessage(
                  organizationAccess.saveError,
                  tForErrors,
                  'edit.organizationAccess.save.error',
                  'Failed to update organization access. Please try again.',
                )
              : undefined)
          }
          onReset={() => {
            updateApplication.reset();
            organizationAccess.reset();
            setEditedApp({});
            setHasValidationErrors(false);
            setMcpAccessInvalid(false);
            setAdvancedSettingsInvalid(false);
            setCustomizationSettingsInvalid(false);
            setAccessSettingsInvalid(false);
            setCredentialsSettingsInvalid(false);
            setSectionResetKey((key) => key + 1);
          }}
          onSave={() => {
            handleSave().catch(() => null);
          }}
        />
      )}

      {justCreatedSecret && (
        <Dialog open={secretDialogOpen} onClose={() => setSecretDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogContent>
            <ShowClientSecret
              clientSecret={justCreatedSecret.clientSecret}
              flowSecret={justCreatedSecret.flowSecret}
              onContinue={() => setSecretDialogOpen(false)}
            />
          </DialogContent>
        </Dialog>
      )}
    </PageContent>
  );
}
