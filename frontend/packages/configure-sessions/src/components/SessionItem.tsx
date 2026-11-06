// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Accordion, AccordionDetails, Box, Button, ButtonBase, Link, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, Monitor, Smartphone} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import PlatformIcon from './PlatformIcon';
import useSessionRoutes from '../hooks/useSessionRoutes';
import type {Session} from '../models/session';
import describeUserAgent from '../utils/describeUserAgent';
import formatRelativeTime from '../utils/formatRelativeTime';

export interface SessionItemProps {
  session: Session;
  expanded: boolean;
  onToggle: () => void;
  /** Whether to name the session's user, as on an application's panel. */
  showUser: boolean;
  onRevoke: () => void;
}

function DetailField({label, children}: {label: string; children: ReactNode}): JSX.Element {
  return (
    <Box sx={{minWidth: 0, wordBreak: 'break-word'}}>
      <Typography variant="caption" color="text.secondary" component="div" sx={{mb: 0.25}}>
        {label}
      </Typography>
      {children}
    </Box>
  );
}

/**
 * One session as an expandable card: the device, when it was last active and the action to revoke it,
 * and when expanded, everything recorded about it.
 */
export default function SessionItem({session, expanded, onToggle, showUser, onRevoke}: SessionItemProps): JSX.Element {
  const {t, i18n} = useTranslation('sessions');
  const routes = useSessionRoutes();
  const notRecorded = t('details.notRecorded', 'Not recorded');
  const {browser, os, device: model, mobile, app} = describeUserAgent(session.userAgent);
  const DeviceIcon = mobile ? Smartphone : Monitor;
  // A mobile SDK's session is named by its device model alone; its operating system and applications are
  // in the details. One without a model has no browser, so the application stands in for it.
  const client = browser ?? session.participants[0]?.appName;
  let device: string;
  if (model) {
    device = model;
  } else if (client && os) {
    device = t('device.browserOnOs', '{{browser}} on {{os}}', {browser: client, os});
  } else {
    device = client ?? os ?? t('device.unknown', 'Unknown device');
  }
  const lastActive = formatRelativeTime(session.lastActiveAt, i18n.language);
  const userName = session.userName ?? session.userId;

  const plain = (value: string | undefined): ReactNode => (
    <Typography variant="body2">{value ?? notRecorded}</Typography>
  );
  const dateTime = (value: string | undefined): ReactNode => plain(value && new Date(value).toLocaleString());
  // Shown only when they differ from the IP address and sign-in time already shown.
  const showCreationIp = session.creationIp !== undefined && session.creationIp !== session.lastActiveIp;
  const showAuthenticatedAt = session.authenticatedAt !== session.createdAt;

  const withIcon = (value: string | undefined, kind: 'browser' | 'os'): ReactNode =>
    value ? (
      <Stack direction="row" spacing={1} alignItems="center">
        <Box sx={{display: 'flex', color: 'text.secondary'}}>
          <PlatformIcon name={value} kind={kind} />
        </Box>
        <Typography variant="body2">{value}</Typography>
      </Stack>
    ) : (
      plain(value)
    );

  return (
    // The border is on a wrapper because the theme drops borders from a Paper nested in another one.
    <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden'}}>
      <Accordion
        expanded={expanded}
        disableGutters
        square
        sx={{bgcolor: 'transparent', boxShadow: 'none', '&:before': {display: 'none'}}}
      >
        {/* A custom header rather than AccordionSummary, which is itself a button and can't hold another one. */}
        <Stack direction="row" alignItems="center" spacing={1} sx={{px: 2, '&:hover': {bgcolor: 'action.hover'}}}>
          <ButtonBase
            onClick={onToggle}
            aria-expanded={expanded}
            sx={{flex: 1, minWidth: 0, justifyContent: 'flex-start', textAlign: 'left', py: 1.5}}
          >
            <Stack direction="row" spacing={1.5} alignItems="center" sx={{minWidth: 0}}>
              <Box sx={{display: 'flex', color: 'text.secondary'}}>
                <DeviceIcon size={20} aria-hidden />
              </Box>
              <Box sx={{minWidth: 0}}>
                <Typography variant="subtitle2" noWrap>
                  {device}
                </Typography>
                <Typography variant="caption" color="text.secondary" component="div" noWrap>
                  {showUser
                    ? t('item.userLastActive', '{{user}}, last active {{time}}', {user: userName, time: lastActive})
                    : t('item.lastActive', 'Last active {{time}}', {time: lastActive})}
                </Typography>
              </Box>
            </Stack>
          </ButtonBase>
          <Button variant="contained" color="error" size="small" onClick={onRevoke} sx={{flexShrink: 0}}>
            {t('revoke.action', 'Revoke')}
          </Button>
          <Box
            onClick={onToggle}
            aria-hidden
            sx={{
              display: 'flex',
              p: 1,
              cursor: 'pointer',
              transition: 'transform 150ms',
              transform: expanded ? 'rotate(180deg)' : 'none',
            }}
          >
            <ChevronDown size={16} />
          </Box>
        </Stack>
        <AccordionDetails sx={{p: 0, borderTop: 1, borderColor: 'divider'}}>
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(4, 1fr)'},
              gap: 2.5,
              p: 2,
            }}
          >
            {showUser && (
              <DetailField label={t('details.user', 'User')}>
                <Link component={RouterLink} to={routes.users.detail(session.userId)} underline="hover" variant="body2">
                  {userName}
                </Link>
              </DetailField>
            )}
            <DetailField label={t('details.os', 'Operating system')}>{withIcon(os, 'os')}</DetailField>
            {/* A native app's session has no browser to record. */}
            {!app && <DetailField label={t('details.browser', 'Browser')}>{withIcon(browser, 'browser')}</DetailField>}
            {model && <DetailField label={t('details.device', 'Device')}>{plain(model)}</DetailField>}
            <DetailField label={t('details.lastActiveIp', 'IP address')}>{plain(session.lastActiveIp)}</DetailField>
            {showCreationIp && (
              <DetailField label={t('details.creationIp', 'Signed in from IP')}>
                {plain(session.creationIp)}
              </DetailField>
            )}
            <DetailField label={t('details.applications', 'Active applications')}>
              {plain(
                session.participants.length > 0
                  ? session.participants.map((p) => p.appName ?? p.appId).join(', ')
                  : t('details.noApplications', 'None'),
              )}
            </DetailField>
            <DetailField label={t('details.signedIn', 'Signed in')}>{dateTime(session.createdAt)}</DetailField>
            <DetailField label={t('details.lastActive', 'Last active')}>{dateTime(session.lastActiveAt)}</DetailField>
            {showAuthenticatedAt && (
              <DetailField label={t('details.lastAuthenticated', 'Last entered credentials')}>
                {dateTime(session.authenticatedAt)}
              </DetailField>
            )}
            <DetailField label={t('details.idleExpiresAt', 'Ends if unused at')}>
              {dateTime(session.idleExpiresAt)}
            </DetailField>
            <DetailField label={t('details.absoluteExpiresAt', 'Ends at the latest at')}>
              {dateTime(session.absoluteExpiresAt)}
            </DetailField>
          </Box>
        </AccordionDetails>
      </Accordion>
    </Box>
  );
}
