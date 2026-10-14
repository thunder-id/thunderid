// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Accordion, AccordionDetails, Box, Button, ButtonBase, Link, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, Monitor, Smartphone} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import LiveDuration from './LiveDuration';
import PlatformIcon from './PlatformIcon';
import SessionApplications from './SessionApplications';
import useNow from '../hooks/useNow';
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
 * When the session was last active, relative to now, such as "5 minutes ago". It is its own component so
 * that its once-a-minute refresh re-renders only this line.
 */
function LastActive({session, showUser}: {session: Session; showUser: boolean}): JSX.Element {
  const {t, i18n} = useTranslation('sessions');
  const now = useNow(60 * 1000);
  const time = formatRelativeTime(session.lastActiveAt, i18n.language, new Date(now));

  return (
    <Typography variant="caption" color="text.secondary" component="div" noWrap>
      {showUser
        ? t('item.userLastActive', '{{user}}, latest SSO {{time}}', {
            user: session.userName ?? session.userId,
            time,
          })
        : t('item.lastActive', 'Latest SSO {{time}}', {time})}
    </Typography>
  );
}

/**
 * One session as an expandable card: the device, when it was last active and the action to revoke it,
 * and when expanded, everything recorded about it.
 */
export default function SessionItem({session, expanded, onToggle, showUser, onRevoke}: SessionItemProps): JSX.Element {
  const {t} = useTranslation('sessions');
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
  const userName = session.userName ?? session.userId;

  const plain = (value: string | undefined): ReactNode => (
    <Typography variant="body2">{value ?? notRecorded}</Typography>
  );
  const dateTime = (value: string | undefined): ReactNode => plain(value && new Date(value).toLocaleString());
  // The date and time, with a live count of the time since or until it below.
  const liveDateTime = (value: string | undefined, direction: 'until' | 'since'): ReactNode =>
    value ? (
      <>
        {dateTime(value)}
        <LiveDuration value={value} direction={direction} />
      </>
    ) : (
      plain(value)
    );
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
        // Unmounted while collapsed, so only an open session's live times keep ticking.
        slotProps={{transition: {unmountOnExit: true}}}
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
                <LastActive session={session} showUser={showUser} />
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
              {session.participants.length > 0 ? (
                <SessionApplications participants={session.participants} size={28} />
              ) : (
                plain(t('details.noApplications', 'None'))
              )}
            </DetailField>
            <DetailField label={t('details.signedIn', 'Signed in')}>
              {liveDateTime(session.createdAt, 'since')}
            </DetailField>
            <DetailField label={t('details.lastActive', 'Latest SSO')}>
              {liveDateTime(session.lastActiveAt, 'since')}
            </DetailField>
            {showAuthenticatedAt && (
              <DetailField label={t('details.lastAuthenticated', 'Last entered credentials')}>
                {dateTime(session.authenticatedAt)}
              </DetailField>
            )}
            <DetailField label={t('details.idleExpiresAt', 'Ends if unused at')}>
              {liveDateTime(session.idleExpiresAt, 'until')}
            </DetailField>
            <DetailField label={t('details.absoluteExpiresAt', 'Ends at the latest at')}>
              {liveDateTime(session.absoluteExpiresAt, 'until')}
            </DetailField>
          </Box>
        </AccordionDetails>
      </Accordion>
    </Box>
  );
}
