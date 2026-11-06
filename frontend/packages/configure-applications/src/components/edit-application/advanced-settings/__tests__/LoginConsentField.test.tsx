// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import LoginConsentField from '../LoginConsentField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

const HINT = 'Leave empty or enter 0 so consent does not expire.';
const ERROR = 'Enter a whole number of seconds.';

const getField = (): HTMLElement => screen.getByRole('spinbutton', {name: 'Consent validity period'});

describe('LoginConsentField', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders the label, unit, and hint without a card around it', () => {
      render(<LoginConsentField onLoginConsentChange={mockOnChange} />);

      expect(getField()).toBeInTheDocument();
      expect(screen.getByText('seconds')).toBeInTheDocument();
      expect(screen.getByText(HINT)).toBeInTheDocument();
    });

    it('shows an empty field when no validity period is set', () => {
      render(<LoginConsentField onLoginConsentChange={mockOnChange} />);

      expect(getField()).toHaveValue(null);
    });

    it('shows 0 when the validity period is 0', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 0}} onLoginConsentChange={mockOnChange} />);

      expect(getField()).toHaveValue(0);
    });

    it('shows the stored validity period', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={mockOnChange} />);

      expect(getField()).toHaveValue(3600);
    });
  });

  describe('Editing', () => {
    it('commits a whole number of seconds', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 0}} onLoginConsentChange={mockOnChange} />);

      fireEvent.change(getField(), {target: {value: '7200'}});

      expect(mockOnChange).toHaveBeenCalledWith({validityPeriod: 7200});
    });

    it('commits 0 when 0 is entered', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={mockOnChange} />);

      fireEvent.change(getField(), {target: {value: '0'}});

      expect(mockOnChange).toHaveBeenCalledWith({validityPeriod: 0});
    });

    it('commits 0 and keeps the field empty when the field is cleared', () => {
      const {rerender} = render(
        <LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={mockOnChange} />,
      );

      fireEvent.change(getField(), {target: {value: ''}});
      rerender(<LoginConsentField loginConsent={{validityPeriod: 0}} onLoginConsentChange={mockOnChange} />);

      expect(mockOnChange).toHaveBeenCalledWith({validityPeriod: 0});
      expect(getField()).toHaveValue(null);
      expect(screen.queryByText(ERROR)).not.toBeInTheDocument();
    });

    it.each([
      ['negative', '-5'],
      ['fractional', '1.5'],
    ])('rejects a %s value without committing it', (_label, value) => {
      const onValidationChange = vi.fn();
      render(
        <LoginConsentField
          loginConsent={{validityPeriod: 3600}}
          onLoginConsentChange={mockOnChange}
          onValidationChange={onValidationChange}
        />,
      );

      fireEvent.change(getField(), {target: {value}});

      expect(mockOnChange).not.toHaveBeenCalled();
      expect(screen.getByText(ERROR)).toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });

    it('clears the error once a valid value is typed', () => {
      const onValidationChange = vi.fn();
      render(
        <LoginConsentField
          loginConsent={{validityPeriod: 3600}}
          onLoginConsentChange={mockOnChange}
          onValidationChange={onValidationChange}
        />,
      );

      fireEvent.change(getField(), {target: {value: '-1'}});
      fireEvent.change(getField(), {target: {value: '60'}});

      expect(screen.queryByText(ERROR)).not.toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(false);
      expect(mockOnChange).toHaveBeenCalledWith({validityPeriod: 60});
    });

    it('takes the stored value again when the configuration is replaced', () => {
      const {rerender} = render(
        <LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={mockOnChange} />,
      );

      fireEvent.change(getField(), {target: {value: '-1'}});
      rerender(<LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={mockOnChange} />);

      expect(getField()).toHaveValue(3600);
      expect(screen.queryByText(ERROR)).not.toBeInTheDocument();
    });

    it('clears a reported error when it unmounts', () => {
      const onValidationChange = vi.fn();
      const {unmount} = render(
        <LoginConsentField
          loginConsent={{validityPeriod: 3600}}
          onLoginConsentChange={mockOnChange}
          onValidationChange={onValidationChange}
        />,
      );

      fireEvent.change(getField(), {target: {value: '-1'}});
      unmount();

      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });
  });

  describe('Read-only', () => {
    it('disables the field when disabled', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={mockOnChange} disabled />);

      expect(getField()).toBeDisabled();
    });

    it('disables the field when no change handler is given', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 0}} />);

      expect(getField()).toBeDisabled();
    });
  });
});
