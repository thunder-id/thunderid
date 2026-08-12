// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {fireEvent, screen} from '@testing-library/react';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {FlowFieldProps} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import CheckboxAdapter from '../CheckboxAdapter';

const baseProps: FlowFieldProps = {
  component: {id: 'active', type: 'BOOLEAN_INPUT', ref: 'active', label: 'Active', required: true},
  values: {active: 'false'},
  isLoading: false,
  resolve: (s) => s,
  onInputChange: vi.fn(),
};

describe('CheckboxAdapter', () => {
  it('renders the label', () => {
    renderWithProviders(<CheckboxAdapter {...baseProps} />);
    expect(screen.getByText('Active')).toBeTruthy();
  });

  it('renders a checkbox reflecting the current value', () => {
    const {container} = renderWithProviders(<CheckboxAdapter {...baseProps} values={{active: 'true'}} />);
    const input = container.querySelector<HTMLInputElement>('input[name="active"]')!;
    expect(input).toBeTruthy();
    expect(input.getAttribute('type')).toBe('checkbox');
    expect(input.checked).toBe(true);
  });

  it('reports the checked state as a boolean string', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(<CheckboxAdapter {...baseProps} onInputChange={onInputChange} />);
    const input = container.querySelector<HTMLInputElement>('input[name="active"]')!;
    fireEvent.click(input);
    expect(onInputChange).toHaveBeenCalledWith('active', 'true');
  });

  it('seeds the unchecked value so a required boolean is present in the submission', () => {
    const onInputChange = vi.fn();
    renderWithProviders(<CheckboxAdapter {...baseProps} values={{}} onInputChange={onInputChange} />);
    expect(onInputChange).toHaveBeenCalledWith('active', 'false');
  });

  it('does not overwrite a value that is already set', () => {
    const onInputChange = vi.fn();
    renderWithProviders(<CheckboxAdapter {...baseProps} values={{active: 'true'}} onInputChange={onInputChange} />);
    expect(onInputChange).not.toHaveBeenCalled();
  });

  it('applies product prefix CSS class names', () => {
    renderWithProviders(<CheckboxAdapter {...baseProps} />);
    const formControl = document.querySelector(`.${TEST_CN_PREFIX}Flow--checkbox`);
    expect(formControl).toBeTruthy();
    expect(formControl?.classList.contains(`${TEST_CN_PREFIX}FormControl--root`)).toBe(true);
  });

  it('returns null when ref is missing', () => {
    const noRefProps = {...baseProps, component: {...baseProps.component, ref: undefined}};
    const {container} = renderWithProviders(<CheckboxAdapter {...noRefProps} />);
    expect(container.innerHTML).toBe('');
  });
});
