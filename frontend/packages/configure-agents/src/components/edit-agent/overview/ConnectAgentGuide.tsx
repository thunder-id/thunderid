// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GuideOptionMenu, LangChainLogo, PythonLogo, SEGMENTED_CONTROL_SX, TypeScriptLogo} from '@thunderid/components';
import {OAuth2GrantTypes, QuickstartGuide} from '@thunderid/configure-applications';
import {useGetFlowById} from '@thunderid/configure-flows';
import {Box, Button, Paper, Stack, ToggleButton, ToggleButtonGroup, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, ArrowUpRight, Bot, CircleAlert, CircleCheck, UserCheck} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetAgentRoles from '../../../api/useGetAgentRoles';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

/** Where the docs sample listens for the user coming back from sign-in. */
const SAMPLE_REDIRECT_URI = 'http://localhost:6274/callback';
/** The executor a flow asks the user for consent with. */
const CONSENT_EXECUTOR = 'ConsentExecutor';

/**
 * How the agent in the guide authenticates, as the docs page names each mode.
 */
type AgentGuideMode = 'own' | 'obo';

/**
 * The language of the guide's code samples, as the docs page names each.
 */
type AgentGuideLanguage = 'python' | 'typescript';

/**
 * Props for the {@link ConnectAgentGuide} component.
 */
export interface ConnectAgentGuideProps {
  /**
   * The URL of the LangChain quickstart on the docs site
   */
  docsUrl: string;
  /**
   * Placeholder text in the docs code samples, mapped to this agent's value for it
   */
  codeValues: Record<string, string | undefined>;
  /**
   * The agent the guide is for
   */
  agent: Agent;
  /**
   * The agent's OAuth2 configuration, when it has one
   */
  oauth2Config?: OAuthAgentConfig;
  /**
   * Whether the agent may act on behalf of a user
   */
  isDelegated: boolean;
  /**
   * Navigates to the agent's Advanced tab, where Delegated mode and redirect URIs are set
   */
  onGoToAdvanced?: () => void;
  /**
   * Navigates to the agent's Access tab, where its roles are
   */
  onGoToAccess?: () => void;
  /**
   * Navigates to the agent's Flows tab, where its sign-in flow is chosen
   */
  onGoToFlows?: () => void;
  /**
   * Opens an external URL, after whatever confirmation the host page wants
   */
  onOpenLink: (url: string) => void;
}

/** One thing the agent needs before the guide's sample runs, and whether it has it. */
interface ReadinessCheck {
  key: string;
  isReady: boolean;
  title: string;
  detail: string;
  /** What to do about it, when something is */
  action?: {label: string; onClick: () => void};
}

/**
 * What this agent still needs before the sample of the chosen mode runs, read from the agent's own
 * settings, each with the way to the tab that settles it. A check whose answer cannot be read is
 * left out rather than guessed.
 */
