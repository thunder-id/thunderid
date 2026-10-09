// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureTemplateIdentity, {type ConfigureTemplateIdentityProps} from '../ConfigureTemplateIdentity';

// Auto-mock the utility library; NameSuggestion is left as the real implementation, so its
// dependency on generateRandomHumanReadableIdentifiers is stubbed here too.
vi.mock('@thunderid/utils');

const {generateHandle, generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

describe('ConfigureTemplateIdentity', () => {
  const onDisplayNameChange = vi.fn();
  const onHandleChange = vi.fn();
  const onHandleEditedChange = vi.fn();
  const onReadyChange = vi.fn();

  const defaultProps: ConfigureTemplateIdentityProps = {
    channel: 'email',
    displayName: '',
    handle: '',
    handleEdited: false,
    onDisplayNameChange,
    onHandleChange,
    onHandleEditedChange,
    onReadyChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(generateHandle).mockImplementation((value: string) =>
      value.toLowerCase().trim().replace(/\s+/g, '-'),
    );
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(['Cyan Carrots Dream']);
  });

  const renderComponent = (props: Partial<ConfigureTemplateIdentityProps> = {}) =>
    render(<ConfigureTemplateIdentity {...defaultProps} {...props} />);

  describe('Rendering', () => {
    it('renders the email title for the email channel', () => {
      renderComponent({channel: 'email'});

      expect(
        screen.getByText("Let's collect some details about your email template"),
      ).toBeInTheDocument();
    });

    it('renders the SMS title for the SMS channel', () => {
      renderComponent({channel: 'sms'});

      expect(screen.getByText("Let's collect some details about your SMS template")).toBeInTheDocument();
    });

    it('renders the display name and handle fields', () => {
      renderComponent();

      expect(screen.getByText('Display name')).toBeInTheDocument();
      expect(screen.getByText('Handle')).toBeInTheDocument();
    });
  });

  describe('Display name and handle derivation', () => {
    it('updates the display name and derives the handle while the handle is not edited', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.type(screen.getByPlaceholderText('e.g. OTP Verification'), 'A');

      expect(onDisplayNameChange).toHaveBeenCalledWith('A');
      expect(onHandleChange).toHaveBeenCalledWith('a');
    });

    it('does not derive the handle once the user has edited it', async () => {
      const user = userEvent.setup();
      renderComponent({handleEdited: true});

      await user.type(screen.getByPlaceholderText('e.g. OTP Verification'), 'A');

      expect(onDisplayNameChange).toHaveBeenCalledWith('A');
      expect(onHandleChange).not.toHaveBeenCalled();
    });

    it('marks the handle as edited and sanitizes input when the handle is typed', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.type(screen.getByPlaceholderText('e.g. otp-verification'), 'A');

      expect(onHandleEditedChange).toHaveBeenCalledWith(true);
      expect(onHandleChange).toHaveBeenCalledWith('a');
    });

    it('strips disallowed characters from the handle input', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.type(screen.getByPlaceholderText('e.g. otp-verification'), '!');

      expect(onHandleChange).toHaveBeenCalledWith('');
    });
  });

  describe('Name suggestion', () => {
    it('applies the suggestion to the display name and derives a handle when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByText('Cyan Carrots Dream'));

      expect(onHandleEditedChange).toHaveBeenCalledWith(false);
      expect(onDisplayNameChange).toHaveBeenCalledWith('Cyan Carrots Dream');
      expect(onHandleChange).toHaveBeenCalledWith('cyan-carrots-dream');
    });
  });

  describe('Validation', () => {
    it('reports ready when both the display name and handle are set and valid', () => {
      renderComponent({displayName: 'OTP Verification', handle: 'otp-verification'});

      expect(onReadyChange).toHaveBeenCalledWith(true);
    });

    it('reports not ready when the display name is empty', () => {
      renderComponent({displayName: '', handle: 'otp-verification'});

      expect(onReadyChange).toHaveBeenCalledWith(false);
    });

    it('shows a duplicate error and blocks readiness for an existing handle', () => {
      renderComponent({
        displayName: 'OTP Verification',
        handle: 'otp-verification',
        existingHandles: ['otp-verification'],
      });

      expect(screen.getByText('A template with this handle already exists.')).toBeInTheDocument();
      expect(onReadyChange).toHaveBeenCalledWith(false);
    });

    it('shows an invalid-format error and blocks readiness for a malformed handle', () => {
      renderComponent({displayName: 'OTP Verification', handle: '-bad'});

      expect(screen.getByText('Use lowercase letters, numbers, hyphens and underscores.')).toBeInTheDocument();
      expect(onReadyChange).toHaveBeenCalledWith(false);
    });
  });
});
