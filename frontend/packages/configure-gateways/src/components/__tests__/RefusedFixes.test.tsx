// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import type {MissingField, RefusedFix} from '../../utils/missingValues';
import RefusedFixes from '../RefusedFixes';

const clientSecret: MissingField = {
  kind: 'secret',
  name: 'AGENT_CLIENT_SECRET',
  list: false,
  references: [
    {name: 'AGENT_CLIENT_SECRET', kind: 'secret', resourceType: 'agent', resourceId: 'ag-1', field: 'clientSecret'},
  ],
};

function renderFixes(fix: RefusedFix): void {
  render(<RefusedFixes fixes={[fix]} target="Dev" checked valueOf={() => ''} onChange={vi.fn()} />);
}

describe('RefusedFixes', () => {
  it('offers no values for a refusal values cannot fix, nor says it passes, and links to the resource', async () => {
    renderFixes({
      change: {key: 'agent/ag-1', resourceType: 'agent', id: 'ag-1', name: 'Calendar Agent', change: 'updated'},
      failure: {code: 'AGT-1039', message: 'Owner not found'},
      fields: [clientSecret],
    });

    const fix = screen.getByTestId('refused-fix-agent/ag-1');
    expect(within(fix).getByTestId('refused-fix-reason')).toHaveTextContent('Last time: AGT-1039: Owner not found');
    expect(within(fix).queryByTestId('refused-fix-passes')).not.toBeInTheDocument();
    expect(within(fix).queryByLabelText(/AGENT_CLIENT_SECRET/)).not.toBeInTheDocument();
    expect(within(fix).getByRole('link', {name: 'Edit Calendar Agent'})).toHaveAttribute(
      'href',
      '/agents/ag-1?tab=advanced',
    );

    await userEvent.click(within(fix).getByTestId('refused-fix-show-values'));
    expect(within(fix).getByLabelText(/AGENT_CLIENT_SECRET/)).toBeInTheDocument();
  });

  it('offers the values a refusal about them names, and says when it passes with what the target holds now', () => {
    renderFixes({
      change: {key: 'agent/ag-1', resourceType: 'agent', id: 'ag-1', name: 'Calendar Agent', change: 'updated'},
      failure: {code: 'AGT-1010', message: 'clientSecret is not valid'},
      fields: [clientSecret],
    });

    const fix = screen.getByTestId('refused-fix-agent/ag-1');
    expect(within(fix).getByLabelText(/AGENT_CLIENT_SECRET/)).toBeInTheDocument();
    expect(within(fix).getByTestId('refused-fix-passes')).toBeInTheDocument();
    expect(within(fix).queryByTestId('refused-fix-not-values')).not.toBeInTheDocument();
  });
});
