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
vi.mock('react-i18next', () => ({
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

// Mock CapabilityCatalog component
vi.mock('../../components/CapabilityCatalog', () => ({
  default: ({variant}: {variant?: string}) => (
    <div data-testid={`capability-catalog-${variant ?? 'full'}`}>CapabilityCatalog Component</div>
  ),
}));

// Mock useGetFlows
const mockUseGetFlows = vi.fn();
vi.mock('../../api/useGetFlows', () => ({
  default: () => mockUseGetFlows() as unknown,
}));

describe('FlowsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetFlows.mockReturnValue({
      data: {flows: [{id: 'flow-1'}]},
      error: null,
      isLoading: false,
    });
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

  describe('Capability Catalog', () => {
    it('should render the compact catalog alongside the list when flows exist', () => {
      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('capability-catalog-compact')).toBeInTheDocument();
      expect(screen.getByTestId('flows-list')).toBeInTheDocument();
    });

    it('should render the full catalog instead of the list when there are no flows', () => {
      mockUseGetFlows.mockReturnValue({data: {flows: []}, error: null, isLoading: false});

      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.getByTestId('capability-catalog-full')).toBeInTheDocument();
      expect(screen.queryByTestId('flows-list')).not.toBeInTheDocument();
    });

    it('should not render any catalog while flows are loading', () => {
      mockUseGetFlows.mockReturnValue({data: undefined, error: null, isLoading: true});

      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.queryByTestId('capability-catalog-full')).not.toBeInTheDocument();
      expect(screen.queryByTestId('capability-catalog-compact')).not.toBeInTheDocument();
      expect(screen.getByTestId('flows-list')).toBeInTheDocument();
    });

    it('should render the list without a catalog when loading fails', () => {
      mockUseGetFlows.mockReturnValue({data: undefined, error: new Error('boom'), isLoading: false});

      render(
        <MemoryRouter>
          <FlowsListPage />
        </MemoryRouter>,
      );

      expect(screen.queryByTestId('capability-catalog-full')).not.toBeInTheDocument();
      expect(screen.queryByTestId('capability-catalog-compact')).not.toBeInTheDocument();
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
