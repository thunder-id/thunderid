// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Badge, Box, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {BellIcon} from '@wso2/oxygen-ui-icons-react';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useUIPanelState from '../../hooks/useUIPanelState';
import useValidationStatus from '../../hooks/useValidationStatus';
import Notification, {NotificationType} from '../../models/notification';

/**
 * Component to render a list of validation status labels.
 *
 * @param props - Props injected to the component.
 * @returns The ValidationStatusLabels component.
 */
function ValidationStatusLabels(): ReactElement {
  const {notifications, setCurrentActiveTab, openValidationPanel, setOpenValidationPanel} = useValidationStatus();
  const {setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {t} = useTranslation();

  const errorCount: number =
    notifications?.filter((notification: Notification) => notification.getType() === NotificationType.ERROR)?.length ||
    0;
  const warningCount: number =
    notifications?.filter((notification: Notification) => notification.getType() === NotificationType.WARNING)
      ?.length || 0;
  const infoCount: number =
    notifications?.filter((notification: Notification) => notification.getType() === NotificationType.INFO)?.length ||
    0;

  const handleNotificationClick = (): void => {
    if (openValidationPanel) {
      setOpenValidationPanel?.(false);
      return;
    }

    // Set active tab based on priority: Error (0) > Warning (1) > Info (2)
    let activeTab = 0;

    if (errorCount > 0) {
      activeTab = 0;
    } else if (warningCount > 0) {
      activeTab = 1;
    }

    setCurrentActiveTab?.(activeTab);
    setIsOpenResourcePropertiesPanel(false);
    setOpenValidationPanel?.(true);
  };

  // Determine priority: Error > Warning > Info
  let badgeContent = 0;
  let badgeColor: 'error' | 'warning' | 'info' = 'info';

  if (errorCount > 0) {
    badgeContent = errorCount;
    badgeColor = 'error';
  } else if (warningCount > 0) {
    badgeContent = warningCount;
    badgeColor = 'warning';
  } else if (infoCount > 0) {
    badgeContent = infoCount;
    badgeColor = 'info';
  }

  return (
    <Box display="flex" justifyContent="center" alignItems="center" className="flow-builder-validation-status-labels">
      <Tooltip title={t('flows:core.notificationPanel.trigger.label')}>
        <IconButton onClick={handleNotificationClick}>
          <BellIcon height={20} width={20} />
          <Badge
            badgeContent={badgeContent}
            color={badgeColor}
            overlap="circular"
            sx={{
              '& .MuiBadge-badge': {
                right: -6,
                top: -12,
              },
            }}
          />
        </IconButton>
      </Tooltip>
    </Box>
  );
}

export default ValidationStatusLabels;
