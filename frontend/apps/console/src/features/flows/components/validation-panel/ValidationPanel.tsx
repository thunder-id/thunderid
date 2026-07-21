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

import {BuilderStaticPanel} from '@thunderid/components';
import {Box, IconButton, Stack, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
import {BellIcon, CircleXIcon, InfoIcon, TriangleAlertIcon, X} from '@wso2/oxygen-ui-icons-react';
import {useReactFlow, type Node} from '@xyflow/react';
import type {PropsWithChildren, ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import ValidationNotificationsList from './ValidationNotificationsList';
import useInteractionState from '../../hooks/useInteractionState';
import useValidationStatus from '../../hooks/useValidationStatus';
import type {Element} from '../../models/elements';
import Notification, {NotificationType} from '../../models/notification';
import type {StepData} from '../../models/steps';

/**
 * Props interface for TabPanel component.
 */
interface TabPanelProps {
  /**
   * Tab panel index.
   */
  index: number;
  /**
   * Current selected tab value.
   */
  value: number;
  /**
   * Tab panel children.
   * @defaultValue undefined
   */
  children?: React.ReactNode;
}

/**
 * TabPanel component to conditionally render tab content.
 */
function TabPanel({children = undefined, value, index}: PropsWithChildren<TabPanelProps>): ReactElement {
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`validation-tabpanel-${index}`}
      aria-labelledby={`validation-tab-${index}`}
    >
      {value === index && <Box>{children}</Box>}
    </div>
  );
}

/**
 * Get the icon for a notification type, tinted with its severity color.
 *
 * @param type - Notification type.
 * @returns Icon component for the notification type.
 */
const getNotificationIcon = (type: NotificationType): ReactElement => {
  const icon = ((): ReactElement => {
    switch (type) {
      case NotificationType.ERROR:
        return <CircleXIcon size={16} />;
      case NotificationType.INFO:
        return <InfoIcon size={16} />;
      case NotificationType.WARNING:
        return <TriangleAlertIcon size={16} />;
      default:
        return <InfoIcon size={16} />;
    }
  })();

  return <Box sx={{display: 'inline-flex', color: `${type}.main`}}>{icon}</Box>;
};

/**
 * Small count pill shown next to a tab label when its list is non-empty.
 */
function TabCount({count}: {count: number}): ReactElement | null {
  if (count === 0) {
    return null;
  }
  return (
    <Box
      component="span"
      sx={{
        px: 0.75,
        py: 0.1,
        borderRadius: 999,
        fontSize: '0.7rem',
        fontWeight: 600,
        lineHeight: 1.4,
        bgcolor: 'action.selected',
        color: 'text.primary',
      }}
    >
      {count}
    </Box>
  );
}

export interface ValidationPanelProps {
  open?: boolean;
}

/**
 * Component to render the notification panel with tabbed notifications.
 *
 * @param props - Props injected to the component.
 * @returns The ValidationPanel component.
 */
