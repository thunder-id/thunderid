// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getInitials} from '@thunderid/components';
import {Avatar, Box, Button, Chip, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {ExternalLink, Server} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useAdministrators, {type Administrator} from '../api/useAdministrators';
import useDeployRoutes, {userRoutesOf} from '../hooks/useDeployRoutes';

/** How many administrators the panel lists before the rest are on the Administrators page. */
const LISTED = 3;

export interface GatewayAdministrationProps {
  /** The gateway's tag, such as DEV. */
  tag: string;
  /** Where the gateway's admin portal is, if it has one. */
  adminConsoleUrl?: string;
}

/** The host an address names, or the address itself when it names none. */
function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function nameOf(user: Administrator): string {
  const given = user.attributes?.['name'] ?? user.attributes?.['username'];
  return user.display ?? (typeof given === 'string' ? given : user.id);
}

function emailOf(user: Administrator): string | undefined {
  const email = user.attributes?.['email'];
  return typeof email === 'string' ? email : undefined;
}

/**
 * Where a gateway's own resources are managed: its admin portal, with what lives there, beside the
 * administrators who can sign in to it.
 */
export default function GatewayAdministration({
  tag,
  adminConsoleUrl = undefined,
}: GatewayAdministrationProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const users = userRoutesOf(useDeployRoutes());
  const administrators = useAdministrators(LISTED);
  const listed = administrators.data?.users ?? [];
  const kinds = [
    t('gateway.portalUsers', 'Users'),
    t('gateway.portalGroups', 'Groups'),
    t('gateway.portalAgents', 'Agents'),
    t('gateway.portalRoles', 'Roles'),
  ];

  return (
    <Box
      component="section"
      aria-label={t('gateway.manageRegion', 'Manage this gateway')}
      data-testid="gateway-administration"
      sx={{
        display: 'grid',
        gridTemplateColumns: {xs: '1fr', md: 'repeat(auto-fit, minmax(340px, 1fr))'},
        borderRadius: '14px',
        border: 1,
        borderColor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.3)',
        bgcolor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.07)',
        overflow: 'hidden',
      }}
    >
      <Stack spacing={1.75} sx={{p: 3}} data-testid="gateway-admin-portal">
        <Stack direction="row" spacing={1.75} alignItems="center">
          <Box
            sx={{
              width: 48,
              height: 48,
              borderRadius: '12px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flex: '0 0 auto',
              color: 'primary.main',
              bgcolor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.14)',
            }}
            aria-hidden
          >
            <Server size={24} />
          </Box>
          <Box sx={{minWidth: 0}}>
            <Typography variant="subtitle1" component="h2" fontWeight={600}>
              {t('gateway.adminPortalTitle', '{{tag}} admin portal', {tag})}
            </Typography>
            {adminConsoleUrl && (
              <Typography sx={{fontFamily: 'monospace', fontSize: 12}} color="text.secondary" noWrap>
                {hostOf(adminConsoleUrl)}
              </Typography>
            )}
          </Box>
        </Stack>
        <Typography variant="body2" color="text.secondary">
          {t(
            'gateway.adminPortalLead',
            'Manage the resources that live in this gateway, such as its users, groups, agents and roles.',
          )}
        </Typography>
        <Stack direction="row" spacing={0.75} sx={{flexWrap: 'wrap', rowGap: 0.75}}>
          {kinds.map((kind) => (
            <Chip key={kind} size="small" label={kind} />
          ))}
        </Stack>
        {adminConsoleUrl && (
          <Button
            variant="contained"
            href={adminConsoleUrl}
            target="_blank"
            rel="noopener noreferrer"
            endIcon={<ExternalLink size={14} aria-hidden />}
            sx={{alignSelf: 'flex-start', borderRadius: 100, mt: 'auto'}}
            data-testid="gateway-open-admin-portal"
          >
            {t('gateway.openAdminPortal', 'Open admin portal')}
            <Box
              component="span"
              sx={{position: 'absolute', width: 1, height: 1, overflow: 'hidden', clip: 'rect(0 0 0 0)'}}
            >
              {t('gateway.opensInNewTab', ' (opens in a new tab)')}
            </Box>
          </Button>
        )}
      </Stack>
      <Stack
        spacing={0.75}
        sx={{
          px: 3,
          py: 2.5,
          borderLeft: {md: 1},
          borderTop: {xs: 1, md: 0},
          borderColor: {xs: 'divider', md: 'divider'},
        }}
        data-testid="gateway-administrators"
      >
        <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={1.5}>
          <Typography variant="subtitle1" component="h2" fontWeight={600}>
            {t('gateway.administrators', 'Administrators')}{' '}
            {administrators.data && (
              <Typography component="span" color="text.secondary" fontWeight={400}>
                {administrators.data.totalResults}
              </Typography>
            )}
          </Typography>
          <Button size="small" variant="text" component={Link} to={users.list()}>
            {t('gateway.viewAllAdministrators', 'View all')}
          </Button>
        </Stack>
        <Typography variant="body2" color="text.secondary">
          {t('gateway.administratorsLead', 'They can sign in to the {{tag}} admin portal', {tag})}
        </Typography>
        {administrators.isLoading && <CircularProgress size={20} />}
        {administrators.isError && (
          <Typography variant="body2" color="text.secondary">
            {t('gateway.administratorsFailed', 'The administrators could not be read.')}
          </Typography>
        )}
        <Box component="ul" sx={{m: 0, mt: 0.5, p: 0, listStyle: 'none'}}>
          {listed.map((user: Administrator, index: number) => {
            const name = nameOf(user);
            const email = emailOf(user);
            return (
              <Stack
                component="li"
                key={user.id}
                direction="row"
                alignItems="center"
                spacing={1.5}
                sx={{py: 1.25, borderTop: index > 0 ? 1 : 0, borderColor: 'divider'}}
                data-testid={`gateway-administrator-${user.id}`}
              >
                <Avatar sx={{width: 32, height: 32, fontSize: 12, fontWeight: 600}} aria-hidden>
                  {getInitials(name)}
                </Avatar>
                <Box sx={{flex: 1, minWidth: 0}}>
                  <Typography variant="body2" fontWeight={500} noWrap>
                    {name}
                  </Typography>
                  {email && (
                    <Typography variant="caption" color="text.secondary" component="div" noWrap>
                      {email}
                    </Typography>
                  )}
                </Box>
                <Chip size="small" label={t('gateway.administratorRole', 'Admin')} />
                <Button
                  size="small"
                  variant="text"
                  component={Link}
                  to={users.detail(user.id)}
                  aria-label={t('gateway.viewAdministrator', 'View details for {{name}}', {name})}
                  sx={{whiteSpace: 'nowrap'}}
                >
                  {t('gateway.viewDetails', 'View details')}
                </Button>
              </Stack>
            );
          })}
        </Box>
      </Stack>
    </Box>
  );
}
