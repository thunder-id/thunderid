// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {NotificationTemplateSummary} from '../../../../models/notification-templates';

/**
 * Builds the template handle options for a picker, keeping the current value in the list even
 * when the server did not return it, so a flow authored elsewhere is not silently blanked.
 *
 * @param templates - The templates returned for the channel (may be undefined while loading).
 * @param currentValue - The handle currently stored on the executor.
 * @returns The handles to offer.
 */
export const buildTemplateOptions = (
  templates: NotificationTemplateSummary[] | undefined,
  currentValue: string,
): string[] => {
  const handles: string[] = (templates ?? []).map((template) => template.handle);

  return currentValue && !handles.includes(currentValue) ? [...handles, currentValue] : handles;
};

/**
 * Maps each template handle to its display name, for resolving picker option labels.
 *
 * @param templates - The templates returned for the channel (may be undefined while loading).
 * @returns A handle to display name lookup.
 */
export const buildTemplateLabels = (templates: NotificationTemplateSummary[] | undefined): Record<string, string> => {
  const labels: Record<string, string> = {};

  (templates ?? []).forEach((template) => {
    labels[template.handle] = template.displayName;
  });

  return labels;
};

/**
 * Parses a comma-separated string into a trimmed, non-empty string array.
 */
export const parseCommaSeparated = (value: string): string[] =>
  value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);

/**
 * Coerces a raw field value into a whole number within the given bounds.
 *
 * Intended as a {@link DraftTextField} `normalize` callback, so it runs once the user is
 * done editing rather than per keystroke — a field that clamps mid-typing fights anyone
 * entering a value digit by digit.
 *
 * @param raw - The raw field text.
 * @param min - Lower bound, and the value an empty field falls back to.
 * @param max - Upper bound, if the property has one.
 * @returns The clamped value as text, or `null` when the input is not a number.
 */
export const clampToInteger = (raw: string, min: number, max?: number): string | null => {
  if (raw.trim() === '') {
    return String(min);
  }

  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) {
    return null;
  }

  const floored = Math.max(min, Math.floor(parsed));

  return String(max === undefined ? floored : Math.min(max, floored));
};
