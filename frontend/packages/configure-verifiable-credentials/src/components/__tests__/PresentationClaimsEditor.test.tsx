// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {ClaimRow} from '../../models/presentation-claims';
import ClaimsEditor from '../PresentationClaimsEditor';

const claims: ClaimRow[] = [{id: 'claim-1', name: 'country', requirement: 'mandatory', values: ['LK', 'SG']}];

describe('ClaimsEditor', () => {
  it('lets each claim value be removed', () => {
    render(<ClaimsEditor claims={claims} onChange={vi.fn()} />);

    expect(screen.getAllByTestId('CancelIcon')).toHaveLength(2);
  });

  it('offers no claim value removal when read-only', () => {
    render(<ClaimsEditor claims={claims} onChange={vi.fn()} readOnly />);

    expect(screen.getByText('LK')).toBeInTheDocument();
    expect(screen.queryByTestId('CancelIcon')).not.toBeInTheDocument();
  });
});
