// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {fireEvent, within} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {FlowFieldProps} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import OtpInputAdapter from '../OtpInputAdapter';

const baseProps: FlowFieldProps = {
  component: {id: 'otp', type: 'OTP_INPUT', ref: 'otp', label: 'One-time code', required: true},
  values: {},
  isLoading: false,
  resolve: (s) => s,
  onInputChange: vi.fn(),
};

const digitBoxes = (container: HTMLElement): HTMLInputElement[] =>
  Array.from(container.querySelectorAll<HTMLInputElement>('input[aria-label^="OTP digit"]'));

describe('OtpInputAdapter', () => {
  it('renders six boxes when the step reports no OTP length', () => {
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} />);
    expect(digitBoxes(container)).toHaveLength(6);
  });

  it('renders the number of boxes reported by the step', () => {
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} additionalData={{otpLength: '8'}} />);
    expect(digitBoxes(container)).toHaveLength(8);
  });

  it('accepts a numeric reported length', () => {
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} additionalData={{otpLength: 4}} />);
    expect(digitBoxes(container)).toHaveLength(4);
  });

  it.each([['0'], ['-1'], ['abc'], ['4.5'], ['']])('falls back to six boxes for %s', (otpLength) => {
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} additionalData={{otpLength}} />);
    expect(digitBoxes(container)).toHaveLength(6);
  });

  it('renders the label', () => {
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} />);
    expect(within(container).getByText('One-time code')).toBeTruthy();
  });

  it('spreads an existing value across the boxes', () => {
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} values={{otp: '12345678'}} additionalData={{otpLength: '8'}} />,
    );
    expect(digitBoxes(container).map((input) => input.value)).toEqual(['1', '2', '3', '4', '5', '6', '7', '8']);
  });

  it('truncates a pasted code to the reported length', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} onInputChange={onInputChange} additionalData={{otpLength: '8'}} />,
    );

    // Chromium ignores clipboardData supplied to a synthetic ClipboardEvent, so attach it directly.
    const pasteEvent = new Event('paste', {bubbles: true, cancelable: true});
    Object.defineProperty(pasteEvent, 'clipboardData', {value: {getData: () => '1234567890'}});
    fireEvent(digitBoxes(container)[0], pasteEvent);

    expect(onInputChange).toHaveBeenCalledWith('otp', '12345678');
  });

  it('rejects a letter when the step reports a numeric-only code', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} onInputChange={onInputChange} additionalData={{otpNumericOnly: 'true'}} />,
    );

    fireEvent.change(digitBoxes(container)[0], {target: {value: 'a'}});

    expect(onInputChange).not.toHaveBeenCalled();
  });

  it('rejects a letter when the step reports no character set', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} onInputChange={onInputChange} />);

    fireEvent.change(digitBoxes(container)[0], {target: {value: 'a'}});

    expect(onInputChange).not.toHaveBeenCalled();
  });

  it('accepts a letter when the step reports an alphanumeric code', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} onInputChange={onInputChange} additionalData={{otpNumericOnly: 'false'}} />,
    );

    fireEvent.change(digitBoxes(container)[0], {target: {value: 'K'}});

    expect(onInputChange).toHaveBeenCalledWith('otp', 'K     ');
  });

  it('upper-cases a lowercase letter for an alphanumeric code', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} onInputChange={onInputChange} additionalData={{otpNumericOnly: 'false'}} />,
    );

    fireEvent.change(digitBoxes(container)[0], {target: {value: 'k'}});

    expect(onInputChange).toHaveBeenCalledWith('otp', 'K     ');
  });

  it('strips surrounding text when pasting a numeric code', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} onInputChange={onInputChange} />);

    const pasteEvent = new Event('paste', {bubbles: true, cancelable: true});
    Object.defineProperty(pasteEvent, 'clipboardData', {value: {getData: () => 'Your code is 123456'}});
    fireEvent(digitBoxes(container)[0], pasteEvent);

    expect(onInputChange).toHaveBeenCalledWith('otp', '123456');
  });

  it('upper-cases and keeps letters when pasting an alphanumeric code', () => {
    const onInputChange = vi.fn();
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} onInputChange={onInputChange} additionalData={{otpNumericOnly: 'false'}} />,
    );

    const pasteEvent = new Event('paste', {bubbles: true, cancelable: true});
    Object.defineProperty(pasteEvent, 'clipboardData', {value: {getData: () => 'k7gx2m'}});
    fireEvent(digitBoxes(container)[0], pasteEvent);

    expect(onInputChange).toHaveBeenCalledWith('otp', 'K7GX2M');
  });

  it('marks the boxes numeric only when the code is digits only', () => {
    const {container} = renderWithProviders(<OtpInputAdapter {...baseProps} />);
    expect(digitBoxes(container)[0].getAttribute('inputmode')).toBe('numeric');
  });

  it('marks the boxes text when the code is alphanumeric', () => {
    const {container} = renderWithProviders(
      <OtpInputAdapter {...baseProps} additionalData={{otpNumericOnly: 'false'}} />,
    );
    expect(digitBoxes(container)[0].getAttribute('inputmode')).toBe('text');
  });

  it('returns null when ref is missing', () => {
    const noRefProps = {...baseProps, component: {...baseProps.component, ref: undefined}};
    const {container} = renderWithProviders(<OtpInputAdapter {...noRefProps} />);
    expect(container.innerHTML).toBe('');
  });
});
