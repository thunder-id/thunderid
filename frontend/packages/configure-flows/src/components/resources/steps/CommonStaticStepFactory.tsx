// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ReactElement} from 'react';
import Start from './start/Start';
import {StaticStepTypes} from '../../../models/steps';

/**
 * Props interface of {@link CommonStaticStepFactory}
 */
export interface CommonStaticStepFactoryPropsInterface {
  /**
   * The resource properties.
   */
  type: StaticStepTypes;
}

/**
 * Factory for creating common static nodes in the visual editor.
 *
 * @param props - Props injected to the component.
 * @returns The CommonStaticStepFactory component.
 */
export function CommonStaticStepFactory({type}: CommonStaticStepFactoryPropsInterface): ReactElement | null {
  if (type === StaticStepTypes.Start) {
    return <Start />;
  }

  return null;
}

export default CommonStaticStepFactory;
