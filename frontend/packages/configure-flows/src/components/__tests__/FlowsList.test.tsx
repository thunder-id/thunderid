// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor, within} from '@testing-library/react';
import {MemoryRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {BasicFlowDefinition} from '../../models/responses';
import FlowsList from '../FlowsList';

// Mock logger with accessible mock functions
const mockLoggerError = vi.fn();
const mockRefetch = vi.fn();
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: mockLoggerError,
    withComponent: vi.fn(() => ({
      debug: vi.fn(),
      info: vi.fn(),
      warn: vi.fn(),
      error: mockLoggerError,
    })),
  }),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, options?: string | Record<string, unknown>) => {
      const translations: Record<string, string> = {
        'flows:listing.columns.name': 'Name',
        'flows:listing.columns.flowType': 'Type',
        'flows:listing.columns.updatedAt': 'Updated At',
        'flows:listing.columns.actions': 'Actions',
        'flows:listing.error.title': 'Error loading flows',
        'flows:listing.error.unknown': 'Unknown error occurred',
        'common:actions.edit': 'Edit',
        'common:actions.delete': 'Delete',
        'common:actions.refresh': 'Refresh',
        'common:actions.view': 'View',
      };
      if (translations[key] !== undefined) return translations[key];
      if (typeof options === 'string') return options || key;
      if (options && typeof options === 'object' && 'defaultValue' in options) {
        return (options['defaultValue'] as string) || '';
      }
      return key;
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

// Mock useDataGridLocaleText
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: () => ({}),
}));

// Mock useGetFlows
const mockFlowsData: {flows: BasicFlowDefinition[]} = {
  flows: [
    {
      id: 'flow-1',
      handle: 'login-flow',
      name: 'Login Flow',
      flowType: 'AUTHENTICATION',
      activeVersion: 1,
      createdAt: '2025-01-01T09:00:00Z',
      updatedAt: '2025-01-01T10:00:00Z',
    },
    {
      id: 'flow-2',
      handle: 'registration-flow',
      name: 'Registration Flow',
      flowType: 'REGISTRATION',
      activeVersion: 2,
      createdAt: '2025-01-02T14:00:00Z',
      updatedAt: '2025-01-02T15:30:00Z',
    },
  ],
};

let mockUseGetFlowsReturn = {
  data: mockFlowsData,
  isLoading: false,
  error: null as Error | null,
  refetch: mockRefetch,
};

vi.mock('../../api/useGetFlows', () => ({
  default: () => mockUseGetFlowsReturn,
}));

