// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactElement} from 'react';
import type {Resource} from '../../models/resources';

/**
 * Props interface of {@link CommonWidgetPropertyFactory}
 */
export interface CommonWidgetPropertyFactoryPropsInterface {
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The key of the property.
   */
  // propertyKey: string;
  /**
   * The value of the property.
   */
  // propertyValue: unknown;
  /**
   * The event handler for the property change.
   * @param propertyKey - The key of the property.
   * @param newValue - The new value of the property.
   * @param resource - The resource associated with the property.
   */
  // onChange: (propertyKey: string, newValue: unknown, resource: Resource) => void;
  /**
   * Additional props.
   */
  [key: string]: unknown;
}

/**
 * Factory to generate the common property configurator for the given widget.
 * TODO: Implement the common widgets like RE-CAPTCHA, etc.
 *
 * @param props - Props injected to the component.
 * @returns The CommonWidgetPropertyFactory component.
 */
function CommonWidgetPropertyFactory({resource}: CommonWidgetPropertyFactoryPropsInterface): ReactElement | null {
  switch (resource.type) {
    default:
      return null;
  }
}

export default CommonWidgetPropertyFactory;
