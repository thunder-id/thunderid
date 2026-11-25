// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CopyableField, ExternalLinkConfirmDialog, useExternalLinkConfirmation} from '@thunderid/components';
import {
  deriveOAuth2Flags,
  EnvironmentBar,
  EnvironmentClientIdField,
  EnvironmentNotDeployedNotice,
  useResourceEnvironments,
} from '@thunderid/configure-applications';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {useGetUsers} from '@thunderid/configure-users';
import {useConfig, useRuntimeUrl} from '@thunderid/contexts';
import {getUserTypeLabel} from '@thunderid/utils';
import {Box, Button, Chip, Divider, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ConnectAgentGuide from './ConnectAgentGuide';
import {PERSONAL_AGENT_KIND, type Agent, type OAuthAgentConfig} from '../../../models/agent';
import {hasDelegationGrant} from '../../../utils/isDelegatedAgent';
import AttributesSummarySection from '../attributes/AttributesSummarySection';

const formatUserLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attrs = user.attributes ?? {};
  const username = typeof attrs['username'] === 'string' ? attrs['username'] : undefined;
  const email = typeof attrs['email'] === 'string' ? attrs['email'] : undefined;
  return username ?? email ?? user.id;
};

/**
 * Props for the {@link AgentOverview} component.
 */