function BeforeYouRun({
  mode,
  agent,
  oauth2Config = undefined,
  isDelegated,
  onGoToAdvanced = undefined,
  onGoToAccess = undefined,
  onGoToFlows = undefined,
}: {
  mode: AgentGuideMode;
} & Pick<
  ConnectAgentGuideProps,
  'agent' | 'oauth2Config' | 'isDelegated' | 'onGoToAdvanced' | 'onGoToAccess' | 'onGoToFlows'
>): JSX.Element | null {
  const {t} = useTranslation();
  // As the Access tab reads them, so the two share one request.
  const {data: roles} = useGetAgentRoles(agent.id, {limit: 100, offset: 0});
  const {data: signInFlow} = useGetFlowById(agent.authFlowId, mode === 'obo');
  const action = (label: string, onClick?: () => void): ReadinessCheck['action'] =>
    onClick ? {label, onClick} : undefined;
  const toAdvanced = (label: string): ReadinessCheck['action'] => action(label, onGoToAdvanced);
  const checks: ReadinessCheck[] = [];

  if (mode === 'own') {
    if (oauth2Config) {
      const canActAlone = oauth2Config.grantTypes.includes(OAuth2GrantTypes.CLIENT_CREDENTIALS);
      checks.push(
        canActAlone
          ? {
              key: 'own',
              isReady: true,
              title: t('agents:edit.overview.quickstart.checks.own.ready', 'This agent can act on its own'),
              detail: t('agents:edit.overview.quickstart.checks.own.readyDetail', 'Client credentials are enabled.'),
            }
          : {
              key: 'own',
              isReady: false,
              title: t('agents:edit.overview.quickstart.checks.own.missing', 'This agent cannot act on its own'),
              detail: t(
                'agents:edit.overview.quickstart.checks.own.missingDetail',
                'The client credentials grant is off.',
              ),
              action: toAdvanced(t('agents:edit.overview.quickstart.checks.turnOnInAdvanced', 'Turn on in Advanced')),
            },
      );
    }
    if (roles) {
      checks.push(
        roles.totalResults > 0
          ? {
              key: 'roles',
              isReady: true,
              title: t('agents:edit.overview.quickstart.checks.roles.ready', 'A role is assigned to this agent'),
              detail: t(
                'agents:edit.overview.quickstart.checks.roles.readyDetail',
                'Its token carries the permissions its roles grant.',
              ),
            }
          : {
              key: 'roles',
              isReady: false,
              title: t('agents:edit.overview.quickstart.checks.roles.missing', 'No role assigned to this agent'),
              detail: t(
                'agents:edit.overview.quickstart.checks.roles.missingDetail',
                'Its token carries a permission only once a role grants it.',
              ),
              action: action(
                t('agents:edit.overview.quickstart.checks.roles.action', 'Assign in Access'),
                onGoToAccess,
              ),
            },
      );
    }
  } else {
    // Delegated mode covers other grants too, and the sample signs the user in with this one.
    const lacksSignInGrant =
      isDelegated && oauth2Config && !oauth2Config.grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);
    if (lacksSignInGrant) {
      checks.push({
        key: 'delegated',
        isReady: false,
        title: t(
          'agents:edit.overview.quickstart.checks.delegated.noSignInGrant',
          'The authorization code grant is off',
        ),
        detail: t(
          'agents:edit.overview.quickstart.checks.delegated.noSignInGrantDetail',
          'The sample signs the user in with it.',
        ),
        action: toAdvanced(t('agents:edit.overview.quickstart.checks.turnOnInAdvanced', 'Turn on in Advanced')),
      });
    } else {
      checks.push(
        isDelegated
          ? {
              key: 'delegated',
              isReady: true,
              title: t('agents:edit.overview.quickstart.checks.delegated.ready', 'Delegated mode is on'),
              detail: t(
                'agents:edit.overview.quickstart.checks.delegated.readyDetail',
                'This agent can act for a user.',
              ),
            }
          : {
              key: 'delegated',
              isReady: false,
              title: t('agents:edit.overview.quickstart.checks.delegated.missing', 'Delegated mode is off'),
              detail: t(
                'agents:edit.overview.quickstart.checks.delegated.missingDetail',
                'This agent cannot act for a user until it is on.',
              ),
              action: toAdvanced(t('agents:edit.overview.quickstart.checks.turnOnInAdvanced', 'Turn on in Advanced')),
            },
      );
    }
    if (oauth2Config) {
      const isRedirectAdded = Boolean(oauth2Config.redirectUris?.includes(SAMPLE_REDIRECT_URI));
      checks.push({
        key: 'redirect',
        isReady: isRedirectAdded,
        title: isRedirectAdded
          ? t('agents:edit.overview.quickstart.checks.redirect.ready', 'Redirect URI added')
          : t('agents:edit.overview.quickstart.checks.redirect.missing', 'Redirect URI not added'),
        detail: t('agents:edit.overview.quickstart.checks.redirect.detail', 'The sample listens on {{uri}}.', {
          uri: SAMPLE_REDIRECT_URI,
        }),
        action: isRedirectAdded
          ? undefined
          : toAdvanced(t('agents:edit.overview.quickstart.checks.redirect.action', 'Add in Advanced')),
      });
    }
    if (signInFlow) {
      const asksForConsent = signInFlow.nodes.some((node) => node.executor?.name === CONSENT_EXECUTOR);
      checks.push(
        asksForConsent
          ? {
              key: 'consent',
              isReady: true,
              title: t('agents:edit.overview.quickstart.checks.consent.ready', 'Sign-in flow asks for consent'),
              detail: t(
                'agents:edit.overview.quickstart.checks.consent.readyDetail',
                '{{flow}}. The user is asked to allow the agent.',
                {flow: signInFlow.name},
              ),
            }
          : {
              key: 'consent',
              isReady: false,
              title: t('agents:edit.overview.quickstart.checks.consent.missing', 'Sign-in flow has no consent step'),
              detail: t(
                'agents:edit.overview.quickstart.checks.consent.missingDetail',
                '{{flow}}. The user must be asked to allow the agent.',
                {flow: signInFlow.name},
              ),
              action: action(t('agents:edit.overview.quickstart.checks.consent.action', 'Open Flows'), onGoToFlows),
            },
      );
    }
    if (oauth2Config?.pkceRequired) {
      checks.push({
        key: 'pkce',
        isReady: true,
        title: t('agents:edit.overview.quickstart.checks.pkce.ready', 'PKCE is required'),
        detail: t(
          'agents:edit.overview.quickstart.checks.pkce.readyDetail',
          'The sample sends an S256 code challenge.',
        ),
      });
    }
  }

  if (checks.length === 0) {
    return null;
  }

  return (
    <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden'}}>
      <Typography
        id="connect-agent-guide-checks"
        variant="overline"
        component="h3"
        color="text.secondary"
        sx={{display: 'block', px: 2, py: 0.5, bgcolor: 'action.hover'}}
      >
        {t('agents:edit.overview.quickstart.checks.title', 'Before you run')}
      </Typography>
      <Stack component="ul" aria-labelledby="connect-agent-guide-checks" sx={{listStyle: 'none', m: 0, p: 0}}>
        {checks.map((check) => (
          <Stack
            key={check.key}
            component="li"
            direction="row"
            alignItems="center"
            flexWrap="wrap"
            columnGap={1.5}
            rowGap={0.5}
            sx={{px: 2, py: 1.25, borderTop: 1, borderColor: 'divider'}}
          >
            <Box sx={{display: 'flex', color: check.isReady ? 'success.main' : 'warning.main'}}>
              {check.isReady ? (
                <CircleCheck
                  size={18}
                  role="img"
                  aria-label={t('agents:edit.overview.quickstart.checks.readyLabel', 'Ready')}
                />
              ) : (
                <CircleAlert
                  size={18}
                  role="img"
                  aria-label={t('agents:edit.overview.quickstart.checks.missingLabel', 'Needs attention')}
                />
              )}
            </Box>
            <Box sx={{flex: '1 1 240px', minWidth: 0}}>
              <Typography variant="subtitle2">{check.title}</Typography>
              <Typography variant="body2" color="text.secondary">
                {check.detail}
              </Typography>
            </Box>
            {check.action && (
              <Button size="small" endIcon={<ArrowRight size={14} aria-hidden />} onClick={check.action.onClick}>
                {check.action.label}
              </Button>
            )}
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}

/**
 * Shows, on the agent's Overview, how to run a LangChain agent as this agent: on its own, or on
 * behalf of a user, in Python or TypeScript. The steps come from the docs quickstart, so they follow
 * the docs version the Console is configured with, with this agent's values filled into the code.
 * Above them is what the agent itself still needs for the chosen mode.
 *
 * @param props - The component props
 * @returns The guide card
 */
export default function ConnectAgentGuide({
  docsUrl,
  codeValues,
  agent,
  oauth2Config = undefined,
  isDelegated,
  onGoToAdvanced = undefined,
  onGoToAccess = undefined,
  onGoToFlows = undefined,
  onOpenLink,
}: ConnectAgentGuideProps): JSX.Element {
  const {t} = useTranslation();
  const [mode, setMode] = useState<AgentGuideMode>('own');
  const [language, setLanguage] = useState<AgentGuideLanguage>('python');

  return (
    <Paper variant="outlined" component="section" aria-labelledby="connect-agent-guide-title" sx={{p: 3}}>
      <Stack spacing={3}>
        <Stack direction="row" alignItems="center" flexWrap="wrap" gap={2}>
          <Box
            sx={{
              width: 44,
              height: 44,
              borderRadius: '50%',
              border: 1,
              borderColor: 'divider',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <LangChainLogo size={20} />
          </Box>
          <Typography id="connect-agent-guide-title" variant="h5" component="h2" sx={{flex: '1 1 240px', minWidth: 0}}>
            {t('agents:edit.overview.quickstart.title', 'Connect with LangChain')}
          </Typography>
          <Button
            size="small"
            endIcon={<ArrowUpRight size={14} aria-hidden />}
            // The product's own docs, so it opens without the leaving-the-console confirmation.
            component="a"
            href={docsUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            {t('agents:edit.overview.quickstart.readGuide', 'Read the full guide')}
          </Button>
        </Stack>

        <Stack spacing={1}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={1.5}>
            <ToggleButtonGroup
              exclusive
              size="small"
              value={mode}
              onChange={(_, value: AgentGuideMode | null) => {
                if (value) {
                  setMode(value);
                }
              }}
              aria-label={t('agents:edit.overview.quickstart.mode.label', 'How the agent authenticates')}
              sx={SEGMENTED_CONTROL_SX}
            >
              <ToggleButton value="own">
                <Bot size={16} aria-hidden />
                {t('agents:edit.overview.quickstart.mode.own', 'Acting on its own')}
              </ToggleButton>
              <ToggleButton value="obo">
                <UserCheck size={16} aria-hidden />
                {t('agents:edit.overview.quickstart.mode.delegated', 'On behalf of a user')}
              </ToggleButton>
            </ToggleButtonGroup>
            <GuideOptionMenu
              id="connect-agent-language-menu"
              menuLabel={t('agents:edit.overview.quickstart.language.label', 'Language')}
              options={[
                {
                  value: 'python',
                  label: t('agents:edit.overview.quickstart.language.python', 'Python'),
                  icon: <PythonLogo size={16} />,
                },
                {
                  value: 'typescript',
                  label: t('agents:edit.overview.quickstart.language.typescript', 'TypeScript'),
                  icon: <TypeScriptLogo size={16} />,
                },
              ]}
              value={language}
              onSelect={(value) => setLanguage(value as AgentGuideLanguage)}
            />
          </Stack>
          <Typography variant="body2" color="text.secondary">
            {mode === 'own'
              ? t(
                  'agents:edit.overview.quickstart.mode.ownHint',
                  'The agent authenticates with its own client ID and secret. No user signs in.',
                )
              : t(
                  'agents:edit.overview.quickstart.mode.delegatedHint',
                  "A user signs in once and consents. The agent then acts with that user's delegated permissions.",
                )}
          </Typography>
        </Stack>

        {/* Up to here the card is about this agent. From here on it builds the docs' sample with it. */}
        <Typography variant="subtitle1" component="h3">
          {t('agents:edit.overview.quickstart.sample.title', 'Try it with a sample use case')}
        </Typography>

        <BeforeYouRun
          mode={mode}
          agent={agent}
          oauth2Config={oauth2Config}
          isDelegated={isDelegated}
          onGoToAdvanced={onGoToAdvanced}
          onGoToAccess={onGoToAccess}
          onGoToFlows={onGoToFlows}
        />

        <QuickstartGuide
          guide={{label: 'LangChain', docsUrl, variants: {mode, lang: language}}}
          codeValues={codeValues}
          onOpenLink={onOpenLink}
          // A step can hold a whole sample file, which would bury the steps after it.
          collapseLongSteps
        />
      </Stack>
    </Paper>
  );
}
