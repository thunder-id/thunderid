// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ChangeCredential, User, UserProfile} from '@thunderid/react';
import {Avatar, Box, CircularProgress, Link, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Navigate, Link as RouterLink} from 'react-router';
import DESTINATION_ICONS from '../components/destinationIcons';
import RouteConfig, {type AccountPage} from '../configs/RouteConfig';
import useAccountAppConfig from '../hooks/useAccountAppConfig';

/**
 * The tint of each page's card badge on the home page.
 */
const CARD_TONES: Record<AccountPage, {background: string; color: string}> = {
  profile: {background: 'rgba(54, 136, 255, 0.14)', color: '#3688FF'},
  security: {background: 'rgba(16, 185, 129, 0.14)', color: '#10b981'},
};

/**
 * The white panel a page's content sits in, on the app's soft background.
 */
function PagePanel({children}: {children: ReactNode}): JSX.Element {
  return (
    <Paper variant="outlined" sx={{borderRadius: 6, px: {xs: 2.5, md: 5}, py: {xs: 3, md: 4.5}}}>
      {children}
    </Paper>
  );
}

function PageHeading({subtitle, title}: {subtitle: string; title: string}): JSX.Element {
  return (
    <Box sx={{mb: 4, textAlign: 'center'}}>
      <Typography variant="h4" component="h1" fontWeight={400} sx={{fontSize: 28}}>
        {title}
      </Typography>
      <Typography color="text.secondary" sx={{mt: 1, fontSize: 15}}>
        {subtitle}
      </Typography>
    </Box>
  );
}

/**
 * Shows a page only while the Console has it on. Otherwise it sends the user home.
 */
function PageGate({children, page}: {children: ReactNode; page: AccountPage}): ReactNode {
  const {t} = useTranslation();
  const {isLoading, visiblePages} = useAccountAppConfig();

  if (isLoading) {
    return (
      <Stack alignItems="center" sx={{py: 8}}>
        <CircularProgress aria-label={t('account:loading', 'Loading')} />
      </Stack>
    );
  }

  if (visiblePages.includes(page)) {
    return children;
  }

  return <Navigate to={RouteConfig.home()} replace />;
}

/**
 * The name to welcome the user by, or `undefined` when the profile has none.
 */
const displayName = (user: Record<string, unknown> | null | undefined): string | undefined =>
  [user?.name, user?.givenName, user?.given_name, user?.username].find(
    (value: unknown): value is string => typeof value === 'string' && value.length > 0,
  );

/**
 * Welcomes the user and offers a card for every page the Console turned on.
 */
