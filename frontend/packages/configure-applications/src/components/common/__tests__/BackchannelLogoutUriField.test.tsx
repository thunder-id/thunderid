// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import BackchannelLogoutUriField from '../BackchannelLogoutUriField';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const INPUT_ID = 'backchannel-logout-uri-input';
const INVALID_KEY = 'applications:edit.general.backchannelLogoutUri.error.invalid';
const REQUIRES_HTTPS_KEY = 'applications:edit.general.backchannelLogoutUri.error.requiresHttps';

const input = (): HTMLInputElement => document.getElementById(INPUT_ID) as HTMLInputElement;

describe('BackchannelLogoutUriField', () => {
  describe('Rendering', () => {
    it('renders the field with the registered value', () => {
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
      // The report follows the shown error, so the page does not name an error the field hides.
      expect(onValidationChange).not.toHaveBeenCalledWith(true);

      await user.tab();

      expect(screen.getByText(INVALID_KEY)).toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
      expect(onChange).not.toHaveBeenCalled();
    });

    it('does not commit an unchanged value on blur', async () => {
      const user = userEvent.setup();
      const onChange = vi.fn();
      render(<BackchannelLogoutUriField value="https://rp.example.com/bcl" onChange={onChange} />);

      await user.click(input());
      await user.tab();

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

    it('shows the error when a confidential client with an http URI becomes public', () => {
      const onValidationChange = vi.fn();
      const {rerender} = render(
        <BackchannelLogoutUriField
          value="http://rp.example.com/bcl"
          onChange={vi.fn()}
          onValidationChange={onValidationChange}
        />,
      );
      expect(screen.queryByText(REQUIRES_HTTPS_KEY)).not.toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(false);

      rerender(
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

    it('drops the draft and its error when the value changes from outside', async () => {
      const user = userEvent.setup();
      const onValidationChange = vi.fn();
      const {rerender} = render(
        <BackchannelLogoutUriField
          value="https://a.example.com/bcl"
          onChange={vi.fn()}
          onValidationChange={onValidationChange}
        />,
      );

      await user.clear(input());
      await user.type(input(), 'https://rp.example.com/bcl#frag');
      await user.tab();
      expect(screen.getByText(INVALID_KEY)).toBeInTheDocument();

      rerender(
        <BackchannelLogoutUriField
          value="https://b.example.com/bcl"
          onChange={vi.fn()}
          onValidationChange={onValidationChange}
        />,
      );

      expect(input()).toHaveValue('https://b.example.com/bcl');
      expect(screen.queryByText(INVALID_KEY)).not.toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });

    it('keeps reporting a mounted invalid field as invalid when the callback changes', async () => {
      const user = userEvent.setup();
      const first = vi.fn();
      const second = vi.fn();
      const {rerender} = render(<BackchannelLogoutUriField onChange={vi.fn()} onValidationChange={first} />);

      await user.type(input(), 'https://rp.example.com/bcl#frag');
      await user.tab();
      expect(first).toHaveBeenLastCalledWith(true);

      rerender(<BackchannelLogoutUriField onChange={vi.fn()} onValidationChange={second} />);

      expect(first).toHaveBeenLastCalledWith(true);
      expect(second).toHaveBeenLastCalledWith(true);
      expect(second).not.toHaveBeenCalledWith(false);
    });

    it('reports no error once unmounted', async () => {
      const user = userEvent.setup();
      const onValidationChange = vi.fn();
      const {unmount} = render(
        <BackchannelLogoutUriField onChange={vi.fn()} onValidationChange={onValidationChange} />,
      );

      await user.type(input(), 'https://rp.example.com/bcl#frag');
      await user.tab();
      expect(onValidationChange).toHaveBeenLastCalledWith(true);

      unmount();

      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });
  });
});
