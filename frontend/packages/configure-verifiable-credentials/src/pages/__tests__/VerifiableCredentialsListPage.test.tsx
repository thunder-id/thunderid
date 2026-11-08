// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import VerifiableCredentialsListPage from '../VerifiableCredentialsListPage';

vi.mock('../../components/VerifiableCredentialsList', () => ({
  default: ({showWalletQr}: {showWalletQr?: boolean}) => (
    <div data-testid="credentials-list" data-show-wallet-qr={String(showWalletQr)} />
  ),
}));

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('VerifiableCredentialsListPage', () => {
  it('hides the Add template button when the admin may not create templates', () => {
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <VerifiableCredentialsListPage />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('button', {name: 'Add template'})).not.toBeInTheDocument();
  });

  it('hides the wallet QR action in the list by default', () => {
    render(<VerifiableCredentialsListPage />);

    expect(screen.getByTestId('credentials-list')).toHaveAttribute('data-show-wallet-qr', 'false');
  });

  it('offers the create action by default', () => {
    render(<VerifiableCredentialsListPage />);

    expect(screen.getByTestId('vc-add-button')).toBeInTheDocument();
  });

  it('offers no create action outside a project when configurations are created in projects only', () => {
    render(<VerifiableCredentialsListPage createInProjectsOnly />);

    expect(screen.queryByTestId('vc-add-button')).not.toBeInTheDocument();
  });

  it('offers the create action inside a project when configurations are created in projects only', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <VerifiableCredentialsListPage createInProjectsOnly />
      </ProjectScopeProvider>,
    );

    expect(screen.getByTestId('vc-add-button')).toBeInTheDocument();
  });

  it('passes showWalletQr through to the list', () => {
    render(<VerifiableCredentialsListPage showWalletQr />);

    expect(screen.getByTestId('credentials-list')).toHaveAttribute('data-show-wallet-qr', 'true');
  });
});
