// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import VerifiableCredentialsListPage from '../VerifiableCredentialsListPage';

vi.mock('../../components/VerifiableCredentialsList', () => ({
  default: ({showWalletQr}: {showWalletQr?: boolean}) => (
    <div data-testid="credentials-list" data-show-wallet-qr={String(showWalletQr)} />
  ),
}));

describe('VerifiableCredentialsListPage', () => {
  it('hides the wallet QR action in the list by default', () => {
    render(<VerifiableCredentialsListPage />);

    expect(screen.getByTestId('credentials-list')).toHaveAttribute('data-show-wallet-qr', 'false');
  });

  it('passes showWalletQr through to the list', () => {
    render(<VerifiableCredentialsListPage showWalletQr />);

    expect(screen.getByTestId('credentials-list')).toHaveAttribute('data-show-wallet-qr', 'true');
  });
});
