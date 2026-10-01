// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import useValidationStatus from './useValidationStatus';
import type Notification from '../models/notification';

/**
 * Resolves the validation message for a resource field from the current
 * notifications, so property panels highlight erroneous fields no matter how
 * the panel was opened (clicking the element directly, not only via the
 * notification panel). The explicitly selected notification takes precedence
 * for its message wording.
 *
 * @param resourceId - Id of the resource being edited.
 * @param fieldKey - Field key within the resource (e.g. `label`, `data.flow.ref`).
 * @returns The validation message for the field, or an empty string.
 */
const useResourceFieldError = (resourceId: string | undefined, fieldKey: string): string => {
  const {notifications, selectedNotification} = useValidationStatus();

  return useMemo(() => {
    const key = `${resourceId}_${fieldKey}`;

    if (selectedNotification?.hasResourceFieldNotification(key)) {
      return selectedNotification.getResourceFieldNotification(key);
    }

    const match = (notifications ?? []).find((notification: Notification) =>
      notification.hasResourceFieldNotification(key),
    );

    return match?.getResourceFieldNotification(key) ?? '';
  }, [resourceId, fieldKey, notifications, selectedNotification]);
};

export default useResourceFieldError;
