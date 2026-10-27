// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {Logo, User, useThunderID} from '@thunderid/react';
import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {Building2} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AppConfig} from '../models/app-config';

/**
 * Props for the {@link AppBrand} component.
 *
 * @public
 */
export interface AppBrandProps {
  config: Pick<AppConfig, 'branding'>;
  /**
   * The app's name, shown when the Console set no title, such as `Admin` or `Account`.
   */
  name: string;
  /**
   * The height of the logo, in pixels.
   *
   * @defaultValue 26
   */
  height?: number;
}

/**
 * The brand for an app's header: the customer's logo, never ThunderID's, then the app's name and the
 * organization unit of the signed-in user. The logo is the one set in the app's runtime configuration
 * (`brand.logo` in `config.js`), else the one in the app's configuration, else the logo of the
 * application the app signs in with, from the flow metadata when the SDK is configured with that
 * application's ID. An image keeps its own width; a generated
 * `avatar:` logo is drawn as its image and an `emoji:` logo as its glyph.
 *
 * @public
 */
export default function AppBrand({config, name, height = 26}: AppBrandProps): JSX.Element {
  const {t} = useTranslation('apps');
  const {config: runtimeConfig} = useConfig();
  const {meta} = useThunderID();
  const title: string = config.branding.title ?? name;
  // The logo set in the app's runtime configuration, then the app's own, then the logo of the
  // application it signs in with.
  const logoUrl: string | undefined = [
    runtimeConfig?.brand?.logo,
    config.branding.logoUrl,
    meta?.application?.logoUrl,
  ].find((candidate: string | undefined): candidate is string => Boolean(candidate));

  return (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{flexShrink: 0, minWidth: 0}}>
      {logoUrl && (
        // The SDK's logo sizes are for sign-in cards; the header keeps it to its own height.
        <Box
          sx={{
            display: 'flex',
            flexShrink: 0,
            '& img': {height, width: 'auto', maxWidth: 'none'},
            '& span[role="img"]': {width: 'auto', height, fontSize: height * 0.85},
          }}
        >
          <Logo src={logoUrl} alt={title} size="small" />
        </Box>
      )}
      <Typography noWrap sx={{fontSize: 16, fontWeight: 600}}>
        {title}
      </Typography>
      <User>
        {(user) => {
          const {ouHandle, ouId, ouName} = (user ?? {}) as {ouHandle?: unknown; ouId?: unknown; ouName?: unknown};
          // The friendliest label the token carries: the organization unit's name, then its handle.
          const organization: string | undefined = [ouName, ouHandle, ouId].find(
            (value: unknown): value is string => typeof value === 'string' && value.length > 0,
          );

          return organization ? (
            <Stack
              direction="row"
              alignItems="center"
              spacing={1.5}
              aria-label={t('brand.organizationUnit.label', 'Organization unit')}
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
                  {t('brand.organizationUnit.badge', 'OU')}
                </Box>
              </Stack>
            </Stack>
          ) : null;
        }}
      </User>
    </Stack>
  );
}
