// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import isDisplayableProperty from './isDisplayableProperty';
import type {SchemaPropertyInput} from '../models/property-definition';

/**
 * Whether a display attribute still points at a property the API accepts, given the schema
 * properties about to be saved. A top-level name must match a displayable property. A nested
 * path such as `address.city` is kept while its top-level object property exists, since this
 * editor does not change nested properties.
 *
 * @param displayAttribute - The display attribute the agent type currently has
 * @param properties - The schema properties about to be saved
 * @returns True when the display attribute can be sent with the schema
 *
 * @example
 * ```ts
 * const keepDisplay = isDisplayAttributeAvailable('name', properties);
 * ```
 *
 * @public
 */
export default function isDisplayAttributeAvailable(
  displayAttribute: string,
  properties: SchemaPropertyInput[],
): boolean {
  const [root, ...nested] = displayAttribute.split('.');

  if (nested.length > 0) {
    return properties.some((property) => property.type === 'object' && property.name.trim() === root);
  }

  return properties.some((property) => isDisplayableProperty(property) && property.name.trim() === root);
}
