// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactElement} from 'react';
import type {CommonStepPropertyFactoryPropsInterface} from './CommonStepPropertyFactory';
import CommonStepPropertyFactory from './CommonStepPropertyFactory';
import RulesProperties from './nodes/RulesProperties';
import {StepTypes} from '../../models/steps';

/**
 * Props interface of {@link StepPropertyFactory}
 */
export type StepPropertyFactoryPropsInterface = CommonStepPropertyFactoryPropsInterface;

/**
 * Factory to generate the property configurator for the given resource.
 *
 * @param props - Props injected to the component.
 * @returns The StepPropertyFactory component.
 */
function StepPropertyFactory({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  ...rest
}: StepPropertyFactoryPropsInterface): ReactElement | null {
  switch (resource.type) {
    case StepTypes.Rule:
      return <RulesProperties />;
    default:
      return (
        <CommonStepPropertyFactory
          resource={resource}
          propertyKey={propertyKey}
          propertyValue={propertyValue}
          onChange={onChange}
          {...rest}
        />
      );
  }
}

export default StepPropertyFactory;
