// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type ReactElement} from 'react';
import CommonElementFactory from './CommonElementFactory';
import type {ElementFactoryProps} from '../../../context/FlowBuilderCoreProvider';
import type {Element} from '../../../models/elements';
import {ResourceTypes} from '../../../models/resources';

/**
 * Factory for creating components.
 *
 * @param props - Props injected to the component.
 * @returns The ElementFactory component.
 */
function ElementFactory({resource, stepId, ...rest}: ElementFactoryProps): ReactElement | null {
  // Allow resources without resourceType (template/widget components) or with resourceType === 'ELEMENT'
  if (!resource || (resource.resourceType && resource.resourceType !== ResourceTypes.Element)) {
    return null;
  }

  return <CommonElementFactory resource={resource as Element} stepId={stepId} {...rest} />;
}

export default ElementFactory;
