// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import VerificationStatus from '../VerificationStatus';

const VERIFIED_AT = '2026-10-16T08:30:00Z';

describe('VerificationStatus', () => {
  it('shows Verified for a verified value', () => {
    render(<VerificationStatus verifiedAt={VERIFIED_AT} />);

    expect(screen.getByText('Verified')).toBeInTheDocument();
    expect(screen.queryByText('Unverified')).not.toBeInTheDocument();
  });

  it('shows Unverified when the value has no verification time', () => {
    render(<VerificationStatus />);

    expect(screen.getByText('Unverified')).toBeInTheDocument();
    expect(screen.queryByText('Verified')).not.toBeInTheDocument();
  });

  it('gives the verification time in a tooltip that keyboard users can reach', async () => {
    const user = userEvent.setup();
    render(<VerificationStatus verifiedAt={VERIFIED_AT} />);

    await user.tab();

    expect(await screen.findByText(`Verified on ${new Date(VERIFIED_AT).toLocaleString()}`)).toBeInTheDocument();
  });

  it('shows no tooltip and takes no focus while unverified', async () => {
    const user = userEvent.setup();
    render(<VerificationStatus />);

    await user.hover(screen.getByText('Unverified'));
    await user.tab();

    expect(screen.queryByText(/Verified on/)).not.toBeInTheDocument();
    expect(document.body).toHaveFocus();
  });
});
