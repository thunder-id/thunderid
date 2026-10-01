// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this feature needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * feature rendered standalone in a unit test), `useFlowRoutes` falls back to
 * `defaultFlowRoutePaths` below.
 *
 * @public
 */
export interface FlowRoutePaths {
  flows: {
    list: () => string;
    create: () => string;
    detail: (flowId: string) => string;
  };
}

/**
 * Default flow paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultFlowRoutePaths: FlowRoutePaths = {
  flows: {
    list: () => '/flows',
    create: () => '/flows/create',
    detail: (flowId) => `/flows/${flowId}`,
  },
};

/**
 * Resolves the flow route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this feature's own defaults.
 *
 * Components should never hardcode flow destination paths; they should call this hook and build
 * the destination from the returned functions instead.
 *
 * @public
 */
export default function useFlowRoutes(): FlowRoutePaths {
  const routes = useRoutes<Partial<FlowRoutePaths>>();
  return {flows: routes.flows ?? defaultFlowRoutePaths.flows};
}
