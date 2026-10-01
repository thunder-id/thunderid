// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {createTheme, type Theme} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ValidationContext, type ValidationContextProps} from '../../../context/ValidationContext';
import Notification, {NotificationType} from '../../../models/notification';
import type {Resource} from '../../../models/resources';
import ValidationErrorBoundary from '../ValidationErrorBoundary';

const theme: Theme = createTheme();

describe('ValidationErrorBoundary', () => {
  const mockResource: Resource = {
    id: 'resource-1',
    type: 'TEXT_INPUT',
    config: {},
  } as Resource;

  // Helper to create a notification with a resource
  const createNotificationWithResource = (
    id: string,
    message: string,
    type: NotificationType,
    resourceId: string,
  ): Notification => {
    const notification = new Notification(id, message, type);
    // addResource expects a Resource object, so we create one
    notification.addResource({id: resourceId, type: 'TEXT_INPUT', config: {}} as Resource);
    return notification;
  };

  const defaultContextValue: ValidationContextProps = {
    isValid: true,
    notifications: [],
    getNotification: vi.fn(),
    validationConfig: {
      isOTPValidationEnabled: false,
      isRecoveryFactorValidationEnabled: false,
      isPasswordExecutorValidationEnabled: false,
    },
  };

  const createWrapper = (contextValue: ValidationContextProps = defaultContextValue) => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ValidationContext.Provider value={contextValue}>{children}</ValidationContext.Provider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering Without Notifications', () => {
    it('should render children without error boundary styling when no notifications exist', () => {
      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div data-testid="child-content">Child Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper()},
      );

      expect(screen.getByTestId('child-content')).toBeInTheDocument();
      expect(screen.getByTestId('child-content').textContent).toBe('Child Content');
    });

    it('should not show alert icon when no notifications exist', () => {
      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper()},
      );

      // CircleAlertIcon should not be rendered
      expect(screen.queryByTestId('validation-boundary-icon')).not.toBeInTheDocument();
    });
  });

  describe('Error Notifications', () => {
    it('should show error styling when error notification exists for resource', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByTestId('validation-error-boundary')).toHaveStyle({borderColor: theme.palette.error.main});
    });

    it('should show alert icon for error notification', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByTestId('validation-boundary-icon')).toBeInTheDocument();
    });
  });

  describe('Warning Notifications', () => {
    it('should show warning styling when warning notification exists for resource', () => {
      const warningNotification = createNotificationWithResource(
        'notification-1',
        'Warning Message',
        NotificationType.WARNING,
        'resource-1',
      );

      const contextWithWarning: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [warningNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithWarning)},
      );

      expect(screen.getByTestId('validation-error-boundary')).toHaveStyle({borderColor: theme.palette.warning.main});
    });
  });

  describe('Info Notifications', () => {
    it('should show info styling when info notification exists for resource', () => {
      const infoNotification = createNotificationWithResource(
        'notification-1',
        'Info Message',
        NotificationType.INFO,
        'resource-1',
      );

      const contextWithInfo: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [infoNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithInfo)},
      );

      expect(screen.getByTestId('validation-error-boundary')).toHaveStyle({borderColor: theme.palette.info.main});
    });
  });

  describe('Notification Priority', () => {
    it('should prioritize error over warning notification', () => {
      const errorNotification = createNotificationWithResource(
        'error-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const warningNotification = createNotificationWithResource(
        'warning-1',
        'Warning Message',
        NotificationType.WARNING,
        'resource-1',
      );

      const contextWithBoth: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [warningNotification, errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithBoth)},
      );

      const boundary = screen.getByTestId('validation-error-boundary');

      expect(boundary).toHaveStyle({borderColor: theme.palette.error.main});
      expect(boundary).not.toHaveStyle({borderColor: theme.palette.warning.main});
    });

    it('should prioritize warning over info notification', () => {
      const warningNotification = createNotificationWithResource(
        'warning-1',
        'Warning Message',
        NotificationType.WARNING,
        'resource-1',
      );

      const infoNotification = createNotificationWithResource(
        'info-1',
        'Info Message',
        NotificationType.INFO,
        'resource-1',
      );

      const contextWithBoth: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [infoNotification, warningNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithBoth)},
      );

      const boundary = screen.getByTestId('validation-error-boundary');

      expect(boundary).toHaveStyle({borderColor: theme.palette.warning.main});
      expect(boundary).not.toHaveStyle({borderColor: theme.palette.info.main});
    });
  });

  describe('Hover Behavior', () => {
    it('should hide error boundary on hover when disableErrorBoundaryOnHover is true', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      const {container} = render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      const boundaryDiv = container.firstChild as HTMLElement;

      // The boundary marks itself initially
      expect(screen.getByTestId('validation-boundary-icon')).toBeInTheDocument();

      // Simulate mouse over
      fireEvent.mouseOver(boundaryDiv);

      // The marking is dropped while hovered
      expect(screen.queryByTestId('validation-boundary-icon')).not.toBeInTheDocument();
    });

    it('should restore error boundary when mouse leaves after hover', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      const {container} = render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      const boundaryDiv = container.firstChild as HTMLElement;

      // Mouse over
      fireEvent.mouseOver(boundaryDiv);
      expect(screen.queryByTestId('validation-boundary-icon')).not.toBeInTheDocument();

      // Mouse out
      fireEvent.mouseOut(boundaryDiv);
      expect(screen.getByTestId('validation-boundary-icon')).toBeInTheDocument();
    });

    it('should handle focus/blur events for accessibility', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      const {container} = render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      const boundaryDiv = container.firstChild as HTMLElement;

      // Focus
      fireEvent.focus(boundaryDiv);
      expect(screen.queryByTestId('validation-boundary-icon')).not.toBeInTheDocument();

      // Blur
      fireEvent.blur(boundaryDiv);
      expect(screen.getByTestId('validation-boundary-icon')).toBeInTheDocument();
    });

    it('should not activate on hover when disableErrorBoundaryOnHover is false', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      const {container} = render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover={false}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      const boundaryDiv = container.firstChild as HTMLElement;

      fireEvent.mouseOver(boundaryDiv);
      expect(screen.getByTestId('validation-boundary-icon')).toBeInTheDocument();
    });
  });

  describe('Alert Icon Visibility', () => {
    it('should hide alert icon when active and disableErrorBoundaryOnHover is true', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      const {container} = render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      // Alert icon should be visible initially
      expect(screen.getByTestId('validation-boundary-icon')).toBeInTheDocument();

      // Mouse over
      const boundaryDiv = container.firstChild as HTMLElement;
      fireEvent.mouseOver(boundaryDiv);

      // Alert icon should be hidden
      expect(screen.queryByTestId('validation-boundary-icon')).not.toBeInTheDocument();
    });
  });

  describe('Padded Radius', () => {
    it('should round the boundary when notification exists and disableErrorBoundaryOnHover is false', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover={false}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByTestId('validation-error-boundary')).toHaveStyle({
        borderRadius: `${theme.shape.borderRadius}px`,
      });
    });

    it('should not round the boundary when disableErrorBoundaryOnHover is true', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource} disableErrorBoundaryOnHover>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByTestId('validation-error-boundary')).not.toHaveStyle({
        borderRadius: `${theme.shape.borderRadius}px`,
      });
    });

    it('should use the borderRadius prop instead of the padded radius', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource} borderRadius="50%">
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.getByTestId('validation-error-boundary')).toHaveStyle({borderRadius: '50%'});
    });
  });

  describe('Different Resources', () => {
    it('should not show error boundary for notifications targeting different resource', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'different-resource-id',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      expect(screen.queryByTestId('validation-error-boundary')).not.toBeInTheDocument();
    });
  });

  describe('Default Props', () => {
    it('should default disableErrorBoundaryOnHover to false', () => {
      const errorNotification = createNotificationWithResource(
        'notification-1',
        'Error Message',
        NotificationType.ERROR,
        'resource-1',
      );

      const contextWithError: ValidationContextProps = {
        ...defaultContextValue,
        notifications: [errorNotification],
      };

      render(
        <ValidationErrorBoundary resource={mockResource}>
          <div>Content</div>
        </ValidationErrorBoundary>,
        {wrapper: createWrapper(contextWithError)},
      );

      // Should be rounded (which means disableErrorBoundaryOnHover is false)
      expect(screen.getByTestId('validation-error-boundary')).toHaveStyle({
        borderRadius: `${theme.shape.borderRadius}px`,
      });
    });

    it('should render with null children by default', () => {
      const {container} = render(<ValidationErrorBoundary resource={mockResource} />, {wrapper: createWrapper()});

      expect(container.firstChild).toBeInTheDocument();
    });
  });
});
