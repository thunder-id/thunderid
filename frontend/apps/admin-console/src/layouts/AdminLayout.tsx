// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Attribution, SignOutButton, User, useThunderID} from '@thunderid/react';
import {resolveAppImage, type AppLink} from '@thunderid/system-apps';
import {
  AppShell,
  Box,
  ColorSchemeImage,
  ColorSchemeToggle,
  Footer,
  Header,
  Sidebar,
  UserMenu,
  useColorScheme,
} from '@wso2/oxygen-ui';
import {Bot, Building2, Group, LogOut, ShieldCheck, UsersRound} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as NavigateLink, Outlet, useLocation} from 'react-router';
import LanguageMenu from '../components/LanguageMenu';
import RouteConfig, {MANAGED_RESOURCES, type ManagedResource} from '../configs/RouteConfig';
import useAdminAppConfig from '../hooks/useAdminAppConfig';
import useAppLanguage from '../hooks/useAppLanguage';
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
 *
 * The Admin App configuration the Console's builder saves decides which pages show and in what order
 * (through {@link useVisibleResources}), the brand's logo and title, the language menu, and the
 * footer's links and "Secured by" attribution.
 */
export default function AdminLayout(): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger();
  const {pathname} = useLocation();
  const {clearSession, discovery} = useThunderID();
  const {config: runtimeConfig, isTrustedIssuerGenericOidc, getTrustedIssuerClientId, getClientUrl} = useConfig();
  const {visible} = useVisibleResources();
  const {config} = useAdminAppConfig();
  useAppLanguage(config.languages);
  const {meta} = useThunderID();
  const {mode, systemMode} = useColorScheme();
  const offeredLanguages: string[] = config.languages?.offered ?? [];
  const links: AppLink[] = config.branding.links ?? [];
  const title: string = config.branding.title ?? 'Admin Console';
  // The logo this deployment's config.js sets, else the Admin App configuration's for the color
  // scheme shown, else that of the application the admin signed in with, else the app's own.
  const configuredLogo: string | undefined =
    runtimeConfig?.brand?.logo ??
    resolveAppImage(config.branding.logo, (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light') ??
    meta?.application?.logoUrl;
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
              {configuredLogo ? (
                <Box component="img" src={configuredLogo} alt={title} sx={{height: 27, width: 'auto', display: 'block'}} />
              ) : (
                <ColorSchemeImage
                  src={{light: `${basePath}/assets/images/logo.svg`, dark: `${basePath}/assets/images/logo-inverted.svg`}}
                  alt={{light: 'Logo (Light)', dark: 'Logo (Dark)'}}
                  height={27}
                  width="auto"
                  alignItems="center"
                  marginBottom="3px"
                />
              )}
            </Header.BrandLogo>
            <Header.BrandTitle>{title}</Header.BrandTitle>
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
            {config.languages?.menu && offeredLanguages.length > 0 && <LanguageMenu offered={offeredLanguages} />}
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

      <AppShell.Main>
        <Outlet />
      </AppShell.Main>

      <AppShell.Footer>
        <Footer>
          <Footer.Copyright>© {new Date().getFullYear()} The ThunderID Authors</Footer.Copyright>
          {links.map((link: AppLink) => (
            <Footer.Link key={`${link.label}-${link.url}`} href={link.url} target="_blank" rel="noopener noreferrer">
              {link.label}
            </Footer.Link>
          ))}
        </Footer>
        {/* The footer keeps only its own parts, so the attribution sits on a row of its own under it. */}
        {config.branding.showSecuredBy && (
          <Box sx={{bgcolor: 'background.paper', display: 'flex', justifyContent: 'center', pb: 2}}>
            <Attribution />
          </Box>
        )}
      </AppShell.Footer>
    </AppShell>
  );
}
