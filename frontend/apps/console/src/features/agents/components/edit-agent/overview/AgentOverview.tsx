// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  CopyableField,
  ExternalLinkConfirmDialog,
  LangChainLogo,
  useExternalLinkConfirmation,
} from '@thunderid/components';
import {OAuth2GrantTypes} from '@thunderid/configure-applications';
import {useGetUsers} from '@thunderid/configure-users';
import {useConfig} from '@thunderid/contexts';
import {Box, Button, Chip, Link, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, ArrowUpRight} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';
import AttributesSummarySection from '../attributes/AttributesSummarySection';

const formatUserLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attrs = user.attributes ?? {};
  const username = typeof attrs.username === 'string' ? attrs.username : undefined;
  const email = typeof attrs.email === 'string' ? attrs.email : undefined;
  return username ?? email ?? user.id;
};

/**
 * Props for the {@link AgentOverview} component.
 */
export interface AgentOverviewProps {
  /**
   * The agent to show the overview for.
   */
  agent: Agent;
  /**
   * OAuth2 configuration containing client credentials (optional).
   */
  oauth2Config?: OAuthAgentConfig;
  /**
   * Navigates to the agent's Advanced tab, where Delegated mode and allowed user types live.
   */
  onGoToAdvanced?: () => void;
}

/** A small rounded, tinted square behind a card's leading icon. */
function IconBadge({children}: {children: ReactNode}): JSX.Element {
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
        bgcolor: 'rgba(54,136,255,0.14)',
        color: 'primary.main',
      }}
    >
      {children}
    </Box>
  );
}

