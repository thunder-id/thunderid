// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {ProjectScopeProvider, RoutesProvider, useProjectScope, useRoutes} from '@thunderid/contexts';
import type {ReactNode} from 'react';
import {describe, expect, it} from 'vitest';
import RouteConfig, {type RouteConfig as RouteConfigPaths} from '../../configs/RouteConfig';
import AdministratorsScope from '../AdministratorsScope';

function Probe(): ReactNode {
  const routes = useRoutes<RouteConfigPaths>();
  const {projectId} = useProjectScope();
  return (
    <>
      <span data-testid="project">{projectId ?? 'none'}</span>
      <span data-testid="users">{routes.users.list()}</span>
      <span data-testid="add">{routes.users.add()}</span>
      <span data-testid="applications">{routes.applications.list()}</span>
    </>
  );
}

describe('AdministratorsScope', () => {
  it("leaves the open project out of scope and sends the users' links to the administrators' routes", () => {
    render(
      <RoutesProvider paths={RouteConfig}>
        <ProjectScopeProvider project={{id: 'project-1'}}>
          <AdministratorsScope>
            <Probe />
          </AdministratorsScope>
        </ProjectScopeProvider>
      </RoutesProvider>,
    );

    expect(screen.getByTestId('project')).toHaveTextContent('none');
    expect(screen.getByTestId('users')).toHaveTextContent('/administrators');
    expect(screen.getByTestId('add')).toHaveTextContent('/administrators/add');
    expect(screen.getByTestId('applications')).toHaveTextContent('/applications');
  });
});
