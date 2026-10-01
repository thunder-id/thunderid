// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UIPanelContext, {type UIPanelContextProps} from '../../../context/UIPanelContext';
import {ValidationContext, type ValidationContextProps} from '../../../context/ValidationContext';
import Notification, {NotificationType} from '../../../models/notification';
import ValidationStatusLabels from '../ValidationStatusLabels';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('ValidationStatusLabels', () => {
  const mockSetCurrentActiveTab = vi.fn();
  const mockSetOpenValidationPanel = vi.fn();
  const mockSetIsOpenResourcePropertiesPanel = vi.fn();

  const defaultValidationContext: ValidationContextProps = {
    isValid: true,
    notifications: [],
    getNotification: vi.fn(),
    validationConfig: {
      isOTPValidationEnabled: false,
      isRecoveryFactorValidationEnabled: false,
      isPasswordExecutorValidationEnabled: false,
    },
    setCurrentActiveTab: mockSetCurrentActiveTab,
    openValidationPanel: false,
    setOpenValidationPanel: mockSetOpenValidationPanel,
  };

  const defaultUIPanelValue: UIPanelContextProps = {
    isResourcePanelOpen: true,
    isResourcePropertiesPanelOpen: false,
    isVersionHistoryPanelOpen: false,
    resourcePropertiesPanelHeading: 'Test Panel Heading',
    setIsResourcePanelOpen: vi.fn(),
    setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel,
    setIsVersionHistoryPanelOpen: vi.fn(),
    setResourcePropertiesPanelHeading: vi.fn(),
    registerCloseValidationPanel: vi.fn(),
  };

  const createWrapper = (validationContext: ValidationContextProps = defaultValidationContext) => {
    function Wrapper({children}: {children: ReactNode}) {
      return (
        <UIPanelContext.Provider value={defaultUIPanelValue}>
          <ValidationContext.Provider value={validationContext}>{children}</ValidationContext.Provider>
        </UIPanelContext.Provider>
      );
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the notification bell button', () => {
      render(<ValidationStatusLabels />, {wrapper: createWrapper()});

      expect(screen.getByRole('button')).toBeInTheDocument();
    });

    it('should render with tooltip', () => {
      render(<ValidationStatusLabels />, {wrapper: createWrapper()});

      // The button should have the tooltip (translation key)
      expect(screen.getByRole('button')).toBeInTheDocument();
    });
  });

  describe('Badge Display - No Notifications', () => {
    it('should display badge with 0 when no notifications exist', () => {
      render(<ValidationStatusLabels />, {wrapper: createWrapper()});

      // Badge with 0 content
      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge).toBeInTheDocument();
    });
  });

  describe('Badge Display - Error Notifications', () => {
    it('should display error count in badge when errors exist', () => {
      const errorNotification1 = new Notification('error-1', 'Error 1', NotificationType.ERROR);
      const errorNotification2 = new Notification('error-2', 'Error 2', NotificationType.ERROR);

      const contextWithErrors: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [errorNotification1, errorNotification2],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithErrors)});

      // Check that badge exists
      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge).toBeInTheDocument();
      expect(badge?.textContent).toBe('2');
    });

    it('should show error badge color when errors exist', () => {
      const errorNotification = new Notification('error-1', 'Error 1', NotificationType.ERROR);

      const contextWithError: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [errorNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithError)});

      const badge = document.querySelector('.MuiBadge-colorError');
      expect(badge).toBeInTheDocument();
    });
  });

  describe('Badge Display - Warning Notifications', () => {
    it('should display warning count in badge when only warnings exist', () => {
      const warningNotification1 = new Notification('warning-1', 'Warning 1', NotificationType.WARNING);
      const warningNotification2 = new Notification('warning-2', 'Warning 2', NotificationType.WARNING);
      const warningNotification3 = new Notification('warning-3', 'Warning 3', NotificationType.WARNING);

      const contextWithWarnings: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [warningNotification1, warningNotification2, warningNotification3],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithWarnings)});

      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge?.textContent).toBe('3');
    });

    it('should show warning badge color when only warnings exist', () => {
      const warningNotification = new Notification('warning-1', 'Warning 1', NotificationType.WARNING);

      const contextWithWarning: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [warningNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithWarning)});

      const badge = document.querySelector('.MuiBadge-colorWarning');
      expect(badge).toBeInTheDocument();
    });
  });

  describe('Badge Display - Info Notifications', () => {
    it('should display info count in badge when only info notifications exist', () => {
      const infoNotification = new Notification('info-1', 'Info 1', NotificationType.INFO);

      const contextWithInfo: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [infoNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithInfo)});

      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge?.textContent).toBe('1');
    });

    it('should show info badge color when only info notifications exist', () => {
      const infoNotification = new Notification('info-1', 'Info 1', NotificationType.INFO);

      const contextWithInfo: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [infoNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithInfo)});

      const badge = document.querySelector('.MuiBadge-colorInfo');
      expect(badge).toBeInTheDocument();
    });
  });

  describe('Badge Priority', () => {
    it('should prioritize error count over warning count', () => {
      const errorNotification = new Notification('error-1', 'Error 1', NotificationType.ERROR);
      const warningNotification1 = new Notification('warning-1', 'Warning 1', NotificationType.WARNING);
      const warningNotification2 = new Notification('warning-2', 'Warning 2', NotificationType.WARNING);

      const contextWithMixed: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [errorNotification, warningNotification1, warningNotification2],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithMixed)});

      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge?.textContent).toBe('1'); // Error count
      expect(document.querySelector('.MuiBadge-colorError')).toBeInTheDocument();
    });

    it('should prioritize warning count over info count', () => {
      const warningNotification = new Notification('warning-1', 'Warning 1', NotificationType.WARNING);
      const infoNotification1 = new Notification('info-1', 'Info 1', NotificationType.INFO);
      const infoNotification2 = new Notification('info-2', 'Info 2', NotificationType.INFO);

      const contextWithMixed: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [warningNotification, infoNotification1, infoNotification2],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithMixed)});

      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge?.textContent).toBe('1'); // Warning count
      expect(document.querySelector('.MuiBadge-colorWarning')).toBeInTheDocument();
    });
  });

  describe('Click Behavior - Opening Panel', () => {
    it('should open validation panel on click when panel is closed', () => {
      render(<ValidationStatusLabels />, {wrapper: createWrapper()});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(true);
    });

    it('should close resource properties panel when opening validation panel', () => {
      render(<ValidationStatusLabels />, {wrapper: createWrapper()});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(false);
    });

    it('should set active tab to 0 (errors) when errors exist', () => {
      const errorNotification = new Notification('error-1', 'Error 1', NotificationType.ERROR);

      const contextWithError: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [errorNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithError)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(0);
    });

    it('should set active tab to 1 (warnings) when only warnings exist', () => {
      const warningNotification = new Notification('warning-1', 'Warning 1', NotificationType.WARNING);

      const contextWithWarning: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [warningNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithWarning)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(1);
    });

    it('should set active tab to 0 when no notifications exist', () => {
      render(<ValidationStatusLabels />, {wrapper: createWrapper()});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(0);
    });
  });

  describe('Click Behavior - Closing Panel', () => {
    it('should close validation panel on click when panel is already open', () => {
      const contextWithOpenPanel: ValidationContextProps = {
        ...defaultValidationContext,
        openValidationPanel: true,
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithOpenPanel)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetOpenValidationPanel).toHaveBeenCalledWith(false);
    });

    it('should not set active tab when closing panel', () => {
      const contextWithOpenPanel: ValidationContextProps = {
        ...defaultValidationContext,
        openValidationPanel: true,
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithOpenPanel)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetCurrentActiveTab).not.toHaveBeenCalled();
    });

    it('should not close resource properties panel when closing validation panel', () => {
      const contextWithOpenPanel: ValidationContextProps = {
        ...defaultValidationContext,
        openValidationPanel: true,
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithOpenPanel)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetIsOpenResourcePropertiesPanel).not.toHaveBeenCalled();
    });
  });

  describe('Empty Notifications Array', () => {
    it('should handle empty notifications array gracefully', () => {
      const contextWithEmptyNotifications: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithEmptyNotifications)});

      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge).toBeInTheDocument();
    });
  });

  describe('Null/Undefined Notifications', () => {
    it('should handle undefined notifications gracefully', () => {
      const contextWithUndefinedNotifications: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: undefined as unknown as Notification[],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithUndefinedNotifications)});

      // Should not throw and should render
      expect(screen.getByRole('button')).toBeInTheDocument();
    });
  });

  describe('Badge Color Priority', () => {
    it('should set info badge color when only info notifications exist (no errors, no warnings)', () => {
      const infoNotification1 = new Notification('info-1', 'Info 1', NotificationType.INFO);
      const infoNotification2 = new Notification('info-2', 'Info 2', NotificationType.INFO);

      const contextWithOnlyInfo: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [infoNotification1, infoNotification2],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithOnlyInfo)});

      const badge = document.querySelector('.MuiBadge-badge');
      expect(badge?.textContent).toBe('2');
      expect(document.querySelector('.MuiBadge-colorInfo')).toBeInTheDocument();
    });

    it('should set error tab (0) when errors exist along with info', () => {
      const errorNotification = new Notification('error-1', 'Error 1', NotificationType.ERROR);
      const infoNotification = new Notification('info-1', 'Info 1', NotificationType.INFO);

      const contextWithErrorAndInfo: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [errorNotification, infoNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithErrorAndInfo)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(0);
    });

    it('should set warning tab (1) when only warnings and info exist', () => {
      const warningNotification = new Notification('warning-1', 'Warning 1', NotificationType.WARNING);
      const infoNotification = new Notification('info-1', 'Info 1', NotificationType.INFO);

      const contextWithWarningAndInfo: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [warningNotification, infoNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithWarningAndInfo)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(1);
    });

    it('should set default tab (0) when only info notifications exist (no errors, no warnings)', () => {
      const infoNotification = new Notification('info-1', 'Info 1', NotificationType.INFO);

      const contextWithOnlyInfo: ValidationContextProps = {
        ...defaultValidationContext,
        notifications: [infoNotification],
      };

      render(<ValidationStatusLabels />, {wrapper: createWrapper(contextWithOnlyInfo)});

      const button = screen.getByRole('button');
      fireEvent.click(button);

      // Default activeTab is 0, and since no errors/warnings, it stays at 0
      expect(mockSetCurrentActiveTab).toHaveBeenCalledWith(0);
    });
  });
});
