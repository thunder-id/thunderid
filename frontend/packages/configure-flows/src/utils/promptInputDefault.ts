// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ElementTypes, type Element} from '../models/elements';

/**
 * Input elements that can carry a default value. Password and OTP inputs cannot hold one, and the
 * choice elements take their value from their options.
 */
export const DEFAULT_VALUE_ELEMENT_TYPES: ReadonlySet<string> = new Set<string>([
  ElementTypes.TextInput,
  ElementTypes.EmailInput,
  ElementTypes.PhoneInput,
  ElementTypes.NumberInput,
  ElementTypes.DateInput,
]);

/**
 * Reads the default value an input element carries for its prompt input.
 *
 * @param component - The input element.
 * @returns The default value, or undefined when the element holds none or cannot hold one.
 */
export const getPromptInputDefault = (component: Element): string | undefined => {
  const {value} = component as Element & {value?: unknown};

  return DEFAULT_VALUE_ELEMENT_TYPES.has(component.type) && typeof value === 'string' && value !== ''
    ? value
    : undefined;
};
