// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState, type JSX} from 'react';
import {describe, it, expect, vi} from 'vitest';
import LockLadderRows from '../LockLadderRows';

/** Holds the ladder like a draft does, so each emitted list comes back as the prop. */
function Harness({initial, onChange}: {initial: number[]; onChange: (durations: number[]) => void}): JSX.Element {
  const [durations, setDurations] = useState(initial);
  return (
    <LockLadderRows
      durations={durations}
      onChange={(next) => {
        setDurations(next);
        onChange(next);
      }}
    />
  );
}

const renderLadder = (initial: number[]) => {
  const onChange = vi.fn();
  render(<Harness initial={initial} onChange={onChange} />);
  return onChange;
};

describe('LockLadderRows', () => {
  // The reason for the separate control: a held end used to remove the Add button, which made the
  // rows look like a fixed set.
  it('keeps inviting another duration when later locks are held until unlocked', async () => {
    const onChange = renderLadder([60, 300, 0]);

    expect(screen.getByRole('radio', {name: 'Hold later locks until an administrator unlocks'})).toBeChecked();
    await userEvent.click(screen.getByRole('button', {name: 'Add duration'}));

    // The new rung goes before the held end, not after it.
    expect(onChange).toHaveBeenLastCalledWith([60, 300, 600, 0]);
  });

  it('appends the held end when it is chosen and removes it when repeating', async () => {
    const onChange = renderLadder([60, 300, 900]);

    expect(screen.getByRole('radio', {name: 'Keep using the last duration'})).toBeChecked();
    await userEvent.click(screen.getByRole('radio', {name: 'Hold later locks until an administrator unlocks'}));
    expect(onChange).toHaveBeenLastCalledWith([60, 300, 900, 0]);

    await userEvent.click(screen.getByRole('radio', {name: 'Keep using the last duration'}));
    expect(onChange).toHaveBeenLastCalledWith([60, 300, 900]);
  });

  // Only a repeating last rung covers the locks after it.
  it('names the repeating rung only when the last duration repeats', () => {
    renderLadder([60, 300, 900]);
    expect(screen.getByText('3rd and later')).toBeVisible();
  });

  it('numbers every timed rung plainly when later locks are held', () => {
    renderLadder([60, 300, 0]);
    expect(screen.getByText('2nd lock')).toBeVisible();
    expect(screen.queryByText(/and later/)).not.toBeInTheDocument();
  });

  // Durations in the rows always lift on their own, so the zero is not on their unit lists.
  it('does not offer the held lock as a row duration', async () => {
    renderLadder([60, 300]);
    await userEvent.click(within(screen.getByTestId('lock-ladder-row-0-unit')).getByRole('combobox'));

    expect(await screen.findByRole('option', {name: 'Minute'})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: 'Until an administrator unlocks'})).not.toBeInTheDocument();
  });

  // A stored ladder of only [0] has no timed rung to follow, so there is no end to choose.
  it('shows a ladder that holds every lock without losing it', async () => {
    const onChange = renderLadder([0]);

    expect(screen.getByText('Every lock is held until an administrator unlocks it.')).toBeVisible();
    expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Add duration'}));

    expect(onChange).toHaveBeenLastCalledWith([300, 0]);
  });

  it('reaches a ladder that holds every lock by removing its only timed rung', async () => {
    const onChange = renderLadder([300, 0]);
    await userEvent.click(screen.getByRole('button', {name: 'Remove the 1st lock duration'}));

    expect(onChange).toHaveBeenLastCalledWith([0]);
  });

  // A zero stored before the last rung is valid, so it stays visible and editing another row keeps it.
  it('keeps a held lock stored mid-list while another row changes', async () => {
    const onChange = renderLadder([300, 0, 900]);

    expect(within(screen.getByTestId('lock-ladder-row-1-unit')).getByRole('combobox')).toHaveTextContent(
      'Until an administrator unlocks',
    );
    await userEvent.click(screen.getByRole('button', {name: 'Remove the 1st lock duration'}));

    expect(onChange).toHaveBeenLastCalledWith([0, 900]);
  });

  it('offers a longer duration while there is a single one', () => {
    renderLadder([900]);

    expect(screen.queryByText('1st lock')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Add a longer duration for repeated locks'})).toBeVisible();
    expect(screen.getByRole('radio', {name: 'Keep using the last duration'})).toBeChecked();
  });
});
