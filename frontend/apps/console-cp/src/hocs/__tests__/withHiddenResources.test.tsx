// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {useHiddenResources} from '@thunderid/contexts';
import type {JSX} from 'react';
import {describe, expect, it} from 'vitest';
import withHiddenResources from '../withHiddenResources';

function Probe(): JSX.Element {
  const {isHiddenApplication, isHiddenResourceServer, isHiddenResource} = useHiddenResources();
  return (
    <ul>
      <li data-testid="admin-console">{String(isHiddenApplication('ADMIN_CONSOLE'))}</li>
      <li data-testid="console">{String(isHiddenApplication('CONSOLE'))}</li>
      <li data-testid="data-plane">{String(isHiddenResourceServer('var:DATA_PLANE_URL'))}</li>
      <li data-testid="app-id">{String(isHiddenResource('application', '01a19b04-d7f7-7790-ad53-9139664d0a6a'))}</li>
      <li data-testid="rs-id">{String(isHiddenResource('resource_server', '01a1864b-ef42-7c41-b0a2-97f53fd3b1e4'))}</li>
    </ul>
  );
}

describe('withHiddenResources', () => {
  it('hides the Admin Console application and the Data Plane resource server', () => {
    const Wrapped = withHiddenResources(Probe);

    render(<Wrapped />);

    expect(screen.getByTestId('admin-console')).toHaveTextContent('true');
    expect(screen.getByTestId('data-plane')).toHaveTextContent('true');
    expect(screen.getByTestId('app-id')).toHaveTextContent('true');
    expect(screen.getByTestId('rs-id')).toHaveTextContent('true');
  });

  it("leaves the console's own application shown", () => {
    const Wrapped = withHiddenResources(Probe);

    render(<Wrapped />);

    expect(screen.getByTestId('console')).toHaveTextContent('false');
  });
});
