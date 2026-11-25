// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {GatewaysOverview, gatewayColor} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {useRoutes} from '@thunderid/contexts';
import {Avatar, Box, Card, CardActionArea, PageContent, Stack, Typography, alpha} from '@wso2/oxygen-ui';
import {Bot, ChevronRight, Cpu, Globe, MCP, Smartphone, UsersRound, Wallet, Webhook} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useRotation, {useSlotFade} from './home/useRotation';
import type RouteConfig from '../configs/RouteConfig';

/** One kind of resource to set up, and where its setup starts. */
interface Onboarding {
  id: string;
  icon: ReactNode;
  title: string;
  /** What the kind is for; not shown for now. */
  description: string;
  path: string;
}

/** The blue the kinds of resource are marked with. */
const ONBOARD_BLUE = gatewayColor(0);

/** How many kinds of resource the overview shows at once, in one row. */
const ONBOARD_SHOWN = 4;

/** One slot: a card that stays in place while the kind it offers fades out and the next fades in. */
function OnboardSlot({
  kind,
  byId,
  still,
}: {
  kind: Onboarding;
  byId: Map<string, Onboarding>;
  still: boolean;
}): JSX.Element {
  const navigate = useNavigate();
  const {heldId, fade} = useSlotFade(kind.id, still);
  const held = byId.get(heldId) ?? kind;
  return (
    <Card
      variant="outlined"
      sx={{
        borderRadius: '12px',
        bgcolor: 'background.default',
        '&:hover': {borderColor: (theme) => alpha(theme.palette.primary.main, 0.5)},
      }}
      data-testid={`project-onboard-${held.id}`}
    >
      <CardActionArea
        onClick={() => void navigate(held.path)}
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-start',
          gap: 1.5,
          height: 68,
          px: 1.75,
          ...fade,
        }}
      >
        <Avatar
          aria-hidden
          variant="rounded"
          sx={(theme) => ({
            width: 40,
            height: 40,
            borderRadius: '10px',
            // Blue, as the first gateway is marked, on the footer's color, in both color schemes.
            color: ONBOARD_BLUE.light,
            bgcolor: 'background.paper',
            ...theme.applyStyles('dark', {color: ONBOARD_BLUE.dark}),
          })}
        >
          {held.icon}
        </Avatar>
        <Box sx={{flex: '1 1 auto', minWidth: 0}}>
          <Typography variant="subtitle2" noWrap>
            {held.title}
          </Typography>
        </Box>
        <ChevronRight size={16} aria-hidden />
      </CardActionArea>
    </Card>
  );
}

/**
 * The kinds of resource a project secures, four at a time in one row, each opening its own setup in the
 * project. Each slot in turn changes to a kind not shown yet.
 */
function OnboardResources(): JSX.Element {
  const {t} = useTranslation();
  const paths = useRoutes<typeof RouteConfig>();
  // An application of one type, created through the application wizard with that type chosen.
  const applicationOf = (type: string): string => `${paths.applications.types()}?type=${type}`;
  const kinds: Onboarding[] = [
    {
      id: 'web',
      icon: <Globe size={20} />,
      title: t('home:project.onboard.web', 'Web application'),
      description: t('home:project.onboard.webHint', 'Sign users in with OpenID Connect'),
      path: applicationOf('FULL_STACK'),
    },
    {
      id: 'mobile',
      icon: <Smartphone size={20} />,
      title: t('home:project.onboard.mobile', 'Mobile app'),
      description: t('home:project.onboard.mobileHint', 'Native sign-in with PKCE'),
      path: applicationOf('MOBILE'),
    },
    {
      id: 'm2m',
      icon: <Cpu size={20} />,
      title: t('home:project.onboard.m2m', 'M2M / service'),
      description: t('home:project.onboard.m2mHint', 'Client credentials for backends'),
      path: applicationOf('BACKEND'),
    },
    {
      id: 'api',
      icon: <Webhook size={20} />,
      title: t('home:project.onboard.api', 'API'),
      description: t('home:project.onboard.apiHint', 'Protect endpoints with scopes'),
      path: paths.resourceServers.create(),
    },
    {
      id: 'agent',
      icon: <Bot size={20} />,
      title: t('home:project.onboard.agent', 'AI agent'),
      description: t('home:project.onboard.agentHint', 'Give an agent its own identity'),
      path: paths.agents.create(),
    },
    {
      id: 'mcp',
      icon: <MCP size={20} />,
      title: t('home:project.onboard.mcp', 'MCP server or client'),
      description: t('home:project.onboard.mcpHint', 'Secure tool access for agents'),
      path: applicationOf('MCP_CLIENT'),
    },
    {
      id: 'workforce',
      icon: <UsersRound size={20} />,
      title: t('home:project.onboard.workforce', 'Workforce SSO'),
      description: t('home:project.onboard.workforceHint', 'Employee sign-in to your apps'),
      path: paths.connections.create(),
    },
    {
      id: 'wallet',
      icon: <Wallet size={20} />,
      title: t('home:project.onboard.wallet', 'Digital wallet'),
      description: t('home:project.onboard.walletHint', 'Issue and verify credentials'),
      path: applicationOf('WALLET'),
    },
  ];
  const byId = new Map(kinds.map((kind: Onboarding) => [kind.id, kind]));
  const {shown, still, holdProps} = useRotation(
    kinds.map((kind: Onboarding) => kind.id),
    ONBOARD_SHOWN,
  );
  return (
    <Stack
      component="section"
      aria-label={t('home:project.onboard.title', 'Onboard the resources you want to secure')}
      spacing={1.75}
      sx={{
        p: '20px 22px 22px',
        borderRadius: '12px',
        // The same surface as the organization home's cards.
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
      }}
      data-testid="project-onboard"
    >
      <Typography component="h2" sx={{fontSize: 20, fontWeight: 600}}>
        {t('home:project.onboard.title', 'Onboard the resources you want to secure')}
      </Typography>
      <Box
        {...holdProps}
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)'},
          gap: '10px',
        }}
      >
        {shown.map((id: string, index: number) => {
          const kind = byId.get(id);
          // eslint-disable-next-line react/no-array-index-key -- a slot keeps its place while its kind changes.
          return kind ? <OnboardSlot key={index} kind={kind} byId={byId} still={still} /> : null;
        })}
      </Box>
    </Stack>
  );
}

/**
 * A project's home: what to set up in it. Its versions and gateways are on its deployment pipeline page.
 */
export default function ProjectHomePage(): JSX.Element {
  const {t} = useTranslation();
  const {selectedProject: project} = useProjectSelection();
  return (
    <PageContent>
      <DocumentTitle title={project?.displayName} />
      <GatewaysOverview
        eyebrow={project?.displayName}
        headline={t('home:project.headline', 'Organized resources, promoted like code')}
        lead={t(
          'home:project.lead',
          "Manage your resources in an organized way and promote them to production through a CI/CD-style pipeline that follows each resource's deployment lifecycle.",
        )}
        beforePipeline={<OnboardResources />}
        showPipeline={false}
      />
    </PageContent>
  );
}
