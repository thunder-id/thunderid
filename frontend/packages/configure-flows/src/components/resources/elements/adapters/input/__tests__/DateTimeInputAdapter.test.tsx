// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Element as FlowElement} from '../../../../../../models/elements';
import DateTimeInputAdapter from '../DateTimeInputAdapter';

interface CapturedFieldProps {
  id: string;
  value: string | null;
  label?: ReactNode;
  required?: boolean;
  helperText?: string;
  onChange: (value: string | null) => void;
}

const fieldSpy = vi.fn();

vi.mock('@thunderid/design', () => ({
  DateTimePickerField: (props: unknown) => {
    fieldSpy(props);
    return <div data-testid="date-time-picker-field">{(props as CapturedFieldProps).label}</div>;
  },
}));

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (value: string) => (value === 'unresolvable' ? undefined : `resolved:${value}`),
  }),
}));

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const capturedProps = (): CapturedFieldProps => {
  const props = fieldSpy.mock.lastCall?.[0] as CapturedFieldProps | undefined;
  if (!props) throw new Error('DateTimePickerField was not rendered');
  return props;
};

const hiddenInput = (container: HTMLElement): HTMLInputElement | null =>
  container.querySelector('input[type="hidden"][name="input-valid-until"]');

describe('DateTimeInputAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'input-valid-until',
      type: 'DATE_TIME_INPUT',
      category: 'FIELD',
      config: {},
      label: 'Valid Until',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    fieldSpy.mockClear();
  });

  it('renders the picker with a resolved label, required flag and hint', () => {
    render(<DateTimeInputAdapter resource={createMockElement({required: true, hint: 'Leave empty for no end'})} />);

    expect(capturedProps()).toEqual(
      expect.objectContaining({
        id: 'input-valid-until-date-time',
        value: null,
        label: 'resolved:Valid Until',
        required: true,
        helperText: 'Leave empty for no end',
      }),
    );
  });

  it('falls back to the raw label when it does not resolve', () => {
    render(<DateTimeInputAdapter resource={createMockElement({label: 'unresolvable'})} />);

    expect(capturedProps().label).toBe('unresolvable');
  });

  it('renders a template literal label as a template placeholder', () => {
    render(<DateTimeInputAdapter resource={createMockElement({label: '{{ t(agents:validUntil) }}'})} />);

    expect(capturedProps().label).not.toBe('{{ t(agents:validUntil) }}');
    expect(screen.getByTestId('date-time-picker-field')).toHaveTextContent('agents:validUntil');
  });

  it('renders an empty label when the element has none', () => {
    render(<DateTimeInputAdapter resource={createMockElement({label: undefined})} />);

    expect(capturedProps().label).toBe('resolved:');
  });

  it('starts from the default value and mirrors it in a hidden form input', () => {
    const {container} = render(
      <DateTimeInputAdapter resource={createMockElement({defaultValue: '2026-12-31T10:30:00.000Z'})} />,
    );

    expect(capturedProps().value).toBe('2026-12-31T10:30:00.000Z');
    expect(hiddenInput(container)?.value).toBe('2026-12-31T10:30:00.000Z');
  });

  it('keeps the hidden form input empty when there is no value', () => {
    const {container} = render(<DateTimeInputAdapter resource={createMockElement()} />);

    expect(hiddenInput(container)?.value).toBe('');
  });

  it('updates the hidden form input when a date is chosen', () => {
    const {container} = render(<DateTimeInputAdapter resource={createMockElement()} />);

    act(() => capturedProps().onChange('2027-01-15T08:00:00.000Z'));

    expect(capturedProps().value).toBe('2027-01-15T08:00:00.000Z');
    expect(hiddenInput(container)?.value).toBe('2027-01-15T08:00:00.000Z');
  });

  it('resets to the new default value when the configured default changes', () => {
    const {container, rerender} = render(
      <DateTimeInputAdapter resource={createMockElement({defaultValue: '2026-12-31T10:30:00.000Z'})} />,
    );
    act(() => capturedProps().onChange('2027-01-15T08:00:00.000Z'));

    rerender(<DateTimeInputAdapter resource={createMockElement({defaultValue: '2028-06-01T00:00:00.000Z'})} />);

    expect(capturedProps().value).toBe('2028-06-01T00:00:00.000Z');
    expect(hiddenInput(container)?.value).toBe('2028-06-01T00:00:00.000Z');
  });

  it('keeps a chosen date while the configured default is unchanged', () => {
    const {rerender} = render(
      <DateTimeInputAdapter resource={createMockElement({defaultValue: '2026-12-31T10:30:00.000Z'})} />,
    );
    act(() => capturedProps().onChange('2027-01-15T08:00:00.000Z'));

    rerender(<DateTimeInputAdapter resource={createMockElement({defaultValue: '2026-12-31T10:30:00.000Z'})} />);

    expect(capturedProps().value).toBe('2027-01-15T08:00:00.000Z');
  });

  it('clears the value when the default value is removed', () => {
    const {rerender} = render(
      <DateTimeInputAdapter resource={createMockElement({defaultValue: '2026-12-31T10:30:00.000Z'})} />,
    );

    rerender(<DateTimeInputAdapter resource={createMockElement()} />);

    expect(capturedProps().value).toBeNull();
  });
});
