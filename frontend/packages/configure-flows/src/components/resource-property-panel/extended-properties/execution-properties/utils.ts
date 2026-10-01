// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {TEMPLATE_SCENARIOS} from './constants';

/**
 * Builds the template scenario options for a picker, keeping the current value in the list
 * even when it is not one this build knows about — a flow authored elsewhere should not have
 * its template silently blanked.
 *
 * @param currentValue - The scenario currently stored on the executor.
 * @returns The scenario values to offer.
 */
export const getTemplateScenarioOptions = (currentValue: string): string[] => {
  const scenarios: string[] = TEMPLATE_SCENARIOS.map((scenario) => scenario.value);

  return currentValue && !scenarios.includes(currentValue) ? [...scenarios, currentValue] : scenarios;
};

/**
 * Resolves the display label for a template scenario. Scenarios this build does not know
 * about have no translation, so they fall back to the raw value.
 *
 * @param scenario - The scenario value.
 * @param translate - Translation function.
 * @returns The human-readable label.
 */
export const getTemplateScenarioLabel = (
  scenario: string,
  translate: (key: string, defaultValue: string) => string,
): string => {
  const known = TEMPLATE_SCENARIOS.find((candidate) => candidate.value === scenario);

  return known ? translate(known.translationKey, known.displayLabel) : scenario;
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
