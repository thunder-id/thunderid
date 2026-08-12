// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AndroidLogo,
  AppleIcon,
  CopyableField,
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
import type {Application, OAuth2Config} from '@thunderid/configure-applications';
import {GatePreview} from '@thunderid/configure-design';
import {useGetOrganizationUnit} from '@thunderid/configure-organization-units';
import {useConfig} from '@thunderid/contexts';
import {DefaultTheme, type Theme, useGetTheme} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {Box, Button, Chip, Link, Paper, Stack, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {ArrowRight, ArrowUpRight, Check, Copy, Sparkles} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import useGetFlowById from '../../../../flows/api/useGetFlowById';
import {getIntegrationGuideForTemplate} from '../../../utils/getIntegrationGuidesForTemplate';
import getPlaygroundsForTemplate from '../../../utils/getPlaygroundsForTemplate';
import getQuickstartsForTemplate from '../../../utils/getQuickstartsForTemplate';
import normalizeTemplateId from '../../../utils/normalizeTemplateId';
import {hasUserAccess} from '../../../utils/oauth2Rules';
import resolveTemplateLink from '../../../utils/resolveTemplateLink';

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

function replacePlaceholders(text: string, values: Record<string, string | undefined>): string {
  return Object.entries(values).reduce((result, [key, value]) => {
    if (!value || value.trim() === '') {
      return result;
    }
    return result.replace(new RegExp(`\\{\\{${key}\\}\\}`, 'g'), value);
  }, text);
}

/** A small rounded, tinted square behind a card's leading icon. */
function IconBadge({children, gradient = false}: {children: ReactNode; gradient?: boolean}): JSX.Element {
  return (
    <Box
      sx={{
        width: 32,
        height: 32,
        borderRadius: '8px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
        ...(gradient
          ? {
              background: 'linear-gradient(135deg, #3688FF, #7fb4ff)',
              color: '#ffffff',
              boxShadow: '0 4px 14px rgba(54,136,255,0.35)',
            }
          : {bgcolor: 'rgba(54,136,255,0.14)', color: 'primary.main'}),
      }}
    >
      {children}
    </Box>
  );
}

/** Vendor logo shown in a quickstart card for a given platform's {@link QuickstartLink.label}. */
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

/** A bordered card with an icon, a title/description, and arbitrary content below. */
function IconCard({
  icon,
  title,
  description,
  children,
  sx = undefined,
}: {
  icon: ReactNode;
  title: ReactNode;
  description: ReactNode;
  children: ReactNode;
  sx?: object;
}): JSX.Element {
  return (
    <Paper variant="outlined" sx={{borderRadius: '10px', p: 2.25, display: 'flex', gap: 1.75, ...sx}}>
      {icon}
      <Box sx={{minWidth: 0, flex: 1, display: 'flex', flexDirection: 'column'}}>
        <Typography variant="subtitle2" sx={{fontWeight: 600, mb: 0.5}}>
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{mb: 1.25}}>
          {description}
        </Typography>
        <Box sx={{mt: 'auto', display: 'flex', justifyContent: 'flex-start'}}>{children}</Box>
      </Box>
    </Paper>
  );
}

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
 * endpoints. When the application's template has a runnable quickstart, it additionally shows a
 * StackBlitz quickstart, a link to the hosted integration guide, a ready-made coding-agent
 * prompt, and a preview of the sign-in experience.
 */
