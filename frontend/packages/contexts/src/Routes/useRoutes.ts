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

import {useContext} from 'react';
import RoutesContext, {type RoutePaths} from './RoutesContext';

/**
 * React hook for reading the host application's route configuration.
 *
 * Returns whatever was supplied to the nearest `RoutesProvider`, or an empty object if none is
 * present. Callers should treat the result as partial: feature packages should read their own
 * domain key and fall back to a local default rather than assuming every key is populated.
 *
 * @typeParam T - The shape of the slice(s) this caller expects, e.g. `Partial<OrganizationUnitRoutePaths>`
 * @returns The current route configuration, cast to `T`
 *
 * @example
 * A feature package building its own typed accessor on top of this hook:
 * ```ts
 * import { useRoutes } from '@thunderid/contexts';
 * import { defaultOrganizationUnitRoutes, type OrganizationUnitRoutePaths } from './routes';
 *
 * export default function useOrganizationUnitRoutes() {
 *   const routes = useRoutes<Partial<OrganizationUnitRoutePaths>>();
 *   return routes.organizationUnits ?? defaultOrganizationUnitRoutes.organizationUnits;
 * }
 * ```
 *
 * @public
 */
export default function useRoutes<T extends RoutePaths = RoutePaths>(): T {
  return useContext(RoutesContext) as T;
}