/** A bordered card with an icon, a title/description, and arbitrary content below. */
function IconCard({
  icon,
  title,
  description,
  children,
}: {
  icon: ReactNode;
  title: ReactNode;
  description: ReactNode;
  children: ReactNode;
}): JSX.Element {
  return (
    <Paper variant="outlined" sx={{borderRadius: '10px', p: 2.25, display: 'flex', gap: 1.75}}>
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
}: {
  title: ReactNode;
  description: ReactNode;
  headerAction?: ReactNode;
  children: ReactNode;
}): JSX.Element {
  return (
    <Paper variant="outlined" sx={{borderRadius: '10px', p: 2.25}}>
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
 * Overview tab for an agent's edit page.
 *
 * Always shows the agent's identifiers, its OAuth2/OIDC endpoints, and a summary of the mode it
 * authenticates in (its own identity, and, when Delegated mode is on, on behalf of a user). Also
 * links out to the hosted quickstart for connecting an AI agent framework (e.g. LangChain).
 */
export default function AgentOverview({
  agent,
  oauth2Config = undefined,
  onGoToAdvanced = undefined,
}: AgentOverviewProps): JSX.Element {
  const {t} = useTranslation();
  const {getServerUrl, getDocumentationLink} = useConfig();
  const externalLinkConfirmation = useExternalLinkConfirmation();
  const {data: usersData} = useGetUsers({limit: 100, offset: 0});

  const serverUrl = getServerUrl();
  const quickstartDocsUrl = getDocumentationLink('agents.quickstarts.langchain.docs');

  const ownerLabel = useMemo(() => {
    if (!agent.owner) return undefined;
    const match = (usersData?.users ?? []).find((user) => user.id === agent.owner);
    return match ? formatUserLabel(match) : agent.owner;
  }, [usersData, agent.owner]);

  const isDelegated = oauth2Config?.grantTypes?.includes(OAuth2GrantTypes.AUTHORIZATION_CODE) ?? false;
  const allowedUserTypes = agent.allowedUserTypes ?? [];

  const endpoints = [
    {
      key: 'wellknown',
      label: t('agents:edit.overview.endpoints.wellknown', 'OpenID configuration'),
      url: `${serverUrl}/.well-known/openid-configuration`,
    },
    {
      key: 'authorization',
      label: t('agents:edit.overview.endpoints.authorization', 'Authorization endpoint'),
      url: `${serverUrl}/oauth2/authorize`,
    },
    {
      key: 'token',
      label: t('agents:edit.overview.endpoints.token', 'Token endpoint'),
      url: `${serverUrl}/oauth2/token`,
    },
    {
      key: 'userinfo',
      label: t('agents:edit.overview.endpoints.userinfo', 'Userinfo endpoint'),
      url: `${serverUrl}/oauth2/userinfo`,
    },
    {
      key: 'jwks',
      label: t('agents:edit.overview.endpoints.jwks', 'JWKS URI'),
      url: `${serverUrl}/oauth2/jwks`,
    },
  ];

  const detailsCard = (
    <OverviewCard
      title={t('agents:edit.overview.agentDetails.title', 'Agent details')}
      description={t('agents:edit.overview.agentDetails.description', 'Identifiers used in your integration code.')}
    >
      <Box>
        <CopyableField label={t('agents:edit.general.labels.agentId', 'Agent ID')} value={agent.id} />
        {agent.clientId && (
          <CopyableField label={t('agents:edit.credentials.clientId.title', 'Client ID')} value={agent.clientId} />
        )}
        {ownerLabel && <CopyableField label={t('agents:edit.general.labels.ownerId', 'Owner ID')} value={ownerLabel} />}
        <CopyableField
          label={t('agents:edit.overview.agentDetails.organizationUnitId', 'Organization Unit ID')}
          value={agent.ouId}
        />
        {agent.ouHandle && (
          <CopyableField
            label={t('agents:edit.overview.agentDetails.organizationUnitHandle', 'Organization Unit Handle')}
            value={agent.ouHandle}
          />
        )}
      </Box>
    </OverviewCard>
  );

  const endpointsCard = (
    <OverviewCard
      title={t('agents:edit.overview.endpoints.title', 'Useful Endpoints')}
      description={t(
        'agents:edit.overview.endpoints.description',
        'For authenticating this agent, on its own or on behalf of a user.',
      )}
    >
      <Box>
        {endpoints.map((endpoint) => (
          <CopyableField key={endpoint.key} label={endpoint.label} value={endpoint.url} />
        ))}
      </Box>
    </OverviewCard>
  );

  const attributesCard = (
    <OverviewCard
      title={t('agents:edit.overview.attributes.title', 'Attributes')}
      description={t(
        'agents:edit.overview.attributes.description',
        "A preview of this agent's attribute values. Manage them from the Attributes tab.",
      )}
    >
      <AttributesSummarySection agent={agent} variant="bare" />
    </OverviewCard>
  );

  const accessModeCard = (
    <OverviewCard
      title={t('agents:edit.overview.accessMode.title', 'Access mode')}
      description={t('agents:edit.overview.accessMode.description', 'How this agent is allowed to authenticate.')}
      headerAction={
        onGoToAdvanced && (
          <Button size="small" endIcon={<ArrowRight size={11} />} onClick={onGoToAdvanced}>
            {t('agents:edit.overview.accessMode.editAdvanced', 'Edit in Advanced')}
          </Button>
        )
      }
    >
      <Stack sx={{border: '1px solid', borderColor: 'divider', borderRadius: '8px', overflow: 'hidden'}}>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          spacing={1.25}
          sx={{px: 1.75, py: 1.25}}
        >
          <Typography variant="body2" color="text.secondary">
            {t('agents:edit.overview.accessMode.own', 'On its own behalf')}
          </Typography>
          <Chip
            label={t('common:status.enabled', 'Enabled')}
            size="small"
            color="success"
            variant="outlined"
            sx={{height: 20, fontSize: '0.7rem'}}
          />
        </Stack>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          spacing={1.25}
          sx={{px: 1.75, py: 1.25, borderTop: '1px solid', borderColor: 'divider'}}
        >
          <Typography variant="body2" color="text.secondary">
            {t('agents:edit.overview.accessMode.delegated', 'On behalf of a user')}
          </Typography>
          <Chip
            label={isDelegated ? t('common:status.enabled', 'Enabled') : t('common:status.disabled', 'Disabled')}
            size="small"
            color={isDelegated ? 'success' : 'default'}
            variant="outlined"
            sx={{height: 20, fontSize: '0.7rem'}}
          />
        </Stack>
        {isDelegated && (
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            spacing={1.25}
            sx={{px: 1.75, py: 1.25, borderTop: '1px solid', borderColor: 'divider'}}
          >
            <Typography variant="body2" color="text.secondary">
              {t('agents:edit.overview.accessMode.allowedUserTypes', 'Allowed user types')}
            </Typography>
            <Typography variant="body2">
              {allowedUserTypes.length > 0
                ? allowedUserTypes.join(', ')
                : t('agents:edit.overview.signInPreview.notConfigured', 'Not configured')}
            </Typography>
          </Stack>
        )}
      </Stack>
    </OverviewCard>
  );

  return (
    <Stack direction="row" flexWrap="wrap" gap={3.5} alignItems="flex-start">
      {/* Left: main content */}
      <Stack sx={{flex: '2 0 480px', minWidth: 0}} spacing={2.25}>
        {quickstartDocsUrl && (
          <IconCard
            icon={
              <IconBadge>
                <LangChainLogo size={18} />
              </IconBadge>
            }
            title={t('agents:edit.overview.quickstart.title', 'Connect with LangChain')}
            description={t(
              'agents:edit.overview.quickstart.description',
              "Register this agent and call it from a LangChain app, running under its own identity or a user's delegated authority.",
            )}
          >
            <Link
              component="button"
              type="button"
              variant="body2"
              underline="hover"
              sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5, fontWeight: 600}}
              onClick={() => externalLinkConfirmation.requestNavigation(quickstartDocsUrl)}
            >
              {t('agents:edit.overview.quickstart.action', 'Open quickstart')}
              <ArrowUpRight size={13} />
            </Link>
          </IconCard>
        )}

        {accessModeCard}
        {attributesCard}
      </Stack>

      {/* Right: identifiers and endpoints sidebar */}
      <Stack spacing={1.75} sx={{flex: '1 0 280px', maxWidth: {xs: '100%', md: 320}}}>
        {detailsCard}
        {endpointsCard}
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
