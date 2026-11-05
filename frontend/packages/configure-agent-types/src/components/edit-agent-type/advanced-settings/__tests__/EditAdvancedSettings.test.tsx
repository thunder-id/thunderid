// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import EditAdvancedSettings from '../EditAdvancedSettings';

describe('EditAdvancedSettings', () => {
  it('should offer deleting the agent type in the danger zone', async () => {
    const user = userEvent.setup();
    const onDeleteClick = vi.fn();
    render(<EditAdvancedSettings onDeleteClick={onDeleteClick} />);

    expect(screen.getByText('Danger zone')).toBeInTheDocument();
    expect(screen.getByText('Delete agent type')).toBeInTheDocument();
    expect(
      screen.getByText(/only when no agent is of this type and no application, agent, or flow lists it/i),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(onDeleteClick).toHaveBeenCalledTimes(1);
  });

  it('should hide the danger zone when deleting is not offered', () => {
    render(<EditAdvancedSettings />);

    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });
});
