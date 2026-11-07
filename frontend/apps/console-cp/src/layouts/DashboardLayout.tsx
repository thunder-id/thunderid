// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getInitials} from '@thunderid/components';
import {
  gatewayColor,
  gatewaySectionOf,
  useDeployRoutes,
  usePipeline,
  useProjectChanges,
  type PipelineStage,
} from '@thunderid/configure-gateways';
import {ProjectSwitcher, useProjectSelection} from '@thunderid/configure-projects';
import {ProjectScopeProvider, useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {SignOutButton, User, useThunderID} from '@thunderid/react';
import {
  AppShell,
  Box,
  Button,
  Chip,
  ColorSchemeImage,
  ColorSchemeToggle,
  Footer,
  Header,
  Sidebar,
  useAppShell,
  useSidebar,
  UserMenu,
} from '@wso2/oxygen-ui';
import {
  Bot,
  Building,
  FolderKanban,
  Group,
  ArrowLeft,
  Circle,
  Compass,
  Globe,
  History,
  Home,
  KeyRound,
  IdCard,
  Languages,
  Layers,
  LayoutGrid,
  LogOut,
  Mail,
  MessageSquare,
  Network,
  Palette,
  Server,
  Settings,
  ShieldCheck,
  SquareArrowRightEnter,
  UserRoundCog,
  UsersRound,
  Wallet,
  Workflow,
} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as NavigateLink, Outlet, useLocation, useMatch, useNavigate} from 'react-router';
import GatewaySwitcher from './components/GatewaySwitcher';
import SignInOnUnauthorized from '../components/SignInOnUnauthorized';
import RouteConfig, {ROUTE_SEGMENTS} from '../configs/RouteConfig';

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
  children?: NavRoute[];
  /** A short note beside the entry, such as how many of its changes are not deployed yet. */
  badge?: string;
  /** Done as well as going to the entry's page, such as leaving the selected project. */
  onSelect?: () => void;
}

interface NavCategory {
  category?: string;
  routes: NavRoute[];
}

