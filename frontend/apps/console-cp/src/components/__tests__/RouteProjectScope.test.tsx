// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {useRoutes} from '@thunderid/contexts';
import type {ReactNode} from 'react';
import {MemoryRouter, Route, Routes} from 'react-router';
import {describe, expect, it, vi} from 'vitest';
import type {RouteConfig} from '../../configs/RouteConfig';
import RouteProjectScope from '../RouteProjectScope';

vi.mock('@thunderid/configure-projects', () => ({
  PROJECT_HANDLE_PARAM: 'projectHandle',
  SelectedProjectScope: ({children}: {children: ReactNode}) => <div data-testid="scope">{children}</div>,
}));

function Links(): ReactNode {
  const routes = useRoutes<RouteConfig>();
  return (
    <>
      <span data-testid="applications">{routes.applications.list()}</span>
      <span data-testid="connections">{routes.connections.list()}</span>
    </>
  );
}

describe('RouteProjectScope', () => {
  it("scopes the pages to the project the route names, and keeps the project's pages under its path", () => {
    render(
      <MemoryRouter initialEntries={['/project/shop/applications']}>
        <Routes>
          <Route path="/project/:projectHandle" element={<RouteProjectScope />}>
            <Route path="applications" element={<Links />} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByTestId('scope')).toBeInTheDocument();
    expect(screen.getByTestId('applications')).toHaveTextContent('/project/shop/applications');
    expect(screen.getByTestId('connections')).toHaveTextContent('/connections');
  });
});
