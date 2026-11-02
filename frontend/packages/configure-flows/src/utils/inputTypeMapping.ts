// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ElementTypes} from '../models/elements';

/**
 * API input type for each Console element whose name differs from the type the flow
 * definition accepts. The Checkbox element is the boolean input; the API calls that type
 * BOOLEAN_INPUT and rejects CHECKBOX as an unknown input type.
 */
const API_INPUT_TYPE_BY_ELEMENT_TYPE: Record<string, string> = {
  [ElementTypes.Checkbox]: 'BOOLEAN_INPUT',
};

/**
 * The inverse of {@link API_INPUT_TYPE_BY_ELEMENT_TYPE}, so a flow loaded from the API
 * renders the same Console element the author saved.
 */
const ELEMENT_TYPE_BY_API_INPUT_TYPE: Record<string, string> = {
  BOOLEAN_INPUT: ElementTypes.Checkbox,
};

export const toApiInputType = (elementType: string): string =>
  API_INPUT_TYPE_BY_ELEMENT_TYPE[elementType] ?? elementType;

export const toElementType = (apiInputType: string): string =>
  ELEMENT_TYPE_BY_API_INPUT_TYPE[apiInputType] ?? apiInputType;
