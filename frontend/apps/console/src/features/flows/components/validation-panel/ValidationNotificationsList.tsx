/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
            <Typography variant="body2" sx={{flex: 1, textAlign: 'left'}}>
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
                  opacity: 0,
                  transition: 'opacity 0.15s ease',
                }}
              >
                <ArrowRight size={14} />
              </Box>
            )}
          </>
        );

        const rowSx = {
          display: 'flex',
          alignItems: 'flex-start',
          gap: 1,
          width: '100%',
          px: 1.5,
          py: 1.25,
          borderRadius: 1.5,
          border: '1px solid',
          borderColor: 'divider',
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
                bgcolor: 'action.hover',
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
