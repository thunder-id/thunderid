// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Attribution, SignOutButton, ThemeContext, User} from '@thunderid/react';
import {AppBrand, type AppLink} from '@thunderid/system-apps';
import {Box, ButtonBase, Link, Paper, Stack, UserMenu, type Theme} from '@wso2/oxygen-ui';
import {useContext, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink, Outlet, useLocation} from 'react-router';
import DESTINATION_ICONS from '../components/destinationIcons';
import LanguageMenu from '../components/LanguageMenu';
import RouteConfig, {type AccountDestination} from '../configs/RouteConfig';
import useAccountAppConfig from '../hooks/useAccountAppConfig';
import useAppLanguage from '../hooks/useAppLanguage';

/** The width a contained app centers its bars and pages in. */
const CONTAINED_WIDTH = 1240;

/** The height of the logo in the top bar. */
const LOGO_HEIGHT = 28;

/** The name of the CSS custom property a `var(--name)` reference reads. */
const variableName = (reference: string | undefined): string | undefined =>
  /^var\((--[^,)]+)/.exec(reference ?? '')?.[1];

/** A primary-colored tint of the theme, for the current navigation entry. */
const primaryTint = (theme: Theme): string => `color-mix(in srgb, ${theme.palette.primary.main} 12%, transparent)`;

/**
 * The dark band at the foot of every page: the links the Console's MyAccount builder sets, and the
 * "Secured by ThunderID" credit when it is on. The credit reads the SDK's text colors, which are set
 * light here so it reads on the dark band.
 */
function AccountFooter({
  frameSx,
  links,
  showSecuredBy,
}: {
  frameSx: Record<string, unknown>;
  links: AppLink[];
  showSecuredBy: boolean;
}): JSX.Element {
  const {t} = useTranslation();
  const sdkVars = useContext(ThemeContext)?.theme.vars;
  const textVariables: string[] = [sdkVars?.colors.text.primary, sdkVars?.colors.text.secondary]
    .map(variableName)
    .filter((name: string | undefined): name is string => Boolean(name));

  return (
    <Box
      component="footer"
      sx={(theme: Theme) => ({
        bgcolor: 'grey.900',
        color: 'grey.300',
        ...Object.fromEntries(textVariables.map((name: string) => [name, theme.palette.grey[300]])),
      })}
    >
      <Stack
        direction={{xs: 'column', sm: 'row'}}
        alignItems={{xs: 'flex-start', sm: 'center'}}
        justifyContent="space-between"
        spacing={2}
        sx={{...frameSx, py: 3}}
      >
        {links.length > 0 ? (
          <Stack
            component="nav"
            aria-label={t('account:footer.links', 'Footer links')}
            direction="row"
            flexWrap="wrap"
            useFlexGap
            spacing={3}
          >
            {links.map((link: AppLink) => (
              <Link
                key={`${link.label}-${link.url}`}
                href={link.url}
                target="_blank"
                rel="noopener noreferrer"
                underline="hover"
                sx={{color: 'common.white', fontSize: 14}}
              >
                {link.label}
              </Link>
            ))}
          </Stack>
        ) : (
          <span />
        )}
        {showSecuredBy && <Attribution />}
      </Stack>
    </Box>
  );
}

/**
 * The MyAccount shell, a self-care portal: a white top bar with the brand on the left and the
 * user's menu on the right, the pages in white cards on the theme's soft page background, and a dark
 * footer. With the top bar layout the navigation runs as tabs under the brand; with the side bar
 * layout it sits in a card beside the pages. A contained app keeps everything in one centered
 * column; a full-width app spans the window.
 */
