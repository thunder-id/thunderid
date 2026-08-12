// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen} from '@testing-library/react';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import type {FlowFieldProps} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import TextInputAdapter from '../TextInputAdapter';

const baseProps: FlowFieldProps = {
  component: {id: 'username', type: 'TEXT_INPUT', ref: 'username', label: 'Username', required: true},
  values: {},
  isLoading: false,
  resolve: (s) => s,
  onInputChange: vi.fn(),
};

describe('TextInputAdapter', () => {
  it('renders the label', () => {
    renderWithProviders(<TextInputAdapter {...baseProps} />);
    expect(screen.getByText('Username')).toBeTruthy();
  });

  it('renders a text input', () => {
    renderWithProviders(<TextInputAdapter {...baseProps} />);
    const input = document.querySelector('input[name="username"]');
    expect(input).toBeTruthy();
    expect(input?.getAttribute('type')).toBe('text');
  });

  it('renders email type for EMAIL_INPUT', () => {
    const emailProps = {
      ...baseProps,
      component: {...baseProps.component, type: 'EMAIL_INPUT', ref: 'email', label: 'Email'},
    };
    renderWithProviders(<TextInputAdapter {...emailProps} />);
    const input = document.querySelector('input[name="email"]');
    expect(input?.getAttribute('type')).toBe('email');
  });

  it('renders number type for NUMBER_INPUT', () => {
    const numberProps = {
      ...baseProps,
      component: {...baseProps.component, type: 'NUMBER_INPUT', ref: 'age', label: 'Age'},
    };
    renderWithProviders(<TextInputAdapter {...numberProps} />);
    const input = document.querySelector('input[name="age"]');
    expect(input?.getAttribute('type')).toBe('number');
  });

  it('applies product prefix CSS class names', () => {
    renderWithProviders(<TextInputAdapter {...baseProps} />);
    const formControl = document.querySelector(`.${TEST_CN_PREFIX}Flow--textInput`);
    expect(formControl).toBeTruthy();
    expect(formControl?.classList.contains(`${TEST_CN_PREFIX}FormControl--root`)).toBe(true);
  });

  it('returns null when ref is missing', () => {
    const noRefProps = {...baseProps, component: {...baseProps.component, ref: undefined}};
    const {container} = renderWithProviders(<TextInputAdapter {...noRefProps} />);
    expect(container.innerHTML).toBe('');
  });
});
