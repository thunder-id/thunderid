/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {AttestationConfig} from '../../../../models/oauth';
import AttestationSection from '../AttestationSection';

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

      expect(
        screen.getByPlaceholderText('applications:edit.advanced.attestation.placeholder.certificateSha256Digest'),
      ).toBeInTheDocument();
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
