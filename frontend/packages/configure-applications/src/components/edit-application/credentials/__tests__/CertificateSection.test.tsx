// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CertificateTypes from '../../../../constants/certificate-types';
import CertificateSection from '../CertificateSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('CertificateSection', () => {
  const mockOnCertificateChange = vi.fn();

  beforeEach(() => {
    mockOnCertificateChange.mockClear();
  });

  describe('Rendering', () => {
    it('should render the certificate section', () => {
      render(<CertificateSection onCertificateChange={mockOnCertificateChange} />);

      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.certificate.intro')).toBeInTheDocument();
    });

    it('should render certificate type dropdown', () => {
      render(<CertificateSection onCertificateChange={mockOnCertificateChange} />);

      expect(screen.getByLabelText('applications:edit.advanced.labels.certificateType')).toBeInTheDocument();
    });

    it('should not show value field when certificate is null', () => {
      render(<CertificateSection certificate={null} onCertificateChange={mockOnCertificateChange} />);

      expect(
        screen.queryByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwks'),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwksUri'),
      ).not.toBeInTheDocument();
    });

    it('should show JWKS value field when certificate type is JWKS', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(
        screen.getByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwks'),
      ).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.certificate.hint.jwks')).toBeInTheDocument();
    });

    it('should show JWKS URI value field when certificate type is JWKS_URI', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS_URI, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(
        screen.getByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwksUri'),
      ).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.certificate.hint.jwksUri')).toBeInTheDocument();
    });
  });

  describe('Certificate Type Selection', () => {
    it('should display JWKS type from certificate prop', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const input = screen.getByRole('combobox');
      expect(input).toHaveValue('applications:edit.advanced.certificate.type.jwks');
    });

    it('should call onCertificateChange with null when NONE is selected', async () => {
      const user = userEvent.setup();
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      const noneOption = within(listbox).getByText('applications:edit.advanced.certificate.type.none');
      await user.click(noneOption);

      expect(mockOnCertificateChange).toHaveBeenCalledWith(null);
    });

    it('should block removal and warn when an encrypted token format depends on the certificate', async () => {
      const user = userEvent.setup();
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: 'jwks'}}
          onCertificateChange={mockOnCertificateChange}
          encryptionDependsOnCert
        />,
      );

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      const noneOption = within(listbox).getByText('applications:edit.advanced.certificate.type.none');
      await user.click(noneOption);

      expect(mockOnCertificateChange).not.toHaveBeenCalled();
      expect(
        screen.getByText('applications:edit.advanced.certificate.error.encryptionDependsOnCert'),
      ).toBeInTheDocument();
    });

    it('should allow switching to another certificate type even when encryption depends on the certificate', async () => {
      const user = userEvent.setup();
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: 'jwks'}}
          onCertificateChange={mockOnCertificateChange}
          encryptionDependsOnCert
        />,
      );

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      const jwksUriOption = within(listbox).getByText('applications:edit.advanced.certificate.type.jwksUri');
      await user.click(jwksUriOption);

      expect(mockOnCertificateChange).toHaveBeenCalledWith({
        type: CertificateTypes.JWKS_URI,
        value: 'jwks',
      });
    });

    it('should call onCertificateChange with certificate when JWKS is selected', async () => {
      const user = userEvent.setup();
      render(<CertificateSection certificate={null} onCertificateChange={mockOnCertificateChange} />);

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      const jwksOption = within(listbox).getByText('applications:edit.advanced.certificate.type.jwks');
      await user.click(jwksOption);

      expect(mockOnCertificateChange).toHaveBeenCalledWith({
        type: CertificateTypes.JWKS,
        value: '',
      });
    });

    it('should preserve certificate value when changing type', async () => {
      const user = userEvent.setup();
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: 'existing-jwks'}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const listbox = screen.getByRole('listbox');
      const jwksUriOption = within(listbox).getByText('applications:edit.advanced.certificate.type.jwksUri');
      await user.click(jwksUriOption);

      expect(mockOnCertificateChange).toHaveBeenCalledWith({
        type: CertificateTypes.JWKS_URI,
        value: 'existing-jwks',
      });
    });
  });

  describe('Certificate Value Input', () => {
    it('should display current certificate value', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: 'test-jwks-value'}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const valueInput = screen.getByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwks');
      expect(valueInput).toHaveValue('test-jwks-value');
    });

    it('should call onCertificateChange when certificate value changes', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const valueInput = screen.getByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwks');
      await user.type(valueInput, 'new-value');

      expect(mockOnCertificateChange).toHaveBeenCalled();
      expect(mockOnCertificateChange).toHaveBeenCalledWith(
        expect.objectContaining({
          type: CertificateTypes.JWKS,
        }),
      );
    });

    it('should preserve certificate type when changing value', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS_URI, value: 'https://example.com'}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const valueInput = screen.getByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwksUri');
      await user.clear(valueInput);
      await user.type(valueInput, 'https://new-url.com');

      expect(mockOnCertificateChange).toHaveBeenCalledWith(
        expect.objectContaining({
          type: CertificateTypes.JWKS_URI,
        }),
      );
    });
  });

  describe('Edge Cases', () => {
    it('should default to NONE when no certificate prop is provided', () => {
      render(<CertificateSection onCertificateChange={mockOnCertificateChange} />);

      const input = screen.getByRole('combobox');
      expect(input).toHaveValue('applications:edit.advanced.certificate.type.none');
    });

    it('should handle multiline JWKS input', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const valueInput = screen.getByPlaceholderText('applications:edit.advanced.certificate.placeholder.jwks');
      expect(valueInput).toHaveAttribute('rows', '3');
    });
  });
});
