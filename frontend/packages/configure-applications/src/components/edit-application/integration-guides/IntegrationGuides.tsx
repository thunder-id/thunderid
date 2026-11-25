// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AndroidLogo,
  AppleIcon,
  CopyableField,
  QueryErrorNotice,
  ExpressIcon,
  ExternalLinkConfirmDialog,
  FlutterLogo,
  JavaScriptIcon,
  NextjsIcon,
  NodeIcon,
  NuxtIcon,
  PythonLogo,
  ReactIcon,
  StackblitzQuickstartCard,
  useExternalLinkConfirmation,
  VueIcon,
} from '@thunderid/components';
import {GatePreview} from '@thunderid/configure-design';
import {useGetFlowById} from '@thunderid/configure-flows';
import {useGetOrganizationUnit} from '@thunderid/configure-organization-units';
import {useConfig, useRuntimeUrl} from '@thunderid/contexts';
import {DefaultTheme, type Theme, useGetTheme} from '@thunderid/design';
import {Box, Button, Collapse, Divider, Paper, Stack, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {ArrowRight, ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import ConnectAppGuide from './ConnectAppGuide';
import EnvironmentBar from './EnvironmentBar';
import EnvironmentClientIdField from './EnvironmentClientIdField';
import type {QuickstartGuideSource} from './QuickstartGuide';
import useResourceEnvironments from '../../../hooks/useResourceEnvironments';
import type {Application} from '../../../models/application';
import type {OAuth2Config} from '../../../models/oauth';
import {getIntegrationGuideForTemplate} from '../../../utils/getIntegrationGuidesForTemplate';
import getPlaygroundsForTemplate from '../../../utils/getPlaygroundsForTemplate';
import getQuickstartsForTemplate from '../../../utils/getQuickstartsForTemplate';
import getTemplateMetadata from '../../../utils/getTemplateMetadata';
import normalizeTemplateId from '../../../utils/normalizeTemplateId';
import {hasUserAccess} from '../../../utils/oauth2Rules';
import resolveTemplateLink from '../../../utils/resolveTemplateLink';
import EnvironmentNotDeployedNotice from '../../environments/EnvironmentNotDeployedNotice';

/**
 * Props for the {@link IntegrationGuides} component.
 */
interface IntegrationGuidesProps {
  /**
   * The application to show the overview for
   */
  application: Application;
  /**
   * OAuth2 configuration containing client credentials (optional)
   */
  oauth2Config?: OAuth2Config;
  /**
   * Navigates to the application's Flows tab.
   */
  onGoToFlows?: () => void;
  /**
   * Navigates to the application's Customization tab.
   */
  onGoToCustomization?: () => void;
}

/** Vendor logo shown next to a quickstart guide for a given platform's {@link QuickstartLink.label}. */
const QUICKSTART_ICONS: Record<string, JSX.Element> = {
  Android: <AndroidLogo size={18} />,
  Flutter: <FlutterLogo size={18} />,
  iOS: <AppleIcon size={18} />,
  React: <ReactIcon size={18} />,
  Vue: <VueIcon size={18} />,
  'Next.js': <NextjsIcon size={18} />,
  Nuxt: <NuxtIcon size={18} />,
  Express: <ExpressIcon size={18} />,
  'Node.js': <NodeIcon size={18} />,
  JavaScript: <JavaScriptIcon size={18} />,
  Python: <PythonLogo size={18} />,
};

/** A bordered card with a title/description header and arbitrary content below. */
function OverviewCard({
  title,
  description,
  headerAction = undefined,
  children,
  sx = undefined,
}: {
  title: ReactNode;
  description: ReactNode;
  headerAction?: ReactNode;
  children: ReactNode;
  sx?: object;
}): JSX.Element {
  return (
    <Paper variant="outlined" sx={{borderRadius: '10px', p: 2.25, ...sx}}>
      <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={2} sx={{mb: 1.75}}>
        <Box>
          <Typography variant="subtitle1" sx={{fontWeight: 600, mb: 0.25}}>
            {title}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {description}
          </Typography>
        </Box>
        {headerAction}
      </Stack>
      {children}
    </Paper>
  );
}

/**
 * Overview tab for an application's edit page.
 *
 * Always shows the application's identifiers, and, for OAuth2-enabled applications, its OIDC
 * endpoints. When the application's template has a quickstart or a coding agent prompt, it leads
 * with a guide to connecting an app, with this application's values filled in. Sign-in-facing
 * applications also get a summary of their sign-in experience: a preview, the theme, and the flows.
 */
export default function IntegrationGuides({
  application,
  oauth2Config = undefined,
  onGoToFlows = undefined,
  onGoToCustomization = undefined,
}: IntegrationGuidesProps): JSX.Element {
  const {t} = useTranslation();
  const [isEndpointsOpen, setIsEndpointsOpen] = useState(false);
  const {config, getDocumentationLink} = useConfig();
  const {data: themeDetails, error: themeError, refetch: refetchTheme} = useGetTheme(application.themeId ?? '');
  const {data: authFlowDetails} = useGetFlowById(application.authFlowId);
  const {data: registrationFlowDetails} = useGetFlowById(
    application.registrationFlowId,
    Boolean(application.isRegistrationFlowEnabled && application.registrationFlowId),
  );
  const {data: recoveryFlowDetails} = useGetFlowById(
    application.recoveryFlowId,
    Boolean(application.isRecoveryFlowEnabled && application.recoveryFlowId),
  );
  const {data: signOutFlowDetails} = useGetFlowById(application.signOutFlowId);
  const {data: organizationUnit} = useGetOrganizationUnit(application.ouId);
  const {mode, systemMode} = useColorScheme();
  const productName = config.brand.product_name;
  const externalLinkConfirmation = useExternalLinkConfirmation();

  const templateId = application.template ?? undefined;
  const guide = getIntegrationGuideForTemplate(templateId);
  const promptUrl = resolveTemplateLink(guide?.llm_prompt.docsUrl, getDocumentationLink);
  const quickstarts = getQuickstartsForTemplate(templateId) ?? [];
  const playgrounds = getPlaygroundsForTemplate(templateId) ?? [];
  // One guide per platform SDK the template covers (e.g. Mobile spans iOS, Android, and Flutter).
  // Entries whose docs link isn't configured (resolves to undefined) are dropped rather than
  // rendered with a dead link.
  const quickstartGuides: QuickstartGuideSource[] = quickstarts.flatMap((entry) => {
    const docsUrl = resolveTemplateLink(entry.docsUrl, getDocumentationLink);
    return docsUrl
      ? [
          {
            label: entry.label,
            docsUrl,
            icon: QUICKSTART_ICONS[entry.label],
            promptUrl: resolveTemplateLink(entry.promptUrl, getDocumentationLink),
            section: entry.section,
          },
        ]
      : [];
  });
  // A template with exactly one playground gets the runnable banner, provided it's on an
  // environment the console knows how to render (currently only StackBlitz). Templates covering
  // multiple platform SDKs, or platforms with no runnable environment (e.g. native mobile), have
  // no banner.
  const primaryPlayground = playgrounds.length === 1 ? playgrounds[0] : undefined;
  const primaryPlaygroundUrl =
    primaryPlayground?.environment === 'stackblitz'
      ? resolveTemplateLink(primaryPlayground.url, getDocumentationLink)
      : undefined;
  // Whether this app exposes the standard OAuth2/OIDC endpoints (see `showOAuthEndpoints` below).
  // These are generic server endpoints, not tied to this application's client credentials.
  // `oauth2Config` isn't always resolved on first paint (e.g. before the parent finishes reading
  // `inboundAuthConfig`), so fall back to the canonical application type, every type except
  // 'custom' (the no-enforced-constraints escape hatch) is OAuth2-based. Only the Client ID row
  // itself needs clientId specifically.
  const isOAuthApplication = Boolean(oauth2Config) || application.type !== 'custom';
  // Whether this application has a real user-facing sign-in page. When the OAuth2 config isn't
  // resolved yet, fall back to the canonical type: 'm2m' and 'custom' apps have no sign-in page,
  // every other type does.
  const isSignInFacing = oauth2Config
    ? hasUserAccess(oauth2Config.grantTypes)
    : application.type !== 'm2m' && application.type !== 'custom';
  const isNativeApplication = application.type === 'mobile';
  // Apps that own a backend/native runtime (native mobile, and fullstack apps like Next.js/Nuxt
  // that run their own server) can implement flows manually against the lower-level flow
  // execution API and manage their own session, unlike a pure browser SPA that only ever does the
  // standard redirect-based OAuth2 dance.
  const hasExtendedFlowEndpoints = application.type === 'mobile' || application.type === 'fullstack';
  // Apps built from the Custom template have no fixed integration shape (its default grant types
  // can make its canonical `type` resolve to anything, e.g. 'm2m'), so key off the template
  // itself and surface every endpoint group (OAuth2/OIDC and App Native flow endpoints) instead of
  // picking just one the way other templates do.
  const isCustomTemplate = normalizeTemplateId(templateId) === 'custom';
  const showOAuthEndpoints = isOAuthApplication && (!hasExtendedFlowEndpoints || isCustomTemplate);
  const showFlowEndpoints = hasExtendedFlowEndpoints || isCustomTemplate;
  const notConfiguredLabel = t('applications:edit.overview.signInPreview.notConfigured', 'Not configured');
  // One row per flow type this application can have, shown in the preview card. Sign-in and
  // sign-out apply to every sign-in-facing app; sign-up and recovery are opt-in per application.
  // Whether an opt-in flow is set up is the application's own setting. Its name is read separately, so a
  // set-up flow whose name has not loaded is still shown as set up.
  const isSignUpSetUp = Boolean(application.isRegistrationFlowEnabled && application.registrationFlowId);
  const isRecoverySetUp = Boolean(application.isRecoveryFlowEnabled && application.recoveryFlowId);
  const setUpLabel = t('applications:edit.overview.signInPreview.configured', 'Configured');
  const flowRows: {label: string; value: string; setUpLabel?: string}[] = [
    {
      label: t('applications:edit.overview.signInPreview.flows.signIn', 'Sign-in'),
      value: authFlowDetails?.name ?? notConfiguredLabel,
    },
    {
      label: t('applications:edit.overview.signInPreview.flows.signUp', 'Sign-up'),
      value: isSignUpSetUp ? (registrationFlowDetails?.name ?? setUpLabel) : notConfiguredLabel,
      setUpLabel: isSignUpSetUp
        ? undefined
        : t('applications:edit.overview.signInPreview.setUpSignUp', 'Set up sign-up'),
    },
    {
      label: t('applications:edit.overview.signInPreview.flows.recovery', 'Password recovery'),
      value: isRecoverySetUp ? (recoveryFlowDetails?.name ?? setUpLabel) : notConfiguredLabel,
      setUpLabel: isRecoverySetUp
        ? undefined
        : t('applications:edit.overview.signInPreview.setUpRecovery', 'Set up recovery'),
    },
    {
      label: t('applications:edit.overview.signInPreview.flows.signOut', 'Sign-out'),
      value: signOutFlowDetails?.name ?? notConfiguredLabel,
    },
  ];

  // The prompt and the endpoints below are for someone to copy into their own application, so they
  // have to name the deployment that answers them rather than the one the console is configured against.
  const runtimeUrl = useRuntimeUrl();
  // Where the application is deployed to more than one environment, its client ID and endpoints differ in
  // each, so the details and endpoints show one chosen environment's: the first in the pipeline until
  // another is chosen. An environment the application is not in has no client ID for it and its endpoints
  // do not accept it, so neither is shown for it: the way to get the application there is, in their place.
  const {
    environments,
    selectedEnvironment,
    shownEnvironment,
    selectEnvironment,
    notDeployedIds,
    isNotDeployed: selectedIsNotDeployed,
    previousEnvironmentName,
    selectedClientId: selectedClientIdValue,
    getValue,
    deployPath,
  } = useResourceEnvironments({
    resourceType: 'application',
    resourceId: application.id,
    hasClientId: Boolean(oauth2Config?.clientId),
  });
  // A single environment's values are shown as one of several is, only with nothing to choose.
  const endpointsBaseUrl = shownEnvironment?.runtimeUrl ?? runtimeUrl;
  // The integration guide and the coding agent prompt are for the environment chosen too. An environment
  // with no client ID for the application leaves the placeholder for the developer to fill in.
  let selectedClientId = oauth2Config?.clientId;
  if (shownEnvironment) {
    selectedClientId = selectedClientIdValue?.status === 'set' ? selectedClientIdValue.value : undefined;
  }
  const placeholderValues = {
    productName,
    baseUrl: endpointsBaseUrl,
    clientId: selectedClientId ?? '<your-client-id>',
    applicationId: application.id,
  };

  const isDirectApiEnabled = config.direct_api?.enabled !== false;
  const flowEndpoints = [
    {
      key: 'flowExecute',
      label: t('applications:edit.overview.endpoints.flowExecute', 'Flow execution endpoint'),
      url: `${endpointsBaseUrl}/flow/execute`,
    },
    {
      key: 'flowMeta',
      label: t('applications:edit.overview.endpoints.flowMeta', 'Flow metadata endpoint'),
      url: `${endpointsBaseUrl}/flow/meta`,
    },
    ...(isDirectApiEnabled
      ? [
          {
            key: 'passkeyRegisterStart',
            label: t('applications:edit.overview.endpoints.passkeyRegisterStart', 'Passkey registration (start)'),
            url: `${endpointsBaseUrl}/register/passkey/start`,
          },
          {
            key: 'passkeyRegisterFinish',
            label: t('applications:edit.overview.endpoints.passkeyRegisterFinish', 'Passkey registration (finish)'),
            url: `${endpointsBaseUrl}/register/passkey/finish`,
          },
        ]
      : []),
  ];
  const oauthEndpoints = [
    {
      key: 'wellknown',
      label: t('applications:edit.overview.endpoints.wellknown', 'OpenID configuration'),
      url: `${endpointsBaseUrl}/.well-known/openid-configuration`,
    },
    {
      key: 'authorization',
      label: t('applications:edit.overview.endpoints.authorization', 'Authorization endpoint'),
      url: `${endpointsBaseUrl}/oauth2/authorize`,
    },
    {
      key: 'token',
      label: t('applications:edit.overview.endpoints.token', 'Token endpoint'),
      url: `${endpointsBaseUrl}/oauth2/token`,
    },
    {
      key: 'userinfo',
      label: t('applications:edit.overview.endpoints.userinfo', 'Userinfo endpoint'),
      url: `${endpointsBaseUrl}/oauth2/userinfo`,
    },
    {
      key: 'jwks',
      label: t('applications:edit.overview.endpoints.jwks', 'JWKS URI'),
      url: `${endpointsBaseUrl}/oauth2/jwks`,
    },
  ];
  // Apps with their own backend/native runtime ("App Native", per the Flow Execution API) drive
  // sign-in/registration step by step against the flow endpoints directly, with a fully custom
  // UI, unlike the browser-redirect OAuth2/OIDC endpoints. Custom apps don't commit to either
  // shape, so they get both groups.
  const endpoints = [...(showOAuthEndpoints ? oauthEndpoints : []), ...(showFlowEndpoints ? flowEndpoints : [])];

  const colorMode: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';
  const themeSwatchColor = themeDetails?.theme.colorSchemes?.[colorMode]?.palette?.primary?.main;
  const hasVisibleQuickstart = quickstartGuides.length > 0;
  const templateDevServerUrl = getTemplateMetadata(templateId)?.devServerUrl;
  // The docs code samples are written against a local instance with placeholder identifiers.
  // Swapping in this application's values leaves the developer nothing to look up.
  const guideCodeValues = {
    '<your-client-id>': selectedClientId,
    '<your-application-id>': application.id,
    // As written where angle brackets would not be valid, such as in an XML property list.
    'your-application-id': application.id,
    'https://localhost:8090': endpointsBaseUrl,
  };
  const hasEndpoints = showOAuthEndpoints || showFlowEndpoints;
  const endpointFields = (
    <Box>
      {endpoints.map((endpoint) => (
        <CopyableField key={endpoint.key} label={endpoint.label} value={endpoint.url} />
      ))}
    </Box>
  );
  const endpointsCard = hasEndpoints && !selectedIsNotDeployed && (
    <OverviewCard
      title={t('applications:edit.overview.endpoints.appNativeTitle', 'Useful endpoints')}
      description={t(
        'applications:edit.overview.endpoints.appNativeDescription',
        'For driving sign-in and registration with a fully custom UI.',
      )}
    >
      {endpointFields}
    </OverviewCard>
  );
  // Sign-in-facing apps already fill the main column with a quickstart and a sizeable preview
  // card, so the endpoints card stays paired with identifiers in the sidebar as before. Apps with
  // no preview (Custom, M2M-only) would otherwise leave the main column nearly empty and the
  // endpoints card cramped into, or lopsided against, the narrow sidebar, so for those the
  // endpoints card moves into the main column instead.
  const showEndpointsInSidebar = isSignInFacing;
  const hasGuide = hasVisibleQuickstart || Boolean(promptUrl);
  const hasMainContent = hasGuide || (!showEndpointsInSidebar && Boolean(endpointsCard));

  const detailsFields = (
    <Box>
      <CopyableField label={t('applications:edit.general.labels.applicationId')} value={application.id} />
      {oauth2Config?.clientId &&
        !selectedIsNotDeployed &&
        (shownEnvironment && getValue ? (
          <EnvironmentClientIdField environment={shownEnvironment} resourceId={application.id} getValue={getValue} />
        ) : (
          <CopyableField
            label={t('applications:edit.general.labels.clientId', 'Client ID')}
            value={oauth2Config.clientId}
          />
        ))}
      {application.ouId && (
        <CopyableField
          label={t('applications:edit.overview.appDetails.organizationUnitId', 'Organization unit ID')}
          value={application.ouId}
        />
      )}
      {organizationUnit?.handle && (
        <CopyableField
          label={t('applications:edit.overview.appDetails.organizationUnitHandle', 'Organization unit handle')}
          value={organizationUnit.handle}
        />
      )}
    </Box>
  );
  // The identifiers and endpoints sit in one card. With several environments the choice of one leads it,
  // so the card reads as one answer: this environment's identifiers and endpoints.
  const sectionTitle = (title: string): JSX.Element => (
    <Typography variant="subtitle1" component="h3" sx={{fontWeight: 600, mt: 2.5, mb: 1.25, '&:first-child': {mt: 0}}}>
      {title}
    </Typography>
  );
  const panelWidth = 520;
  const detailsPanel = (
    <Paper
      variant="outlined"
      sx={{
        borderRadius: '10px',
        p: 2.25,
      }}
    >
      {shownEnvironment && selectedIsNotDeployed && (
        <Box sx={{mb: 2.5}}>
          <EnvironmentNotDeployedNotice
            resourceName={application.name}
            environmentName={shownEnvironment.name}
            previousEnvironmentName={previousEnvironmentName}
            deployPath={deployPath}
          />
        </Box>
      )}
      {sectionTitle(t('applications:edit.overview.appDetails.title', 'Application details'))}
      {detailsFields}
      {!selectedIsNotDeployed && hasEndpoints && showEndpointsInSidebar && (
        <>
          <Divider sx={{mt: 2.5}} />
          {/* Folded by default, so the sign-in experience below stays in reach on a sign-in-facing app. */}
          <Button
            fullWidth
            color="inherit"
            aria-expanded={isEndpointsOpen}
            endIcon={isEndpointsOpen ? <ChevronUp size={16} aria-hidden /> : <ChevronDown size={16} aria-hidden />}
            onClick={() => setIsEndpointsOpen((open) => !open)}
            disableRipple
            // A section header rather than a button: no fill on hover, which the theme draws as a pill
            // hugging the text. The chevron and the focus ring still mark it as one.
            sx={{
              mt: 1.5,
              px: 0,
              justifyContent: 'space-between',
              textAlign: 'start',
              textTransform: 'none',
              '&:hover': {bgcolor: 'transparent'},
            }}
          >
            <Box component="span" sx={{display: 'flex', flexDirection: 'column'}}>
              <Typography variant="subtitle1" component="span">
                {t('applications:edit.overview.endpoints.appNativeTitle', 'Useful endpoints')}
              </Typography>
              {endpoints.length > 0 && (
                <Typography variant="caption" color="text.secondary" component="span">
                  {t('applications:edit.overview.endpoints.summary', '{{first}} and {{count}} more', {
                    first: endpoints[0].label,
                    count: endpoints.length - 1,
                  })}
                </Typography>
              )}
            </Box>
          </Button>
          <Collapse in={isEndpointsOpen}>{endpointFields}</Collapse>
        </>
      )}
    </Paper>
  );

  const sectionLabel = (label: string, action: ReactNode): JSX.Element => (
    <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{mb: 1}}>
      <Typography variant="overline" color="text.secondary">
        {label}
      </Typography>
      {action}
    </Stack>
  );
  const signInExperienceCard = isSignInFacing && (
    <Paper variant="outlined" component="section" aria-labelledby="sign-in-experience-title" sx={{p: 2.25}}>
      <Typography id="sign-in-experience-title" variant="subtitle1" component="h3" sx={{mb: 0.25}}>
        {t('applications:edit.overview.signInPreview.title', 'Sign-in experience')}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
        {t('applications:edit.overview.signInPreview.description', 'What users see when they sign in to {{name}}.', {
          name: application.name,
        })}
      </Typography>
      <Box
        sx={{
          // Fixed so the frame reads as a peek of the live sign-in page rather than a full
          // screenshot, whatever the flows configured.
          height: 320,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          pointerEvents: 'none',
          border: 1,
          borderColor: 'divider',
          mb: 2.5,
          ...(isNativeApplication
            ? {width: 240, mx: 'auto', bgcolor: 'common.black', borderRadius: 2.5, p: 1.25}
            : {borderRadius: 1.5}),
        }}
      >
        <Box sx={{flex: 1, minHeight: 0, borderRadius: isNativeApplication ? 1.75 : 0, overflow: 'hidden'}}>
          <GatePreview
            frameless
            theme={themeDetails?.theme}
            baseTheme={DefaultTheme as Theme}
            showToolbar={false}
            colorScheme={colorMode}
          />
        </Box>
      </Box>

      {sectionLabel(
        t('applications:edit.overview.signInPreview.themeLabel', 'Theme used'),
        onGoToCustomization && (
          <Button size="small" endIcon={<ArrowRight size={14} aria-hidden />} onClick={onGoToCustomization}>
            {t('applications:edit.overview.signInPreview.editCustomization', 'Edit in customization')}
          </Button>
        ),
      )}
      {/* A theme that could not be read says so, rather than passing for the default theme. */}
      {application.themeId && themeError ? (
        <Box sx={{mb: 2.5}}>
          <QueryErrorNotice
            variant="inline"
            error={themeError}
            t={t}
            fallbackKey="applications:edit.overview.signInPreview.themeError"
            fallbackDefaultValue="The theme could not be loaded."
            onRetry={() => {
              void refetchTheme();
            }}
          />
        </Box>
      ) : (
        <Stack
          direction="row"
          alignItems="center"
          spacing={1.25}
          sx={{border: 1, borderColor: 'divider', borderRadius: 1, px: 1.75, py: 1.25, mb: 2.5}}
        >
          <Box
            sx={{width: 18, height: 18, borderRadius: 0.75, flexShrink: 0, bgcolor: themeSwatchColor ?? 'primary.main'}}
          />
          <Typography variant="body2">
            {themeDetails?.displayName ??
              (application.themeId
                ? t('applications:edit.overview.signInPreview.themeLoading', 'Loading the theme')
                : t('applications:edit.overview.signInPreview.defaultTheme', 'Default'))}
          </Typography>
        </Stack>
      )}

      {sectionLabel(
        t('applications:edit.overview.signInPreview.flowsLabel', 'Flows'),
        onGoToFlows && (
          <Button size="small" endIcon={<ArrowRight size={14} aria-hidden />} onClick={onGoToFlows}>
            {t('applications:edit.overview.signInPreview.editFlows', 'Edit in flows')}
          </Button>
        ),
      )}
      <Stack sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden'}}>
        {flowRows.map((flow, index) => (
          <Stack
            key={flow.label}
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            spacing={1.25}
            sx={{px: 1.75, py: 1, borderTop: index === 0 ? 0 : 1, borderColor: 'divider', minHeight: 44}}
          >
            <Typography variant="body2" color="text.secondary">
              {flow.label}
            </Typography>
            {/* A flow that is not set up sends the developer to where it is, rather than only saying so. */}
            {flow.setUpLabel && onGoToFlows ? (
              <Button size="small" endIcon={<ArrowRight size={14} aria-hidden />} onClick={onGoToFlows}>
                {flow.setUpLabel}
              </Button>
            ) : (
              <Typography variant="body2" color={flow.value === notConfiguredLabel ? 'text.secondary' : undefined}>
                {flow.value}
              </Typography>
            )}
          </Stack>
        ))}
      </Stack>
    </Paper>
  );

  return (
    <Stack spacing={3}>
      {/* The environment leads the tab: the guide, the client ID, and the endpoints all follow it. */}
      {selectedEnvironment && (
        <EnvironmentBar
          environments={environments}
          selectedEnvironment={selectedEnvironment}
          onSelect={selectEnvironment}
          notDeployedIds={notDeployedIds}
          deployPath={deployPath}
        />
      )}
      <Stack
        direction="row"
        flexWrap="wrap"
        gap={3.5}
        alignItems="flex-start"
        justifyContent={hasMainContent ? undefined : 'center'}
      >
        {/* Left: main content */}
        {hasMainContent && (
          <Stack sx={{flex: '2 1 480px', minWidth: 0}} spacing={2.25}>
            {primaryPlaygroundUrl && (
              <StackblitzQuickstartCard
                url={primaryPlaygroundUrl}
                heading={t('applications:edit.overview.stackblitz.heading', 'Try the live quickstart')}
                subheading={
                  <Trans
                    i18nKey="applications:edit.overview.stackblitz.subheading"
                    defaults="Run <code>{{name}}</code> in StackBlitz"
                    values={{name: application.name}}
                    components={{
                      code: (
                        <Box component="code" sx={{fontFamily: 'monospace', fontSize: '0.85em', color: 'inherit'}} />
                      ),
                    }}
                  />
                }
                ctaLabel={t('applications:edit.overview.stackblitz.cta', 'Open on StackBlitz')}
              />
            )}

            {hasGuide && (
              <ConnectAppGuide
                guides={quickstartGuides}
                promptUrl={promptUrl}
                promptValues={placeholderValues}
                codeValues={guideCodeValues}
                devServerUrl={templateDevServerUrl}
                redirectUris={oauth2Config?.redirectUris}
                isSelfSignUpEnabled={Boolean(application.isRegistrationFlowEnabled)}
                isSignInFacing={isSignInFacing}
                onGoToFlows={onGoToFlows}
                onOpenLink={externalLinkConfirmation.requestNavigation}
              />
            )}

            {!showEndpointsInSidebar && endpointsCard}
          </Stack>
        )}

        {/* Right: for sign-in-facing apps the sign-in experience, then identifiers and endpoints. */}
        <Stack
          spacing={2.25}
          // Beside a guide, the column shares the row. On its own, it is centered at a readable width.
          sx={
            hasMainContent
              ? {flex: '1 1 320px', minWidth: 0}
              : {flex: `0 1 ${panelWidth}px`, minWidth: 0, maxWidth: '100%'}
          }
        >
          {signInExperienceCard}
          {detailsPanel}
        </Stack>

        <ExternalLinkConfirmDialog
          isOpen={externalLinkConfirmation.isOpen}
          pendingUrl={externalLinkConfirmation.pendingUrl}
          onCancel={externalLinkConfirmation.cancel}
          onConfirm={externalLinkConfirmation.confirm}
        />
      </Stack>
    </Stack>
  );
}