export default function IntegrationGuides({
  application,
  oauth2Config = undefined,
  onGoToFlows = undefined,
  onGoToCustomization = undefined,
}: IntegrationGuidesProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('IntegrationGuides');
  const {config, getServerUrl, getDocumentationLink} = useConfig();
  const {data: themeDetails} = useGetTheme(application.themeId ?? '');
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
  const [promptCopied, setPromptCopied] = useState(false);
  const [isFetchingPrompt, setIsFetchingPrompt] = useState(false);
  const externalLinkConfirmation = useExternalLinkConfirmation();

  const templateId = application.template ?? undefined;
  const guide = getIntegrationGuideForTemplate(templateId);
  const promptUrl = resolveTemplateLink(guide?.llm_prompt.docsUrl, getDocumentationLink);
  const quickstarts = getQuickstartsForTemplate(templateId) ?? [];
  const playgrounds = getPlaygroundsForTemplate(templateId) ?? [];
  // A template with exactly one quickstart guide gets a single generic guide card. Templates
  // covering multiple platform SDKs (e.g. Mobile, which spans iOS/Android/Flutter) get one guide
  // card per platform instead. Entries whose docs link isn't configured (resolves to undefined)
  // are dropped rather than rendered with a dead link.
  const primaryQuickstart = quickstarts.length === 1 ? quickstarts[0] : undefined;
  const primaryQuickstartDocsUrl = resolveTemplateLink(primaryQuickstart?.docsUrl, getDocumentationLink);
  const visibleQuickstarts = primaryQuickstart
    ? []
    : quickstarts
        .map((entry) => ({...entry, docsUrl: resolveTemplateLink(entry.docsUrl, getDocumentationLink)}))
        .filter((entry): entry is {label: string; docsUrl: string} => Boolean(entry.docsUrl));
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
  const flowRows: {label: string; value: string}[] = [
    {
      label: t('applications:edit.overview.signInPreview.flows.signIn', 'Sign-in'),
      value: authFlowDetails?.name ?? notConfiguredLabel,
    },
    {
      label: t('applications:edit.overview.signInPreview.flows.signUp', 'Sign-up'),
      value:
        application.isRegistrationFlowEnabled && application.registrationFlowId
          ? (registrationFlowDetails?.name ?? notConfiguredLabel)
          : notConfiguredLabel,
    },
    {
      label: t('applications:edit.overview.signInPreview.flows.recovery', 'Password recovery'),
      value:
        application.isRecoveryFlowEnabled && application.recoveryFlowId
          ? (recoveryFlowDetails?.name ?? notConfiguredLabel)
          : notConfiguredLabel,
    },
    {
      label: t('applications:edit.overview.signInPreview.flows.signOut', 'Sign-out'),
      value: signOutFlowDetails?.name ?? notConfiguredLabel,
    },
  ];

  const placeholderValues = {
    productName,
    clientId: oauth2Config?.clientId,
    applicationId: application.id,
  };

  const handleCopyPrompt = async () => {
    if (!promptUrl) {
      return;
    }

    setIsFetchingPrompt(true);

    try {
      const response = await fetch(promptUrl);

      if (!response.ok) {
        throw new Error(`Failed to fetch prompt: ${response.status}`);
      }

      const promptText = await response.text();

      await navigator.clipboard.writeText(replacePlaceholders(promptText, placeholderValues));
      setPromptCopied(true);
      setTimeout(() => setPromptCopied(false), 1500);
    } catch {
      logger.error('Failed to copy the coding agent prompt to clipboard');
    } finally {
      setIsFetchingPrompt(false);
    }
  };

  const serverUrl = getServerUrl();
  const flowEndpoints = [
    {
      key: 'flowExecute',
      label: t('applications:edit.overview.endpoints.flowExecute', 'Flow execution endpoint'),
      url: `${serverUrl}/flow/execute`,
    },
    {
      key: 'flowMeta',
      label: t('applications:edit.overview.endpoints.flowMeta', 'Flow metadata endpoint'),
      url: `${serverUrl}/flow/meta`,
    },
    {
      key: 'passkeyRegisterStart',
      label: t('applications:edit.overview.endpoints.passkeyRegisterStart', 'Passkey registration (start)'),
      url: `${serverUrl}/register/passkey/start`,
    },
    {
      key: 'passkeyRegisterFinish',
      label: t('applications:edit.overview.endpoints.passkeyRegisterFinish', 'Passkey registration (finish)'),
      url: `${serverUrl}/register/passkey/finish`,
    },
  ];
  const oauthEndpoints = [
    {
      key: 'wellknown',
      label: t('applications:edit.overview.endpoints.wellknown', 'OpenID configuration'),
      url: `${serverUrl}/.well-known/openid-configuration`,
    },
    {
      key: 'authorization',
      label: t('applications:edit.overview.endpoints.authorization', 'Authorization endpoint'),
      url: `${serverUrl}/oauth2/authorize`,
    },
    {
      key: 'token',
      label: t('applications:edit.overview.endpoints.token', 'Token endpoint'),
      url: `${serverUrl}/oauth2/token`,
    },
    {
      key: 'userinfo',
      label: t('applications:edit.overview.endpoints.userinfo', 'Userinfo endpoint'),
      url: `${serverUrl}/oauth2/userinfo`,
    },
    {
      key: 'jwks',
      label: t('applications:edit.overview.endpoints.jwks', 'JWKS URI'),
      url: `${serverUrl}/oauth2/jwks`,
    },
  ];
  // Apps with their own backend/native runtime ("App Native", per the Flow Execution API) drive
  // sign-in/registration step by step against the flow endpoints directly, with a fully custom
  // UI, unlike the browser-redirect OAuth2/OIDC endpoints. Custom apps don't commit to either
  // shape, so they get both groups.
  const endpoints = [...(showOAuthEndpoints ? oauthEndpoints : []), ...(showFlowEndpoints ? flowEndpoints : [])];

  const colorMode: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';
  const themeSwatchColor = themeDetails?.theme.vars?.palette.primary.main;
  const hasVisibleQuickstart = Boolean(primaryQuickstartDocsUrl) || visibleQuickstarts.length > 0;
  const endpointsCard = (showOAuthEndpoints || showFlowEndpoints) && (
    <OverviewCard
      title={t('applications:edit.overview.endpoints.appNativeTitle', 'Useful Endpoints')}
      description={t(
        'applications:edit.overview.endpoints.appNativeDescription',
        'For driving sign-in and registration with a fully custom UI.',
      )}
    >
      <Box>
        {endpoints.map((endpoint) => (
          <CopyableField key={endpoint.key} label={endpoint.label} value={endpoint.url} />
        ))}
      </Box>
    </OverviewCard>
  );
  // Sign-in-facing apps already fill the main column with a quickstart and a sizeable preview
  // card, so the endpoints card stays paired with identifiers in the sidebar as before. Apps with
  // no preview (Custom, M2M-only) would otherwise leave the main column nearly empty and the
  // endpoints card cramped into, or lopsided against, the narrow sidebar, so for those the
  // endpoints card moves into the main column instead.
  const showEndpointsInSidebar = isSignInFacing;
  const hasMainContent = hasVisibleQuickstart || Boolean(promptUrl) || isSignInFacing || Boolean(endpointsCard);

  const detailsCard = (
    <OverviewCard
      title={t('applications:edit.overview.appDetails.title', 'Application details')}
      description={t('applications:edit.overview.appDetails.description', 'Identifiers used in your integration code.')}
      sx={hasMainContent ? undefined : {flex: '1 1 320px', maxWidth: {xs: '100%', md: 320}}}
    >
      <Box>
        <CopyableField label={t('applications:edit.general.labels.applicationId')} value={application.id} />
        {oauth2Config?.clientId && (
          <CopyableField label={t('applications:edit.general.labels.clientId')} value={oauth2Config.clientId} />
        )}
        {application.ouId && (
          <CopyableField
            label={t('applications:edit.overview.appDetails.organizationUnitId', 'Organization Unit ID')}
            value={application.ouId}
          />
        )}
        {organizationUnit?.handle && (
          <CopyableField
            label={t('applications:edit.overview.appDetails.organizationUnitHandle', 'Organization Unit Handle')}
            value={organizationUnit.handle}
          />
        )}
      </Box>
    </OverviewCard>
  );

  return (
    <Stack
      direction="row"
      flexWrap="wrap"
      gap={3.5}
      alignItems="flex-start"
      justifyContent={hasMainContent ? undefined : 'center'}
    >
      {/* Left: main content */}
      {hasMainContent && (
        <Stack sx={{flex: '2 0 480px', minWidth: 0}} spacing={2.25}>
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
                    code: <Box component="code" sx={{fontFamily: 'monospace', fontSize: '0.85em', color: 'inherit'}} />,
                  }}
                />
              }
              ctaLabel={t('applications:edit.overview.stackblitz.cta', 'Open on StackBlitz')}
            />
          )}

          {(hasVisibleQuickstart || promptUrl) && (
            <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1.75}}>
              {primaryQuickstartDocsUrl && (
                <IconCard
                  icon={
                    <IconBadge>{QUICKSTART_ICONS[primaryQuickstart!.label] ?? <ArrowUpRight size={16} />}</IconBadge>
                  }
                  title={t('applications:edit.overview.readGuide.title', 'Read the quickstart guide')}
                  description={t(
                    'applications:edit.overview.readGuide.description',
                    'Step-by-step integration guide on the docs site.',
                  )}
                  sx={{flex: '1 1 240px'}}
                >
                  <Link
                    component="button"
                    type="button"
                    variant="body2"
                    underline="hover"
                    sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5, fontWeight: 600}}
                    onClick={() => externalLinkConfirmation.requestNavigation(primaryQuickstartDocsUrl)}
                  >
                    {t('applications:edit.overview.readGuide.action', 'Open quickstart')}
                    <ArrowUpRight size={13} />
                  </Link>
                </IconCard>
              )}

              {visibleQuickstarts.map((entry) => (
                <IconCard
                  key={entry.label}
                  icon={<IconBadge>{QUICKSTART_ICONS[entry.label] ?? <ArrowUpRight size={16} />}</IconBadge>}
                  title={t('applications:edit.overview.readGuide.titleFor', '{{label}} quickstart guide', {
                    label: entry.label,
                  })}
                  description={t(
                    'applications:edit.overview.readGuide.descriptionFor',
                    'Step-by-step {{label}} integration guide on the docs site.',
                    {label: entry.label},
                  )}
                  sx={{flex: '1 1 240px'}}
                >
                  <Link
                    component="button"
                    type="button"
                    variant="body2"
                    underline="hover"
                    sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5, fontWeight: 600}}
                    onClick={() => externalLinkConfirmation.requestNavigation(entry.docsUrl)}
                  >
                    {t('applications:edit.overview.readGuide.action', 'Open quickstart')}
                    <ArrowUpRight size={13} />
                  </Link>
                </IconCard>
              ))}

              {promptUrl && (
                <IconCard
                  icon={
                    <IconBadge>
                      <Sparkles size={16} />
                    </IconBadge>
                  }
                  title={
                    <Stack direction="row" alignItems="center" spacing={1}>
                      <span>{t('applications:edit.overview.agentPrompt.title', 'Integrate with a coding agent')}</span>
                      <Chip
                        label={t('applications:edit.overview.agentPrompt.badge', 'AI')}
                        size="small"
                        color="primary"
                        sx={{height: 18, fontSize: '0.65rem', fontWeight: 600}}
                      />
                    </Stack>
                  }
                  description={t(
                    'applications:edit.overview.agentPrompt.description',
                    'Copy a ready-made prompt for Claude, Cursor, or any agent.',
                  )}
                  sx={{flex: '1 1 240px'}}
                >
                  <Button
                    variant="outlined"
                    size="small"
                    disabled={isFetchingPrompt}
                    startIcon={promptCopied ? <Check size={13} /> : <Copy size={13} />}
                    onClick={() => {
                      handleCopyPrompt().catch(() => {
                        /* Error already handled */
                      });
                    }}
                  >
                    {promptCopied
                      ? t('common:actions.copied')
                      : t('applications:edit.overview.agentPrompt.action', 'Copy prompt')}
                  </Button>
                </IconCard>
              )}
            </Box>
          )}

          {isSignInFacing && (
            <OverviewCard
              title={t('applications:edit.overview.signInPreview.title', 'Preview')}
              description={t(
                'applications:edit.overview.signInPreview.description',
                'What users see when they sign in to {{name}}.',
                {name: application.name},
              )}
            >
              <Stack direction="row" flexWrap="wrap" gap={3} alignItems="flex-start">
                <Box
                  sx={{
                    width: 260,
                    // Fixed so the frame reads as a peek of the live sign-in page rather than a
                    // full screenshot, and its height no longer depends on how many flows are
                    // configured. The inner card below stretches to fill this height so the
                    // frame's own background never shows through underneath it.
                    height: 340,
                    display: 'flex',
                    flexDirection: 'column',
                    flexShrink: 0,
                    overflow: 'hidden',
                    pointerEvents: 'none',
                    ...(isNativeApplication
                      ? {
                          bgcolor: '#0a0a0a',
                          borderRadius: '32px',
                          p: '10px',
                          border: '1px solid',
                          borderColor: 'divider',
                        }
                      : {borderRadius: '10px', border: '1px solid', borderColor: 'divider'}),
                  }}
                >
                  {isNativeApplication && (
                    <Box sx={{display: 'flex', justifyContent: 'center', pb: 1}}>
                      <Box sx={{width: 56, height: 5, borderRadius: 3, bgcolor: 'rgba(255,255,255,0.25)'}} />
                    </Box>
                  )}
                  <Box sx={{flex: 1, minHeight: 0, borderRadius: isNativeApplication ? '22px' : 0, overflow: 'hidden'}}>
                    <GatePreview
                      frameless
                      theme={themeDetails?.theme}
                      baseTheme={DefaultTheme as Theme}
                      showToolbar={false}
                      colorScheme={colorMode}
                    />
                  </Box>
                </Box>

                <Stack sx={{flex: 1, minWidth: 220}} spacing={2} justifyContent="center">
                  <Box>
                    <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{mb: 1}}>
                      <Typography
                        variant="caption"
                        sx={{
                          fontWeight: 600,
                          letterSpacing: '0.03em',
                          color: 'text.secondary',
                          textTransform: 'uppercase',
                        }}
                      >
                        {t('applications:edit.overview.signInPreview.themeLabel', 'Theme used')}
                      </Typography>
                      {onGoToCustomization && (
                        <Button size="small" endIcon={<ArrowRight size={11} />} onClick={onGoToCustomization}>
                          {t('applications:edit.overview.signInPreview.editCustomization', 'Edit in Customization')}
                        </Button>
                      )}
                    </Stack>
                    <Stack
                      direction="row"
                      alignItems="center"
                      spacing={1.25}
                      sx={{border: '1px solid', borderColor: 'divider', borderRadius: '8px', px: 1.75, py: 1.25}}
                    >
                      <Box
                        sx={{
                          width: 20,
                          height: 20,
                          borderRadius: '6px',
                          flexShrink: 0,
                          bgcolor: themeSwatchColor ?? 'primary.main',
                        }}
                      />
                      <Typography variant="body2">
                        {themeDetails?.displayName ??
                          t('applications:edit.overview.signInPreview.defaultTheme', 'Default')}
                      </Typography>
                    </Stack>
                  </Box>

                  <Box>
                    <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{mb: 1}}>
                      <Typography
                        variant="caption"
                        sx={{
                          fontWeight: 600,
                          letterSpacing: '0.03em',
                          color: 'text.secondary',
                          textTransform: 'uppercase',
                        }}
                      >
                        {t('applications:edit.overview.signInPreview.flowsLabel', 'Flows')}
                      </Typography>
                      {onGoToFlows && (
                        <Button size="small" endIcon={<ArrowRight size={11} />} onClick={onGoToFlows}>
                          {t('applications:edit.overview.signInPreview.editFlows', 'Edit in Flows')}
                        </Button>
                      )}
                    </Stack>
                    <Stack sx={{border: '1px solid', borderColor: 'divider', borderRadius: '8px', overflow: 'hidden'}}>
                      {flowRows.map((flow, index) => (
                        <Stack
                          key={flow.label}
                          direction="row"
                          alignItems="center"
                          justifyContent="space-between"
                          spacing={1.25}
                          sx={{
                            px: 1.75,
                            py: 1.25,
                            borderTop: index === 0 ? 'none' : '1px solid',
                            borderColor: 'divider',
                          }}
                        >
                          <Typography variant="body2" color="text.secondary">
                            {flow.label}
                          </Typography>
                          <Typography
                            variant="body2"
                            color={flow.value === notConfiguredLabel ? 'text.secondary' : undefined}
                          >
                            {flow.value}
                          </Typography>
                        </Stack>
                      ))}
                    </Stack>
                  </Box>
                </Stack>
              </Stack>
            </OverviewCard>
          )}

          {!showEndpointsInSidebar && endpointsCard}
        </Stack>
      )}

      {/* Right: identifiers sidebar. Also holds the endpoints card for sign-in-facing apps, whose
          main column is already full without it. */}
      <Stack spacing={1.75} sx={hasMainContent ? {flex: '1 0 280px', maxWidth: {xs: '100%', md: 320}} : undefined}>
        {detailsCard}
        {showEndpointsInSidebar && endpointsCard}
      </Stack>

      <ExternalLinkConfirmDialog
        isOpen={externalLinkConfirmation.isOpen}
        pendingUrl={externalLinkConfirmation.pendingUrl}
        onCancel={externalLinkConfirmation.cancel}
        onConfirm={externalLinkConfirmation.confirm}
      />
    </Stack>
  );
}
