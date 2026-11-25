// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {render} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FlowFieldProps} from '../../../../models/flow';
import DateTimeInputAdapter from '../DateTimeInputAdapter';

interface CapturedFieldProps {
  id: string;
  value: string | null;
  label?: ReactNode;
  required?: boolean;
  disabled?: boolean;
  helperText?: string;
  onChange: (value: string | null) => void;
  onBlur?: () => void;
}

const fieldSpy = vi.fn();

vi.mock('../../../DateTimePickerField', () => ({
  default: (props: unknown) => {
    fieldSpy(props);
    return <div data-testid="date-time-picker-field" />;
  },
}));

const capturedProps = (): CapturedFieldProps => {
  const props = fieldSpy.mock.lastCall?.[0] as CapturedFieldProps | undefined;
  if (!props) throw new Error('DateTimePickerField was not rendered');
  return props;
};

const baseProps: FlowFieldProps = {
  component: {
    id: 'input_valid_until',
    type: 'DATE_TIME_INPUT',
    ref: 'validUntil',
    label: 'Valid Until',
    required: true,
  },
  values: {},
  isLoading: false,
  resolve: (s) => s,
  onInputChange: vi.fn(),
};

describe('DateTimeInputAdapter', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns null when ref is missing', () => {
    const {container} = render(
      <DateTimeInputAdapter {...baseProps} component={{id: 'input_valid_until', type: 'DATE_TIME_INPUT'}} />,
    );

    expect(container.innerHTML).toBe('');
    expect(fieldSpy).not.toHaveBeenCalled();
  });

  it('renders the picker for the field ref with its label, required flag and loading state', () => {
    render(<DateTimeInputAdapter {...baseProps} isLoading />);

    expect(capturedProps()).toEqual(
      expect.objectContaining({
        id: 'validUntil',
        value: null,
        label: 'Valid Until',
        required: true,
        disabled: true,
        helperText: undefined,
      }),
    );
  });

  it('shows the value the flow holds for the field', () => {
    render(<DateTimeInputAdapter {...baseProps} values={{validUntil: '2026-12-31T10:30:00.000Z'}} />);

    expect(capturedProps().value).toBe('2026-12-31T10:30:00.000Z');
  });

  it('shows the resolved hint as helper text', () => {
    render(
      <DateTimeInputAdapter
        {...baseProps}
        component={{...baseProps.component, hint: '{{ t(hint.key) }}'}}
        resolve={(s) => (s === '{{ t(hint.key) }}' ? 'Leave empty for no end' : s)}
      />,
    );

    expect(capturedProps().helperText).toBe('Leave empty for no end');
  });

  it('falls back to the raw hint when it does not resolve', () => {
    render(
      <DateTimeInputAdapter
        {...baseProps}
        component={{...baseProps.component, hint: 'Leave empty for no end'}}
        resolve={(s) => (s === 'Leave empty for no end' ? undefined : s)}
      />,
    );

    expect(capturedProps().helperText).toBe('Leave empty for no end');
  });

  it('shows the field error instead of the hint once the field is touched', () => {
    render(
      <DateTimeInputAdapter
        {...baseProps}
        component={{...baseProps.component, hint: 'Leave empty for no end'}}
        touched={{validUntil: true}}
        fieldErrors={{validUntil: 'Valid Until is required'}}
      />,
    );

    expect(capturedProps().helperText).toBe('Valid Until is required');
  });

  it('keeps the hint while an errored field is untouched', () => {
    render(
      <DateTimeInputAdapter
        {...baseProps}
        component={{...baseProps.component, hint: 'Leave empty for no end'}}
        fieldErrors={{validUntil: 'Valid Until is required'}}
      />,
    );

    expect(capturedProps().helperText).toBe('Leave empty for no end');
  });

  it('passes a chosen date to the flow under the field ref', () => {
    const onInputChange = vi.fn();
    render(<DateTimeInputAdapter {...baseProps} onInputChange={onInputChange} />);

    capturedProps().onChange('2027-01-15T08:00:00.000Z');

    expect(onInputChange).toHaveBeenCalledWith('validUntil', '2027-01-15T08:00:00.000Z');
  });

  it('passes a cleared date to the flow as an empty value', () => {
    const onInputChange = vi.fn();
    render(<DateTimeInputAdapter {...baseProps} onInputChange={onInputChange} />);

    capturedProps().onChange(null);

    expect(onInputChange).toHaveBeenCalledWith('validUntil', '');
  });

  it('reports a blur under the field ref', () => {
    const onBlur = vi.fn();
    render(<DateTimeInputAdapter {...baseProps} onBlur={onBlur} />);

    capturedProps().onBlur?.();

    expect(onBlur).toHaveBeenCalledWith('validUntil');
  });

  it('tolerates a blur when no blur listener is given', () => {
    render(<DateTimeInputAdapter {...baseProps} />);

    expect(() => capturedProps().onBlur?.()).not.toThrow();
  });
});
