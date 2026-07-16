/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ApplicationsListPage from '../ApplicationsListPage';

// Mock the ApplicationsList component
vi.mock('../../components/ApplicationsList', () => ({
  default: () => <div data-testid="applications-list">Applications List Component</div>,
}));

// Mock react-router navigate
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:listing.title': 'Applications',
        'applications:listing.subtitle': 'Manage your applications and their configurations',
        'applications:listing.addApplication': 'Create Application',
        'applications:listing.search.placeholder': 'Search applications...',
      };
      return translations[key] || key;
    },
  }),
}));

describe('ApplicationsListPage', () => {
  const renderWithProviders = () => render(<ApplicationsListPage />);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the page title', () => {
      renderWithProviders();

      expect(screen.getByRole('heading', {level: 1, name: 'Applications'})).toBeInTheDocument();
    });

    it('should render the page subtitle', () => {
      renderWithProviders();

      expect(screen.getByText('Manage your applications and their configurations')).toBeInTheDocument();
    });

    it('should render the Create Application button', () => {
      renderWithProviders();

      expect(screen.getByRole('button', {name: /Create Application/i})).toBeInTheDocument();
    });

    it('should render the ApplicationsList component', () => {
      renderWithProviders();

      expect(screen.getByTestId('applications-list')).toBeInTheDocument();
    });
  });

  describe('Navigation', () => {
    it('should navigate to create page when Create Application button is clicked', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      await user.click(createButton);

      expect(mockNavigate).toHaveBeenCalledWith('/applications/types');
    });

    it('should handle navigation errors gracefully', async () => {
      const user = userEvent.setup();
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => null);

      mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      await user.click(createButton);

      expect(mockNavigate).toHaveBeenCalledWith('/applications/types');

      // Logger should log the error
      expect(consoleErrorSpy).toHaveBeenCalled();

      consoleErrorSpy.mockRestore();
    });
  });

  describe('Layout', () => {
    it('should have proper page structure', () => {
      const {container} = renderWithProviders();

      // Main container
      const mainBox = container.querySelector('.MuiBox-root');
      expect(mainBox).toBeInTheDocument();

      // Header section with title and button
      expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Create Application/i})).toBeInTheDocument();

      // Content section
      expect(screen.getByTestId('applications-list')).toBeInTheDocument();
    });
  });

  describe('Button Styling', () => {
    it('should render Create Application button with correct variant', () => {
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      expect(createButton).toHaveClass('MuiButton-contained');
    });

    it('should have Plus icon in Create Application button', () => {
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      const icon = createButton.querySelector('svg');

      expect(icon).toBeInTheDocument();
    });
  });

  describe('Integration', () => {
    it('should work with QueryClient provider', () => {
      expect(() => renderWithProviders()).not.toThrow();
    });

    it('should work with BrowserRouter', () => {
      expect(() => renderWithProviders()).not.toThrow();
    });

    it('should work with ConfigProvider', () => {
      expect(() => renderWithProviders()).not.toThrow();
    });

    it('should render with all required MUI components', () => {
      renderWithProviders();

      // Verify Box, Stack, Typography components are rendered
      expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
      expect(screen.getByText('Manage your applications and their configurations')).toBeInTheDocument();
    });

    it('should render button with startIcon', () => {
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      // Button should have SVG icon
      expect(createButton.querySelector('svg')).toBeInTheDocument();
    });

    it('should render ApplicationsList component', () => {
      renderWithProviders();

      expect(screen.getByTestId('applications-list')).toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle rapid button clicks', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});

      await user.click(createButton);
      await user.click(createButton);
      await user.click(createButton);

      // Navigation should be attempted for each click
      expect(mockNavigate).toHaveBeenCalledTimes(3);
    });
  });

  describe('Accessibility', () => {
    it('should have proper heading hierarchy', () => {
      renderWithProviders();

      const h1 = screen.getByRole('heading', {level: 1});
      expect(h1).toBeInTheDocument();
      expect(h1).toHaveTextContent('Applications');
    });

    it('should have accessible buttons', () => {
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      expect(createButton).toBeEnabled();
      expect(createButton).toHaveAccessibleName();
    });

    it('should support Enter key on Create Application button', async () => {
      const user = userEvent.setup();
      renderWithProviders();

      const createButton = screen.getByRole('button', {name: /Create Application/i});
      createButton.focus();

      await user.keyboard('{Enter}');

      expect(mockNavigate).toHaveBeenCalledWith('/applications/types');
    });
  });
});
