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
      category="user"
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

    expect(screen.getByRole('radio', {name: /^Require account recovery or administrator intervention/})).toBeChecked();
    await userEvent.click(screen.getByRole('button', {name: 'Add duration'}));

    // The new rung goes before the held end, not after it.
    expect(onChange).toHaveBeenLastCalledWith([60, 300, 600, 0]);
  });

  it('appends the held end when it is chosen and removes it when repeating', async () => {
    const onChange = renderLadder([60, 300, 900]);

    expect(screen.getByRole('radio', {name: /^Continue using the final duration/})).toBeChecked();
    await userEvent.click(screen.getByRole('radio', {name: /^Require account recovery or administrator intervention/}));
    expect(onChange).toHaveBeenLastCalledWith([60, 300, 900, 0]);

    await userEvent.click(screen.getByRole('radio', {name: /^Continue using the final duration/}));
    expect(onChange).toHaveBeenLastCalledWith([60, 300, 900]);
  });

  // Only a repeating last rung covers the locks after it.
  it('names the repeating rung only when the last duration repeats', () => {
    renderLadder([60, 300, 900]);
    expect(screen.getByText('3rd and later')).toBeVisible();
  });

  it('numbers every timed rung plainly when later locks are held', () => {
    renderLadder([60, 300, 0]);
    expect(screen.getByText('2nd lockout')).toBeVisible();
    expect(screen.queryByText(/and later/)).not.toBeInTheDocument();
  });

  // The ladder has no cap, so positions past the teens take the English ordinal suffixes.
  it('names positions past the first few with the right ordinal suffix', () => {
    renderLadder(Array.from({length: 23}, (_, index) => 60 * (index + 1)));

    ['4th', '11th', '12th', '13th', '21st', '22nd'].forEach((position) => {
      expect(screen.getByText(`${position} lockout`)).toBeInTheDocument();
    });
    expect(screen.getByText('23rd and later')).toBeInTheDocument();
  });

  // Durations in the rows always lift on their own, so the zero is not on their unit lists.
  it('does not offer the held lock as a row duration', async () => {
    renderLadder([60, 300]);
    await userEvent.click(within(screen.getByTestId('lock-ladder-row-0-unit')).getByRole('combobox'));

    expect(await screen.findByRole('option', {name: 'Minute'})).toBeInTheDocument();
    expect(
      screen.queryByRole('option', {name: 'Until account recovery or an administrator unlocks'}),
    ).not.toBeInTheDocument();
  });

  // A stored ladder of only [0] has no timed rung to follow, so there is no end to choose. The policy
  // summary above already says each lockout is held, so the ladder adds no hint of its own.
  it('shows a ladder that holds every lock without losing it', async () => {
    const onChange = renderLadder([0]);

    expect(screen.queryByText(/^Each lockout lasts/)).not.toBeInTheDocument();
    expect(screen.queryByText('How long a lockout lasts before it ends on its own.')).not.toBeInTheDocument();
    expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Add duration'}));

    expect(onChange).toHaveBeenLastCalledWith([300, 0]);
  });

  it('reaches a ladder that holds every lock by removing its only timed rung', async () => {
    const onChange = renderLadder([300, 0]);
    await userEvent.click(screen.getByRole('button', {name: 'Remove the 1st lockout duration'}));

    expect(onChange).toHaveBeenLastCalledWith([0]);
  });

  // A zero stored before the last rung is valid, so it stays visible and editing another row keeps it.
  it('keeps a held lock stored mid-list while another row changes', async () => {
    const onChange = renderLadder([300, 0, 900]);

    expect(within(screen.getByTestId('lock-ladder-row-1-unit')).getByRole('combobox')).toHaveTextContent(
      'Until account recovery or an administrator unlocks',
    );
    await userEvent.click(screen.getByRole('button', {name: 'Remove the 1st lockout duration'}));

    expect(onChange).toHaveBeenLastCalledWith([0, 900]);
  });

  it('offers a longer duration while there is a single one', () => {
    renderLadder([900]);

    expect(screen.getByText('How long a lockout lasts before it ends on its own.')).toBeVisible();
    expect(screen.queryByText('1st lockout')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Add a longer duration for subsequent lockouts'})).toBeVisible();
    expect(screen.getByRole('radio', {name: /^Continue using the final duration/})).toBeChecked();
  });
});
