/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
