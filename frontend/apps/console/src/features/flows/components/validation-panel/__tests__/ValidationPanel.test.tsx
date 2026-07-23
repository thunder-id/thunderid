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

/* eslint-disable @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-argument, @typescript-eslint/no-unsafe-assignment */

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Notification, {NotificationType} from '../../../models/notification';
import ValidationPanel from '../ValidationPanel';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.notificationPanel.header': 'Notifications',
        'flows:core.notificationPanel.tabs.errors': 'Errors',
        'flows:core.notificationPanel.tabs.warnings': 'Warnings',
        'flows:core.notificationPanel.tabs.info': 'Info',
        'flows:core.notificationPanel.emptyMessages.errors': 'No errors found',
        'flows:core.notificationPanel.emptyMessages.warnings': 'No warnings found',
        'flows:core.notificationPanel.emptyMessages.info': 'No info messages found',
      };
      return translations[key] || key;
    },
  }),
}));

// Mock ValidationNotificationsList
vi.mock('../ValidationNotificationsList', () => ({
  default: ({
    notifications,
    emptyMessage,
    onNotificationClick,
  }: {
    notifications: Notification[];
    emptyMessage: string;
    onNotificationClick: (n: Notification) => void;
  }) => (
    <div data-testid="notifications-list" data-count={notifications.length} data-empty-message={emptyMessage}>
      {notifications.map((n) => (
        <button
          type="button"
          key={n.getId()}
          onClick={() => onNotificationClick(n)}
          data-testid={`notification-${n.getId()}`}
        >
          {n.getMessage()}
        </button>
      ))}
    </div>
  ),
}));

// Mock hooks
const mockSetOpenValidationPanel = vi.fn();
const mockSetSelectedNotification = vi.fn();
const mockSetCurrentActiveTab = vi.fn();
const mockSetLastInteractedResource = vi.fn();

let mockNotifications: Notification[] = [];
let mockOpenValidationPanel = true;
let mockCurrentActiveTab = 0;

vi.mock('../../../hooks/useValidationStatus', () => ({
  default: () => ({
    notifications: mockNotifications,
    openValidationPanel: mockOpenValidationPanel,
    setOpenValidationPanel: mockSetOpenValidationPanel,
    setSelectedNotification: mockSetSelectedNotification,
    currentActiveTab: mockCurrentActiveTab,
    setCurrentActiveTab: mockSetCurrentActiveTab,
  }),
}));

const mockSetLastInteractedStepId = vi.fn();
const mockGetNodes = vi.fn().mockReturnValue([]);
const mockFitView = vi.fn().mockResolvedValue(true);

vi.mock('../../../hooks/useInteractionState', () => ({
  default: () => ({
    setLastInteractedResource: mockSetLastInteractedResource,
    setLastInteractedStepId: mockSetLastInteractedStepId,
  }),
}));

vi.mock('@xyflow/react', () => ({
  useReactFlow: () => ({
    getNodes: mockGetNodes,
    fitView: mockFitView,
  }),
}));

