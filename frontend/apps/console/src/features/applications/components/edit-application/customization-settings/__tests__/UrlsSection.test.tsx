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

import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Application} from '../../../../models/application';
import UrlsSection from '../UrlsSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('UrlsSection', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test Description',
    template: 'custom',
    tosUri: 'https://example.com/terms',
    policyUri: 'https://example.com/privacy',
  } as Application;

  const mockOnFieldChange = vi.fn();

  beforeEach(() => {
    mockOnFieldChange.mockClear();
  });

  describe('Rendering', () => {
    it('should render the URLs section', () => {
      render(<UrlsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.sections.urls')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.sections.urls.description')).toBeInTheDocument();
    });

    it('should render Terms of Service URL field', () => {
      render(<UrlsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.labels.tosUri')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder')).toBeInTheDocument();
    });

    it('should render Privacy Policy URL field', () => {
      render(<UrlsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.labels.policyUri')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder')).toBeInTheDocument();
    });

    it('should display helper text for both fields', () => {
      render(<UrlsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.tosUri.hint')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.policyUri.hint')).toBeInTheDocument();
    });
  });

  describe('Initial Values', () => {
    it('should display URLs from application', () => {
      render(<UrlsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');

      expect(tosField).toHaveValue('https://example.com/terms');
      expect(policyField).toHaveValue('https://example.com/privacy');
    });

    it('should prioritize editedApp URLs over application', () => {
      const editedApp = {
        tosUri: 'https://edited.com/terms',
        policyUri: 'https://edited.com/privacy',
      };

      render(<UrlsSection application={mockApplication} editedApp={editedApp} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');

      expect(tosField).toHaveValue('https://edited.com/terms');
      expect(policyField).toHaveValue('https://edited.com/privacy');
    });

    it('should display empty strings when URLs are not provided', () => {
      const appWithoutUrls = {...mockApplication};
      delete (appWithoutUrls as Partial<Application>).tosUri;
      delete (appWithoutUrls as Partial<Application>).policyUri;

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');

      expect(tosField).toHaveValue('');
      expect(policyField).toHaveValue('');
    });
  });

  describe('URL Validation', () => {
    it('should show error for invalid ToS URL', async () => {
      const user = userEvent.setup({delay: null});
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      await user.type(tosField, 'invalid-url');
      await user.tab();

      await waitFor(() => {
        expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
      });
    });

    it('should show error for invalid Policy URL', async () => {
      const user = userEvent.setup({delay: null});
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');
      await user.type(policyField, 'not-a-url');
      await user.tab();

      await waitFor(() => {
        expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
      });
    });

    it('should not show error for valid ToS URL', async () => {
      const user = userEvent.setup({delay: null});
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      await user.type(tosField, 'https://example.com/terms');
      await user.tab();

      await waitFor(() => {
        expect(screen.queryByText('Please enter a valid URL')).not.toBeInTheDocument();
        expect(screen.getByText('applications:edit.customization.tosUri.hint')).toBeInTheDocument();
      });
    });

    it('should accept empty string as valid', async () => {
      const user = userEvent.setup({delay: null});

      render(<UrlsSection application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      await user.clear(tosField);
      await user.tab();

      await waitFor(() => {
        expect(screen.queryByText('Please enter a valid URL')).not.toBeInTheDocument();
      });
    });
  });

  describe('User Input', () => {
    it('should accept valid ToS URL input', async () => {
      const user = userEvent.setup({delay: null});
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      await user.type(tosField, 'https://new-url.com/terms');

      // Verify the field accepts input
      expect(tosField).toHaveValue('https://new-url.com/terms');
    });

    it('should accept valid Policy URL input', async () => {
      const user = userEvent.setup({delay: null});
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');
      await user.type(policyField, 'https://new-url.com/privacy');

      // Verify the field accepts input
      expect(policyField).toHaveValue('https://new-url.com/privacy');
    });
  });

  describe('Validation Change Callback', () => {
    it('should notify parent with no errors for valid default values', async () => {
      const mockOnValidationChange = vi.fn();

      render(
        <UrlsSection
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          onValidationChange={mockOnValidationChange}
        />,
      );

      await waitFor(() => {
        expect(mockOnValidationChange).toHaveBeenCalledWith(false);
      });
    });

    it('should notify parent with errors when a URL becomes invalid', async () => {
      const user = userEvent.setup({delay: null});
      const mockOnValidationChange = vi.fn();
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(
        <UrlsSection
          application={appWithoutUrls}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          onValidationChange={mockOnValidationChange}
        />,
      );

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      await user.type(tosField, 'invalid-url');
      await user.tab();

      await waitFor(() => {
        expect(mockOnValidationChange).toHaveBeenCalledWith(true);
      });
    });
  });

  describe('Edge Cases', () => {
    it('should handle missing URLs in application', () => {
      const appWithoutUrls = {...mockApplication};
      delete (appWithoutUrls as Partial<Application>).tosUri;
      delete (appWithoutUrls as Partial<Application>).policyUri;

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder')).toHaveValue('');
      expect(screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder')).toHaveValue('');
    });

    it('should validate URLs with different protocols', async () => {
      const user = userEvent.setup({delay: null});
      const appWithoutUrls = {...mockApplication, tosUri: '', policyUri: ''};

      render(<UrlsSection application={appWithoutUrls} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      await user.type(tosField, 'http://example.com/terms');
      await user.tab();

      await waitFor(() => {
        expect(screen.queryByText('Please enter a valid URL')).not.toBeInTheDocument();
      });
    });
  });
});
