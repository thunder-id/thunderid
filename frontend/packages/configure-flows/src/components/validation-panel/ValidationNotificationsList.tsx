// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ButtonBase, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, CircleCheckIcon, CircleXIcon, InfoIcon, TriangleAlertIcon} from '@wso2/oxygen-ui-icons-react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import Notification, {NotificationType} from '../../models/notification';

/**
 * Props interface of {@link ValidationNotificationsList}
 */
export interface ValidationNotificationsListPropsInterface {
  /**
   * Array of notifications to display.
   */
  notifications: Notification[];
  /**
   * Message to display when no notifications are available.
   */
  emptyMessage: string;
  /**
   * Callback fired when a notification is clicked.
   */
  onNotificationClick: (notification: Notification) => void;
}

const severityIcon = (type: NotificationType): ReactElement => {
  switch (type) {
    case NotificationType.ERROR:
      return <CircleXIcon size={16} />;
    case NotificationType.WARNING:
      return <TriangleAlertIcon size={16} />;
    default:
      return <InfoIcon size={16} />;
  }
};

/**
 * Component to render a list of validation notifications. Notifications wired to
 * a resource render as clickable rows (styled like the flow preview's option
 * rows) that navigate straight to the offending resource.
 *
 * @param props - Props injected to the component.
 * @returns The ValidationNotificationsList component.
 */
function ValidationNotificationsList({
  notifications,
  emptyMessage,
  onNotificationClick,
}: ValidationNotificationsListPropsInterface): ReactElement {
  const {t} = useTranslation();

  if (!notifications || notifications.length === 0) {
    return (
      <Stack alignItems="center" justifyContent="center" gap={1} minHeight="200px" sx={{color: 'text.secondary'}}>
        <Box sx={{color: 'success.main', display: 'inline-flex'}}>
          <CircleCheckIcon size={24} />
        </Box>
        <Typography variant="body2" color="textSecondary">
          {emptyMessage}
        </Typography>
      </Stack>
    );
  }

  return (
    <Stack gap={1}>
      {notifications.map((notification: Notification) => {
        const type = notification.getType();
        const isNavigable = notification.hasResources() || notification.hasPanelNotification();

        const content = (
          <>
            <Box sx={{color: `${type}.main`, display: 'inline-flex', flexShrink: 0, mt: '2px'}}>
              {severityIcon(type)}
            </Box>
            <Typography
              variant="body2"
              sx={{
                flex: 1,
                textAlign: 'left',
                lineHeight: 1.5,
                // Messages name the offending resource in a <code> tag. Left
                // unstyled it reads as raw monospace mid-sentence; as a chip it
                // scans as an identifier, and breaking on long ids keeps it
                // inside the panel.
                '& code': {
                  fontFamily: 'monospace',
                  fontSize: '0.8125em',
                  px: 0.5,
                  py: '1px',
                  borderRadius: 0.5,
                  bgcolor: 'action.hover',
                  wordBreak: 'break-all',
                },
              }}
            >
              {notification.getMessage()}
            </Typography>
            {isNavigable && (
              <Box
                className="notification-open-icon"
                sx={{
                  display: 'inline-flex',
                  flexShrink: 0,
                  alignSelf: 'center',
                  color: `${type}.main`,
                  // Kept faintly visible rather than hover-only, so the row
                  // advertises that it navigates to the resource.
                  opacity: 0.45,
                  transition: 'opacity 0.15s ease',
                }}
              >
                <ArrowRight size={14} />
              </Box>
            )}
          </>
        );

        // Severity is otherwise carried only by a small icon, which makes a
        // mixed list hard to scan. A tint and a matching border give each row
        // its severity at a glance without shouting.
        const rowSx = {
          display: 'flex',
          alignItems: 'flex-start',
          gap: 1.25,
          width: '100%',
          px: 1.5,
          py: 1.25,
          borderRadius: 1.5,
          border: '1px solid',
          borderColor: `color-mix(in srgb, var(--oxygen-palette-${type}-main) 28%, transparent)`,
          bgcolor: `color-mix(in srgb, var(--oxygen-palette-${type}-main) 7%, transparent)`,
        } as const;

        if (!isNavigable) {
          return (
            <Box key={notification.getId()} className="notification-item" sx={rowSx}>
              {content}
            </Box>
          );
        }

        return (
          <ButtonBase
            key={notification.getId()}
            className="notification-item"
            onClick={() => onNotificationClick(notification)}
            aria-label={t('common:show')}
            sx={{
              ...rowSx,
              '&:hover': {
                borderColor: `${type}.main`,
                bgcolor: `color-mix(in srgb, var(--oxygen-palette-${type}-main) 14%, transparent)`,
                '& .notification-open-icon': {opacity: 1},
              },
            }}
          >
            {content}
          </ButtonBase>
        );
      })}
    </Stack>
  );
}

export default ValidationNotificationsList;
