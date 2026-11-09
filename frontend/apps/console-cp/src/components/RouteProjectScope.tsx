// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PROJECT_HANDLE_PARAM, SelectedProjectScope} from '@thunderid/configure-projects';
import {RoutesProvider} from '@thunderid/contexts';
import {useMemo, type JSX} from 'react';
import {Outlet, useParams} from 'react-router';
import {projectRouteConfig} from '../configs/RouteConfig';

/**
 * The pages of the project the route names: they work in that project, and every page they link to
 * that works in a project stays under the project's path.
 */
export default function RouteProjectScope(): JSX.Element {
  const handle = useParams()[PROJECT_HANDLE_PARAM] ?? '';
  const paths = useMemo(() => projectRouteConfig(handle), [handle]);
  return (
    <RoutesProvider paths={paths}>
      <SelectedProjectScope>
        <Outlet />
      </SelectedProjectScope>
    </RoutesProvider>
  );
}
