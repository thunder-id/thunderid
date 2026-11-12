// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {BotCogIcon, CapabilitiesErrorNotice, ForbiddenNotice, getInitials} from '@thunderid/components';
import {
  gatewaySectionOf,
  useDeployRoutes,
  usePipeline,
  useProjectChanges,
  type PipelineStage,
} from '@thunderid/configure-gateways';
import {ProjectSwitcher, useProjectSelection} from '@thunderid/configure-projects';
import {ProjectScopeProvider, useCapabilitiesContext, useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {SignOutButton, User, useThunderID} from '@thunderid/react';
import {
  AppShell,
  alpha,
  Box,
  Button,
  ColorSchemeImage,
  ColorSchemeToggle,
  Footer,
  Header,
  Sidebar,
  Tooltip,
  Typography,
  useAppShell,
  useSidebar,
  UserMenu,
} from '@wso2/oxygen-ui';
import {
  Bot,
  Building,
  Group,
  ArrowLeft,
  Compass,
  Globe,
  History,
  Home,
  KeyRound,
  IdCard,
  Languages,
  Layers,
  LayoutGrid,
  ListChecks,
  LogOut,
  Mail,
  MessageSquare,
  Network,
  Palette,
  Rocket,
  Server,
  Settings,
  ShieldCheck,
  SquareArrowRightEnter,
  UserRoundCog,
  UserStar,
  UsersRound,
  Wallet,
  Workflow,
} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as NavigateLink, Outlet, useLocation, useNavigate} from 'react-router';
import GatewaySwitcher from './components/GatewaySwitcher';
import useOpenGatewayId from './useOpenGatewayId';
import PendingDeployBanner from '../components/PendingDeployBanner';
import SignInOnUnauthorized from '../components/SignInOnUnauthorized';
import RouteConfig, {projectRouteConfig} from '../configs/RouteConfig';

const ICON_BUTTON_SX = {
  minWidth: 40,
  width: 40,
  height: 40,
  borderRadius: '50%',
  px: 0,
  '& .MuiButton-startIcon': {margin: 0},
} as const;

const FULL_BUTTON_SX = {
  height: 40,
  borderRadius: 40,
  wordBreak: 'keep-all',
  whiteSpace: 'nowrap',
  px: 2,
} as const;

/** A sidebar navigation entry. A leaf has a `path`; a parent has `children`. */
interface NavRoute {
  id: string;
  text: string;
  icon: JSX.Element;
  path?: string;
  /** The catalog resources the page shows; the entry is hidden from admins who can't view each of them. */
  resource?: string | string[];
  children?: NavRoute[];
  /** What a dot beside the entry says on hover, such as how many of its changes are not deployed yet. */
  badge?: string;
  /** A dot beside a parent entry while it is collapsed, standing in for its entries' own dots. */
  collapsedBadge?: string;
  /** Done as well as going to the entry's page, such as leaving the selected project. */
  onSelect?: () => void;
  /** A line under the entry's name saying what it holds. */
  description?: string;
}

interface NavCategory {
  category?: string;
  routes: NavRoute[];
  /** Belongs to the organization's resources that every project shares, which sit together below the rest. */
  shared?: boolean;
  /** Shown above the group's entries, such as the gateway selector on a gateway's pages. */
  before?: ReactNode;
}

/** Names the organization's resources every project shares, while the sidebar shows names. */
function SharedHeading(): ReactNode {
  const {t} = useTranslation();
  const {state} = useAppShell();
  if (state.sidebarCollapsed) return null;
  return (
    <Box sx={{pl: '22px', pr: 1.5, pb: 1.5}} data-testid="nav-shared-heading">
      <Typography
        component="div"
        sx={{display: 'flex', alignItems: 'center', gap: 1, fontSize: 12, fontWeight: 600, color: 'text.secondary'}}
      >
        <Globe size={14} />
        {t('navigation:categories.shared', 'Shared by all projects')}
      </Typography>
      <Typography sx={{fontSize: 12, color: 'text.secondary', opacity: 0.85}}>
        {t('navigation:categories.sharedHint', 'Deployed with every project')}
      </Typography>
    </Box>
  );
}

