// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {formatRelativeTime} from '@thunderid/utils';
import {Accordion, AccordionDetails, Box, ButtonBase, Link, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, Smartphone} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import PlatformIcon from './PlatformIcon';
import SessionApplications from './SessionApplications';
import useNow from '../hooks/useNow';
import useSessionRoutes from '../hooks/useSessionRoutes';
import type {Session} from '../models/session';
import describeUserAgent from '../utils/describeUserAgent';

export interface SessionItemProps {
  session: Session;
  expanded: boolean;
  onToggle: () => void;
  /** Whether to name the session's user, as on an application's panel. */
  showUser: boolean;
}

function DetailField({label, children}: {label: string; children: ReactNode}): JSX.Element {
  return (
    <Box sx={{minWidth: 0, wordBreak: 'break-word'}}>
      <Typography variant="caption" color="text.secondary" component="div" sx={{mb: 0.5}}>
        {label}
      </Typography>
      {children}
    </Box>
  );
}

/**
 * A tooltip with a timestamp in UTC to the second, for an admin matching the session against logs or other
 * systems. The time takes focus, so a keyboard user can open the tooltip too.
 */
function UtcTooltip({value, children}: {value: string; children: JSX.Element}): JSX.Element {
  const {i18n} = useTranslation('sessions');
  const utc = new Date(value).toLocaleString(i18n.language, {dateStyle: 'medium', timeStyle: 'long', timeZone: 'UTC'});

  // Describes the time rather than naming it, so a screen reader still reads the time shown.
  return (
    <Tooltip title={utc} describeChild>
      <Box component="time" dateTime={value} tabIndex={0} sx={{display: 'block', width: 'fit-content'}}>
        {children}
      </Box>
    </Tooltip>
  );
}

/**
 * The operating system's logo in a circle, with the browser's logo as a badge on its corner, the same as the
 * SDKs' session list. A native app's session has no browser, so it has no badge.
 */
function DeviceAvatar({
  os = undefined,
  browser = undefined,
  app = false,
}: {
  os?: string;
  browser?: string;
  app?: boolean;
}): JSX.Element {
  return (
    <Box sx={{position: 'relative', flexShrink: 0, width: 40, height: 40}}>
      <Box
        title={os}
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: 40,
          height: 40,
          border: 1,
          borderColor: 'divider',
          borderRadius: '50%',
          bgcolor: 'action.hover',
          color: 'text.primary',
        }}
      >
        <PlatformIcon name={os} kind="os" size={20} />
      </Box>
      {!app && (
        <Box
          title={browser}
          sx={{
            position: 'absolute',
            right: -3,
            bottom: -3,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 20,
            height: 20,
            border: 1,
            borderColor: 'divider',
            borderRadius: '50%',
            bgcolor: 'background.paper',
            color: 'text.primary',
          }}
        >
          <PlatformIcon name={browser} kind="browser" size={12} />
        </Box>
      )}
    </Box>
  );
}

/**
 * When the session was last active, relative to now, such as "5 minutes ago". It is its own component so
 * that its once-a-minute refresh re-renders only this line.
 */
function LastActive({session, showUser}: {session: Session; showUser: boolean}): JSX.Element {
  const {t, i18n} = useTranslation('sessions');
  // Re-renders every minute, so the relative time keeps moving.
  useNow(60 * 1000);
  const time = formatRelativeTime(new Date(session.lastActiveAt), i18n.language);

  // No UTC tooltip here: the line is inside the card's button, so it can't take focus. The details have it.
  return (
    <Typography variant="caption" color="text.secondary" component="div" noWrap>
      {showUser
        ? t('item.userLastActive', '{{user}}, last active {{time}}', {
            user: session.userDisplayName ?? session.userId,
            time,
          })
        : t('item.lastActive', 'Last active {{time}}', {time})}
    </Typography>
  );
}

/**
 * One session as an expandable card: the device and when it was last active, and when expanded, what was
 * recorded about it in two rows: what and when it was last used, then where and when it began and ends.
 */