function ValidationPanel({open = false}: ValidationPanelProps): ReactElement {
  const {t} = useTranslation();
  const {notifications, setOpenValidationPanel, setSelectedNotification, currentActiveTab, setCurrentActiveTab} =
    useValidationStatus();
  const {setLastInteractedResource, setLastInteractedStepId} = useInteractionState();
  const {getNodes, fitView} = useReactFlow();

  const errorNotifications: Notification[] = notifications.filter(
    (notification: Notification) => notification.getType() === NotificationType.ERROR,
  );
  const infoNotifications: Notification[] = notifications.filter(
    (notification: Notification) => notification.getType() === NotificationType.INFO,
  );
  const warningNotifications: Notification[] = notifications.filter(
    (notification: Notification) => notification.getType() === NotificationType.WARNING,
  );

  const handleTabChange = (_event: React.SyntheticEvent, newValue: number): void => {
    setCurrentActiveTab?.(newValue);
  };

  const handleClose = (): void => {
    setOpenValidationPanel?.(false);
  };

  /**
   * Finds the React Flow node that contains the given resource id.
   * For steps, the node id matches directly. For elements, searches
   * the node's components tree for a matching element id.
   */
  const findNodeForResource = (resourceId: string): Node | undefined => {
    const nodes = getNodes();

    // Direct match — resource is a step node itself
    const directMatch = nodes.find((node) => node.id === resourceId);
    if (directMatch) return directMatch;

    // Search inside node components for elements
    return nodes.find((node) => {
      const components = (node.data as StepData)?.components;
      if (!components) return false;

      const containsElement = (elements: Element[]): boolean =>
        elements.some((el) => el.id === resourceId || (el.components && containsElement(el.components)));

      return containsElement(components);
    });
  };

  const handleNotificationClick = (notification: Notification): void => {
    setSelectedNotification?.(notification);
    setOpenValidationPanel?.(false);
    if (notification.getResources().length === 1) {
      const resource = notification.getResources()[0];
      setLastInteractedResource(resource);

      const targetNode = findNodeForResource(resource.id);
      if (targetNode) {
        setLastInteractedStepId(targetNode.id);
        // Same focus framing as clicking a step during the flow preview — an
        // uncapped fitView on a single node zooms in far too close.
        void fitView({nodes: [{id: targetNode.id}], padding: 0.3, maxZoom: 1.2, duration: 400});
      }
    }
  };

  return (
    <BuilderStaticPanel
      open={open}
      width={350}
      anchor="right"
      header={
        <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%'}}>
          <Stack direction="row" gap={1} alignItems="center">
            <BellIcon size={16} />
            <Typography variant="h6">{t('flows:core.notificationPanel.header')}</Typography>
          </Stack>
          <IconButton onClick={handleClose} size="small" aria-label="Close notifications panel">
            <X height={16} width={16} />
          </IconButton>
        </Box>
      }
    >
      {/* Tabs */}
      <Box
        sx={{
          px: 2,
          borderBottom: '1px solid',
          borderColor: 'divider',
        }}
      >
        <Tabs
          value={currentActiveTab}
          onChange={handleTabChange}
          variant="fullWidth"
          sx={{
            minHeight: 44,
            '& .MuiTab-root': {
              minHeight: 44,
              py: 1,
              px: 1.5,
              textTransform: 'none',
              fontSize: '0.875rem',
              fontWeight: 500,
            },
          }}
        >
          <Tab
            label={
              <Box display="flex" alignItems="center" gap={0.5}>
                {getNotificationIcon(NotificationType.ERROR)}
                <Typography variant="h6" sx={{fontSize: '0.8rem'}}>
                  {t('flows:core.notificationPanel.tabs.errors')}
                </Typography>
                <TabCount count={errorNotifications.length} />
              </Box>
            }
          />
          <Tab
            label={
              <Box display="flex" alignItems="center" gap={0.5}>
                {getNotificationIcon(NotificationType.WARNING)}
                <Typography variant="h6" sx={{fontSize: '0.8rem'}}>
                  {t('flows:core.notificationPanel.tabs.warnings')}
                </Typography>
                <TabCount count={warningNotifications.length} />
              </Box>
            }
          />
          <Tab
            label={
              <Box display="flex" alignItems="center" gap={0.5}>
                {getNotificationIcon(NotificationType.INFO)}
                <Typography variant="h6" sx={{fontSize: '0.8rem'}}>
                  {t('flows:core.notificationPanel.tabs.info')}
                </Typography>
                <TabCount count={infoNotifications.length} />
              </Box>
            }
          />
        </Tabs>
      </Box>

      {/* Content */}
      <Box
        sx={{
          p: 2,
          flex: 1,
          minHeight: 0,
          overflowY: 'auto',
          overflowX: 'hidden',
          '&::-webkit-scrollbar': {width: '6px'},
          '&::-webkit-scrollbar-track': {background: 'transparent'},
          '&::-webkit-scrollbar-thumb': {
            background: 'rgba(0, 0, 0, 0.2)',
            borderRadius: '3px',
            '&:hover': {background: 'rgba(0, 0, 0, 0.3)'},
          },
        }}
      >
        <TabPanel value={currentActiveTab ?? 0} index={0}>
          <ValidationNotificationsList
            notifications={errorNotifications}
            emptyMessage={t('flows:core.notificationPanel.emptyMessages.errors')}
            onNotificationClick={handleNotificationClick}
          />
        </TabPanel>
        <TabPanel value={currentActiveTab ?? 0} index={1}>
          <ValidationNotificationsList
            notifications={warningNotifications}
            emptyMessage={t('flows:core.notificationPanel.emptyMessages.warnings')}
            onNotificationClick={handleNotificationClick}
          />
        </TabPanel>
        <TabPanel value={currentActiveTab ?? 0} index={2}>
          <ValidationNotificationsList
            notifications={infoNotifications}
            emptyMessage={t('flows:core.notificationPanel.emptyMessages.info')}
            onNotificationClick={handleNotificationClick}
          />
        </TabPanel>
      </Box>
    </BuilderStaticPanel>
  );
}

export default ValidationPanel;
