// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Logo, useThunderID} from '@thunderid/react';
import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {Building2} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useOrganizationName from '../hooks/useOrganizationName';

/**
 * Height of the logo in the top bar. The SDK's logo sizes are for sign-in cards.
 */
const LOGO_HEIGHT = 26;

/**
 * The top bar's brand: the logo of the application the admin signed in with, the app's name, and the
 * organization unit the admin manages.
 */
export default function AppBrand(): JSX.Element {
  const {t} = useTranslation();
  const {meta} = useThunderID();
  const organization: string | undefined = useOrganizationName();
  const title: string = t('manage:title', 'Admin');
  const logoUrl: string | undefined = meta?.application?.logoUrl;

  return (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{flexShrink: 0, minWidth: 0}}>
      {logoUrl && (
        <Box
          sx={{
            display: 'flex',
            flexShrink: 0,
            '& img': {height: LOGO_HEIGHT, width: 'auto', maxWidth: 'none'},
            '& span[role="img"]': {width: 'auto', height: LOGO_HEIGHT, fontSize: LOGO_HEIGHT * 0.85},
          }}
        >
          <Logo src={logoUrl} alt={title} size="small" />
        </Box>
      )}
      {/* The app's name is the page's one top-level heading; each page's own title follows it. */}
      <Typography component="h1" noWrap sx={{fontSize: 16, fontWeight: 600}}>
        {title}
      </Typography>
      {organization && (
        <Stack
          direction="row"
          alignItems="center"
          spacing={1.5}
          aria-label={t('manage:organizationUnit.label', 'Organization unit')}
          sx={{minWidth: 0}}
        >
          <Typography color="text.disabled" aria-hidden>
            /
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1} sx={{minWidth: 0, color: 'text.primary'}}>
            <Building2 size={16} strokeWidth={1.75} aria-hidden />
            <Typography noWrap sx={{fontSize: 15, fontWeight: 500}}>
              {organization}
            </Typography>
            <Box
              component="span"
              sx={{
                px: 0.75,
                py: 0.25,
                borderRadius: 1,
                bgcolor: 'action.hover',
                color: 'text.secondary',
                fontSize: 11,
                fontWeight: 600,
                letterSpacing: '0.04em',
              }}
            >
              {t('manage:organizationUnit.badge', 'OU')}
            </Box>
          </Stack>
        </Stack>
      )}
    </Stack>
  );
}
