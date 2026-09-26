// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AttestationSection from '../AttestationSection';
import type {AttestationConfig} from '@thunderid/configure-applications';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Selects a platform from the attestation platform Autocomplete.
async function selectPlatform(user: ReturnType<typeof userEvent.setup>, optionKey: string) {
  await user.click(screen.getByRole('combobox'));
  await user.click(await screen.findByRole('option', {name: optionKey}));
}

describe('AttestationSection', () => {
  const mockOnAttestationChange = vi.fn();

  beforeEach(() => {
    mockOnAttestationChange.mockClear();
  });

  describe('Rendering', () => {
    it('should render the attestation section with the platform selector', () => {
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      expect(screen.getByText('applications:edit.advanced.labels.attestation')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.attestation.intro')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.attestation.labels.platform')).toBeInTheDocument();
    });

    it('should not render platform fields when no platform is configured', () => {
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      expect(
        screen.queryByLabelText('applications:edit.advanced.attestation.labels.packageName'),
      ).not.toBeInTheDocument();
      expect(screen.queryByLabelText('applications:edit.advanced.attestation.labels.teamId')).not.toBeInTheDocument();
    });

    it('should render the Android fields when an android config is present', () => {
      render(
        <AttestationSection
          attestation={{android: {packageName: 'com.example.app'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      expect(screen.getByLabelText('applications:edit.advanced.attestation.labels.packageName')).toBeInTheDocument();
      expect(
        screen.getByLabelText('applications:edit.advanced.attestation.labels.serviceAccountCredentials'),
      ).toBeInTheDocument();
    });

    it('should render the configured package name and digests', () => {
      render(
        <AttestationSection
          attestation={{android: {packageName: 'com.example.app', certificateSha256Digests: ['AA:BB', 'CC:DD']}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      expect(screen.getByDisplayValue('com.example.app')).toBeInTheDocument();
      expect(screen.getByDisplayValue('AA:BB')).toBeInTheDocument();
      expect(screen.getByDisplayValue('CC:DD')).toBeInTheDocument();
    });

    it('should not render the service account credentials value even when configured', () => {
      // The credentials field is write-only; the component never displays a stored value.
      render(
        <AttestationSection
          attestation={{android: {packageName: 'com.example.app', serviceAccountCredentials: 'secret-json'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      expect(screen.queryByDisplayValue('secret-json')).not.toBeInTheDocument();
    });

    it('should render the dev mode toggle unchecked by default', () => {
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      const toggle = screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode');
      expect(toggle).not.toBeChecked();
    });

    it('should render the dev mode toggle checked when dev mode is enabled', () => {
      render(<AttestationSection attestation={{devMode: true}} onAttestationChange={mockOnAttestationChange} />);

      const toggle = screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode');
      expect(toggle).toBeChecked();
    });

    it('should render the Apple fields with values when an apple config is present', () => {
      render(
        <AttestationSection
          attestation={{apple: {teamId: 'ABCDE12345', bundleId: 'com.example.myapp'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      expect(screen.getByDisplayValue('ABCDE12345')).toBeInTheDocument();
      expect(screen.getByDisplayValue('com.example.myapp')).toBeInTheDocument();
    });
  });

  describe('Editing', () => {
    it('should emit an android config after selecting Android and setting the package name', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.android');
      const input = screen.getByLabelText('applications:edit.advanced.attestation.labels.packageName');
      await user.type(input, 'x');

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith({android: {packageName: 'x'}});
    });

    it('should not emit an apple config while only the team id is set', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.apple');
      const input = screen.getByLabelText('applications:edit.advanced.attestation.labels.teamId');
      await user.type(input, 'A');

      // Selecting the platform emits the (empty, i.e. null) config once; typing into a lone,
      // incomplete field must not emit again with a partial apple config the backend can't verify.
      expect(mockOnAttestationChange).toHaveBeenCalledTimes(1);
      expect(mockOnAttestationChange).toHaveBeenLastCalledWith(null);
    });

    it('should emit a complete apple config once both team id and bundle id are set', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.apple');
      await user.type(screen.getByLabelText('applications:edit.advanced.attestation.labels.teamId'), 'ABCDE12345');
      await user.type(
        screen.getByLabelText('applications:edit.advanced.attestation.labels.bundleId'),
        'com.example.myapp',
      );

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith({
        apple: {teamId: 'ABCDE12345', bundleId: 'com.example.myapp'},
      });
    });

    it('should show a validation hint on the empty field while the apple config is incomplete', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{apple: {teamId: 'ABCDE12345', bundleId: 'com.example.myapp'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      // Clearing bundleId leaves teamId alone (incomplete) — the last valid (complete) config is
      // never overwritten with the partial state, and a validation hint appears on the empty field.
      await user.clear(screen.getByLabelText('applications:edit.advanced.attestation.labels.bundleId'));

      expect(mockOnAttestationChange).not.toHaveBeenCalled();
      expect(screen.getByText('applications:edit.advanced.attestation.error.appleIncomplete')).toBeInTheDocument();
    });

    it('should emit null when the platform is set to None', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{android: {packageName: 'com.example.app'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.none');

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith(null);
    });

    it('should emit null when the only configured value is cleared', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{android: {packageName: 'x'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      const input = screen.getByLabelText('applications:edit.advanced.attestation.labels.packageName');
      await user.clear(input);

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith(null);
    });

    it('should add a digest row when Add Digest is clicked', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{android: {packageName: 'x'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      await user.click(screen.getByText('applications:edit.advanced.attestation.addDigest'));

      // An empty list already shows one placeholder row; clicking Add appends a second real row.
      expect(
        screen.getAllByPlaceholderText('applications:edit.advanced.attestation.placeholder.certificateSha256Digest'),
      ).toHaveLength(2);
    });

    it('should emit the service account credentials when entered', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{android: {packageName: 'com.example.app'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      const creds = screen.getByLabelText('applications:edit.advanced.attestation.labels.serviceAccountCredentials');
      await user.type(creds, '{{"type":"service_account"}');

      const calls = mockOnAttestationChange.mock.calls as [AttestationConfig | null][];
      const lastArg = calls[calls.length - 1][0];
      expect(lastArg?.android?.serviceAccountCredentials).toBe('{"type":"service_account"}');
    });
  });

  describe('Dev Mode', () => {
    it('should open the confirm dialog without emitting when toggled on', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));

      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(mockOnAttestationChange).not.toHaveBeenCalled();
    });

    it('should emit a dev mode config when the confirm dialog is accepted', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));
      await user.click(screen.getByTestId('dev-mode-confirm-button'));

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith({devMode: true});
    });

    it('should not emit or check the toggle when the confirm dialog is canceled', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));
      await user.click(screen.getByText('applications:edit.advanced.attestation.devModeConfirmDialog.cancelButton'));

      expect(mockOnAttestationChange).not.toHaveBeenCalled();
      expect(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode')).not.toBeChecked();
    });

    it('should emit null immediately when dev mode is disabled again, without a confirm dialog', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection attestation={{devMode: true}} onAttestationChange={mockOnAttestationChange} />);

      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith(null);
    });

    it('should keep dev mode set alongside an android config once confirmed', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{android: {packageName: 'com.example.app'}}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));
      await user.click(screen.getByTestId('dev-mode-confirm-button'));

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith({
        android: {packageName: 'com.example.app'},
        devMode: true,
      });
    });

    it('should still emit enabling dev mode when the apple config is left incomplete', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.apple');
      await user.type(screen.getByLabelText('applications:edit.advanced.attestation.labels.teamId'), 'A');
      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));
      await user.click(screen.getByTestId('dev-mode-confirm-button'));

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith({devMode: true});
    });

    it('should still emit the preserved apple config when disabling dev mode while apple is left incomplete', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <AttestationSection
          attestation={{apple: {teamId: 'ABCDE12345', bundleId: 'com.example.myapp'}, devMode: true}}
          onAttestationChange={mockOnAttestationChange}
        />,
      );

      await user.clear(screen.getByLabelText('applications:edit.advanced.attestation.labels.bundleId'));
      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));

      expect(mockOnAttestationChange).toHaveBeenLastCalledWith({
        apple: {teamId: 'ABCDE12345', bundleId: 'com.example.myapp'},
      });
    });

    it('should not show the warning banner when dev mode is disabled', () => {
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      expect(screen.queryByText('applications:edit.advanced.attestation.warning.devMode')).not.toBeInTheDocument();
    });

    it('should show the warning banner when dev mode is enabled', () => {
      render(<AttestationSection attestation={{devMode: true}} onAttestationChange={mockOnAttestationChange} />);

      expect(screen.getByText('applications:edit.advanced.attestation.warning.devMode')).toBeInTheDocument();
    });

    it('should not show the warning banner until the confirm dialog is accepted', async () => {
      const user = userEvent.setup({delay: null});
      render(<AttestationSection onAttestationChange={mockOnAttestationChange} />);

      await user.click(screen.getByLabelText('applications:edit.advanced.attestation.labels.devMode'));
      expect(screen.queryByText('applications:edit.advanced.attestation.warning.devMode')).not.toBeInTheDocument();

      await user.click(screen.getByTestId('dev-mode-confirm-button'));

      expect(screen.getByText('applications:edit.advanced.attestation.warning.devMode')).toBeInTheDocument();
    });
  });

  describe('Validation', () => {
    it('reports a validation error while the apple config is incomplete, resolving once both fields are set', async () => {
      const user = userEvent.setup({delay: null});
      const mockOnValidationChange = vi.fn();
      render(
        <AttestationSection
          onAttestationChange={mockOnAttestationChange}
          onValidationChange={mockOnValidationChange}
        />,
      );

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.apple');
      await user.type(screen.getByLabelText('applications:edit.advanced.attestation.labels.teamId'), 'ABCDE12345');

      expect(mockOnValidationChange).toHaveBeenLastCalledWith(true);

      await user.type(
        screen.getByLabelText('applications:edit.advanced.attestation.labels.bundleId'),
        'com.example.myapp',
      );

      expect(mockOnValidationChange).toHaveBeenLastCalledWith(false);
    });

    it('resolves the validation error when the section is cleared back to no platform', async () => {
      const user = userEvent.setup({delay: null});
      const mockOnValidationChange = vi.fn();
      render(
        <AttestationSection
          attestation={{apple: {teamId: 'ABCDE12345', bundleId: 'com.example.myapp'}}}
          onAttestationChange={mockOnAttestationChange}
          onValidationChange={mockOnValidationChange}
        />,
      );

      await user.clear(screen.getByLabelText('applications:edit.advanced.attestation.labels.bundleId'));
      expect(mockOnValidationChange).toHaveBeenLastCalledWith(true);

      await selectPlatform(user, 'applications:edit.advanced.attestation.platform.none');
      expect(mockOnValidationChange).toHaveBeenLastCalledWith(false);
    });
  });
});