function SidebarFooterButtons(): ReactNode {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {collapsed} = useSidebar();
  const {can, isLoading} = useCapabilitiesContext();

  if (!isLoading && !can('import', 'create') && !can('export', 'create')) {
    return null;
  }

  const buttonSx = collapsed ? ICON_BUTTON_SX : FULL_BUTTON_SX;

  return (
    <Box
      sx={{
        p: 1.5,
        display: 'flex',
        flexDirection: collapsed ? 'column' : 'row',
        justifyContent: 'center',
        gap: 1,
      }}
    >
      <Button
        variant="outlined"
        aria-label={t('navigation:pages.importExport', 'Import / Export')}
        startIcon={<SquareArrowRightEnter size={18} />}
        onClick={() => void navigate(RouteConfig.importExport.list())}
        sx={buttonSx}
      >
        {!collapsed && t('navigation:pages.importExport', 'Import / Export')}
      </Button>
    </Box>
  );
}

/**
 * Props interface of {@link DashboardLayout}
 */
export interface DashboardLayoutProps {
  /**
   * Collapses the navigation sidebar to icon-only mode. Set by routes whose
   * pages need the full screen width (e.g. the flow builder canvas).
   */
  collapseSidebar?: boolean;
}

/**
 * Applies a route's sidebar preference to the shell.
 *
 * The preference is applied when it changes rather than being bound to the sidebar,
 * so builder routes still start collapsed while the header toggle stays free to
 * collapse and expand it afterwards.
 */
