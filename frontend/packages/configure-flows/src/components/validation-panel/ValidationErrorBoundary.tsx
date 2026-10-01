// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keyframes} from '@emotion/react';
import {Box} from '@wso2/oxygen-ui';
import {CircleAlertIcon} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type PropsWithChildren, type ReactElement} from 'react';
import useValidationStatus from '../../hooks/useValidationStatus';
import Notification, {NotificationType} from '../../models/notification';
import type {Resource} from '../../models/resources';

const errorPulse = keyframes`
  0% { box-shadow: 0 0 0 0 rgba(var(--oxygen-palette-error-mainChannel) / 1); }
  70% { box-shadow: 0 0 0 15px rgba(var(--oxygen-palette-error-mainChannel) / 0); }
  100% { box-shadow: 0 0 0 0 rgba(var(--oxygen-palette-error-mainChannel) / 0); }
`;

const warningPulse = keyframes`
  0% { box-shadow: 0 0 0 0 rgba(var(--oxygen-palette-warning-mainChannel) / 1); }
  70% { box-shadow: 0 0 0 15px rgba(var(--oxygen-palette-warning-mainChannel) / 0); }
  100% { box-shadow: 0 0 0 0 rgba(var(--oxygen-palette-warning-mainChannel) / 0); }
`;

const SEVERITY_COLORS: Record<NotificationType, string> = {
  [NotificationType.ERROR]: 'error.main',
  [NotificationType.INFO]: 'info.main',
  [NotificationType.WARNING]: 'warning.main',
};

/**
 * Props interface of {@link ValidationErrorBoundary}
 */
export interface ValidationErrorBoundaryPropsInterface {
  /**
   * The resource to check for validation errors.
   */
  resource: Resource;
  /**
   * Whether to disable the error boundary on hover.
   */
  disableErrorBoundaryOnHover?: boolean;
  /**
   * Radius of the boundary outline, for nodes that are not rounded rectangles.
   */
  borderRadius?: string | number;
}

/**
 * Validation error boundary component that wraps components and shows error indicators.
 * TEST 11: Restore full validation logic.
 *
 * @param props - Props injected to the component.
 * @returns ValidationErrorBoundary component.
 */
function ValidationErrorBoundary({
  resource,
  children = null,
  disableErrorBoundaryOnHover = false,
  borderRadius = undefined,
}: PropsWithChildren<ValidationErrorBoundaryPropsInterface>): ReactElement {
  const {notifications} = useValidationStatus();
  const [active, setActive] = useState<boolean>(false);

  /**
   * Finds the notification for this resource (if any).
   * Prioritizes error notifications over warnings and info.
   */
  const resourceNotification: Notification | null = useMemo(() => {
    // First check for error notifications
    const errorNotification = notifications.find(
      (n: Notification) => n.hasResource(resource.id) && n.getType() === NotificationType.ERROR,
    );

    if (errorNotification) {
      return errorNotification;
    }

    // Then check for warning notifications
    const warningNotification = notifications.find(
      (n: Notification) => n.hasResource(resource.id) && n.getType() === NotificationType.WARNING,
    );

    if (warningNotification) {
      return warningNotification;
    }

    // Finally check for info notifications
    const infoNotification = notifications.find(
      (n: Notification) => n.hasResource(resource.id) && n.getType() === NotificationType.INFO,
    );

    return infoNotification ?? null;
  }, [resource.id, notifications]);

  /**
   * Checks if the resource has any notifications.
   */
  const hasNotification: boolean = resourceNotification !== null;

  /**
   * Gets the notification type for styling.
   */
  const notificationType: NotificationType | null = resourceNotification?.getType() ?? null;

  const isPadded: boolean = hasNotification && !disableErrorBoundaryOnHover;

  return (
    <Box
      data-testid={hasNotification ? 'validation-error-boundary' : undefined}
      sx={{
        border: '2px solid transparent',
        position: 'relative',
        ...(hasNotification && {borderRadius: borderRadius ?? (isPadded ? 1 : undefined)}),
        ...(notificationType === NotificationType.ERROR && {
          animation: `${errorPulse} 1s infinite`,
          borderColor: 'error.main',
        }),
        ...(notificationType === NotificationType.WARNING && {
          animation: `${warningPulse} 1s infinite`,
          borderColor: 'warning.main',
        }),
        ...(notificationType === NotificationType.INFO && {
          backgroundColor: 'info.light',
          borderColor: 'info.main',
        }),
      }}
      onMouseOver={() => hasNotification && disableErrorBoundaryOnHover && setActive(true)}
      onFocus={() => hasNotification && disableErrorBoundaryOnHover && setActive(true)}
      onMouseOut={() => hasNotification && disableErrorBoundaryOnHover && setActive(false)}
      onBlur={() => hasNotification && disableErrorBoundaryOnHover && setActive(false)}
    >
      {hasNotification && !(active && disableErrorBoundaryOnHover) && (
        <Box
          component="span"
          data-testid="validation-boundary-icon"
          sx={{
            backgroundColor: 'background.default',
            borderRadius: '50%',
            color: notificationType ? SEVERITY_COLORS[notificationType] : undefined,
            display: 'inline-flex',
            position: 'absolute',
            right: 0,
            top: 0,
            transform: 'translate(50%, -50%)',
            zIndex: 10,
          }}
        >
          <CircleAlertIcon size={24} />
        </Box>
      )}
      {children}
    </Box>
  );
}

export default ValidationErrorBoundary;