export default function SessionItem({session, expanded, onToggle, showUser}: SessionItemProps): JSX.Element {
  const {t, i18n} = useTranslation('sessions');
  const routes = useSessionRoutes();
  const notRecorded = t('details.notRecorded', 'Not recorded');
  const {browser, os, device: model, app} = describeUserAgent(session.userAgent);
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
  const userName = session.userDisplayName ?? session.userId;

  const formatDateTime = (value: string): string =>
    new Date(value).toLocaleString(i18n.language, {dateStyle: 'medium', timeStyle: 'short'});
  const plain = (value: string | undefined): ReactNode => (
    <Typography variant="body2">{value ?? notRecorded}</Typography>
  );
  const dateTime = (value: string | undefined): ReactNode =>
    value ? (
      <UtcTooltip value={value}>
        <Typography variant="body2">{formatDateTime(value)}</Typography>
      </UtcTooltip>
    ) : (
      plain(value)
    );
  const note = (text: string): JSX.Element => (
    <Typography variant="caption" color="text.secondary" component="div">
      {text}
    </Typography>
  );
  const platform = (value: string, icon: ReactNode): ReactNode => (
    <Stack direction="row" spacing={1} alignItems="center">
      <Box sx={{display: 'flex', color: 'text.secondary'}}>{icon}</Box>
      <Typography variant="body2">{value}</Typography>
    </Stack>
  );
  // A native app's session has no browser to record, so its operating system and model are the device.
  const deviceParts: {key: string; label: string; icon: ReactNode}[] = [];
  if (os) deviceParts.push({key: 'os', label: os, icon: <PlatformIcon name={os} kind="os" />});
  if (!app && browser) {
    deviceParts.push({key: 'browser', label: browser, icon: <PlatformIcon name={browser} kind="browser" />});
  }
  if (model) deviceParts.push({key: 'model', label: model, icon: <Smartphone size={16} aria-hidden />});
  // Shown only when they differ from the IP address and sign-in time already shown.
  const showCreationIp = session.creationIp !== undefined && session.creationIp !== session.lastActiveIp;
  const showAuthenticatedAt = session.authenticatedAt !== session.createdAt;

  return (
    // The border is on a wrapper because the theme drops borders from a Paper nested in another one.
    <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden'}}>
      <Accordion
        expanded={expanded}
        disableGutters
        square
        // Unmounted while collapsed, so only an open session's details are rendered.
        slotProps={{transition: {unmountOnExit: true}}}
        sx={{bgcolor: 'transparent', boxShadow: 'none', '&:before': {display: 'none'}}}
      >
        <ButtonBase
          onClick={onToggle}
          aria-expanded={expanded}
          sx={{
            width: '100%',
            justifyContent: 'space-between',
            gap: 1,
            textAlign: 'left',
            px: 2,
            py: 1.5,
            '&:hover': {bgcolor: 'action.hover'},
          }}
        >
          <Stack direction="row" spacing={3} alignItems="center" sx={{minWidth: 0}}>
            <DeviceAvatar os={os} browser={browser} app={app} />
            <Box sx={{minWidth: 0}}>
              <Typography variant="subtitle2" noWrap>
                {device}
              </Typography>
              <LastActive session={session} showUser={showUser} />
            </Box>
          </Stack>
          <Box
            aria-hidden
            sx={{
              display: 'flex',
              flexShrink: 0,
              p: 1,
              transition: 'transform 150ms',
              transform: expanded ? 'rotate(180deg)' : 'none',
            }}
          >
            <ChevronDown size={16} />
          </Box>
        </ButtonBase>
        <AccordionDetails sx={{p: 0, borderTop: 1, borderColor: 'divider'}}>
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: {xs: '1fr', sm: 'repeat(3, 1fr)'},
              columnGap: 3,
              rowGap: 2.5,
              p: 2,
            }}
          >
            <DetailField label={app ? t('details.device', 'Device') : t('details.osAndBrowser', 'OS and browser')}>
              {deviceParts.length > 0 ? (
                <Stack direction="row" alignItems="center" flexWrap="wrap" columnGap={1} rowGap={0.5}>
                  {deviceParts.map(({key, label, icon}, index) => (
                    <Stack key={key} direction="row" alignItems="center" columnGap={1}>
                      {index > 0 && (
                        <Typography variant="body2" color="text.disabled" aria-hidden>
                          ·
                        </Typography>
                      )}
                      {platform(label, icon)}
                    </Stack>
                  ))}
                </Stack>
              ) : (
                plain(undefined)
              )}
            </DetailField>
            <DetailField label={t('details.lastActive', 'Last active')}>{dateTime(session.lastActiveAt)}</DetailField>
            <DetailField label={t('details.applications', 'Active applications')}>
              {session.participants.length > 0 ? (
                <SessionApplications participants={session.participants} size={28} />
              ) : (
                plain(t('details.noApplications', 'None'))
              )}
            </DetailField>
            <DetailField label={t('details.lastActiveIp', 'IP address')}>
              {plain(session.lastActiveIp)}
              {showCreationIp && note(t('details.creationIp', 'Signed in from {{ip}}', {ip: session.creationIp}))}
            </DetailField>
            <DetailField label={t('details.signedIn', 'Signed in')}>
              {dateTime(session.createdAt)}
              {showAuthenticatedAt && (
                <UtcTooltip value={session.authenticatedAt}>
                  {note(
                    t('details.lastAuthenticated', 'Last entered credentials {{time}}', {
                      time: formatDateTime(session.authenticatedAt),
                    }),
                  )}
                </UtcTooltip>
              )}
            </DetailField>
            <DetailField label={t('details.sessionEnds', 'Session ends')}>
              {dateTime(session.absoluteExpiresAt)}
            </DetailField>
            {showUser && (
              <DetailField label={t('details.user', 'User')}>
                <Link component={RouterLink} to={routes.users.detail(session.userId)} underline="hover" variant="body2">
                  {userName}
                </Link>
              </DetailField>
            )}
          </Box>
        </AccordionDetails>
      </Accordion>
    </Box>
  );
}
