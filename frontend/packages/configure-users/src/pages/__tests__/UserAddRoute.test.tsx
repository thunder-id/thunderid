// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes, AdministrationProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import UserAddRoute from '../UserAddRoute';

vi.mock('../UserAddFormPage', () => ({
  default: () => <div data-testid="user-add-form-page" />,
}));

vi.mock('../UserAddPage', () => ({
  default: () => <div data-testid="user-add-flow-page" />,
}));

describe('UserAddRoute', () => {
  it('shows the flow-driven journey when no administration mode is declared', () => {
    render(<UserAddRoute />);

    expect(screen.getByTestId('user-add-flow-page')).toBeInTheDocument();
    expect(screen.queryByTestId('user-add-form-page')).not.toBeInTheDocument();
  });

  it('shows the flow-driven journey in flow mode', () => {
    render(
      <AdministrationProvider administration={{mode: AdministrationModes.FLOW}}>
        <UserAddRoute />
      </AdministrationProvider>,
    );

    expect(screen.getByTestId('user-add-flow-page')).toBeInTheDocument();
  });

  it('shows the form in native mode', () => {
    render(
      <AdministrationProvider administration={{mode: AdministrationModes.NATIVE}}>
        <UserAddRoute />
      </AdministrationProvider>,
    );

    expect(screen.getByTestId('user-add-form-page')).toBeInTheDocument();
    expect(screen.queryByTestId('user-add-flow-page')).not.toBeInTheDocument();
  });

  it('follows an operation-level override over the console-wide mode', () => {
    render(
      <AdministrationProvider
        administration={{
          mode: AdministrationModes.NATIVE,
          features: {users: {operations: {create: AdministrationModes.FLOW}}},
        }}
      >
        <UserAddRoute />
      </AdministrationProvider>,
    );

    expect(screen.getByTestId('user-add-flow-page')).toBeInTheDocument();
  });
});
