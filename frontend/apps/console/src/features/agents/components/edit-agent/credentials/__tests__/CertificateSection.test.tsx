// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {CertificateTypes} from '@thunderid/configure-applications';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CertificateSection from '../CertificateSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('CertificateSection (agent)', () => {
  const mockOnCertificateChange = vi.fn();

  beforeEach(() => {
    mockOnCertificateChange.mockClear();
  });

  describe('Rendering', () => {
    it('renders the certificate section', () => {
      render(<CertificateSection onCertificateChange={mockOnCertificateChange} />);

      expect(screen.getByText('agents:edit.credentials.certificate.title')).toBeInTheDocument();
      expect(screen.getByText('agents:edit.credentials.certificate.description')).toBeInTheDocument();
    });

    it('renders the certificate type dropdown', () => {
      render(<CertificateSection onCertificateChange={mockOnCertificateChange} />);

      expect(screen.getByLabelText('agents:edit.credentials.certificate.sourceLabel')).toBeInTheDocument();
    });

    it('does not render the value field when certificate is null', () => {
      render(<CertificateSection certificate={null} onCertificateChange={mockOnCertificateChange} />);

      expect(
        screen.queryByPlaceholderText('agents:edit.credentials.certificate.placeholder.jwks'),
      ).not.toBeInTheDocument();
    });

    it('renders the JWKS value field when type is JWKS', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(screen.getByPlaceholderText('agents:edit.credentials.certificate.placeholder.jwks')).toBeInTheDocument();
    });

    it('renders the JWKS URI value field when type is JWKS_URI', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS_URI, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(
        screen.getByPlaceholderText('agents:edit.credentials.certificate.placeholder.jwksUri'),
      ).toBeInTheDocument();
    });
  });

  describe('Certificate Type Selection', () => {
    it('reflects the certificate type from the certificate prop', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(screen.getByRole('combobox')).toHaveValue('agents:edit.credentials.certificate.type.jwks');
    });

    it('calls onCertificateChange with null when NONE is selected', async () => {
      const user = userEvent.setup();
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      await user.click(screen.getByRole('combobox'));
      const listbox = screen.getByRole('listbox');
      await user.click(within(listbox).getByText('agents:edit.credentials.certificate.type.none'));

      expect(mockOnCertificateChange).toHaveBeenCalledWith(null);
    });

    it('calls onCertificateChange with certificate when JWKS is selected', async () => {
      const user = userEvent.setup();
      render(<CertificateSection certificate={null} onCertificateChange={mockOnCertificateChange} />);

      await user.click(screen.getByRole('combobox'));
      const listbox = screen.getByRole('listbox');
      await user.click(within(listbox).getByText('agents:edit.credentials.certificate.type.jwks'));

      expect(mockOnCertificateChange).toHaveBeenCalledWith({
        type: CertificateTypes.JWKS,
        value: '',
      });
    });

    it('preserves certificate value when type changes', async () => {
      const user = userEvent.setup();
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: 'existing'}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      await user.click(screen.getByRole('combobox'));
      const listbox = screen.getByRole('listbox');
      await user.click(within(listbox).getByText('agents:edit.credentials.certificate.type.jwksUri'));

      expect(mockOnCertificateChange).toHaveBeenCalledWith({
        type: CertificateTypes.JWKS_URI,
        value: 'existing',
      });
    });
  });

  describe('Certificate Value Input', () => {
    it('reflects certificate value from the certificate prop', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: 'jwks-value'}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(screen.getByPlaceholderText('agents:edit.credentials.certificate.placeholder.jwks')).toHaveValue(
        'jwks-value',
      );
    });

    it('calls onCertificateChange when the value changes', async () => {
      const user = userEvent.setup({delay: null});
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      const valueInput = screen.getByPlaceholderText('agents:edit.credentials.certificate.placeholder.jwks');
      await user.type(valueInput, 'X');

      expect(mockOnCertificateChange).toHaveBeenCalledWith(expect.objectContaining({type: CertificateTypes.JWKS}));
    });
  });

  describe('Edge Cases', () => {
    it('defaults to NONE when no certificate prop is provided', () => {
      render(<CertificateSection onCertificateChange={mockOnCertificateChange} />);

      expect(screen.getByRole('combobox')).toHaveValue('agents:edit.credentials.certificate.type.none');
    });

    it('renders the JWKS value input as a multiline textarea with 3 rows', () => {
      render(
        <CertificateSection
          certificate={{type: CertificateTypes.JWKS, value: ''}}
          onCertificateChange={mockOnCertificateChange}
        />,
      );

      expect(screen.getByPlaceholderText('agents:edit.credentials.certificate.placeholder.jwks')).toHaveAttribute(
        'rows',
        '3',
      );
    });
  });
});