export default function AccountLayout(): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger();
  const {pathname} = useLocation();
  const {config, visiblePages} = useAccountAppConfig();
  useAppLanguage(config.languages);
  const isSideBar: boolean = config.navigation.layout === 'SIDE_BAR';
  const isContained: boolean = config.content?.width !== 'FULL';
  const frameSx = {
    px: {xs: 2, md: 4},
    width: '100%',
    ...(isContained ? {maxWidth: CONTAINED_WIDTH, mx: 'auto'} : {}),
  };
  const links: AppLink[] = config.branding.links ?? [];
  const offeredLanguages: string[] = config.languages?.menu ? (config.languages.offered ?? []) : [];
  const destinations: AccountDestination[] = ['home', ...visiblePages];
  const active: AccountDestination =
    visiblePages.find((page: AccountDestination) => pathname.startsWith(RouteConfig[page]())) ?? 'home';
  const navLabel: string = t('account:nav.label', 'MyAccount');

  // A tab under the brand, underlined in the primary color when current.
  const tab = (destination: AccountDestination): JSX.Element => {
    const selected: boolean = destination === active;

    return (
      <ButtonBase
        key={destination}
        component={RouterLink}
        to={RouteConfig[destination]()}
        aria-current={selected ? 'page' : undefined}
        sx={{
          gap: 1,
          px: 0.5,
          py: 1.5,
          fontSize: 15,
          fontWeight: selected ? 600 : 500,
          whiteSpace: 'nowrap',
          color: selected ? 'primary.main' : 'text.secondary',
          borderBottom: 2,
          borderColor: selected ? 'primary.main' : 'transparent',
          '&:hover': {color: selected ? 'primary.main' : 'text.primary'},
          '& svg': {width: 18, height: 18},
        }}
      >
        {DESTINATION_ICONS[destination]}
        {t(`account:nav.${destination}`, destination)}
      </ButtonBase>
    );
  };

  // An entry of the side bar card, tinted in the primary color when current.
  const sideItem = (destination: AccountDestination): JSX.Element => {
    const selected: boolean = destination === active;

    return (
      <ButtonBase
        key={destination}
        component={RouterLink}
        to={RouteConfig[destination]()}
        aria-current={selected ? 'page' : undefined}
        sx={{
          justifyContent: 'flex-start',
          gap: 1.5,
          px: 2,
          minHeight: 44,
          borderRadius: 2,
          fontSize: 15,
          fontWeight: selected ? 600 : 500,
          whiteSpace: 'nowrap',
          color: selected ? 'primary.main' : 'text.primary',
          bgcolor: selected ? primaryTint : 'transparent',
          '&:hover': {bgcolor: selected ? primaryTint : 'action.hover'},
          '& svg': {width: 20, height: 20},
        }}
      >
        {DESTINATION_ICONS[destination]}
        {t(`account:nav.${destination}`, destination)}
      </ButtonBase>
    );
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column', bgcolor: 'background.default'}}>
      <Box
        component="header"
        sx={{
          bgcolor: 'background.paper',
          borderBottom: 1,
          borderColor: 'divider',
          position: 'sticky',
          top: 0,
          zIndex: 'appBar',
        }}
      >
        <Box sx={frameSx}>
          <Stack direction="row" alignItems="center" spacing={2} sx={{minHeight: 64}}>
            <AppBrand
              config={config}
              name={t('account:title', 'MyAccount')}
              height={LOGO_HEIGHT}
              showOrganization={false}
            />
            <Box sx={{flex: 1}} />
            {offeredLanguages.length > 1 && <LanguageMenu offered={offeredLanguages} />}
            <User>
              {(user) => {
                const avatar = typeof user?.picture === 'string' ? user.picture : undefined;

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
          {!isSideBar && (
            <Box component="nav" aria-label={navLabel} sx={{display: 'flex', gap: 3, overflowX: 'auto', mb: '-1px'}}>
              {destinations.map(tab)}
            </Box>
          )}
        </Box>
      </Box>
      <Box
        sx={{
          ...frameSx,
          flex: 1,
          display: 'flex',
          flexDirection: {xs: 'column', md: 'row'},
          gap: {xs: 2, md: 4},
          alignItems: {xs: 'stretch', md: 'flex-start'},
          py: {xs: 3, md: 5},
        }}
      >
        {isSideBar && (
          <Paper
            component="nav"
            variant="outlined"
            aria-label={navLabel}
            sx={{
              width: {xs: '100%', md: 260},
              flexShrink: 0,
              display: 'flex',
              flexDirection: {xs: 'row', md: 'column'},
              gap: 0.5,
              p: 1.5,
              borderRadius: 3,
              overflowX: 'auto',
              position: {md: 'sticky'},
              top: {md: 96},
            }}
          >
            {destinations.map(sideItem)}
          </Paper>
        )}
        {/* Without a side bar beside them, pages keep a readable width in the middle. */}
        <Box
          component="main"
          sx={{flex: 1, minWidth: 0, ...(isSideBar ? {} : {maxWidth: 960, mx: 'auto', width: '100%'})}}
        >
          <Outlet />
        </Box>
      </Box>
      {(config.branding.showSecuredBy || links.length > 0) && (
        <AccountFooter frameSx={frameSx} links={links} showSecuredBy={config.branding.showSecuredBy} />
      )}
    </Box>
  );
}
