// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Attribution, SignOutButton, User} from '@thunderid/react';
import {AppBrand} from '@thunderid/system-apps';
import {Box, ButtonBase, ColorSchemeToggle, Stack, UserMenu} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, Outlet, useLocation} from 'react-router';
import DESTINATION_ICONS from '../components/destinationIcons';
import RouteConfig, {type AccountDestination} from '../configs/RouteConfig';
import useAccountAppConfig from '../hooks/useAccountAppConfig';

// A raised white surface on the soft page background, used for the navigation pill, the round icon
// buttons and the page panel.
const SURFACE_SX = {
  bgcolor: 'background.paper',
  border: 1,
  borderColor: 'divider',
} as const;

/**
 * The Account App shell. The top bar carries the brand, the navigation and the user menu; with the
 * side bar layout the navigation moves beside the pages instead. A contained app keeps everything in
 * one centered frame; a full-width app spans the window.
 */
export default function AccountLayout(): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger();
  const {pathname} = useLocation();
  const {config, visiblePages} = useAccountAppConfig();
  const isSideBar: boolean = config.navigation.layout === 'SIDE_BAR';
  const isContained: boolean = config.content?.width !== 'FULL';
  const frameSx = {
    px: {xs: 2, md: 4},
    width: '100%',
    ...(isContained ? {maxWidth: 1240, mx: 'auto'} : {}),
  };
  const destinations: AccountDestination[] = ['home', ...visiblePages];
  const active: AccountDestination =
    visiblePages.find((page: AccountDestination) => pathname.startsWith(RouteConfig[page]())) ?? 'home';

  const navItem = (destination: AccountDestination): JSX.Element => {
    const selected: boolean = destination === active;

    return (
      <ButtonBase
        key={destination}
        component={Link}
        to={RouteConfig[destination]()}
        aria-current={selected ? 'page' : undefined}
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-start',
          gap: 1.25,
          px: 2.25,
          minHeight: 44,
          borderRadius: 999,
          fontSize: 14,
          fontWeight: selected ? 600 : 500,
          whiteSpace: 'nowrap',
          color: selected ? 'primary.contrastText' : 'text.primary',
          bgcolor: selected ? 'primary.main' : 'transparent',
          transition: 'background-color 0.15s',
          '&:hover': {bgcolor: selected ? 'primary.main' : 'action.hover'},
          '& svg': {width: 18, height: 18},
        }}
      >
        {DESTINATION_ICONS[destination]}
        {t(`account:nav.${destination}`, destination)}
      </ButtonBase>
    );
  };

  const brand: JSX.Element = <AppBrand config={config} name={t('account:title', 'Account')} />;

  const actions: JSX.Element = (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{flexShrink: 0}}>
      <Box sx={{...SURFACE_SX, borderRadius: '50%', display: 'grid', placeItems: 'center', width: 44, height: 44}}>
        <ColorSchemeToggle />
      </Box>
      <User>
        {(user) => {
          const avatar = typeof user?.picture === 'string' ? user.picture : undefined;

          return (
            <UserMenu>
              <UserMenu.Trigger name={String(user?.name ?? '')} avatar={avatar} />
              <UserMenu.Header name={String(user?.name ?? '')} email={String(user?.email ?? '')} avatar={avatar} />
              <UserMenu.Divider />
              <SignOutButton>
                {({signOut}) => (
                  <UserMenu.Logout
                    label={t('account:userMenu.signOut', 'Sign out')}
                    onClick={() => {
                      signOut().catch((error: unknown) => logger.error('Sign out failed', {error}));
                    }}
                  />
                )}
              </SignOutButton>
            </UserMenu>
          );
        }}
      </User>
    </Stack>
  );

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column', bgcolor: 'background.default'}}>
      <Box component="header" sx={{...frameSx, py: 2.5}}>
        <Stack direction="row" alignItems="center" spacing={3}>
          {brand}
          <Box sx={{flex: 1, display: 'flex', justifyContent: 'center', minWidth: 0}}>
            {!isSideBar && (
              <Box
                component="nav"
                aria-label={t('account:nav.label', 'Account')}
                sx={{
                  ...SURFACE_SX,
                  display: 'flex',
                  gap: 0.5,
                  p: 0.75,
                  borderRadius: 999,
                  overflowX: 'auto',
                  maxWidth: '100%',
                }}
              >
                {destinations.map(navItem)}
              </Box>
            )}
          </Box>
          {actions}
        </Stack>
      </Box>
      <Box sx={{...frameSx, flex: 1, display: 'flex', gap: 4, alignItems: 'flex-start', pt: 1, pb: 4}}>
        {isSideBar && (
          <Box
            component="nav"
            aria-label={t('account:nav.label', 'Account')}
            sx={{
              width: 240,
              flexShrink: 0,
              display: {xs: 'none', md: 'flex'},
              flexDirection: 'column',
              gap: 0.5,
              position: 'sticky',
              top: 24,
            }}
          >
            {destinations.map(navItem)}
          </Box>
        )}
        {/* Without a side bar beside them, pages keep a readable width in the middle. */}
        <Box component="main" sx={{flex: 1, minWidth: 0, ...(isSideBar ? {} : {maxWidth: 960, mx: 'auto'})}}>
          <Outlet />
        </Box>
      </Box>
      {config.branding.showSecuredBy && (
        <Box component="footer" sx={{...frameSx, display: 'flex', justifyContent: 'center', py: 3}}>
          <Attribution />
        </Box>
      )}
    </Box>
  );
}
