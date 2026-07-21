/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