interface AgentOverviewProps {
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
  /**
   * Navigates to the agent's Access tab, where its roles are.
   */
  onGoToAccess?: () => void;
  /**
   * Navigates to the agent's Flows tab, where its sign-in flow is chosen.
   */
  onGoToFlows?: () => void;
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
 * shows the docs quickstart for connecting an AI agent framework (e.g. LangChain), with this agent's
 * values filled in, except for a personal agent, which its owner connects. A personal agent acts only for its owner, so it never
 * authenticates on its own and only the owner signs in through it.
 */
export default function AgentOverview({
  agent,
  oauth2Config = undefined,
  onGoToAdvanced = undefined,
  onGoToAccess = undefined,
  onGoToFlows = undefined,
}: AgentOverviewProps): JSX.Element {
  const {t} = useTranslation();
  const {getDocumentationLink} = useConfig();
  const externalLinkConfirmation = useExternalLinkConfirmation();
  const {data: usersData} = useGetUsers({limit: 100, offset: 0});
  const {data: userTypesData} = useGetUserTypes();

  // The endpoints below are for someone to copy into their own agent, so they have to name
  // the deployment that answers them rather than the one the console is configured against.
  const runtimeUrl = useRuntimeUrl();
  // Where the agent is deployed to more than one environment, its client ID and endpoints differ in each,
  // so the details and endpoints show one chosen environment's. An environment the agent is not in has no
  // client ID for it and its endpoints do not accept it, so neither is shown for it.
  const {
    environments,
    selectedEnvironment,
    shownEnvironment,
    selectEnvironment,
    notDeployedIds,
    isNotDeployed,
    previousEnvironmentName,
    selectedClientId: selectedClientIdValue,
    getValue,
    deployPath,
  } = useResourceEnvironments({resourceType: 'agent', resourceId: agent.id, hasClientId: Boolean(agent.clientId)});
  // A single environment's values are shown as one of several is, only with nothing to choose.
  const endpointsBaseUrl = shownEnvironment?.runtimeUrl ?? runtimeUrl;
  const quickstartDocsUrl = getDocumentationLink('agents.quickstarts.langchain.docs');
  // The guide is for the environment chosen too. An environment with no client ID for the agent leaves
  // the placeholder for the developer to fill in.
  let selectedClientId = agent.clientId;
  if (shownEnvironment) {
    selectedClientId = selectedClientIdValue?.status === 'set' ? selectedClientIdValue.value : undefined;
  }
  // The docs code samples are written against a local instance, or an environment's data plane, with
  // placeholder identifiers. Swapping in this agent's values leaves the developer nothing to look up.
  const guideCodeValues = {
    'https://localhost:8090': endpointsBaseUrl,
    'https://<DATA_PLANE_URL>': endpointsBaseUrl,
    'replace-with-your-client-id': selectedClientId,
    '<agent-id>': agent.id,
  };

  const ownerLabel = useMemo(() => {
    const owner = agent.owner;
    if (!owner?.id) return undefined;
    // Only a local owner is one of this deployment's users.
    const match = owner.isLocal === false ? undefined : (usersData?.users ?? []).find((user) => user.id === owner.id);
    return match ? formatUserLabel(match) : owner.id;
  }, [usersData, agent.owner]);
  const externalOwnerTypeLabel = (idType?: string): string => {
    if (idType === 'email') return t('agents:owner.idType.email', 'Email');
    if (idType === 'mobile') return t('agents:owner.idType.mobile', 'Mobile number');
    return t('agents:owner.idType.uuid', 'User ID');
  };

  const isPersonal = agent.kind === PERSONAL_AGENT_KIND;
  const isDelegated = hasDelegationGrant(oauth2Config?.grantTypes);
  // Allowed user types apply only to users signing in through this agent, which takes the
  // authorization code grant, the same rule as the Advanced tab's Allowed user types section.
  const hasAuthorizationCodeGrant = Boolean(oauth2Config && deriveOAuth2Flags(oauth2Config).hasAuthorizationCodeGrant);
  const allowedUserTypes = (agent.allowedUserTypes ?? []).map((handle) =>
    getUserTypeLabel(userTypesData?.types ?? [], handle),
  );

  const endpoints = [
    {
      key: 'wellknown',
      label: t('agents:edit.overview.endpoints.wellknown', 'OpenID configuration'),
      url: `${endpointsBaseUrl}/.well-known/openid-configuration`,
    },
    {
      key: 'authorization',
      label: t('agents:edit.overview.endpoints.authorization', 'Authorization endpoint'),
      url: `${endpointsBaseUrl}/oauth2/authorize`,
    },
    {
      key: 'token',
      label: t('agents:edit.overview.endpoints.token', 'Token endpoint'),
      url: `${endpointsBaseUrl}/oauth2/token`,
    },
    {
      key: 'userinfo',
      label: t('agents:edit.overview.endpoints.userinfo', 'Userinfo endpoint'),
      url: `${endpointsBaseUrl}/oauth2/userinfo`,
    },
    {
      key: 'jwks',
      label: t('agents:edit.overview.endpoints.jwks', 'JWKS URI'),
      url: `${endpointsBaseUrl}/oauth2/jwks`,
    },
  ];

  const sectionHeader = (title: string, description: string): JSX.Element => (
    <Box sx={{mt: 2.5, mb: 1.25, '&:first-child': {mt: 0}}}>
      <Typography variant="subtitle1" component="h3" sx={{fontWeight: 600, mb: 0.25}}>
        {title}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {description}
      </Typography>
    </Box>
  );
  const clientIdLabel = t('agents:edit.credentials.clientId.title', 'Client ID');

  // The identifiers and the endpoints share one card.
  const detailsPanel = (
    <Paper variant="outlined" sx={{borderRadius: '10px', p: 2.25}}>
      {shownEnvironment && isNotDeployed && (
        <Box sx={{mb: 2.5}}>
          <EnvironmentNotDeployedNotice
            resourceName={agent.display ?? agent.id}
            environmentName={shownEnvironment.name}
            previousEnvironmentName={previousEnvironmentName}
            deployPath={deployPath}
          />
        </Box>
      )}
      {sectionHeader(
        t('agents:edit.overview.agentDetails.title', 'Agent details'),
        t('agents:edit.overview.agentDetails.description', 'Identifiers used in your integration code.'),
      )}
      <Box>
        <CopyableField label={t('agents:edit.general.labels.agentId', 'Agent ID')} value={agent.id} />
        {agent.clientId &&
          !isNotDeployed &&
          (shownEnvironment && getValue ? (
            <EnvironmentClientIdField
              environment={shownEnvironment}
              resourceType="agent"
              resourceId={agent.id}
              getValue={getValue}
              label={clientIdLabel}
            />
          ) : (
            <CopyableField label={clientIdLabel} value={agent.clientId} />
          ))}
        {ownerLabel && (
          <CopyableField
            label={
              agent.owner?.isLocal === false
                ? t('agents:edit.general.labels.externalOwner', 'External owner ({{idType}})', {
                    idType: externalOwnerTypeLabel(agent.owner.idType),
                  })
                : t('agents:edit.general.labels.ownerId', 'Owner ID')
            }
            value={ownerLabel}
          />
        )}
        <CopyableField
          label={t('agents:edit.overview.agentDetails.organizationUnitId', 'Organization unit ID')}
          value={agent.ouId}
        />
        {agent.ouHandle && (
          <CopyableField
            label={t('agents:edit.overview.agentDetails.organizationUnitHandle', 'Organization unit handle')}
            value={agent.ouHandle}
          />
        )}
      </Box>
      {!isNotDeployed && (
        <>
          <Divider sx={{mt: 2.5}} />
          {sectionHeader(
            t('agents:edit.overview.endpoints.title', 'Useful endpoints'),
            t(
              'agents:edit.overview.endpoints.description',
              'For authenticating this agent, on its own or on behalf of a user.',
            ),
          )}
          <Box>
            {endpoints.map((endpoint) => (
              <CopyableField key={endpoint.key} label={endpoint.label} value={endpoint.url} />
            ))}
          </Box>
        </>
      )}
    </Paper>
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
            {t('agents:edit.overview.accessMode.editAdvanced', 'Edit in advanced')}
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
            label={isPersonal ? t('common:status.disabled', 'Disabled') : t('common:status.enabled', 'Enabled')}
            size="small"
            color={isPersonal ? 'default' : 'success'}
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
        {isPersonal ? (
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            spacing={1.25}
            sx={{px: 1.75, py: 1.25, borderTop: '1px solid', borderColor: 'divider'}}
          >
            <Typography variant="body2" color="text.secondary">
              {t('agents:edit.overview.accessMode.allowedUsers', 'Allowed users')}
            </Typography>
            <Typography variant="body2">{t('agents:edit.overview.accessMode.ownerOnly', 'Owner only')}</Typography>
          </Stack>
        ) : (
          hasAuthorizationCodeGrant && (
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
          )
        )}
      </Stack>
    </OverviewCard>
  );

  const hasGuide = Boolean(quickstartDocsUrl) && !isPersonal;

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
      <Stack direction="row" flexWrap="wrap" gap={3.5} alignItems="flex-start">
        {/* Left: the guide, or, without one, the agent's mode and attributes */}
        <Stack sx={{flex: '2 1 480px', minWidth: 0}} spacing={2.25}>
          {hasGuide && quickstartDocsUrl ? (
            <ConnectAgentGuide
              docsUrl={quickstartDocsUrl}
              codeValues={guideCodeValues}
              agent={agent}
              oauth2Config={oauth2Config}
              isDelegated={isDelegated}
              onGoToAdvanced={onGoToAdvanced}
              onGoToAccess={onGoToAccess}
              onGoToFlows={onGoToFlows}
              onOpenLink={externalLinkConfirmation.requestNavigation}
            />
          ) : (
            <>
              {accessModeCard}
              {attributesCard}
            </>
          )}
        </Stack>

        {/* Right: identifiers and endpoints, then, beside a guide, the agent's mode and attributes */}
        <Stack spacing={2.25} sx={{flex: '1 1 320px', minWidth: 0, maxWidth: {xs: '100%', md: 520}}}>
          {detailsPanel}
          {hasGuide && accessModeCard}
          {hasGuide && attributesCard}
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
