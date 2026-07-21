/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {SignOutButton, User, useThunderID} from '@thunderid/react';
import {
  AppShell,
  Box,
  Button,
  ColorSchemeImage,
  ColorSchemeToggle,
  Divider,
  Footer,
  Header,
  Sidebar,
  useSidebar,
  UserMenu,
} from '@wso2/oxygen-ui';
import {
  Bot,
  Building,
  Group,
  Home,
  IdCard,
  Languages,
  Layers,
  LayoutGrid,
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
import {Link as NavigateLink, Outlet, useLocation, useNavigate} from 'react-router';

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
        onClick={() => void navigate('/import-export')}
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

export default function DashboardLayout({collapseSidebar = false}: DashboardLayoutProps): ReactNode {
  const {clearSession, discovery} = useThunderID();
  const {isTrustedIssuerGenericOidc, getTrustedIssuerClientId, getClientUrl} = useConfig();
  const {t} = useTranslation();
  const logger = useLogger();
  const navigate = useNavigate();

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

    signOut().catch((error: unknown) => {
      logger.error('Sign out failed', {error});
    });
  };

  const appRoutes: NavCategory[] = useMemo(
    () => [
      {
        routes: [
          {
            id: 'home',
            text: t('navigation:pages.home'),
            icon: <Home />,
            path: '/home',
          },
        ],
      },
      {
        category: t('navigation:categories.resources'),
        routes: [
          {
            id: 'applications',
            text: t('navigation:pages.applications'),
            icon: <LayoutGrid />,
            path: '/applications',
          },
          {
            id: 'resource-servers',
            text: t('navigation:pages.resourceServers', 'Resource Servers'),
            icon: <Server size={16} />,
            path: '/resource-servers',
          },
        ],
      },
      {
        category: t('navigation:categories.identities'),
        routes: [
          {
            id: 'users',
            text: t('navigation:pages.users'),
            icon: <UsersRound />,
            path: '/users',
          },
          {
            id: 'agents',
            text: t('navigation:pages.agents', 'Agents'),
            icon: <Bot />,
            path: '/agents',
          },
          {
            id: 'groups',
            text: t('navigation:pages.groups'),
            icon: <Group />,
            path: '/groups',
          },
          {
            id: 'roles',
            text: t('navigation:pages.roles'),
            icon: <ShieldCheck />,
            path: '/roles',
          },
          {
            id: 'user-types',
            text: t('navigation:pages.userTypes'),
            icon: <UserRoundCog />,
            path: '/user-types',
          },
        ],
      },
      {
        category: t('navigation:categories.configure'),
        routes: [
          {
            id: 'organization-units',
            text: t('navigation:pages.organizationUnits'),
            icon: <Building />,
            path: '/organization-units',
          },
          {
            id: 'flows',
            text: t('navigation:pages.flows'),
            icon: <Workflow />,
            path: '/flows',
          },
          {
            id: 'connections',
            text: t('navigation:pages.connections'),
            icon: <Layers />,
            path: '/connections',
          },
          {
            id: 'verifiable-credentials',
            text: t('navigation:pages.verifiableCredentials'),
            icon: <Wallet />,
            children: [
              {
                id: 'credentials',
                text: t('navigation:pages.credentials'),
                icon: <IdCard />,
                path: '/verifiable-credentials',
              },
              {
                id: 'presentations',
                text: t('navigation:pages.presentations'),
                icon: <ShieldCheck />,
                path: '/verifiable-presentations',
              },
            ],
          },
        ],
      },
      {
        category: t('navigation:categories.customize'),
        routes: [
          {
            id: 'design',
            text: t('navigation:pages.design', 'Design'),
            icon: <Palette size={16} />,
            path: '/design',
          },
          {
            id: 'translations',
            text: t('navigation:pages.translations'),
            icon: <Languages size={16} />,
            path: '/translations',
          },
        ],
      },
      {
        category: t('navigation:categories.system'),
        routes: [
          {
            id: 'settings',
            text: t('navigation:pages.settings'),
            icon: <Settings size={16} />,
            path: '/settings',
          },
        ],
      },
    ],
    [t],
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
    <AppShell>
      <AppShell.Navbar>
        <Header>
          <Header.Toggle />
          <Header.Brand>
            <Header.BrandLogo>
              <ColorSchemeImage
                src={{
                  light: `${import.meta.env.BASE_URL}/assets/images/logo.svg`,
                  dark: `${import.meta.env.BASE_URL}/assets/images/logo-inverted.svg`,
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
          <Header.Spacer />
          <Header.Actions>
            <ColorSchemeToggle />
            <Divider orientation="vertical" flexItem sx={{mx: 1, display: {xs: 'none', sm: 'block'}}} />
            <User>
              {(user) => (
                <UserMenu>
                  <UserMenu.Trigger name={String(user?.name ?? '')} showName />
                  <UserMenu.Header name={String(user?.name ?? '')} email={String(user?.email ?? '')} />
                  <UserMenu.Divider />
                  <UserMenu.Item
                    label={t('common:userMenu.welcome')}
                    onClick={() => {
                      void navigate('/welcome');
                    }}
                  />
                  <UserMenu.Divider />
                  <SignOutButton>
                    {({signOut}) => (
                      <UserMenu.Logout label={t('common:userMenu.signOut')} onClick={() => handleSignOut(signOut)} />
                    )}
                  </SignOutButton>
                </UserMenu>
              )}
            </User>
          </Header.Actions>
        </Header>
      </AppShell.Navbar>

      <AppShell.Sidebar>
        <Sidebar
          activeItem={activeItem}
          expandedMenus={expandedMenus}
          onToggleExpand={handleToggleExpand}
          collapsed={collapseSidebar}
        >
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
                    <Sidebar.Item key={route.id} id={route.id} link={<NavigateLink to={route.path ?? ''} />}>
                      <Sidebar.ItemIcon>{route.icon}</Sidebar.ItemIcon>
                      <Sidebar.ItemLabel>{route.text}</Sidebar.ItemLabel>
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
        <Outlet />
      </AppShell.Main>

      <AppShell.Footer>
        <Footer>
          <Footer.Copyright>© {new Date().getFullYear()} WSO2 LLC.</Footer.Copyright>
          <Footer.Divider />
          <Footer.Version>{`${VERSION}`}</Footer.Version>
        </Footer>
      </AppShell.Footer>
    </AppShell>
  );
}
