// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider, RoutesProvider, useRoutes} from '@thunderid/contexts';
import {useMemo, type JSX, type PropsWithChildren} from 'react';
import type {RouteConfig} from '../configs/RouteConfig';

/**
 * Scopes the administrators' pages to the organization: no project is in scope, so they list, show and add the
 * Control Plane's own users. Their user links go to the administrators' routes, which are outside any project.
 */
export default function AdministratorsScope({children}: PropsWithChildren): JSX.Element {
  const routes = useRoutes<RouteConfig>();
  const paths = useMemo(() => ({...routes, users: routes.administrators}), [routes]);
  return (
    <ProjectScopeProvider>
      <RoutesProvider paths={paths}>{children}</RoutesProvider>
    </ProjectScopeProvider>
  );
}