// Mock FlowDeleteDialog
vi.mock('../FlowDeleteDialog', () => ({
  default: ({open, flowId, onClose}: {open: boolean; flowId: string | null; onClose: () => void}) =>
    open ? (
      <div data-testid="flow-delete-dialog" data-flow-id={flowId}>
        <button type="button" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

const renderList = (): ReturnType<typeof render> =>
  render(
    <MemoryRouter>
      <FlowsList />
    </MemoryRouter>,
  );

const rowOf = (name: string): HTMLElement => screen.getByText(name).closest<HTMLElement>('tr')!;

const bodyNames = (): string[] =>
  screen
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[0].textContent ?? '');

describe('FlowsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockUseGetFlowsReturn = {
      data: mockFlowsData,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    };
  });

  describe('Rendering', () => {
    it('should render the flows in a card table', () => {
      renderList();

      expect(screen.getByRole('table')).toBeInTheDocument();
      expect(screen.getAllByRole('row')).toHaveLength(3);
    });

    it('should render column headers', () => {
      renderList();

      expect(screen.getByRole('columnheader', {name: 'Name'})).toBeInTheDocument();
      expect(screen.getByRole('columnheader', {name: 'Type'})).toBeInTheDocument();
      expect(screen.getByRole('columnheader', {name: 'Updated At'})).toBeInTheDocument();
      expect(screen.getByRole('columnheader', {name: 'Actions'})).toBeInTheDocument();
    });

    it('should render the name, handle, type and last update of each flow', () => {
      renderList();

      const row = rowOf('Login Flow');
      expect(within(row).getByText('login-flow')).toBeInTheDocument();
      expect(within(row).getByText('AUTHENTICATION')).toBeInTheDocument();
      expect(within(row).getByText(/Jan.*2025|2025.*Jan/)).toBeInTheDocument();
      expect(screen.getByText('Registration Flow')).toBeInTheDocument();
      expect(screen.getByText('REGISTRATION')).toBeInTheDocument();
    });

    it('should sort the flows when a column header is clicked', () => {
      renderList();

      fireEvent.click(screen.getByRole('button', {name: 'Name'}));
      expect(bodyNames()).toEqual(['Login Flowlogin-flow', 'Registration Flowregistration-flow']);

      fireEvent.click(screen.getByRole('button', {name: 'Name'}));
      expect(bodyNames()).toEqual(['Registration Flowregistration-flow', 'Login Flowlogin-flow']);
    });
  });

  describe('Loading State', () => {
    it('should mark the table busy while loading', () => {
      mockUseGetFlowsReturn = {
        data: null as unknown as {flows: BasicFlowDefinition[]},
        isLoading: true,
        error: null,
        refetch: mockRefetch,
      };

      renderList();

      expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
    });
  });

  describe('Error State', () => {
    it('should render a resolved error message via QueryErrorNotice, never the raw server text', () => {
      mockUseGetFlowsReturn = {
        data: null as unknown as {flows: BasicFlowDefinition[]},
        isLoading: false,
        error: new Error('Failed to fetch flows'),
        refetch: mockRefetch,
      };

      renderList();

      expect(screen.getByText('Error loading flows')).toBeInTheDocument();
      expect(screen.getByText('Unknown error occurred')).toBeInTheDocument();
      expect(screen.queryByText('Failed to fetch flows')).not.toBeInTheDocument();
    });

    it('should display unknown error message when error has no message', () => {
      mockUseGetFlowsReturn = {
        data: null as unknown as {flows: BasicFlowDefinition[]},
        isLoading: false,
        error: {} as Error,
        refetch: mockRefetch,
      };

      renderList();

      expect(screen.getByText('Error loading flows')).toBeInTheDocument();
      expect(screen.getByText('Unknown error occurred')).toBeInTheDocument();
    });

    it('should call refetch when the retry action is clicked', () => {
      mockUseGetFlowsReturn = {
        data: null as unknown as {flows: BasicFlowDefinition[]},
        isLoading: false,
        error: new Error('Failed to fetch flows'),
        refetch: mockRefetch,
      };

      renderList();

      fireEvent.click(screen.getByRole('button', {name: 'Refresh'}));

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('Row Click Navigation', () => {
    it('should navigate to the flow when its row is clicked', async () => {
      renderList();

      fireEvent.click(rowOf('Login Flow'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/flow-1');
      });
    });

    it('should navigate when a non-authentication flow row is clicked', async () => {
      renderList();

      fireEvent.click(rowOf('Registration Flow'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/flow-2');
      });
    });

    it('should navigate when a read-only flow row is clicked', async () => {
      mockUseGetFlowsReturn = {
        data: {flows: [{...mockFlowsData.flows[0], isReadOnly: true}]},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      };

      renderList();

      fireEvent.click(rowOf('Login Flow'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/flow-1');
      });
    });

    it('should log navigation errors without crashing', async () => {
      mockNavigate.mockRejectedValue(new Error('Navigation failed'));

      renderList();

      fireEvent.click(rowOf('Login Flow'));

      await waitFor(() => {
        expect(mockLoggerError).toHaveBeenCalledWith(
          'Failed to navigate to flow builder',
          expect.objectContaining({
            error: expect.any(Error) as Error,
            flowId: 'flow-1',
          }),
        );
      });
      expect(screen.getByText('Login Flow')).toBeInTheDocument();
    });
  });

  describe('Empty State', () => {
    it('should show the empty row when no flows exist', () => {
      mockUseGetFlowsReturn = {
        data: {flows: []},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      };

      renderList();

      expect(screen.getByRole('table')).toBeInTheDocument();
      expect(screen.queryByText('Login Flow')).not.toBeInTheDocument();
      expect(screen.getByText('No rows')).toBeInTheDocument();
    });

    it('should handle undefined data gracefully', () => {
      mockUseGetFlowsReturn = {
        data: undefined as unknown as {flows: BasicFlowDefinition[]},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      };

      renderList();

      expect(screen.getByRole('table')).toBeInTheDocument();
    });
  });

  describe('Row Actions', () => {
    it('should name the edit and delete actions', () => {
      renderList();

      const row = rowOf('Login Flow');
      expect(within(row).getByRole('button', {name: 'Edit'})).toBeInTheDocument();
      expect(within(row).getByRole('button', {name: 'Delete'})).toBeInTheDocument();
    });

    it('should offer only a view action on a read-only flow, which navigates', async () => {
      mockUseGetFlowsReturn = {
        data: {flows: [{...mockFlowsData.flows[0], isReadOnly: true}]},
        isLoading: false,
        error: null,
        refetch: mockRefetch,
      };

      renderList();

      const row = rowOf('Login Flow');
      expect(within(row).getAllByRole('button')).toHaveLength(1);
      fireEvent.click(within(row).getByRole('button', {name: 'View'}));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/flow-1');
      });
    });

    it('should navigate to the flow builder when Edit is clicked', async () => {
      renderList();

      fireEvent.click(within(rowOf('Login Flow')).getByRole('button', {name: 'Edit'}));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/flow-1');
      });
      expect(mockNavigate).toHaveBeenCalledTimes(1);
    });

    it('should not render the delete dialog initially', () => {
      renderList();

      expect(screen.queryByTestId('flow-delete-dialog')).not.toBeInTheDocument();
    });

    it('should open the delete dialog for the flow without opening the flow', async () => {
      renderList();

      fireEvent.click(within(rowOf('Login Flow')).getByRole('button', {name: 'Delete'}));

      await waitFor(() => {
        expect(screen.getByTestId('flow-delete-dialog')).toHaveAttribute('data-flow-id', 'flow-1');
      });
      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('should close the delete dialog and reset the selected flow', async () => {
      renderList();

      fireEvent.click(within(rowOf('Login Flow')).getByRole('button', {name: 'Delete'}));
      await waitFor(() => {
        expect(screen.getByTestId('flow-delete-dialog')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Close'));

      await waitFor(() => {
        expect(screen.queryByTestId('flow-delete-dialog')).not.toBeInTheDocument();
      });
    });
  });
});
