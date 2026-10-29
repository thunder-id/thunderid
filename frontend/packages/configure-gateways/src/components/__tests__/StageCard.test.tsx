// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import StageCard, {type StageCardProps} from '../StageCard';

function renderStage(props: Partial<StageCardProps>): void {
  render(
    <StageCard
      name="Staging"
      version={{number: 3, hash: 'abc123'}}
      onExplore={vi.fn()}
      onConfigure={vi.fn()}
      onRedeploy={vi.fn()}
      {...props}
    />,
  );
}

describe('StageCard refused resources', () => {
  it('says how many resources the stage refused, naming them on hover', async () => {
    renderStage({
      refused: [
        {key: 'application/app-2', resourceType: 'application', id: 'app-2', name: 'Billing', state: 'failed'},
        {key: 'flow/f-9', resourceType: 'flow', id: 'f-9', state: 'failed'},
      ],
    });

    const refused = screen.getByTestId('stage-refused');
    expect(refused).toHaveTextContent('2 refused, not running');
    await userEvent.hover(refused);
    expect(await screen.findByText('Billing, f-9')).toBeInTheDocument();
  });

  it('says nothing of refusals when there are none', () => {
    renderStage({refused: []});

    expect(screen.queryByTestId('stage-refused')).not.toBeInTheDocument();
  });
});

describe('StageCard redeploy', () => {
  it('offers a redeploy while values changed since the deploy', () => {
    renderStage({changed: {secrets: ['APPLICATION_MY_APP_CLIENT_SECRET']}});

    expect(screen.getByTestId('stage-redeploy')).toBeEnabled();
  });

  it('disables the redeploy when no value changed', () => {
    renderStage({changed: {}});

    expect(screen.getByTestId('stage-redeploy')).toBeDisabled();
  });

  it('offers to check again, not that nothing changed, when the check failed', async () => {
    const retry = vi.fn();
    renderStage({changedCheckFailed: true, onRetryChangedCheck: retry});

    expect(screen.queryByTestId('stage-redeploy')).not.toBeInTheDocument();
    await userEvent.hover(screen.getByTestId('stage-changed-check-retry'));
    expect(await screen.findByText(/Could not check whether any value changed/)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing to redeploy/)).not.toBeInTheDocument();

    await userEvent.click(screen.getByTestId('stage-changed-check-retry'));
    expect(retry).toHaveBeenCalledTimes(1);
  });
});
