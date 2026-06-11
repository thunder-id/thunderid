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
  Download,
  FolderOpen,
  Group,
  Home,
  Languages,
  Layers,
  LayoutGrid,
  Palette,
  Server,
  ShieldCheck,
  UserRoundCog,
  UsersRound,
  Workflow,
} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as NavigateLink, Outlet, useNavigate} from 'react-router';

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
        aria-label={t('navigation:pages.openProject')}
        startIcon={<FolderOpen size={18} />}
        onClick={() => void navigate('/open-project')}
        sx={ICON_BUTTON_SX}
      />
      <Button
        variant="outlined"
        startIcon={<Download size={18} />}
        onClick={() => void navigate('/export')}
        sx={buttonSx}
      >
        {!collapsed && t('navigation:pages.export', 'Export Config')}
      </Button>
    </Box>
  );
}

export default function DashboardLayout(): ReactNode {
  const {signIn, clearSession, discovery} = useThunderID();
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

    signOut()
      .then(() => signIn())
      .catch((error: unknown) => {
        logger.error('Sign out/in failed', {error});
      });
  };

  const appRoutes = useMemo(
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
            id: 'integrations',
            text: t('navigation:pages.integrations'),
            icon: <Layers />,
            path: '/integrations',
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
    ],
    [t],
  );

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
        <Sidebar>
          <Sidebar.Nav>
            {appRoutes.map((categoryGroup) => (
              <Sidebar.Category key={categoryGroup.category}>
                {categoryGroup.category && <Sidebar.CategoryLabel>{categoryGroup.category}</Sidebar.CategoryLabel>}
                {categoryGroup.routes.map((route) => (
                  <Sidebar.Item key={route.id} id={route.id} link={<NavigateLink to={route.path} />}>
                    <Sidebar.ItemIcon>{route.icon}</Sidebar.ItemIcon>
                    <Sidebar.ItemLabel>{route.text}</Sidebar.ItemLabel>
                  </Sidebar.Item>
                ))}
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
