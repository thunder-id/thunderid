// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import DateTimePickerField from '../DateTimePickerField';

interface CapturedPickerProps {
  value: Date | null;
  disablePast: boolean;
  disabled: boolean;
  onError: (error: unknown) => void;
  onChange: (date: Date | null) => void;
  slotProps: {
    field: {clearable: boolean};
    textField: Record<string, unknown>;
  };
}

const pickerSpy = vi.fn();

vi.mock('@wso2/oxygen-ui', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@wso2/oxygen-ui')>()),
  DatePickers: {
    LocalizationProvider: ({children}: {children: ReactNode}) => children,
    DateTimePicker: (props: unknown) => {
      pickerSpy(props);
      return <div data-testid="date-time-picker" />;
    },
  },
}));

const capturedProps = (): CapturedPickerProps => {
  const props = pickerSpy.mock.lastCall?.[0] as CapturedPickerProps | undefined;
  if (!props) throw new Error('DateTimePicker was not rendered');
  return props;
};

describe('DateTimePickerField', () => {
  beforeEach(() => {
    pickerSpy.mockClear();
  });

  it('passes no date to the picker when there is no value', () => {
    render(<DateTimePickerField id="valid-until" value={null} onChange={vi.fn()} />);

    expect(capturedProps().value).toBeNull();
  });

  it('passes the stored ISO value to the picker as a date', () => {
    render(<DateTimePickerField id="valid-until" value="2026-12-31T10:30:00.000Z" onChange={vi.fn()} />);

    expect(capturedProps().value).toEqual(new Date('2026-12-31T10:30:00.000Z'));
  });

  it('reports a chosen date as an ISO string', () => {
    const onChange = vi.fn();
    render(<DateTimePickerField id="valid-until" onChange={onChange} />);

    capturedProps().onChange(new Date('2027-01-15T08:00:00.000Z'));

    expect(onChange).toHaveBeenCalledWith('2027-01-15T08:00:00.000Z');
  });

  it('reports a cleared date as null', () => {
    const onChange = vi.fn();
    render(<DateTimePickerField id="valid-until" value="2026-12-31T10:30:00.000Z" onChange={onChange} />);

    capturedProps().onChange(null);

    expect(onChange).toHaveBeenCalledWith(null);
  });

  it('ignores a partially typed, invalid date', () => {
    const onChange = vi.fn();
    render(<DateTimePickerField id="valid-until" onChange={onChange} />);

    capturedProps().onChange(new Date(Number.NaN));

    expect(onChange).not.toHaveBeenCalled();
  });

  it('reports whether the picker holds a validation error', () => {
    const onErrorChange = vi.fn();
    render(<DateTimePickerField id="valid-until" onChange={vi.fn()} onErrorChange={onErrorChange} />);

    capturedProps().onError('disablePast');
    expect(onErrorChange).toHaveBeenLastCalledWith(true);

    capturedProps().onError(null);
    expect(onErrorChange).toHaveBeenLastCalledWith(false);
  });

  it('tolerates a validation error when no error listener is given', () => {
    render(<DateTimePickerField id="valid-until" onChange={vi.fn()} />);

    expect(() => capturedProps().onError('invalidDate')).not.toThrow();
  });

  it('forwards the field props to the text field and keeps the value clearable', () => {
    const onBlur = vi.fn();
    render(
      <DateTimePickerField
        id="valid-until"
        onChange={vi.fn()}
        onBlur={onBlur}
        label="Valid Until"
        required
        helperText="Leave empty for no end"
      />,
    );

    const {slotProps} = capturedProps();
    expect(slotProps.field.clearable).toBe(true);
    expect(slotProps.textField).toEqual(
      expect.objectContaining({
        id: 'valid-until',
        label: 'Valid Until',
        required: true,
        helperText: 'Leave empty for no end',
        onBlur,
      }),
    );
  });

  it('defaults to an optional, enabled field that allows past dates', () => {
    render(<DateTimePickerField id="valid-until" onChange={vi.fn()} />);

    const props = capturedProps();
    expect(props.disablePast).toBe(false);
    expect(props.disabled).toBe(false);
    expect(props.slotProps.textField['required']).toBe(false);
  });

  it('forwards disablePast and disabled to the picker', () => {
    render(<DateTimePickerField id="valid-until" onChange={vi.fn()} disablePast disabled />);

    const props = capturedProps();
    expect(props.disablePast).toBe(true);
    expect(props.disabled).toBe(true);
  });
});
