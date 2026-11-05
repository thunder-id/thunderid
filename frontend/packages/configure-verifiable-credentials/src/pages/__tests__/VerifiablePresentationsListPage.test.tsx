// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import VerifiablePresentationsListPage from '../VerifiablePresentationsListPage';

vi.mock('../../components/VerifiablePresentationsList', () => ({
  default: ({showWalletQr}: {showWalletQr?: boolean}) => (
    <div data-testid="presentations-list" data-show-wallet-qr={String(showWalletQr)} />
  ),
}));

describe('VerifiablePresentationsListPage', () => {
  it('hides the wallet QR action in the list by default', () => {
    render(<VerifiablePresentationsListPage />);

    expect(screen.getByTestId('presentations-list')).toHaveAttribute('data-show-wallet-qr', 'false');
  });

  it('passes showWalletQr through to the list', () => {
    render(<VerifiablePresentationsListPage showWalletQr />);

    expect(screen.getByTestId('presentations-list')).toHaveAttribute('data-show-wallet-qr', 'true');
  });
});