function SidebarPreference({collapsed}: {collapsed: boolean}): null {
  const {actions} = useAppShell();
  const {collapseSidebar, expandSidebar} = actions;

  useEffect(() => {
    if (collapsed) {
      collapseSidebar();
    } else {
      expandSidebar();
    }
    // Runs on the route's preference only: re-running whenever the shell actions are
    // re-created would undo a manual toggle on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [collapsed]);

  return null;
}

/**
 * The console's frame: header, navigation and the page. The selected project scopes all of it, so the
 * header and the navigation show the project's gateways and what is not deployed yet, and its pages
 * deploy and promote within it.
 */
export default function DashboardLayout({collapseSidebar = false}: DashboardLayoutProps): ReactNode {
  const {selectedProject} = useProjectSelection();
  return (
    <ProjectScopeProvider project={selectedProject}>
      <ScopedDashboardLayout collapseSidebar={collapseSidebar} />
    </ProjectScopeProvider>
  );
}

function ScopedDashboardLayout({collapseSidebar = false}: DashboardLayoutProps): ReactNode {
  const {clearSession, discovery} = useThunderID();
  const {projects, selectedProject: project} = useProjectSelection();
  // How many of the project's own applications and agents the first gateway does not run yet.
  const undeployed = useProjectChanges();
  // On one of a gateway's pages, the navigation is the gateway's.
  const gatewayId = useOpenGatewayId();
  const pipeline = usePipeline();
  const deployRoutes = useDeployRoutes();
  const stages: PipelineStage[] = pipeline.data?.stages ?? [];
  const gatewayIndex = stages.findIndex((stage: PipelineStage) => stage.gateway.id === gatewayId);
  const gateway = gatewayIndex < 0 ? undefined : stages[gatewayIndex];
  const {isTrustedIssuerGenericOidc, getTrustedIssuerClientId, getClientUrl, config} = useConfig();
  const {t} = useTranslation();
  const logger = useLogger();
  // When sign-in runs on a backend (BFF), the backend holds the session and its sign-out endpoint
  // ends it at the IdP, so signOut() must go there instead of the IdP directly.
  const signsOutThroughBackend = Boolean(
    (config.sdk?.endpoints as {backend?: {signOut?: string}} | undefined)?.backend?.signOut,
  );

  const handleSignOut = (signOut: () => Promise<void>): void => {
    if (isTrustedIssuerGenericOidc() && !signsOutThroughBackend) {
      try {
        clearSession();
      } catch (error: unknown) {
        logger.error('Failed to clear local session before IdP sign out', {error});
      }

      const endSessionEndpoint = discovery?.wellKnown?.end_session_endpoint;
      if (!endSessionEndpoint) {
        logger.warn('end_session_endpoint missing from IdP discovery document; ending local session only');
        // eslint-disable-next-line react-hooks/immutability
        window.location.href = getClientUrl();
        return;
      }

      const logoutUrl = new URL(endSessionEndpoint);
      logoutUrl.searchParams.set('client_id', getTrustedIssuerClientId());
      logoutUrl.searchParams.set('post_logout_redirect_uri', getClientUrl());
      // eslint-disable-next-line react-hooks/immutability
      window.location.href = logoutUrl.toString();
      return;
    }

    // Native ThunderID session: signOut() performs OIDC RP-Initiated Logout, the SDK's default
    // behavior (no client config required). It clears the local session and redirects to ThunderID's
    // end_session_endpoint, which confirms the sign-out, terminates the SSO session server-side, and
    // returns to the console. It falls back to a local-only sign out when no end_session_endpoint is
    // advertised.
    signOut().catch((error: unknown) => {
      logger.error('Sign out failed', {error});
    });
  };

  const appRoutes: NavCategory[] = useMemo(() => {
    // In a project, its pages are under its path.
    const paths = project ? projectRouteConfig(project.handle) : RouteConfig;
    const applications: NavRoute = {
      id: 'applications',
      resource: 'app',
      text: t('navigation:pages.applications'),
      icon: <LayoutGrid />,
      path: paths.applications.list(),
    };
    const agents: NavRoute = {
      id: 'agents',
      resource: 'agent',
      text: t('navigation:pages.agents', 'Agents'),
      icon: <Bot />,
      path: paths.agents.list(),
    };
    const resourceServers: NavRoute = {
      id: 'resource-servers',
      resource: 'rs',
      text: t('navigation:pages.resourceServers', 'Resource servers'),
      icon: <Server />,
      path: paths.resourceServers.list(),
    };
    const roles: NavRoute = {
      id: 'roles',
      resource: 'role',
      text: t('navigation:pages.roles'),
      icon: <ShieldCheck />,
      path: paths.roles.list(),
    };
    const flows: NavRoute = {
      id: 'flows',
      resource: 'flow',
      text: t('navigation:pages.flows'),
      icon: <Workflow />,
      path: paths.flows.list(),
    };
    const groups: NavRoute = {
      id: 'groups',
      resource: 'group',
      text: t('navigation:pages.groups', 'Groups'),
      icon: <Group />,
      path: paths.groups.list(),
    };
    const design: NavRoute = {
      id: 'design',
      resource: 'design',
      text: t('navigation:pages.design', 'Design'),
      icon: <Palette />,
      path: paths.design.list(),
    };
    const notifications: NavRoute = {
      id: 'notifications',
      text: t('navigation:pages.notifications', 'Notifications'),
      icon: <Mail />,
      children: [
        {
          id: 'email-templates',
          resource: 'notificationtemplate',
          text: t('navigation:pages.emailTemplates', 'Email templates'),
          icon: <Mail />,
          path: paths.notificationTemplates.channel('email'),
        },
        {
          id: 'sms-templates',
          resource: 'notificationtemplate',
          text: t('navigation:pages.smsTemplates', 'SMS templates'),
          icon: <MessageSquare />,
          path: paths.notificationTemplates.channel('sms'),
        },
      ],
    };
    const badgeOf = (count: number | undefined): string | undefined =>
      count ? t('navigation:badges.undeployed', '{{count}} undeployed', {count}) : undefined;
    // On a gateway's pages the navigation is the gateway's, the same from wherever it was opened: a
    // gateway belongs to the organization. The way back leads to the project it was opened from, if any.
    if (gateway && gatewayId) {
      const section = (name: 'overview' | 'values' | 'endpoints' | 'history' | 'explore'): string =>
        gatewaySectionOf(deployRoutes, gatewayId, undefined, name);
      // As the design has it: back to the organization, the gateway open with a way to switch it, then its pages.
      return [
        {
          routes: [
            project
              ? {
                  id: 'gateway-back',
                  text: project.displayName,
                  icon: <ArrowLeft />,
                  path: RouteConfig.projects.home(project.handle),
                }
              : {
                  id: 'gateway-back',
                  text: t('navigation:pages.organizationOverview', 'Organization overview'),
                  icon: <ArrowLeft />,
                  path: RouteConfig.home.list(),
                },
          ],
        },
        {
          before: <GatewaySwitcher />,
          routes: [
            {
              id: 'gateway-overview',
              resource: 'gateway',
              text: t('navigation:pages.overview', 'Overview'),
              icon: <Home />,
              path: section('overview'),
            },
          ],
        },
        {
          category: t('navigation:categories.configure'),
          routes: [
            {
              id: 'gateway-values',
              resource: ['gatewayvariable', 'gatewaysecret'],
              text: t('navigation:pages.variablesAndSecrets', 'Variables & secrets'),
              icon: <KeyRound />,
              path: section('values'),
            },
          ],
        },
        {
          category: t('navigation:categories.test', 'Test'),
          routes: [
            {
              id: 'gateway-endpoints',
              resource: 'gateway',
              text: t('navigation:pages.endpoints', 'Endpoints'),
              icon: <Globe />,
              path: section('endpoints'),
            },
          ],
        },
        {
          category: t('navigation:categories.releases', 'Releases'),
          routes: [
            {
              id: 'gateway-explore',
              resource: 'configversion',
              text: t('navigation:pages.deployedConfiguration', 'Deployed configuration'),
              icon: <Compass />,
              path: section('explore'),
            },
            {
              id: 'gateway-history',
              resource: 'configversion',
              text: t('navigation:pages.deploymentHistory', 'Deployment history'),
              icon: <History />,
              path: section('history'),
            },
          ],
        },
      ];
    }
    // A project's navigation, as its home lays it out: its overview, then what it builds. The way back
    // to the organization is beside the project in the header.
    if (project) {
      return [
        {
          routes: [
            {
              id: 'project-home',
              resource: 'project',
              text: t('navigation:pages.overview', 'Overview'),
              icon: <Home />,
              path: RouteConfig.projects.home(project.handle),
            },
            {
              id: 'project-pipeline',
              resource: 'gateway',
              text: t('navigation:pages.deploymentPipeline', 'Deployment pipeline'),
              icon: <Rocket />,
              path: RouteConfig.projects.home(project.handle, '/pipeline'),
            },
          ],
        },
        {
          category: t('navigation:categories.build', 'Build'),
          routes: [
            {...applications, badge: badgeOf(undeployed.application)},
            {...agents, badge: badgeOf(undeployed.agent)},
            {...resourceServers, badge: badgeOf(undeployed.resource_server)},
            {
              id: 'authorization-detail-types',
              resource: 'authzdetail',
              text: t('navigation:pages.authorizationDetailTypes', 'Intents'),
              icon: <ListChecks />,
              path: paths.authorizationDetailTypes.list(),
              badge: badgeOf(undeployed.authorization_detail_type),
            },
            {...roles, badge: badgeOf(undeployed.role)},
            {...flows, badge: badgeOf(undeployed.flow)},
            {
              id: 'verifiable-credentials',
              text: t('navigation:pages.verifiableCredentials', 'Verifiable credentials'),
              icon: <Wallet />,
              // While collapsed it stands for its entries' changes; once expanded each entry shows its own.
              collapsedBadge: badgeOf(
                (undeployed.credential_configuration ?? 0) + (undeployed.presentation_definition ?? 0),
              ),
              children: [
                {
                  id: 'credentials',
                  resource: 'vc',
                  text: t('navigation:pages.credentials', 'Templates'),
                  icon: <IdCard />,
                  path: paths.verifiableCredentials.list(),
                  badge: badgeOf(undeployed.credential_configuration),
                },
                {
                  id: 'presentations',
                  resource: 'vc',
                  text: t('navigation:pages.presentations', 'Presentations'),
                  icon: <ShieldCheck />,
                  path: paths.verifiablePresentations.list(),
                  badge: badgeOf(undeployed.presentation_definition),
                },
              ],
            },
          ],
        },
        {
          category: t('navigation:categories.identities', 'Identities'),
          routes: [
            {
              id: 'users',
              resource: 'user',
              text: t('navigation:pages.users', 'Users'),
              icon: <UsersRound />,
              path: paths.users.list(),
            },
            groups,
            {
              id: 'organization-units',
              resource: 'ou',
              text: t('navigation:pages.organizationUnits', 'Organization units'),
              icon: <Building />,
              path: paths.organizationUnits.list(),
              badge: badgeOf(undeployed.organization_unit),
            },
          ],
        },
        {
          category: t('navigation:categories.customize', 'Customize'),
          routes: [
            {...design, badge: badgeOf((undeployed.layout ?? 0) + (undeployed.theme ?? 0))},
            {...notifications, badge: badgeOf(undeployed.notification_template)},
          ],
        },
      ];
    }
    // The organization's navigation, as the organization overview lays it out.
    return [
      {
        routes: [
          {
            id: 'home',
            text: t('navigation:pages.overview', 'Overview'),
            icon: <Home />,
            path: RouteConfig.home.list(),
          },
        ],
      },
      {
        routes: [
          {
            id: 'projects',
            resource: 'project',
            text: t('navigation:pages.projects', 'Projects'),
            description: t('navigation:pages.projectsHint', 'Manage applications, agents, flows etc.'),
            icon: <LayoutGrid />,
            path: RouteConfig.projects.list(),
          },
          {
            id: 'gateways',
            resource: 'gateway',
            text: t('navigation:pages.gateways', 'Deployment'),
            icon: <Network />,
            path: RouteConfig.gateways.list(),
          },
        ],
      },
      {
        shared: true,
        category: t('navigation:categories.identitySetup', 'Identity setup'),
        routes: [
          groups,
          {
            id: 'user-types',
            resource: 'usertype',
            text: t('navigation:pages.userTypes'),
            icon: <UserRoundCog />,
            path: RouteConfig.userTypes.list(),
          },
          {
            id: 'agent-types',
            resource: 'agenttype',
            text: t('navigation:pages.agentTypes', 'Agent types'),
            icon: <BotCogIcon />,
            path: RouteConfig.agentTypes.list(),
          },
          roles,
        ],
      },
      {
        shared: true,
        category: t('navigation:categories.signInExperience', 'Sign-in experience'),
        routes: [
          {
            id: 'connections',
            resource: 'connection',
            text: t('navigation:pages.connections'),
            icon: <Layers />,
            path: RouteConfig.connections.list(),
          },
          {
            id: 'translations',
            resource: 'i18n',
            text: t('navigation:pages.translations'),
            icon: <Languages />,
            path: RouteConfig.translations.list(),
          },
        ],
      },
      {
        shared: true,
        category: t('navigation:categories.apisAndCredentials', 'APIs and credentials'),
        routes: [resourceServers],
      },
      {
        shared: true,
        category: t('navigation:categories.organization', 'Organization'),
        routes: [
          {
            id: 'administrators',
            resource: 'user',
            text: t('navigation:pages.administrators', 'Administrators'),
            icon: <UserStar />,
            path: paths.administrators.list(),
          },
          {
            id: 'settings',
            resource: 'serverconfig',
            text: t('navigation:pages.settings', 'Configurations'),
            icon: <Settings />,
            path: RouteConfig.settings.list(),
          },
        ],
      },
    ];
  }, [
    t,
    project,
    undeployed.application,
    undeployed.agent,
    undeployed.resource_server,
    undeployed.authorization_detail_type,
    undeployed.role,
    undeployed.flow,
    undeployed.layout,
    undeployed.theme,
    undeployed.notification_template,
    undeployed.presentation_definition,
    undeployed.credential_configuration,
    undeployed.organization_unit,
    gateway,
    gatewayId,
    deployRoutes,
  ]);

  const {can, isLoading: isLoadingCapabilities, error: capabilitiesError} = useCapabilitiesContext();
  const isVisible = useCallback(
    (route: NavRoute): boolean =>
      isLoadingCapabilities || [route.resource ?? []].flat().every((resource: string) => can(resource)),
    [can, isLoadingCapabilities],
  );

  // The navigation the admin may see: entries for pages they can't view, and groups left empty, are hidden.
  const visibleRoutes: NavCategory[] = useMemo(
    () =>
      appRoutes
        .map((group) => ({
          ...group,
          routes: group.routes
            .map((route) => (route.children ? {...route, children: route.children.filter(isVisible)} : route))
            .filter((route) => (route.children ? route.children.length > 0 : isVisible(route))),
        }))
        .filter((group) => group.routes.length > 0),
    [appRoutes, isVisible],
  );

  const {pathname} = useLocation();

  // Resolve the active leaf item by the longest matching path prefix, so nested
  // routes (e.g. /verifiable-credentials/create) still highlight their menu item.
  const activeItem = useMemo((): string | undefined => {
    const leaves = appRoutes.flatMap((group) => group.routes.flatMap((route) => route.children ?? [route]));
    const match = leaves
      .filter(
        (route): route is NavRoute & {path: string} =>
          route.path !== undefined && (pathname === route.path || pathname.startsWith(`${route.path}/`)),
      )
      .sort((a, b) => b.path.length - a.path.length)[0];
    return match?.id;
  }, [appRoutes, pathname]);

  const activeRoute = useMemo(
    () =>
      appRoutes
        .flatMap((group) => group.routes.flatMap((route) => route.children ?? [route]))
        .find((route) => route.id === activeItem),
    [appRoutes, activeItem],
  );
  const isPageForbidden = !isLoadingCapabilities && activeRoute !== undefined && !isVisible(activeRoute);

  const [expandedMenus, setExpandedMenus] = useState<Record<string, boolean>>({});
  const handleToggleExpand = (id: string): void => {
    setExpandedMenus((prev) => ({...prev, [id]: !prev[id]}));
  };

  // Auto-expand a parent whose child is the active route.
  useEffect(() => {
    appRoutes.forEach((group) => {
      group.routes.forEach((route) => {
        if (route.children?.some((child) => child.id === activeItem)) {
          setExpandedMenus((prev) => (prev[route.id] ? prev : {...prev, [route.id]: true}));
        }
      });
    });
  }, [appRoutes, activeItem]);

  // The entry whose dot's hint is showing: on hover, or while the entry has keyboard focus, as a dot cannot take focus
  // inside its entry. An entry is found by the dot inside what took focus, whether that is a link or a menu's toggle.
  const [hintEntry, setHintEntry] = useState<string | null>(null);
  useEffect(() => {
    const showHintOnFocus = (event: FocusEvent): void => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      const entry = target?.querySelector<HTMLElement>('[data-hint-entry]')?.dataset.hintEntry;
      setHintEntry(entry && target?.matches(':focus-visible') ? entry : null);
    };
    document.addEventListener('focusin', showHintOnFocus);
    return () => document.removeEventListener('focusin', showHintOnFocus);
  }, []);

  // An entry's label, with its badge at the end of the entry: a small dot that says on hover what it stands for, so it
  // fits beside any entry's name. With a badge the label keeps its whole width and takes the entry's free space, so every
  // dot lines up at the end of its entry. The hint is also part of the entry's name, so screen readers read it.
  // The Sidebar picks an item's parts out of its direct children, so they are returned as a list, not a fragment, and
  // the hint wraps the dot inside the badge rather than the badge. The hint's target is larger than the dot, so the
  // dot is easy to hover.
  const labelOf = (id: string, text: ReactNode, badge: string | undefined): JSX.Element[] => [
    <Sidebar.ItemLabel key="label" sx={badge ? {flex: '1 0 auto', overflow: 'visible'} : undefined}>
      {text}
    </Sidebar.ItemLabel>,
    ...(badge
      ? [
          <Sidebar.ItemBadge
            key="badge"
            sx={{ml: 1, flexShrink: 0, height: 'auto', bgcolor: 'transparent', '& .MuiChip-label': {p: 0}}}
          >
            <Tooltip
              title={badge}
              placement="right"
              open={hintEntry === id}
              onOpen={() => setHintEntry(id)}
              onClose={() => setHintEntry(null)}
            >
              <Box component="span" aria-hidden sx={{display: 'flex', p: 0.75}}>
                <Box component="span" sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: 'warning.main'}} />
              </Box>
            </Tooltip>
            <Box
              component="span"
              data-hint-entry={id}
              sx={{
                position: 'absolute',
                width: '1px',
                height: '1px',
                overflow: 'hidden',
                clip: 'rect(0 0 0 0)',
                whiteSpace: 'nowrap',
              }}
            >
              {badge}
            </Box>
          </Sidebar.ItemBadge>,
        ]
      : []),
  ];

  const renderGroup = (categoryGroup: NavCategory, index: number): ReactNode => (
    <Sidebar.Category key={categoryGroup.category ?? `group-${index}`}>
      {categoryGroup.before}
      {categoryGroup.category && <Sidebar.CategoryLabel>{categoryGroup.category}</Sidebar.CategoryLabel>}
      {categoryGroup.routes.map((route) =>
        route.children ? (
          <Sidebar.Item key={route.id} id={route.id}>
            <Sidebar.ItemIcon>{route.icon}</Sidebar.ItemIcon>
            {labelOf(route.id, route.text, route.badge ?? (expandedMenus[route.id] ? undefined : route.collapsedBadge))}
            {route.children.map((child) => (
              <Sidebar.Item key={child.id} id={child.id} link={<NavigateLink to={child.path ?? ''} />}>
                <Sidebar.ItemIcon>{child.icon}</Sidebar.ItemIcon>
                {labelOf(child.id, child.text, child.badge)}
              </Sidebar.Item>
            ))}
          </Sidebar.Item>
        ) : (
          <Sidebar.Item
            key={route.id}
            id={route.id}
            link={<NavigateLink to={route.path ?? ''} onClick={route.onSelect} />}
          >
            <Sidebar.ItemIcon>{route.icon}</Sidebar.ItemIcon>
            {labelOf(
              route.id,
              route.description ? (
                <Box component="span" sx={{display: 'flex', flexDirection: 'column', py: 0.75, whiteSpace: 'normal'}}>
                  <span>{route.text}</span>
                  <Typography component="span" sx={{fontSize: 12, lineHeight: 1.35, color: 'text.secondary'}}>
                    {route.description}
                  </Typography>
                </Box>
              ) : (
                route.text
              ),
              route.badge,
            )}
          </Sidebar.Item>
        ),
      )}
    </Sidebar.Category>
  );

  return (
    // `collapseSidebar` seeds AppShell's own state rather than force-controlling the
    // Sidebar. Passing `collapsed` to the Sidebar would override the shell's state and
    // leave the header toggle with nothing to drive.
    <AppShell initialCollapsed={collapseSidebar}>
      <SidebarPreference collapsed={collapseSidebar} />
      <AppShell.Navbar>
        <Header>
          <Header.Toggle />
          <Header.Brand>
            <Header.BrandLogo>
              <ColorSchemeImage
                src={{
                  light: `${import.meta.env.BASE_URL.replace(/\/$/, '')}/assets/images/logo.svg`,
                  dark: `${import.meta.env.BASE_URL.replace(/\/$/, '')}/assets/images/logo-inverted.svg`,
                }}
                alt={{light: 'Logo (Light)', dark: 'Logo (Dark)'}}
                height={27}
                width="auto"
                alignItems="center"
                marginBottom="3px"
              />
            </Header.BrandLogo>
            <Header.BrandTitle>Console</Header.BrandTitle>
          </Header.Brand>
          {/* On a gateway's pages the header names no project; the gateway is chosen in the sidebar. */}
          {projects.length > 0 && !gateway && (
            <Header.Switchers showDivider={false}>
              <ProjectSwitcher />
            </Header.Switchers>
          )}
          <Header.Spacer />
          <Header.Actions>
            <ColorSchemeToggle />
            <User>
              {(user) => {
                const name = String(user?.name ?? '');
                // Two-letter initials when there is no picture, matching the other WSO2 consoles.
                const avatar = typeof user?.picture === 'string' ? user.picture : name ? getInitials(name) : undefined;

                return (
                  <UserMenu>
                    <UserMenu.Trigger name={String(user?.name ?? '')} avatar={avatar} />
                    <UserMenu.Header
                      name={String(user?.name ?? '')}
                      email={String(user?.email ?? '')}
                      avatar={avatar}
                    />
                    <UserMenu.Divider />
                    <SignOutButton>
                      {({signOut}) => (
                        <UserMenu.Logout
                          icon={<LogOut />}
                          label={t('common:actions.signOut', 'Sign out')}
                          onClick={() => handleSignOut(signOut)}
                        />
                      )}
                    </SignOutButton>
                  </UserMenu>
                );
              }}
            </User>
          </Header.Actions>
        </Header>
      </AppShell.Navbar>

      <AppShell.Sidebar>
        {/* Wide enough for the longest page names, such as a gateway's deployed configuration, to show whole. */}
        <Sidebar width={272} activeItem={activeItem} expandedMenus={expandedMenus} onToggleExpand={handleToggleExpand}>
          <Sidebar.Nav>
            {visibleRoutes.filter((group) => !group.shared).map(renderGroup)}
            {visibleRoutes.some((group) => group.shared) && (
              <Box
                sx={(theme) => ({
                  // A band of its own to the sidebar's edges and foot, as the design shades it.
                  flex: '1 0 auto',
                  mt: 2,
                  mb: -1,
                  pt: 3,
                  pb: 2,
                  bgcolor: alpha(theme.palette.text.primary, 0.03),
                  ...theme.applyStyles('dark', {bgcolor: 'rgba(150, 170, 210, 0.06)'}),
                  // The band starts the shared part on its own, without the divider before it.
                  'hr:has(+ &)': {display: 'none'},
                })}
                data-testid="nav-shared"
              >
                <SharedHeading />
                {visibleRoutes.filter((group) => group.shared).map(renderGroup)}
              </Box>
            )}
          </Sidebar.Nav>
          <Sidebar.Footer>
            <SidebarFooterButtons />
          </Sidebar.Footer>
        </Sidebar>
      </AppShell.Sidebar>

      <AppShell.Main>
        <SignInOnUnauthorized />
        {capabilitiesError ? (
          <CapabilitiesErrorNotice />
        ) : isPageForbidden ? (
          <ForbiddenNotice />
        ) : (
          /* The banner over every page, scrolling away with it: the two scroll together, the page at its full height. */
          <Box
            sx={{
              display: 'flex',
              flexDirection: 'column',
              flex: 1,
              minWidth: 0,
              minHeight: 0,
              overflowY: 'auto',
              '& > :last-child': {flex: '1 0 auto', height: 'auto', overflow: 'visible'},
            }}
          >
            {/* An environment's own pages tell what it lacks themselves. */}
            {!gatewayId && <PendingDeployBanner />}
            <Outlet />
          </Box>
        )}
      </AppShell.Main>

      <AppShell.Footer>
        <Footer>
          <Footer.Copyright>© {new Date().getFullYear()} The ThunderID Authors</Footer.Copyright>
          <Footer.Divider />
          <Footer.Version>{`${VERSION}`}</Footer.Version>
        </Footer>
      </AppShell.Footer>
    </AppShell>
  );
}
