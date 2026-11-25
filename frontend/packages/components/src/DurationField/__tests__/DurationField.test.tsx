// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState, type JSX} from 'react';
import {describe, it, expect, vi} from 'vitest';
import DurationField from '../DurationField';

/** Holds the value like a draft does, so each emitted value comes back as the prop. */
function Harness({initial, onChange}: {initial: number; onChange: (seconds: number) => void}): JSX.Element {
  const [seconds, setSeconds] = useState(initial);
  return (
    <DurationField
      label="Lock duration"
      seconds={seconds}
      testId="duration"
      onChange={(next) => {
        setSeconds(next);
        onChange(next);
      }}
    />
  );
}

const unitText = (): string | null => within(screen.getByTestId('duration-unit')).getByRole('combobox').textContent;

describe('DurationField', () => {
  it('keeps the unit while typing a value another unit divides', async () => {
    const onChange = vi.fn();
    render(<Harness initial={300} onChange={onChange} />);
    const input = screen.getByTestId('duration');

    await userEvent.clear(input);
    await userEvent.type(input, '60');

    expect(onChange).toHaveBeenLastCalledWith(3600);
    expect(input).toHaveValue(60);
    expect(unitText()).toBe('Minutes');
  });

  it('allows an empty box without emitting, then emits the next valid value', async () => {
    const onChange = vi.fn();
    render(<Harness initial={600} onChange={onChange} />);
    const input = screen.getByTestId('duration');

    await userEvent.clear(input);
    expect(input).toHaveValue(null);
    expect(onChange).not.toHaveBeenCalled();

    await userEvent.type(input, '5');
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(300);
  });

  // The shipped agent ladder starts at one minute, which should read as a phrase.
  it('names the unit in the singular beside a value of one', async () => {
    render(<Harness initial={60} onChange={vi.fn()} />);
    const input = screen.getByTestId('duration');

    expect(input).toHaveValue(1);
    expect(unitText()).toBe('Minute');

    await userEvent.type(input, '5');
    expect(unitText()).toBe('Minutes');
  });

  it('re-derives the display when the value changes from outside', () => {
    const {rerender} = render(
      <DurationField label="Lock duration" seconds={300} testId="duration" onChange={vi.fn()} />,
    );
    rerender(<DurationField label="Lock duration" seconds={7200} testId="duration" onChange={vi.fn()} />);

    expect(screen.getByTestId('duration')).toHaveValue(2);
    expect(unitText()).toBe('Hours');
  });

  it('shows the stored value again when an emptied box loses focus', async () => {
    const onChange = vi.fn();
    render(<Harness initial={600} onChange={onChange} />);
    const input = screen.getByTestId('duration');

    await userEvent.clear(input);
    await userEvent.tab();

    expect(input).toHaveValue(10);
    expect(unitText()).toBe('Minutes');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('shows why while the box is empty or not a positive whole number', async () => {
    render(<Harness initial={600} onChange={vi.fn()} />);
    const input = screen.getByTestId('duration');

    await userEvent.clear(input);
    expect(screen.getByText('Enter a whole number of 1 or more.')).toBeInTheDocument();

    await userEvent.type(input, '-5');
    expect(screen.getByText('Enter a whole number of 1 or more.')).toBeInTheDocument();

    await userEvent.clear(input);
    await userEvent.type(input, '15');
    expect(screen.queryByText('Enter a whole number of 1 or more.')).not.toBeInTheDocument();
  });

  it('shows the message it was given instead of its own', async () => {
    render(<DurationField label="Lock duration" seconds={600} testId="duration" error="Too long" onChange={vi.fn()} />);

    await userEvent.clear(screen.getByTestId('duration'));

    expect(screen.getByText('Too long')).toBeInTheDocument();
    expect(screen.queryByText('Enter a whole number of 1 or more.')).not.toBeInTheDocument();
  });

  it('puts the id on the number box, or on the unit list while the box is hidden', () => {
    const {rerender} = render(<DurationField id="lock" label="Lock duration" seconds={300} onChange={vi.fn()} />);
    expect(screen.getByRole('spinbutton')).toHaveAttribute('id', 'lock');

    rerender(<DurationField id="lock" label="Lock duration" seconds={0} zeroLabel="No limit" onChange={vi.fn()} />);
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
    expect(screen.getByRole('combobox')).toHaveAttribute('id', 'lock');
  });

  // Reset and Save are buttons, so pressing one blurs the box before the parent value is read.
  it('does not leave an empty box behind a button press that keeps the value', async () => {
    render(
      <>
        <DurationField label="Lock duration" seconds={600} testId="duration" onChange={vi.fn()} />
        <button type="button">Reset</button>
      </>,
    );
    const input = screen.getByTestId('duration');

    await userEvent.clear(input);
    await userEvent.click(screen.getByRole('button', {name: 'Reset'}));

    expect(input).toHaveValue(10);
  });

  // Inline fields have no visible labels, so each control needs its own accessible name.
  it('names the number box and the unit list when shown inline', () => {
    render(<DurationField inline label="Lock duration" seconds={300} testId="duration" onChange={vi.fn()} />);

    expect(screen.getByRole('spinbutton', {name: 'Lock duration'})).toHaveValue(5);
    expect(screen.getByRole('combobox', {name: 'Unit'})).toHaveTextContent('Minutes');
  });

  it('names the unit list after the field when it shows the zero entry inline', () => {
    render(
      <DurationField
        inline
        label="Lock duration"
        seconds={0}
        zeroLabel="Until an administrator unlocks"
        testId="duration"
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByRole('combobox', {name: 'Lock duration'})).toHaveTextContent('Until an administrator unlocks');
  });
});
