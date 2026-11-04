// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ElementTypes, type Element} from '../models/elements';

/**
 * Input type of a boolean field. The Console calls the element a Checkbox.
 */
export const BOOLEAN_INPUT_TYPE = 'BOOLEAN_INPUT';

/**
 * Input types that cannot carry a default or constant value, since the value would be stored in the
 * flow definition. Must stay in step with the sensitive input types in the backend
 * (backend/pkg/thunderidengine/providers/model.go).
 */
export const SENSITIVE_INPUT_TYPES: ReadonlySet<string> = new Set<string>([
  ElementTypes.PasswordInput,
  ElementTypes.OtpInput,
]);

/**
 * API input type for each Console element whose name differs from the type the flow
 * definition accepts. The Checkbox element is the boolean input; the API calls that type
 * BOOLEAN_INPUT and rejects CHECKBOX as an unknown input type.
 */
const API_INPUT_TYPE_BY_ELEMENT_TYPE: Record<string, string> = {
  [ElementTypes.Checkbox]: BOOLEAN_INPUT_TYPE,
};

/**
 * The inverse of {@link API_INPUT_TYPE_BY_ELEMENT_TYPE}, so a flow loaded from the API
 * renders the same Console element the author saved.
 */
const ELEMENT_TYPE_BY_API_INPUT_TYPE: Record<string, string> = {
  [BOOLEAN_INPUT_TYPE]: ElementTypes.Checkbox,
};

export const toApiInputType = (elementType: string): string =>
  API_INPUT_TYPE_BY_ELEMENT_TYPE[elementType] ?? elementType;

export const toElementType = (apiInputType: string): string =>
  ELEMENT_TYPE_BY_API_INPUT_TYPE[apiInputType] ?? apiInputType;

/**
 * Set of input element types for quick lookup
 */
export const INPUT_ELEMENT_TYPES: ReadonlySet<string> = new Set<string>([
  ElementTypes.TextInput,
  ElementTypes.PasswordInput,
  ElementTypes.EmailInput,
  ElementTypes.PhoneInput,
  ElementTypes.NumberInput,
  ElementTypes.DateInput,
  ElementTypes.OtpInput,
  ElementTypes.Checkbox,
  ElementTypes.Dropdown,
  ElementTypes.Select,
  ElementTypes.UserSelect,
]);

/**
 * Returns the identifier an input element is saved under: its name, else its ref, else its id.
 *
 * @param component - The input element.
 * @returns The input identifier.
 */
export function getInputIdentifier(component: Element): string {
  const {name, ref} = component as Element & {name?: unknown; ref?: unknown};
  if (typeof name === 'string') {
    return name;
  }

  return typeof ref === 'string' ? ref : component.id;
}
