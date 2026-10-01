// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactElement} from 'react';
import CommonElementPropertyFactory from './CommonElementPropertyFactory';
import CommonWidgetPropertyFactory from './CommonWidgetPropertyFactory';
import StepPropertyFactory from './StepPropertyFactory';
import {ResourceTypes, type Resource} from '../../models/resources';

/**
 * Props interface of {@link ResourcePropertyFactory}
 */
export interface ResourcePropertyFactoryPropsInterface {
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The key of the property.
   */
  propertyKey: string;
  /**
   * The value of the property.
   */
  propertyValue: unknown;
  /**
   * The event handler for the property change.
   * @param propertyKey - The key of the property.
   * @param newValue - The new value of the property.
   * @param resource - The resource associated with the property.
   */
  onChange: (propertyKey: string, newValue: unknown, resource: Resource) => void;
  /**
   * Additional props.
   */
  [key: string]: unknown;
}

/**
 * Factory to generate the property configurator for the given password recovery flow resource.
 *
 * @param props - Props injected to the component.
 * @returns The ResourcePropertyFactory component.
 */
function ResourcePropertyFactory({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  ...rest
}: ResourcePropertyFactoryPropsInterface): ReactElement | null {
  switch (resource.resourceType) {
    case ResourceTypes.Element:
      return (
        <CommonElementPropertyFactory
          resource={resource}
          propertyKey={propertyKey}
          propertyValue={propertyValue}
          onChange={onChange}
          {...rest}
        />
      );
    case ResourceTypes.Step:
      return (
        <StepPropertyFactory
          resource={resource}
          propertyKey={propertyKey}
          propertyValue={propertyValue}
          onChange={onChange}
          {...rest}
        />
      );
    case ResourceTypes.Widget:
      return (
        <CommonWidgetPropertyFactory
          resource={resource}
          propertyKey={propertyKey}
          propertyValue={propertyValue}
          onChange={onChange}
          {...rest}
        />
      );
    default:
      return null;
  }
}

export default ResourcePropertyFactory;
