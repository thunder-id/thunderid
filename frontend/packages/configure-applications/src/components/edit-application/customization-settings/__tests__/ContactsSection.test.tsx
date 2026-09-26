// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ContactsSection from '../ContactsSection';
import type {Application} from '@thunderid/configure-applications';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({i18nKey}: {i18nKey: string}) => i18nKey,
}));

describe('ContactsSection', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test Description',
    template: 'custom',
    contacts: ['contact1@example.com', 'contact2@example.com'],
  } as Application;

  const mockOnFieldChange = vi.fn();

  beforeEach(() => {
    mockOnFieldChange.mockClear();
  });

  describe('Rendering', () => {
    it('should render the section title and description', () => {
      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.general.sections.contacts')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.general.sections.contacts.description')).toBeInTheDocument();
    });

    it('should render the autocomplete input with a placeholder', () => {
      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByPlaceholderText('applications:edit.general.contacts.placeholder')).toBeInTheDocument();
    });

    it('should display hint text when there is no error', () => {
      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.general.contacts.hint')).toBeInTheDocument();
    });

    it('should not show an error state initially', () => {
      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.general.contacts.placeholder');

      expect(input).not.toHaveAttribute('aria-invalid', 'true');
    });
  });

  describe('Initial Values', () => {
    it('should display contacts from application as chips', () => {
      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('contact1@example.com')).toBeInTheDocument();
      expect(screen.getByText('contact2@example.com')).toBeInTheDocument();
    });

    it('should prioritize editedApp contacts over application contacts', () => {
      const editedApp = {contacts: ['edited1@example.com', 'edited2@example.com']};

      render(<ContactsSection application={mockApplication} editedApp={editedApp} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('edited1@example.com')).toBeInTheDocument();
      expect(screen.getByText('edited2@example.com')).toBeInTheDocument();
      expect(screen.queryByText('contact1@example.com')).not.toBeInTheDocument();
    });

    it('should render no chips when contacts list is empty', () => {
      const appWithoutContacts = {...mockApplication, contacts: []};

      render(<ContactsSection application={appWithoutContacts} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByText(/@example\.com/)).not.toBeInTheDocument();
    });

    it('should render one chip for a single contact', () => {
      const appWithOneContact = {...mockApplication, contacts: ['single@example.com']};

      render(<ContactsSection application={appWithOneContact} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('single@example.com')).toBeInTheDocument();
    });
  });

  describe('Email Validation', () => {
    it('should call onFieldChange with updated contacts when a valid email is entered', async () => {
      const user = userEvent.setup({delay: null});
      const appWithNoContacts = {...mockApplication, contacts: []};

      render(<ContactsSection application={appWithNoContacts} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.general.contacts.placeholder');
      await user.type(input, 'valid@example.com');
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(mockOnFieldChange).toHaveBeenCalledWith('contacts', ['valid@example.com']);
      });
    });

    it('should show an error message when an invalid email is entered', async () => {
      const user = userEvent.setup({delay: null});
      const appWithNoContacts = {...mockApplication, contacts: []};

      render(<ContactsSection application={appWithNoContacts} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.general.contacts.placeholder');
      await user.type(input, 'not-an-email');
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(screen.getByText('applications:edit.general.contacts.error.invalid')).toBeInTheDocument();
      });
    });

    it('should not call onFieldChange when an invalid email is entered', async () => {
      const user = userEvent.setup({delay: null});
      const appWithNoContacts = {...mockApplication, contacts: []};

      render(<ContactsSection application={appWithNoContacts} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.general.contacts.placeholder');
      await user.type(input, 'not-an-email');
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(screen.getByText('applications:edit.general.contacts.error.invalid')).toBeInTheDocument();
      });
      expect(mockOnFieldChange).not.toHaveBeenCalled();
    });

    it('should clear the error when the user starts typing again after an error', async () => {
      const user = userEvent.setup({delay: null});
      const appWithNoContacts = {...mockApplication, contacts: []};

      render(<ContactsSection application={appWithNoContacts} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.general.contacts.placeholder');

      await user.type(input, 'bad-email');
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(screen.getByText('applications:edit.general.contacts.error.invalid')).toBeInTheDocument();
      });

      await user.type(input, 'a');

      await waitFor(() => {
        expect(screen.queryByText('applications:edit.general.contacts.error.invalid')).not.toBeInTheDocument();
      });
    });

    it('should append a new valid contact to existing ones', async () => {
      const user = userEvent.setup({delay: null});

      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByPlaceholderText('applications:edit.general.contacts.placeholder');
      await user.type(input, 'new@example.com');
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(mockOnFieldChange).toHaveBeenCalledWith('contacts', [
          'contact1@example.com',
          'contact2@example.com',
          'new@example.com',
        ]);
      });
    });
  });

  describe('Edge Cases', () => {
    it('should handle missing contacts property in application gracefully', () => {
      const appWithoutContactsProp = {...mockApplication} as Partial<Application>;
      delete appWithoutContactsProp.contacts;

      render(
        <ContactsSection
          application={appWithoutContactsProp as Application}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText(/@/)).not.toBeInTheDocument();
    });

    it('should not call onFieldChange on initial render', () => {
      render(<ContactsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(mockOnFieldChange).not.toHaveBeenCalled();
    });
  });
});
