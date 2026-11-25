// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isContactAttributeType} from '@thunderid/utils';
import type {SchemaPropertyInput} from '../models/property-definition';

/**
 * Whether a schema property can name agents as the display attribute: it has a name, holds a
 * string, email, mobile, number, or enum value, and is not a credential. The API rejects any other
 * property as the display attribute.
 *
 * @param property - The schema property being edited
 * @returns True when the property can be the display attribute
 *
 * @example
 * ```ts
 * const candidates = properties.filter(isDisplayableProperty);
 * ```
 *
 * @public
 */
export default function isDisplayableProperty(property: SchemaPropertyInput): boolean {
  return (
    (property.type === 'string' ||
      isContactAttributeType(property.type) ||
      property.type === 'number' ||
      property.type === 'enum') &&
    !property.credential &&
    property.name.trim().length > 0
  );
}