describe('ValidationPanel', () => {
  const createNotification = (id: string, message: string, type: NotificationType): Notification =>
    new Notification(id, message, type);

  beforeEach(() => {
    vi.clearAllMocks();
    mockNotifications = [];
    mockOpenValidationPanel = true;
    mockCurrentActiveTab = 0;
  });

  describe('Rendering', () => {
    it('should render panel header', () => {
      render(<ValidationPanel open />);

      expect(screen.getByText('Notifications')).toBeInTheDocument();
    });

    it('should render tabs for errors, warnings, and info', () => {
      render(<ValidationPanel open />);

      expect(screen.getByRole('tab', {name: /Errors/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Warnings/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Info/i})).toBeInTheDocument();
    });

    it('should render close button', () => {
      render(<ValidationPanel open />);

      // Close button is an IconButton
      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });
  });

  describe('Tab Navigation', () => {
    it('should show errors tab content by default', () => {
      mockCurrentActiveTab = 0;
      mockNotifications = [createNotification('1', 'Error message', NotificationType.ERROR)];

      render(<ValidationPanel open />);

      const visibleList = screen.getByTestId('notifications-list');
      expect(visibleList).toHaveAttribute('data-empty-message', 'No errors found');
    });

    it('should call setCurrentActiveTab when switching tabs', () => {
      render(<ValidationPanel open />);

      const warningsTab = screen.getByRole('tab', {name: /Warnings/i});
      fireEvent.click(warningsTab);

      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(1);
    });
  });

  describe('Notification Filtering', () => {
    it('should filter error notifications for errors tab', () => {
      mockCurrentActiveTab = 0;
      mockNotifications = [
        createNotification('1', 'Error', NotificationType.ERROR),
        createNotification('2', 'Warning', NotificationType.WARNING),
        createNotification('3', 'Info', NotificationType.INFO),
      ];

      render(<ValidationPanel open />);

      const errorTabPanel = document.getElementById('validation-tabpanel-0');
      expect(errorTabPanel).not.toHaveAttribute('hidden');
    });

    it('should filter warning notifications for warnings tab', () => {
      mockCurrentActiveTab = 1;
      mockNotifications = [
        createNotification('1', 'Error', NotificationType.ERROR),
        createNotification('2', 'Warning', NotificationType.WARNING),
      ];

      render(<ValidationPanel open />);

      const warningTabPanel = document.getElementById('validation-tabpanel-1');
      expect(warningTabPanel).not.toHaveAttribute('hidden');
    });
  });

  describe('Close Functionality', () => {
    it('should call setOpenValidationPanel(false) when close button clicked', () => {
      render(<ValidationPanel open />);

      // Find the close button (IconButton with X icon)
      const closeButtons = screen.getAllByRole('button');
      const closeButton = closeButtons.find((btn) => btn.querySelector('svg'));

      expect(closeButton).toBeDefined();
      fireEvent.click(closeButton!);
      expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(false);
    });
  });

  describe('Notification Click Handling', () => {
    it('should set selected notification when notification is clicked', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Error', NotificationType.ERROR);
      mockNotifications = [notification];

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetSelectedNotification).toHaveBeenCalledWith(notification);
    });

    it('should close panel when notification is clicked', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Error', NotificationType.ERROR);
      mockNotifications = [notification];

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(false);
    });

    it('should set last interacted resource when notification has single resource', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Error', NotificationType.ERROR);
      const resource = {id: 'resource-1', type: 'TEST', category: 'TEST'} as any;
      notification.addResource(resource);
      mockNotifications = [notification];

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(resource);
    });
  });

  describe('Tab Panel Accessibility', () => {
    it('should have correct aria attributes on tab panels', () => {
      render(<ValidationPanel open />);

      const tabPanel0 = document.getElementById('validation-tabpanel-0');
      expect(tabPanel0).toHaveAttribute('role', 'tabpanel');
      expect(tabPanel0).toHaveAttribute('aria-labelledby', 'validation-tab-0');
    });
  });

  describe('Notification with Multiple Resources', () => {
    it('should not set last interacted resource when notification has multiple resources', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Error', NotificationType.ERROR);
      const resource1 = {id: 'resource-1', type: 'TEST', category: 'TEST'} as any;
      const resource2 = {id: 'resource-2', type: 'TEST', category: 'TEST'} as any;
      notification.addResource(resource1);
      notification.addResource(resource2);
      mockNotifications = [notification];

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetLastInteractedResource).not.toHaveBeenCalled();
    });

    it('should not set last interacted resource when notification has no resources', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Error', NotificationType.ERROR);
      mockNotifications = [notification];

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetLastInteractedResource).not.toHaveBeenCalled();
    });
  });

  describe('Tab Content Display', () => {
    it('should display info notifications in info tab', () => {
      mockCurrentActiveTab = 2;
      mockNotifications = [createNotification('1', 'Info message', NotificationType.INFO)];

      render(<ValidationPanel open />);

      const infoTabPanel = document.getElementById('validation-tabpanel-2');
      expect(infoTabPanel).not.toHaveAttribute('hidden');
    });
  });

  describe('Nested Resource Node Search', () => {
    it('should find a node when resource is nested inside components', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Element error', NotificationType.ERROR);
      const resource = {id: 'nested-element-1', type: 'TEXT_INPUT', category: 'INPUT'} as any;
      notification.addResource(resource);
      mockNotifications = [notification];

      // Mock getNodes to return a node whose components contain the nested element
      mockGetNodes.mockReturnValue([
        {
          id: 'step-node-1',
          data: {
            components: [
              {id: 'other-element', type: 'BUTTON', components: []},
              {id: 'wrapper', type: 'BLOCK', components: [{id: 'nested-element-1', type: 'TEXT_INPUT'}]},
            ],
          },
        },
      ]);

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(resource);
      expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('step-node-1');
      expect(mockFitView).toHaveBeenCalledWith({
        nodes: [{id: 'step-node-1'}],
        padding: 0.3,
        maxZoom: 1.2,
        duration: 400,
      });
    });
  });

  describe('Notification Click with Matching Node', () => {
    it('should call fitView when the resource matches a direct node', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Step error', NotificationType.ERROR);
      const resource = {id: 'step-1', type: 'VIEW', category: 'STEP'} as any;
      notification.addResource(resource);
      mockNotifications = [notification];

      mockGetNodes.mockReturnValue([{id: 'step-1', data: {}}]);

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('step-1');
      expect(mockFitView).toHaveBeenCalledWith({nodes: [{id: 'step-1'}], padding: 0.3, maxZoom: 1.2, duration: 400});
    });

    it('should not call fitView when no node matches the resource', () => {
      mockCurrentActiveTab = 0;
      const notification = createNotification('1', 'Orphan error', NotificationType.ERROR);
      const resource = {id: 'missing-node', type: 'TEXT_INPUT', category: 'INPUT'} as any;
      notification.addResource(resource);
      mockNotifications = [notification];

      mockGetNodes.mockReturnValue([{id: 'step-1', data: {components: [{id: 'other', type: 'BUTTON'}]}}]);

      render(<ValidationPanel open />);

      const notificationButton = screen.getByTestId('notification-1');
      fireEvent.click(notificationButton);

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(resource);
      expect(mockSetLastInteractedStepId).not.toHaveBeenCalled();
      expect(mockFitView).not.toHaveBeenCalled();
    });
  });
});
