// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Attribution, SignOutButton, User} from '@thunderid/react';
import {AppBrand} from '@thunderid/system-apps';
import {Box, ButtonBase, ColorSchemeToggle, Stack, UserMenu} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, Outlet, useLocation} from 'react-router';
import RESOURCE_ICONS from '../components/resourceIcons';
import RouteConfig, {MANAGED_RESOURCES, type ManagedResource} from '../configs/RouteConfig';
import useAdminAppConfig from '../hooks/useAdminAppConfig';

// A raised white surface on the soft page background, used for the navigation pill and the round
// icon buttons.
const SURFACE_SX = {
  bgcolor: 'background.paper',
  border: 1,
  borderColor: 'divider',
} as const;

/**
 * The Admin App shell: a two-row top bar, with the brand, the organization the admin manages and the
 * user menu above a row of tabs for the pages that are on. With the side bar layout the
 * pages move beside the content instead. A contained app keeps everything in one centered frame; a full-width app spans the window.
 */
export default function AdminLayout(): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger();
  const {pathname} = useLocation();
  const {config, visibleResources} = useAdminAppConfig();
  const isSideBar: boolean = config.navigation.layout === 'SIDE_BAR';
  // The top bar and footer span the window, like a product's chrome; the pages sit in the frame.
  const headerSx = {px: {xs: 2, md: 4}, width: '100%'};
  const frameSx = {
    px: {xs: 2, md: 4},
    width: '100%',
    ...(config.content?.width === 'CONTAINED' ? {maxWidth: 1240, mx: 'auto'} : {}),
  };

  const activeResource: ManagedResource | undefined = MANAGED_RESOURCES.find((resource: ManagedResource) =>
    pathname.startsWith(RouteConfig[resource].list()),
  );

  // The top bar's second row: Ory-style tabs, underlined in the primary color when current.
  const tabItem = (resource: ManagedResource): JSX.Element => {
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

  // The side bar: rounded items, the current one filled.
  const navItem = (resource: ManagedResource): JSX.Element => {
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
        }}
      >
        {RESOURCE_ICONS[resource]}
        {t(`manage:nav.${resource}`, resource)}
      </ButtonBase>
    );
  };

  const brand: JSX.Element = <AppBrand config={config} name={t('manage:title', 'Admin')} />;

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
                    label={t('manage:userMenu.signOut', 'Sign out')}
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
      <Box component="header" sx={{bgcolor: 'background.paper', borderBottom: 1, borderColor: 'divider', mb: 4}}>
        <Box sx={headerSx}>
          <Stack direction="row" alignItems="center" spacing={3} sx={{py: 1.75}}>
            {brand}
            <Box sx={{flex: 1}} />
            {actions}
          </Stack>
          {!isSideBar && visibleResources.length > 0 && (
            <Box
              component="nav"
              aria-label={t('manage:nav.label', 'Admin App')}
              sx={{display: 'flex', overflowX: 'auto', ml: -1.5}}
            >
              {visibleResources.map(tabItem)}
            </Box>
          )}
        </Box>
      </Box>
      <Box sx={{...frameSx, flex: 1, display: 'flex', gap: 4, alignItems: 'flex-start', pt: 1, pb: 4}}>
        {isSideBar && visibleResources.length > 0 && (
          <Box
            component="nav"
            aria-label={t('manage:nav.label', 'Admin App')}
            sx={{
              width: 220,
              flexShrink: 0,
              display: {xs: 'none', md: 'flex'},
              flexDirection: 'column',
              gap: 0.5,
              position: 'sticky',
              top: 24,
            }}
          >
            {visibleResources.map(navItem)}
          </Box>
        )}
        <Box component="main" sx={{flex: 1, minWidth: 0}}>
          <Outlet />
        </Box>
      </Box>
      {config.branding.showSecuredBy && (
        <Box component="footer" sx={{...headerSx, display: 'flex', justifyContent: 'center', py: 3}}>
          <Attribution />
        </Box>
      )}
    </Box>
  );
}
