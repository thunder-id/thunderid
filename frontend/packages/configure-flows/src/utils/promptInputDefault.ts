// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SENSITIVE_INPUT_TYPES} from './inputTypeMapping';
import type {Element} from '../models/elements';

/**
 * Reports whether an input element can carry a default value. Password and OTP inputs cannot hold one,
 * so their values never reach the flow definition.
 *
 * @param type - The element type.
 * @returns True when the element can carry a default.
 */
export const canCarryDefault = (type: string): boolean => !SENSITIVE_INPUT_TYPES.has(type);

/**
 * Reads the default value an input element carries for its prompt input.
 *
 * @param component - The input element.
 * @returns The default value, or undefined when the element holds none or cannot hold one.
 */
export const getPromptInputDefault = (component: Element): string | undefined => {
  const {value} = component as Element & {value?: unknown};

  return canCarryDefault(component.type) && typeof value === 'string' && value !== '' ? value : undefined;
};
