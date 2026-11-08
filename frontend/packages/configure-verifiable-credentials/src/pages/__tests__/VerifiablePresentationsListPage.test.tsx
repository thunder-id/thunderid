// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider, ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import VerifiablePresentationsListPage from '../VerifiablePresentationsListPage';

vi.mock('../../components/VerifiablePresentationsList', () => ({
  default: ({showWalletQr}: {showWalletQr?: boolean}) => (
    <div data-testid="presentations-list" data-show-wallet-qr={String(showWalletQr)} />
  ),
}));

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('VerifiablePresentationsListPage', () => {
  it('hides the Add definition button when the admin may not create definitions', () => {
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <VerifiablePresentationsListPage />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByRole('button', {name: 'Add definition'})).not.toBeInTheDocument();
  });

  it('hides the wallet QR action in the list by default', () => {
    render(<VerifiablePresentationsListPage />);

    expect(screen.getByTestId('presentations-list')).toHaveAttribute('data-show-wallet-qr', 'false');
  });

  it('offers the create action by default', () => {
    render(<VerifiablePresentationsListPage />);

    expect(screen.getByTestId('vp-add-button')).toBeInTheDocument();
  });

  it('offers no create action outside a project when definitions are created in projects only', () => {
    render(<VerifiablePresentationsListPage createInProjectsOnly />);

    expect(screen.queryByTestId('vp-add-button')).not.toBeInTheDocument();
  });

  it('offers the create action inside a project when definitions are created in projects only', () => {
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <VerifiablePresentationsListPage createInProjectsOnly />
      </ProjectScopeProvider>,
    );

    expect(screen.getByTestId('vp-add-button')).toBeInTheDocument();
  });

  it('passes showWalletQr through to the list', () => {
    render(<VerifiablePresentationsListPage showWalletQr />);

    expect(screen.getByTestId('presentations-list')).toHaveAttribute('data-show-wallet-qr', 'true');
  });
});
