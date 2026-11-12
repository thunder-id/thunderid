// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesErrorNotice} from '@thunderid/components';
import {useCapabilitiesContext, useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {SignOutButton, User, useThunderID} from '@thunderid/react';
import {AppShell, Box, ColorSchemeImage, ColorSchemeToggle, Footer, Header, Sidebar, UserMenu} from '@wso2/oxygen-ui';
import {Bot, Building2, Group, LogOut, ShieldCheck, UsersRound} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as NavigateLink, Outlet, useLocation} from 'react-router';
import RouteConfig, {MANAGED_RESOURCES, type ManagedResource} from '../configs/RouteConfig';
import useOrganizationName from '../hooks/useOrganizationName';
import useVisibleResources from '../hooks/useVisibleResources';

const RESOURCE_LABELS: Record<ManagedResource, string> = {
  agents: 'Agents',
  groups: 'Groups',
  roles: 'Roles',
  users: 'Users',
};

const RESOURCE_ICONS: Record<ManagedResource, JSX.Element> = {
  agents: <Bot />,
  groups: <Group />,
  roles: <ShieldCheck />,
  users: <UsersRound />,
};

/**
 * The Admin Console shell, laid out like the Control Plane console: a top bar with the brand and the
 * user menu, a sidebar with the pages the admin may see, and a footer.
 */
export default function AdminLayout(): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger();
  const {pathname} = useLocation();
  const {clearSession, discovery} = useThunderID();
  const {isTrustedIssuerGenericOidc, getTrustedIssuerClientId, getClientUrl} = useConfig();
  const {visible} = useVisibleResources();
  const {error: capabilitiesError} = useCapabilitiesContext();
  const organization: string | undefined = useOrganizationName();
  const basePath: string = import.meta.env.BASE_URL.replace(/\/$/, '');

  // A generic OIDC trusted issuer is signed out of through its own end-session endpoint, since the
  // SDK's sign-out only knows ThunderID's.
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

  const activeItem: ManagedResource | undefined = useMemo(
    () => MANAGED_RESOURCES.find((resource: ManagedResource) => pathname.startsWith(RouteConfig[resource].list())),
    [pathname],
  );

  return (
    <AppShell>
      <AppShell.Navbar>
        <Header>
          <Header.Toggle />
          <Header.Brand>
            <Header.BrandLogo>
              <ColorSchemeImage
                src={{light: `${basePath}/assets/images/logo.svg`, dark: `${basePath}/assets/images/logo-inverted.svg`}}
                alt={{light: 'Logo (Light)', dark: 'Logo (Dark)'}}
                height={27}
                width="auto"
                alignItems="center"
                marginBottom="3px"
              />
            </Header.BrandLogo>
            <Header.BrandTitle>{'Admin Console'}</Header.BrandTitle>
          </Header.Brand>
          {organization && (
            <Box sx={{alignItems: 'center', color: 'text.secondary', display: 'flex', gap: 1, ml: 2, minWidth: 0}}>
              <Building2 size={16} strokeWidth={1.75} aria-hidden />
              <Box
                component="span"
                sx={{fontSize: 14, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}
              >
                {organization}
              </Box>
            </Box>
          )}
          <Header.Spacer />
          <Header.Actions>
            <ColorSchemeToggle />
            <User>
              {(user) => {
                const avatar: string | undefined = typeof user?.picture === 'string' ? user.picture : undefined;

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
        <Sidebar activeItem={activeItem}>
          <Sidebar.Nav>
            <Sidebar.Category>
              {visible.map((resource: ManagedResource) => (
                <Sidebar.Item key={resource} id={resource} link={<NavigateLink to={RouteConfig[resource].list()} />}>
                  <Sidebar.ItemIcon>{RESOURCE_ICONS[resource]}</Sidebar.ItemIcon>
                  <Sidebar.ItemLabel>{t(`navigation:pages.${resource}`, RESOURCE_LABELS[resource])}</Sidebar.ItemLabel>
                </Sidebar.Item>
              ))}
            </Sidebar.Category>
          </Sidebar.Nav>
        </Sidebar>
      </AppShell.Sidebar>

      <AppShell.Main>{capabilitiesError ? <CapabilitiesErrorNotice /> : <Outlet />}</AppShell.Main>

      <AppShell.Footer>
        <Footer>
          <Footer.Copyright>© {new Date().getFullYear()} The ThunderID Authors</Footer.Copyright>
        </Footer>
      </AppShell.Footer>
    </AppShell>
  );
}
