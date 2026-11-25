// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
 * // In hooks/useOrganizationUnitRoutes.ts, below `OrganizationUnitRoutePaths` and `defaultOrganizationUnitRoutePaths`:
 *
 * export default function useOrganizationUnitRoutes() {
 *   const routes = useRoutes<Partial<OrganizationUnitRoutePaths>>();
 *   return routes.organizationUnits ?? defaultOrganizationUnitRoutePaths.organizationUnits;
 * }
 * ```
 *
 * @public
 */
export default function useRoutes<T extends RoutePaths = RoutePaths>(): T {
  return useContext(RoutesContext) as T;
}
