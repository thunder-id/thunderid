// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactElement} from 'react';
import Start from './start/Start';
import Variables from './variables/Variables';
import {StaticStepTypes} from '../../../models/steps';

/**
 * Props interface of {@link CommonStaticStepFactory}
 */
export interface CommonStaticStepFactoryPropsInterface {
  /**
   * The resource properties.
   */
  type: StaticStepTypes;
  /**
   * The id of the node, which the handles of a step that has none of its own are named after.
   */
  id?: string;
}

/**
 * Factory for creating common static nodes in the visual editor.
 *
 * @param props - Props injected to the component.
 * @returns The CommonStaticStepFactory component.
 */
export function CommonStaticStepFactory({type, id = ''}: CommonStaticStepFactoryPropsInterface): ReactElement | null {
  if (type === StaticStepTypes.Start) {
    return <Start />;
  }

  if (type === StaticStepTypes.Variables) {
    return <Variables id={id} />;
  }

  return null;
}

export default CommonStaticStepFactory;
