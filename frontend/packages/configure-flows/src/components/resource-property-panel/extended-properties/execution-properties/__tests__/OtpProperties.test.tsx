// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import OtpProperties from '../OtpProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.otp.description': 'Configure the OTP step behavior.',
        'flows:core.executions.otp.otpLength.label': 'OTP length',
        'flows:core.executions.otp.otpLength.placeholder': '6',
        'flows:core.executions.otp.otpLength.hint': 'Between 4 and 10 digits.',
        'flows:core.executions.otp.otpValidityPeriodSeconds.label': 'OTP validity period',
        'flows:core.executions.otp.otpValidityPeriodSeconds.placeholder': '300',
        'flows:core.executions.otp.otpValidityPeriodSeconds.hint': 'Between 30 and 600 seconds.',
        'flows:core.executions.otp.otpUseNumericOnly.label': 'Numeric only',
        'flows:core.executions.otp.otpUseNumericOnly.hint': 'Restrict the OTP to digits.',
        'flows:core.executions.otp.maxAttempts.label': 'Max attempts',
        'flows:core.executions.otp.maxAttempts.placeholder': '3',
        'flows:core.executions.otp.maxAttempts.hint': 'Minimum 1.',
      };
      return translations[key] ?? key;
    },
  }),
}));

describe('OtpProperties', () => {
  const mockOnChange = vi.fn();

  const createResource = (properties: Record<string, unknown> = {}): Resource =>
    ({data: {properties}}) as unknown as Resource;

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and field labels', () => {
    render(<OtpProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByText('Configure the OTP step behavior.')).toBeInTheDocument();
    expect(screen.getByText('OTP length')).toBeInTheDocument();
    expect(screen.getByText('OTP validity period')).toBeInTheDocument();
  });

  it('should render empty length and validity fields when the node has no properties', () => {
    render(<OtpProperties resource={createResource()} onChange={mockOnChange} />);

    expect(screen.getByLabelText('OTP length')).toHaveValue(null);
    expect(screen.getByLabelText('OTP validity period')).toHaveValue(null);
  });

  it('should render the configured length and validity values', () => {
    render(
      <OtpProperties
        resource={createResource({otpLength: 6, otpValidityPeriodSeconds: 300})}
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByLabelText('OTP length')).toHaveValue(6);
    expect(screen.getByLabelText('OTP validity period')).toHaveValue(300);
  });

  it('should commit a new OTP length clamped to the allowed range', () => {
    render(<OtpProperties resource={createResource({otpLength: 6})} onChange={mockOnChange} />);

    const lengthInput = screen.getByLabelText('OTP length');
    fireEvent.change(lengthInput, {target: {value: '999'}});
    fireEvent.blur(lengthInput);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.otpLength', 10, expect.anything());
  });
});
