// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState, type JSX} from 'react';
import {describe, it, expect, vi} from 'vitest';
import type {LoginConsentConfig} from '../../../models/application';
import LoginConsentField from '../LoginConsentField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

const HINT =
  'How long consent is remembered before the user is asked again. Choose No expiry for consent that does not expire.';

/** Holds the configuration like the edit page does, so each change comes back as the prop. */
function Harness({
  initial = undefined,
  onChange,
}: {
  initial?: LoginConsentConfig;
  onChange: (loginConsent: LoginConsentConfig) => void;
}): JSX.Element {
  const [loginConsent, setLoginConsent] = useState<LoginConsentConfig | undefined>(initial);
  return (
    <LoginConsentField
      loginConsent={loginConsent}
      onLoginConsentChange={(next) => {
        setLoginConsent(next);
        onChange(next);
      }}
    />
  );
}

const NUMBER_NAME = 'Consent validity period';

const queryNumber = (): HTMLElement | null => screen.queryByRole('spinbutton', {name: NUMBER_NAME});
const getNumber = (): HTMLElement => screen.getByRole('spinbutton', {name: NUMBER_NAME});
// The unit list carries the field's name while the number box is hidden, and is named Unit beside it.
const getUnit = (): HTMLElement => screen.getByRole('combobox', {name: /^(Unit|Consent validity period)$/});

const chooseUnit = async (name: string): Promise<void> => {
  await userEvent.click(getUnit());
  await userEvent.click(screen.getByRole('option', {name}));
};

describe('LoginConsentField', () => {
  describe('Rendering', () => {
    it('shows No expiry, with no number box, and the hint when no validity period is set', () => {
      render(<Harness onChange={vi.fn()} />);

      expect(getUnit()).toHaveTextContent('No expiry');
      expect(queryNumber()).not.toBeInTheDocument();
      expect(screen.getByText(HINT)).toBeInTheDocument();
    });

    it('shows the label above the controls', () => {
      render(<Harness initial={{validityPeriod: 3600}} onChange={vi.fn()} />);

      const label = screen.getByText('Consent validity period');

      expect(label.compareDocumentPosition(getNumber()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(label.compareDocumentPosition(getUnit()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    it('links the label to the number box', () => {
      render(<Harness initial={{validityPeriod: 3600}} onChange={vi.fn()} />);

      expect(screen.getByText('Consent validity period')).toHaveAttribute('for', getNumber().id);
    });

    it('links the label to the unit list while the number box is hidden', () => {
      render(<Harness initial={{validityPeriod: 0}} onChange={vi.fn()} />);

      expect(screen.getByText('Consent validity period')).toHaveAttribute('for', getUnit().id);
    });

    it('shows No expiry when the validity period is 0', () => {
      render(<Harness initial={{validityPeriod: 0}} onChange={vi.fn()} />);

      expect(getUnit()).toHaveTextContent('No expiry');
      expect(queryNumber()).not.toBeInTheDocument();
    });

    it.each([
      [3600, 1, 'Hour'],
      [172800, 2, 'Days'],
      [5400, 90, 'Minutes'],
      [45, 45, 'Seconds'],
    ])('shows %i seconds as %i in the largest unit that divides it exactly', (seconds, value, unit) => {
      render(<Harness initial={{validityPeriod: seconds}} onChange={vi.fn()} />);

      expect(getNumber()).toHaveValue(value);
      expect(getUnit()).toHaveTextContent(unit);
    });
  });

  describe('Editing', () => {
    it('starts at one of the chosen unit, sent as seconds', async () => {
      const onChange = vi.fn();
      render(<Harness initial={{validityPeriod: 0}} onChange={onChange} />);

      await chooseUnit('Hours');

      expect(onChange).toHaveBeenLastCalledWith({validityPeriod: 3600});
      expect(getNumber()).toHaveValue(1);
    });

    it('sends the typed number in the chosen unit as seconds', async () => {
      const onChange = vi.fn();
      render(<Harness initial={{validityPeriod: 3600}} onChange={onChange} />);

      await userEvent.clear(getNumber());
      await userEvent.type(getNumber(), '2');

      expect(onChange).toHaveBeenLastCalledWith({validityPeriod: 7200});
    });

    it('keeps the number and rescales it when the unit changes', async () => {
      const onChange = vi.fn();
      render(<Harness initial={{validityPeriod: 1800}} onChange={onChange} />);

      await chooseUnit('Hours');

      expect(getNumber()).toHaveValue(30);
      expect(onChange).toHaveBeenLastCalledWith({validityPeriod: 108000});
    });

    it('sends 0 when No expiry is chosen', async () => {
      const onChange = vi.fn();
      render(<Harness initial={{validityPeriod: 3600}} onChange={onChange} />);

      await chooseUnit('No expiry');

      expect(onChange).toHaveBeenLastCalledWith({validityPeriod: 0});
      expect(queryNumber()).not.toBeInTheDocument();
    });

    it('says why when the number box is cleared, and stops once a number is typed', async () => {
      render(<Harness initial={{validityPeriod: 3600}} onChange={vi.fn()} />);

      await userEvent.clear(getNumber());

      expect(screen.getByText('Enter a whole number of 1 or more.')).toBeInTheDocument();

      await userEvent.type(getNumber(), '2');

      expect(screen.queryByText('Enter a whole number of 1 or more.')).not.toBeInTheDocument();
    });

    it('sends nothing while the number box is empty', async () => {
      const onChange = vi.fn();
      render(<Harness initial={{validityPeriod: 3600}} onChange={onChange} />);

      await userEvent.clear(getNumber());

      expect(onChange).not.toHaveBeenCalled();
    });
  });

  describe('Read-only', () => {
    it('disables both controls when disabled', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 3600}} onLoginConsentChange={vi.fn()} disabled />);

      expect(getNumber()).toBeDisabled();
      expect(getUnit()).toHaveAttribute('aria-disabled', 'true');
    });

    it('disables both controls when no change handler is given', () => {
      render(<LoginConsentField loginConsent={{validityPeriod: 3600}} />);

      expect(getNumber()).toBeDisabled();
      expect(getUnit()).toHaveAttribute('aria-disabled', 'true');
    });
  });
});
