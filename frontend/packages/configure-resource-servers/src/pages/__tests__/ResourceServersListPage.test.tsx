// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider} from '@thunderid/contexts';
import {renderWithProviders, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import ResourceServersListPage from '../ResourceServersListPage';

vi.mock('../../components/ResourceServersList', () => ({
  default: () => <div data-testid="resource-servers-list" />,
}));

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('ResourceServersListPage', () => {
  it('shows the add button by default', () => {
    renderWithProviders(<ResourceServersListPage />);

    expect(screen.getByRole('button', {name: 'Add resource server'})).toBeInTheDocument();
  });

  it('hides the add button when the admin may not create resource servers', () => {
    renderWithProviders(
      <CapabilitiesProvider capabilities={viewOnly}>
        <ResourceServersListPage />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('button', {name: 'Add resource server'})).not.toBeInTheDocument();
  });
});
