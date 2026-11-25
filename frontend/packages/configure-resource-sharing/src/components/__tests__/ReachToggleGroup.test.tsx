// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import ReachToggleGroup from '../ReachToggleGroup';

describe('ReachToggleGroup', () => {
  it('explains each reach in a tooltip and reports the picked one', async () => {
    const onChange = vi.fn();
    renderWithProviders(<ReachToggleGroup name="Acme Retail" value="subtree" onChange={onChange} />);

    const unitOnly = screen.getByRole('button', {name: 'Only this OU'});
    expect(unitOnly).toHaveAccessibleDescription("Its child organization units don't get access.");
    expect(screen.getByRole('button', {name: 'Include all OUs below'})).toHaveAccessibleDescription(
      'This OU and every OU below it, now and later.',
    );
    expect(unitOnly.querySelector('svg')).not.toBeNull();

    fireEvent.mouseOver(unitOnly);
    expect(await screen.findByRole('tooltip')).toHaveTextContent("Its child organization units don't get access.");

    fireEvent.click(unitOnly);
    expect(onChange).toHaveBeenCalledWith('unit');
  });

  it('drops the tooltips and their icons when read-only', () => {
    renderWithProviders(<ReachToggleGroup name="Acme Retail" value="unit" disabled onChange={vi.fn()} />);

    const unitOnly = screen.getByRole('button', {name: 'Only this OU'});
    expect(unitOnly).toBeDisabled();
    expect(unitOnly).not.toHaveAccessibleDescription();
    expect(unitOnly.querySelector('svg')).toBeNull();
  });
});
