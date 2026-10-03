// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import BackchannelLogoutUriField from '../BackchannelLogoutUriField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const {mockUseThunderID} = vi.hoisted(() => ({
  mockUseThunderID: vi.fn(),
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: mockUseThunderID,
}));

const INPUT_ID = 'backchannel-logout-uri-input';
const INVALID_KEY = 'applications:edit.general.backchannelLogoutUri.error.invalid';
const REQUIRES_HTTPS_KEY = 'applications:edit.general.backchannelLogoutUri.error.requiresHttps';

const setSupported = (supported: boolean | undefined): void => {
  mockUseThunderID.mockReturnValue({discovery: {wellKnown: {backchannel_logout_supported: supported}}});
};

const input = (): HTMLInputElement => document.getElementById(INPUT_ID) as HTMLInputElement;

describe('BackchannelLogoutUriField', () => {
  beforeEach(() => {
    setSupported(true);
  });

  describe('Visibility', () => {
    it.each([false, undefined])('renders nothing when discovery reports support as %j', (supported) => {
      setSupported(supported);
      const onValidationChange = vi.fn();

      const {container} = render(
        <BackchannelLogoutUriField value="http://bad" publicClient onValidationChange={onValidationChange} />,
      );

      expect(container.firstChild).toBeNull();
      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });

    it('renders the field with the registered value when supported', () => {
      render(<BackchannelLogoutUriField value="https://rp.example.com/bcl" onChange={vi.fn()} />);

      expect(screen.getByText('applications:edit.general.backchannelLogoutUri.title')).toBeInTheDocument();
      expect(input()).toHaveValue('https://rp.example.com/bcl');
      expect(input()).toBeEnabled();
    });

    it('is read-only without onChange or when disabled', () => {
      const {rerender} = render(<BackchannelLogoutUriField value="https://rp.example.com/bcl" />);
      expect(input()).toBeDisabled();

      rerender(<BackchannelLogoutUriField value="https://rp.example.com/bcl" onChange={vi.fn()} disabled />);
      expect(input()).toBeDisabled();
    });
  });

  describe('Editing', () => {
    it('commits the trimmed URI on blur', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      render(<BackchannelLogoutUriField onChange={onChange} />);

      await user.type(input(), '  https://rp.example.com/bcl  ');
      await user.tab();

      expect(onChange).toHaveBeenCalledWith('https://rp.example.com/bcl');
    });

    it('commits an empty value, which turns notifications off', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      render(<BackchannelLogoutUriField value="https://rp.example.com/bcl" onChange={onChange} />);

      await user.clear(input());
      await user.tab();

      expect(onChange).toHaveBeenCalledWith('');
    });

    it('shows the error only after blur, does not commit, and reports the field invalid', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      const onValidationChange = vi.fn();
      render(<BackchannelLogoutUriField onChange={onChange} onValidationChange={onValidationChange} />);

      await user.type(input(), 'https://rp.example.com/bcl#frag');
      expect(screen.queryByText(INVALID_KEY)).not.toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(true);

      await user.tab();

      expect(screen.getByText(INVALID_KEY)).toBeInTheDocument();
      expect(onChange).not.toHaveBeenCalled();
    });

    it('requires https for a public client', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      render(<BackchannelLogoutUriField publicClient onChange={onChange} />);

      await user.type(input(), 'http://rp.example.com/bcl');
      await user.tab();

      expect(screen.getByText(REQUIRES_HTTPS_KEY)).toBeInTheDocument();
      expect(onChange).not.toHaveBeenCalled();
    });

    it('shows the error at once when the saved value becomes invalid', () => {
      const onValidationChange = vi.fn();
      render(
        <BackchannelLogoutUriField
          value="http://rp.example.com/bcl"
          publicClient
          onChange={vi.fn()}
          onValidationChange={onValidationChange}
        />,
      );

      expect(screen.getByText(REQUIRES_HTTPS_KEY)).toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
  });
});
