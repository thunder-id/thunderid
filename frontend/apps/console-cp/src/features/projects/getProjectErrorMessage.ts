// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ApiError} from '@thunderid/types';

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

/**
 * The reason a project request failed, for showing to the user.
 *
 * The project API answers with its own `PRJ-` codes and passes the organization unit service's `OU-`
 * codes through (deleting a project deletes its root organization unit), so a code-only lookup
 * finds no message for most of them. The reason is resolved in this order:
 *
 * 1. A console message for the code (`common:errors.<code>`), so a reason can be reworded locally.
 * 2. The description the API sent, translated by its key and falling back to its default text.
 * 3. The given fallback message.
 */
export default function getProjectErrorMessage(
  error: Error,
  t: TranslateFn,
  fallbackKey: string,
  fallbackDefaultValue: string,
): string {
  const apiError = (error as {response?: {data?: Partial<ApiError>}}).response?.data;

  if (apiError?.code) {
    const local = t(`common:errors.${apiError.code}`, {defaultValue: ''});
    if (local) {
      return local;
    }
  }

  const {description} = apiError ?? {};
  if (typeof description === 'string' && description.trim()) {
    return description;
  }
  if (typeof description === 'object' && description?.defaultValue) {
    return t(description.key, {...description.params, defaultValue: description.defaultValue});
  }

  return t(fallbackKey, {defaultValue: fallbackDefaultValue});
}
