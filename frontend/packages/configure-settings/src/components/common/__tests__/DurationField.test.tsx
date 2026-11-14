// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {Box} from '@wso2/oxygen-ui';
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

  // A message as long as a sentence does not fit under the narrow number box, so it runs beneath both
  // controls and describes each of them.
  it('shows an inline error beneath both controls', () => {
    render(
      <Box sx={{width: 600}}>
        <DurationField
          inline
          label="Idle timeout"
          seconds={7200}
          error="Too long."
          testId="duration"
          onChange={vi.fn()}
        />
      </Box>,
    );

    const number = screen.getByRole('spinbutton', {name: 'Idle timeout'});
    expect(number).toHaveAttribute('aria-invalid', 'true');
    expect(number).toHaveAccessibleDescription('Too long.');
    expect(screen.getByRole('combobox', {name: 'Unit'})).toHaveAccessibleDescription('Too long.');
    const box = number.closest('.MuiInputBase-root')!.getBoundingClientRect();
    expect(screen.getByText('Too long.').getBoundingClientRect().width).toBeGreaterThan(box.width);
  });

  // A hint beside the field describes both controls, with any error after it.
  it('describes both controls with the elements it is given, then the error', () => {
    render(
      <>
        <p id="hint">Signs the user out after this long without activity.</p>
        <DurationField
          inline
          label="Idle timeout"
          seconds={7200}
          error="Too long."
          describedBy="hint"
          testId="duration"
          onChange={vi.fn()}
        />
      </>,
    );

    const description = 'Signs the user out after this long without activity. Too long.';
    expect(screen.getByRole('spinbutton', {name: 'Idle timeout'})).toHaveAccessibleDescription(description);
    expect(screen.getByRole('combobox', {name: 'Unit'})).toHaveAccessibleDescription(description);
  });

  // Outside a row, the field's own helper text describes the number, as the text field sets it up.
  it('keeps the helper text as the description of a field outside a row', () => {
    render(<DurationField label="Lock duration" seconds={300} error="Too short." onChange={vi.fn()} />);

    expect(screen.getByRole('spinbutton', {name: 'Lock duration'})).toHaveAccessibleDescription('Too short.');
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

  // A value of zero has no unit of its own, as for a field reset to a default not yet known.
  it('empties the box and keeps the unit when the value becomes zero from outside', () => {
    const {rerender} = render(
      <DurationField label="Idle timeout" seconds={7200} testId="duration" onChange={vi.fn()} />,
    );

    rerender(<DurationField label="Idle timeout" seconds={0} testId="duration" onChange={vi.fn()} />);

    expect(screen.getByTestId('duration')).toHaveValue(null);
    expect(unitText()).toBe('Hours');
  });
});
