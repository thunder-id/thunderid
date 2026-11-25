// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {VARIABLE_PLACEHOLDER_PATTERN} from './variableIdentifier';

/**
 * The properties of a step with the bindings to one variable pointed at another, or taken off when `to` is `null`.
 * Returns `undefined` when no property is bound to the variable, so the step is left as it is.
 *
 * @param properties - The properties of a step
 * @param from - The identifier the bindings name now
 * @param to - The identifier they should name, or `null` to unbind them
 * @returns The new properties, or `undefined` when none was bound to the variable
 */
export const rewriteBindings = (
  properties: Record<string, unknown> | undefined,
  from: string,
  to: string | null,
): Record<string, unknown> | undefined => {
  const next: Record<string, unknown> = {};
  let changed = false;

  Object.entries(properties ?? {}).forEach(([key, value]) => {
    const bound: string | undefined =
      typeof value === 'string' ? VARIABLE_PLACEHOLDER_PATTERN.exec(value.trim())?.[1] : undefined;

    if (bound !== from) {
      next[key] = value;
      return;
    }

    changed = true;
    if (to !== null) next[key] = `{{var(${to})}}`;
  });

  return changed ? next : undefined;
};
