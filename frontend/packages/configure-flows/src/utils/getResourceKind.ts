// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ResourceTypes, type Resource} from '../models/resources';
import {StepCategories, StepTypes} from '../models/steps';

/**
 * Kinds of resources shown on the resource panel.
 */
export const ResourceKinds = {
  Widget: 'widget',
  View: 'view',
  Step: 'step',
  Executor: 'executor',
  Element: 'element',
} as const;

export type ResourceKinds = (typeof ResourceKinds)[keyof typeof ResourceKinds];

/**
 * Resolves the kind of a resource. Feeds the resource panel's search haystack so
 * queries like "executor" or "widget" match resources by their kind.
 *
 * @param resource - Resource to resolve the kind for.
 * @returns The resource kind.
 */
function getResourceKind(resource: Resource): ResourceKinds {
  if (resource.resourceType === ResourceTypes.Widget) {
    return ResourceKinds.Widget;
  }

  if (resource.resourceType === ResourceTypes.Element) {
    return ResourceKinds.Element;
  }

  if (resource.category === StepCategories.Executor) {
    return ResourceKinds.Executor;
  }

  if (resource.type === StepTypes.View) {
    return ResourceKinds.View;
  }

  return ResourceKinds.Step;
}

export default getResourceKind;
