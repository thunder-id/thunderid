// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Attribution, SignOutButton, User, useThunderID} from '@thunderid/react';
import {Box, ButtonBase, ColorSchemeToggle, Stack, UserMenu} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, Outlet, useLocation} from 'react-router';
import AppBrand from '../components/AppBrand';
import RouteConfig, {MANAGED_RESOURCES, type ManagedResource} from '../configs/RouteConfig';
import useVisibleResources from '../hooks/useVisibleResources';

/**
 * The widest the pages grow; wider windows center them.
 */
const CONTENT_MAX_WIDTH = 1240;

/**
 * The Admin Console shell: a two-row top bar, with the brand and the organization unit the admin
 * manages beside the user menu, above a row of tabs for the pages the admin may see. The pages sit
 * in a centered frame.
 */
export default function AdminLayout(): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger();
  const {pathname} = useLocation();
  const {clearSession, discovery} = useThunderID();
  const {isTrustedIssuerGenericOidc, getTrustedIssuerClientId, getClientUrl} = useConfig();
  const {visible} = useVisibleResources();
  const gutterSx = {px: {xs: 2, md: 4}, width: '100%'};

  const activeResource: ManagedResource | undefined = MANAGED_RESOURCES.find((resource: ManagedResource) =>
    pathname.startsWith(RouteConfig[resource].list()),
  );

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

  // A tab of the top bar's second row, underlined in the primary color when current.
  const tab = (resource: ManagedResource): JSX.Element => {
    const selected: boolean = resource === activeResource;

    return (
      <ButtonBase
        key={resource}
        component={Link}
        to={RouteConfig[resource].list()}
        aria-current={selected ? 'page' : undefined}
        sx={{
          display: 'flex',
          alignItems: 'center',
          px: 0.5,
          mx: 1.5,
          pt: 1,
          pb: 1.25,
          mb: '-1px',
          fontSize: 15,
          fontWeight: selected ? 600 : 400,
          whiteSpace: 'nowrap',
          color: selected ? 'text.primary' : 'text.secondary',
          borderBottom: 2,
          borderColor: selected ? 'primary.main' : 'transparent',
          transition: 'color 0.15s, border-color 0.15s',
          '&:hover': {color: 'text.primary'},
        }}
      >
        {t(`manage:nav.${resource}`, resource)}
      </ButtonBase>
    );
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column', bgcolor: 'background.default'}}>
      <Box component="header" sx={{bgcolor: 'background.paper', borderBottom: 1, borderColor: 'divider', mb: 4}}>
        <Box sx={gutterSx}>
          <Stack direction="row" alignItems="center" spacing={3} sx={{py: 1.75}}>
            <AppBrand />
            <Box sx={{flex: 1}} />
            <Stack direction="row" alignItems="center" spacing={1.5} sx={{flexShrink: 0}}>
              <Box
                sx={{
                  display: 'grid',
                  placeItems: 'center',
                  width: 44,
                  height: 44,
                  borderRadius: '50%',
                  border: 1,
                  borderColor: 'divider',
                  bgcolor: 'background.paper',
                }}
              >
                <ColorSchemeToggle />
              </Box>
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
                            label={t('manage:userMenu.signOut', 'Sign out')}
                            onClick={() => handleSignOut(signOut)}
                          />
                        )}
                      </SignOutButton>
                    </UserMenu>
                  );
                }}
              </User>
            </Stack>
          </Stack>
          {visible.length > 0 && (
            <Box
              component="nav"
              aria-label={t('manage:nav.label', 'Admin Console')}
              sx={{display: 'flex', overflowX: 'auto', ml: -1.5}}
            >
              {visible.map(tab)}
            </Box>
          )}
        </Box>
      </Box>
      <Box component="main" sx={{...gutterSx, maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', flex: 1, pt: 1, pb: 4}}>
        <Outlet />
      </Box>
      <Box component="footer" sx={{...gutterSx, display: 'flex', justifyContent: 'center', py: 3}}>
        <Attribution />
      </Box>
    </Box>
  );
}