export function HomePage(): JSX.Element {
  const {t} = useTranslation();
  const {isLoading, visiblePages} = useAccountAppConfig();

  return (
    <>
      <User>
        {(user) => {
          const profile = user as Record<string, unknown> | null;
          const name: string | undefined = displayName(profile);
          const picture: string | undefined = typeof profile?.picture === 'string' ? profile.picture : undefined;

          return (
            <Stack alignItems="center" spacing={2} sx={{pt: 3, pb: 4, textAlign: 'center'}}>
              <Avatar src={picture} alt={name ?? ''} sx={{width: 96, height: 96, fontSize: 40, bgcolor: '#7cb342'}}>
                {name?.charAt(0).toUpperCase()}
              </Avatar>
              <Typography variant="h4" component="h1" fontWeight={400} sx={{fontSize: 28}}>
                {t('account:home.greeting', 'Welcome, {{name}}', {
                  name: name ?? t('account:home.fallbackName', 'there'),
                })}
              </Typography>
              <Typography color="text.secondary" sx={{fontSize: 15}}>
                {t('account:home.subtitle', 'Manage your info and security across your apps.')}
              </Typography>
            </Stack>
          );
        }}
      </User>
      {isLoading && (
        <Stack alignItems="center">
          <CircularProgress aria-label={t('account:loading', 'Loading')} />
        </Stack>
      )}
      {!isLoading && visiblePages.length === 0 && (
        <Typography color="text.secondary" sx={{textAlign: 'center'}}>
          {t('account:empty.description', 'Your administrator has not turned on any pages for this app.')}
        </Typography>
      )}
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)'}, gap: 3}}>
        {visiblePages.map((page: AccountPage) => (
          <Paper key={page} variant="outlined" sx={{borderRadius: 5, display: 'flex', flexDirection: 'column'}}>
            <Stack direction="row" spacing={2} sx={{p: 3, flex: 1}}>
              <Box sx={{flex: 1}}>
                <Typography component="h2" fontWeight={400} sx={{fontSize: 22, lineHeight: 1.3}}>
                  {t(`account:home.cards.${page}.title`, page)}
                </Typography>
                <Typography color="text.secondary" sx={{mt: 1, fontSize: 14}}>
                  {t(`account:home.cards.${page}.description`, '')}
                </Typography>
              </Box>
              <Box
                aria-hidden
                sx={{
                  width: 72,
                  height: 72,
                  flexShrink: 0,
                  borderRadius: '50%',
                  display: 'grid',
                  placeItems: 'center',
                  bgcolor: CARD_TONES[page].background,
                  color: CARD_TONES[page].color,
                  '& svg': {width: 34, height: 34},
                }}
              >
                {DESTINATION_ICONS[page]}
              </Box>
            </Stack>
            <Box sx={{borderTop: 1, borderColor: 'divider', px: 3, py: 1.75}}>
              <Link
                component={RouterLink}
                to={RouteConfig[page]()}
                underline="hover"
                sx={{fontSize: 14, fontWeight: 500}}
              >
                {t(`account:home.cards.${page}.action`, page)}
              </Link>
            </Box>
          </Paper>
        ))}
      </Box>
    </>
  );
}

/**
 * The signed-in user's profile, editable when the Console allows it.
 */
export function ProfilePage(): JSX.Element {
  const {t} = useTranslation();
  const {page} = useAccountAppConfig();

  return (
    <PageGate page="profile">
      <PageHeading
        title={t('account:nav.profile', 'Personal info')}
        subtitle={t('account:profile.subtitle', 'Manage the basic profile info others may see.')}
      />
      <PagePanel>
        <UserProfile editable={page('profile')?.actions.edit === true} />
      </PagePanel>
    </PageGate>
  );
}

/**
 * A credential the user can change, as a card with the change form open. With only a password to
 * manage today, there is nothing to tuck away.
 */
function CredentialCard({attribute}: {attribute: string}): JSX.Element {
  const {t} = useTranslation();

  return (
    <PagePanel>
      <Typography component="h2" fontWeight={600} sx={{fontSize: 16}}>
        {t(`account:security.${attribute}.title`, attribute)}
      </Typography>
      <Typography color="text.secondary" sx={{fontSize: 13.5, mt: 0.5}}>
        {t(`account:security.${attribute}.description`, '')}
      </Typography>
      <Box sx={{pt: 3, mt: 3, borderTop: 1, borderColor: 'divider'}}>
        <ChangeCredential attribute={attribute} title="" />
      </Box>
    </PagePanel>
  );
}

/**
 * The signed-in user's credentials. The password card is shown when the Console allows it.
 */
export function SecurityPage(): JSX.Element {
  const {t} = useTranslation();
  const {page} = useAccountAppConfig();

  return (
    <PageGate page="security">
      <PageHeading
        title={t('account:nav.security', 'Security')}
        subtitle={t('account:security.subtitle', 'Manage your password and other credentials.')}
      />
      {page('security')?.actions.password === true ? (
        <Stack spacing={2}>
          <CredentialCard attribute="password" />
        </Stack>
      ) : (
        <Typography color="text.secondary">
          {t('account:security.nothing', 'There is nothing you can change here yet.')}
        </Typography>
      )}
    </PageGate>
  );
}