function SidebarFooterButtons(): ReactNode {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {collapsed} = useSidebar();

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
  const {projects, selectedProject: project, clearSelection} = useProjectSelection();
  // How many of the project's own applications and agents the first gateway does not run yet.
  const undeployed = useProjectChanges();
  // On one of a gateway's pages, the navigation is the gateway's.
  const gatewayMatch = useMatch(`/${ROUTE_SEGMENTS.deployTargets}/:gatewayId/*`);
  const gatewayId = gatewayMatch?.params.gatewayId;
  const pipeline = usePipeline();
  const deployRoutes = useDeployRoutes();
  const stages: PipelineStage[] = pipeline.data?.stages ?? [];
  const gatewayIndex = stages.findIndex((stage: PipelineStage) => stage.gateway.id === gatewayId);
  const gateway = gatewayIndex < 0 ? undefined : stages[gatewayIndex];
  const {isTrustedIssuerGenericOidc, getTrustedIssuerClientId, getClientUrl} = useConfig();
  const {t} = useTranslation();
  const logger = useLogger();

  const handleSignOut = (signOut: () => Promise<void>): void => {
    if (isTrustedIssuerGenericOidc()) {
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
    const applications: NavRoute = {
      id: 'applications',
      text: t('navigation:pages.applications'),
      icon: <LayoutGrid />,
      path: RouteConfig.applications.list(),
    };
    const agents: NavRoute = {
      id: 'agents',
      text: t('navigation:pages.agents', 'Agents'),
      icon: <Bot />,
      path: RouteConfig.agents.list(),
    };
    const resourceServers: NavRoute = {
      id: 'resource-servers',
      text: t('navigation:pages.resourceServers', 'Resource servers'),
      icon: <Server />,
      path: RouteConfig.resourceServers.list(),
    };
    const roles: NavRoute = {
      id: 'roles',
      text: t('navigation:pages.roles'),
      icon: <ShieldCheck />,
      path: RouteConfig.roles.list(),
    };
    const badgeOf = (count: number | undefined): string | undefined =>
      count ? t('navigation:badges.undeployed', '{{count}} undeployed', {count}) : undefined;
    // On a gateway's pages the navigation is the gateway's, the same from wherever it was opened: a
    // gateway belongs to the organization. The way back leads to the project it was opened from, if any.
    if (gateway && gatewayId) {
      const section = (name: 'overview' | 'values' | 'endpoints' | 'history' | 'explore'): string =>
        gatewaySectionOf(deployRoutes, gatewayId, undefined, name);
      const color = gatewayColor(gatewayIndex);
      return [
        {
          routes: [
            project
              ? {
                  id: 'gateway-back',
                  text: project.displayName,
                  icon: <ArrowLeft />,
                  path: RouteConfig.projects.home(project.id),
                }
              : {
                  id: 'gateway-back',
                  text: t('navigation:pages.overview', 'Overview'),
                  icon: <ArrowLeft />,
                  path: RouteConfig.home.list(),
                },
          ],
        },
        {
          category: t('navigation:categories.gateway', 'Gateway: {{name}}', {name: gateway.name}),
          routes: [
            {
              id: 'gateway-overview',
              text: t('navigation:pages.overview', 'Overview'),
              icon: <Circle fill={color.light} stroke={color.light} />,
              path: section('overview'),
            },
          ],
        },
        {
          category: t('navigation:categories.configure'),
          routes: [
            {
              id: 'gateway-values',
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
              text: t('navigation:pages.deployedConfiguration', 'Deployed configuration'),
              icon: <Compass />,
              path: section('explore'),
            },
            {
              id: 'gateway-history',
              text: t('navigation:pages.deploymentHistory', 'Deployment history'),
              icon: <History />,
              path: section('history'),
            },
          ],
        },
      ];
    }
    if (project) {
      return [
        {
          routes: [
            {
              id: 'organization',
              text: t('navigation:pages.organizationSettings', 'Organization settings'),
              icon: <ArrowLeft />,
              path: RouteConfig.home.list(),
              onSelect: clearSelection,
            },
          ],
        },
        {
          category: t('navigation:categories.project', 'Project: {{name}}', {name: project.displayName}),
          routes: [
            {
              id: 'project-gateways',
              text: t('navigation:pages.deploymentPipeline', 'Deployment pipeline'),
              icon: <FolderKanban />,
              path: RouteConfig.projects.home(project.id),
            },
          ],
        },
        {
          category: t('navigation:categories.build', 'Build'),
          routes: [
            {...applications, badge: badgeOf(undeployed.application)},
            {...agents, badge: badgeOf(undeployed.agent)},
            {...resourceServers, badge: badgeOf(undeployed.resource_server)},
            {...roles, badge: badgeOf(undeployed.role)},
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
        category: t('navigation:categories.administration', 'Administration'),
        routes: [
          {
            id: 'users',
            text: t('navigation:pages.administrators', 'Administrators'),
            icon: <UsersRound />,
            path: RouteConfig.users.list(),
          },
          {
            id: 'gateways',
            text: t('navigation:pages.gateways', 'Deployment'),
            icon: <Network />,
            path: RouteConfig.gateways.list(),
          },
        ],
      },
      {
        category: t('navigation:categories.identitySetup', 'Identity setup'),
        routes: [
          {
            id: 'groups',
            text: t('navigation:pages.groups'),
            icon: <Group />,
            path: RouteConfig.groups.list(),
          },
          {
            id: 'user-types',
            text: t('navigation:pages.userTypes'),
            icon: <UserRoundCog />,
            path: RouteConfig.userTypes.list(),
          },
          roles,
          {
            id: 'organization-units',
            text: t('navigation:pages.organizationUnits'),
            icon: <Building />,
            path: RouteConfig.organizationUnits.list(),
          },
        ],
      },
      {
        category: t('navigation:categories.signInExperience', 'Sign-in experience'),
        routes: [
          {
            id: 'flows',
            text: t('navigation:pages.flows'),
            icon: <Workflow />,
            path: RouteConfig.flows.list(),
          },
          {
            id: 'connections',
            text: t('navigation:pages.connections'),
            icon: <Layers />,
            path: RouteConfig.connections.list(),
          },
          {
            id: 'design',
            text: t('navigation:pages.design', 'Design'),
            icon: <Palette />,
            path: RouteConfig.design.list(),
          },
          {
            id: 'notifications',
            text: t('navigation:pages.notifications', 'Notifications'),
            icon: <Mail />,
            children: [
              {
                id: 'email-templates',
                text: t('navigation:pages.emailTemplates', 'Email templates'),
                icon: <Mail />,
                path: RouteConfig.notificationTemplates.channel('email'),
              },
              {
                id: 'sms-templates',
                text: t('navigation:pages.smsTemplates', 'SMS templates'),
                icon: <MessageSquare />,
                path: RouteConfig.notificationTemplates.channel('sms'),
              },
            ],
          },
          {
            id: 'translations',
            text: t('navigation:pages.translations'),
            icon: <Languages />,
            path: RouteConfig.translations.list(),
          },
        ],
      },
      {
        category: t('navigation:categories.apisAndCredentials', 'APIs and credentials'),
        routes: [
          resourceServers,
          {
            id: 'verifiable-credentials',
            text: t('navigation:pages.verifiableCredentials'),
            icon: <Wallet />,
            children: [
              {
                id: 'credentials',
                text: t('navigation:pages.credentials'),
                icon: <IdCard />,
                path: RouteConfig.verifiableCredentials.list(),
              },
              {
                id: 'presentations',
                text: t('navigation:pages.presentations'),
                icon: <ShieldCheck />,
                path: RouteConfig.verifiablePresentations.list(),
              },
            ],
          },
        ],
      },
      {
        category: t('navigation:categories.system', 'System'),
        routes: [
          {
            id: 'settings',
            text: t('navigation:pages.settings', 'Settings'),
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
    undeployed.role,
    gateway,
    gatewayId,
    gatewayIndex,
    deployRoutes,
    clearSelection,
  ]);

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
          {projects.length > 0 && (
            <Header.Switchers showDivider={false}>
              <ProjectSwitcher />
            </Header.Switchers>
          )}
          <Header.Spacer />
          <Header.Actions>
            {/* Switching gateways is offered on a gateway's own pages only. */}
            {gateway && <GatewaySwitcher />}
            {gateway && (
              <Chip
                size="small"
                variant="outlined"
                label={t('common:header.readOnly', 'Configuration read-only')}
                sx={{mr: 1}}
                data-testid="header-read-only"
              />
            )}
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
            {appRoutes.map((categoryGroup) => (
              <Sidebar.Category key={categoryGroup.category}>
                {categoryGroup.category && <Sidebar.CategoryLabel>{categoryGroup.category}</Sidebar.CategoryLabel>}
                {categoryGroup.routes.map((route) =>
                  route.children ? (
                    <Sidebar.Item key={route.id} id={route.id}>
                      <Sidebar.ItemIcon>{route.icon}</Sidebar.ItemIcon>
                      <Sidebar.ItemLabel>{route.text}</Sidebar.ItemLabel>
                      {route.children.map((child) => (
                        <Sidebar.Item key={child.id} id={child.id} link={<NavigateLink to={child.path ?? ''} />}>
                          <Sidebar.ItemIcon>{child.icon}</Sidebar.ItemIcon>
                          <Sidebar.ItemLabel>{child.text}</Sidebar.ItemLabel>
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
                      {/* With a badge the label keeps its whole width and the badge follows it. */}
                      <Sidebar.ItemLabel sx={route.badge ? {flex: '0 0 auto', overflow: 'visible'} : undefined}>
                        {route.text}
                      </Sidebar.ItemLabel>
                      {route.badge && (
                        <Sidebar.ItemBadge color="warning" sx={{ml: 1, flexShrink: 0, '& .MuiChip-label': {px: 0.75}}}>
                          {route.badge}
                        </Sidebar.ItemBadge>
                      )}
                    </Sidebar.Item>
                  ),
                )}
              </Sidebar.Category>
            ))}
          </Sidebar.Nav>
          <Sidebar.Footer>
            <SidebarFooterButtons />
          </Sidebar.Footer>
        </Sidebar>
      </AppShell.Sidebar>

      <AppShell.Main>
        <SignInOnUnauthorized />
        <Outlet />
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
