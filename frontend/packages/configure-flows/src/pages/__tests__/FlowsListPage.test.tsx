// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {MemoryRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowsListPage from '../FlowsListPage';

// Mock logger
const mockLoggerError = vi.fn();

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: mockLoggerError,
  }),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:listing.title': 'Flows',
        'flows:listing.subtitle': 'Manage your authentication and registration flows',
        'flows:listing.addFlow': 'Add Flow',
      };
      return translations[key] || key;
    },
  }),
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock FlowsList component
vi.mock('../../components/FlowsList', () => ({
  default: () => <div data-testid="flows-list">FlowsList Component</div>,
}));

// Mock useConfig (consumed by ExternalLink in the page header)
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getDocumentationLink: () => undefined}),
  };
});

describe('FlowsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the page title', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.getByText('Flows')).toBeInTheDocument();
    });

    it('should render the page subtitle', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.getByText('Manage your authentication and registration flows')).toBeInTheDocument();
    });

    it('should render the Add Flow button', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.getByRole('button', {name: /add flow/i})).toBeInTheDocument();
    });

    it('should render FlowsList component', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('flows-list')).toBeInTheDocument();
    });
  });

  describe('Add Flow Button', () => {
    it('should navigate to login-builder when Add Flow is clicked', async () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      const addButton = screen.getByRole('button', {name: /add flow/i});
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/create');
      });
    });

    it('should render button with contained variant', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      const addButton = screen.getByRole('button', {name: /add flow/i});
      expect(addButton).toHaveClass('MuiButton-contained');
    });
  });

  describe('Layout', () => {
    it('should render title as h1', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      const title = screen.getByRole('heading', {level: 1});
      expect(title).toHaveTextContent('Flows');
    });

    it('should have proper structure with header and list', () => {
      const {container} = render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      // Check that the page has a box container
      expect(container.querySelector('.MuiBox-root')).toBeInTheDocument();
    });
  });

  describe('Navigation Error Handling', () => {
    it('should handle navigation errors when add flow button is clicked', async () => {
      const navigationError = new Error('Navigation failed');
      mockNavigate.mockRejectedValueOnce(navigationError);

      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      const addButton = screen.getByRole('button', {name: /add flow/i});
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/create');
      });

      // Verify that the error was caught and logged
      await waitFor(() => {
        expect(mockLoggerError).toHaveBeenCalledWith('Failed to navigate to flow builder page', {
          error: navigationError,
        });
      });

      // Component should still be rendered (no crash)
      expect(addButton).toBeInTheDocument();
    });
  });
});
