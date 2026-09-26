// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, afterEach} from 'vitest';
import VerifiableCredentialsList from '../VerifiableCredentialsList';

vi.mock('../../api/useGetVerifiableCredentials', () => ({
  default: (): unknown => ({
    data: {credentials: [{handle: 'pid', id: '01900000-0000-7000-8000-000000000001', name: 'PID'}]},
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  }),
}));

describe('VerifiableCredentialsList', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  // Issuing a credential offer is served by the OpenID4VCI endpoints, which a Control Plane does not
  // have. That console passes no renderer, and the listing must not offer the action at all rather
  // than present a button that cannot work.
  it('renders no offer action when the console supplies no renderer', async () => {
    render(<VerifiableCredentialsList />);

    await waitFor(() => {
      expect(screen.queryByLabelText(/offer/i)).toBeNull();
    });
    expect(screen.queryByTestId('credential-offer-dialog')).toBeNull();
  });

  // A Data Plane console supplies one, and the listing shows it.
  it('renders the offer dialog the console supplies', async () => {
    render(<VerifiableCredentialsList renderOffer={() => <div data-testid="supplied-offer" />} />);

    await waitFor(() => {
      expect(screen.getByTestId('supplied-offer')).toBeTruthy();
    });
  });
});
