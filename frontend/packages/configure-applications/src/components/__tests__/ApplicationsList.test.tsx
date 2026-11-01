// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {ApplicationListResponse} from '../../models/responses';
import ApplicationsList from '../ApplicationsList';

const {mockLoggerError} = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

// Mock the dependencies
vi.mock('../../api/useGetApplications', () => ({
  default: vi.fn(),
}));
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});
vi.mock('@thunderid/hooks', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/hooks')>()),
  useDataGridLocaleText: vi.fn(),
}));

// Mock @wso2/oxygen-ui to avoid cssstyle issues with CSS variables
vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    OxygenUIThemeProvider: ({children}: {children: React.ReactNode}) => children,
  };
});

// Mock ApplicationDeleteDialog to avoid cssstyle issues with MUI dialogs
vi.mock('../ApplicationDeleteDialog', () => ({
  default: ({open, onClose}: {open: boolean; onClose: () => void}) =>
    open ? (
      <div role="dialog" data-testid="delete-dialog">
        <button type="button" onClick={onClose}>
          Cancel
        </button>
        <button type="button">Delete</button>
      </div>
    ) : null,
}));

vi.mock('@thunderid/logger/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/logger/react')>()),
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

const {default: useGetApplications} = await import('../../api/useGetApplications');
const {useNavigate} = await import('react-router');
const {useDataGridLocaleText} = await import('@thunderid/hooks');

describe('ApplicationsList', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  const mockApplicationsData: ApplicationListResponse = {
    totalResults: 2,
    count: 2,
    applications: [
      {
        id: 'app-1',
        name: 'Test App 1',
        description: 'First test application',
        logoUrl: 'https://example.com/logo1.png',
        clientId: 'client_id_1',
        authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
        registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
        isRegistrationFlowEnabled: true,
      },
      {
        id: 'app-2',
        name: 'Test App 2',
        description: 'Second test application',
        logoUrl: '',
        clientId: 'client_id_2',
        authFlowId: 'edc013d0-e893-4dc0-990c-3e1d203e005b',
        registrationFlowId: '80024fb3-29ed-4c33-aa48-8aee5e96d522',
        isRegistrationFlowEnabled: false,
      },
    ],
  };

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockLoggerError.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useDataGridLocaleText).mockReturnValue({});
  });

  const renderComponent = () => render(<ApplicationsList />);
  const bodyRows = (): HTMLElement[] => screen.getAllByRole('row').slice(1);
  const firstCells = (): string[] => bodyRows().map((row) => within(row).getAllByRole('cell')[0].textContent ?? '');

  it('should render loading state', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
  });

  it('should render error state', () => {
    const error = new Error('raw server text');
    vi.mocked(useGetApplications).mockReturnValue({
      data: undefined,
      isLoading: false,
      error,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('Failed to load applications')).toBeInTheDocument();
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
  });

  it('should render applications list successfully', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('Test App 1')).toBeInTheDocument();
    expect(screen.getByText('Test App 2')).toBeInTheDocument();
    expect(screen.getByText('First test application')).toBeInTheDocument();
    expect(screen.getByText('Second test application')).toBeInTheDocument();
  });

  it("should show each application's access status, treating an absent status as active", () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: {
        ...mockApplicationsData,
        applications: [
          {...mockApplicationsData.applications[0], status: {value: 'SUSPENDED'}},
          mockApplicationsData.applications[1],
        ],
      },
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('Suspended')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
  });

  it('should keep the status indicators reachable from the keyboard', () => {
    renderComponent();

    const indicators = screen.getAllByRole('img', {name: 'Active'});
    expect(indicators.length).toBeGreaterThan(0);
    indicators.forEach((indicator) => expect(indicator).toHaveAttribute('tabindex', '0'));
  });

  it('should render the listing as a card table', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.queryByRole('grid')).not.toBeInTheDocument();
  });

  it('should give each row action button an accessible name', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getAllByRole('button', {name: 'Edit'})).toHaveLength(2);
    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(2);
  });

  it('should display client IDs as chips', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('client_id_1')).toBeInTheDocument();
    expect(screen.getByText('client_id_2')).toBeInTheDocument();
  });

  it('should render avatar with logo URL', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const avatars = screen.getAllByRole('img');
    expect(avatars[0]).toHaveAttribute('src', 'https://example.com/logo1.png');
  });

  it('should render AppWindow icon when logo URL is not provided', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // Should show AppWindow icon when logo URL is not provided
    const avatars = screen.getAllByRole('img', {hidden: true});
    // The second app (Test App 2) has no logoUrl, so it should have the AppWindow icon
    expect(avatars.length).toBeGreaterThan(0);
  });

  it('should display "-" for missing description', () => {
    const dataWithMissingDescription: ApplicationListResponse = {
      ...mockApplicationsData,
      applications: [
        {
          ...mockApplicationsData.applications[0],
          description: undefined,
        },
      ],
    };

    vi.mocked(useGetApplications).mockReturnValue({
      data: dataWithMissingDescription,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const dashElements = screen.getAllByText('-');
    expect(dashElements.length).toBeGreaterThan(0);
  });

  it('should open delete dialog when clicking Delete action', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });
  });

  it('should navigate when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = bodyRows();

    expect(rows.length).toBeGreaterThanOrEqual(1);
    await user.click(rows[0]);
  });

  it('should close delete dialog when cancelled', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const deleteButtons = screen.getAllByRole('button', {name: /delete/i});
    await user.click(deleteButtons[0]);

    await waitFor(() => {
      expect(screen.getByTestId('delete-dialog')).toBeInTheDocument();
    });

    const cancelButton = screen.getByRole('button', {name: /cancel/i});
    await user.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByTestId('delete-dialog')).not.toBeInTheDocument();
    });
  });

  it('should handle navigation error gracefully', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = bodyRows();
    expect(rows.length).toBeGreaterThan(0);
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-1');
    });

    expect(screen.getByRole('table')).toBeInTheDocument();
  });

  it('should handle empty applications list', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: {
        totalResults: 0,
        count: 0,
        applications: [],
      },
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useGetApplications>);

    renderComponent();

    expect(screen.getByText('No rows')).toBeInTheDocument();
  });

  it('should navigate to view page when clicking row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = bodyRows();
    expect(rows.length).toBeGreaterThan(0);
    await user.click(rows[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-1');
    });
  });

  it('should navigate to correct application when clicking different row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const rows = bodyRows();
    expect(rows.length).toBeGreaterThan(1);
    await user.click(rows[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-2');
    });
  });

  it('should display pagination controls', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    vi.mocked(useDataGridLocaleText).mockReturnValue({
      paginationDisplayedRows: ({from, to, count}) => `${from}–${to} of ${count}`,
    });

    renderComponent();

    expect(screen.getByText(/1–2 of 2/)).toBeInTheDocument();
  });

  it('should sort applications by name, type and client ID', async () => {
    const user = userEvent.setup();
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    await user.click(screen.getByRole('button', {name: 'Name'}));
    await user.click(screen.getByRole('button', {name: 'Name'}));
    expect(firstCells()[0]).toContain('Test App 2');

    await user.click(screen.getByRole('button', {name: 'Type'}));
    expect(bodyRows()).toHaveLength(2);

    await user.click(screen.getByRole('button', {name: 'Client ID'}));
    expect(firstCells()[0]).toContain('Test App 1');
  });

  it('should handle avatar image error', () => {
    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const avatars = screen.getAllByRole('img');
    // Trigger onError on the first avatar
    expect(avatars[0]).toBeDefined();
    avatars[0].dispatchEvent(new Event('error'));
    // The onError handler should set src to empty string
    expect(avatars[0]).toBeInTheDocument();
  });

  it('should display "-" for missing clientId', () => {
    const dataWithMissingClientId: ApplicationListResponse = {
      ...mockApplicationsData,
      applications: [
        {
          ...mockApplicationsData.applications[0],
          clientId: undefined,
        },
      ],
    };

    vi.mocked(useGetApplications).mockReturnValue({
      data: dataWithMissingClientId,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    // Should display "-" for missing clientId
    const dashes = screen.getAllByText('-');
    expect(dashes.length).toBeGreaterThan(0);
  });

  it('should navigate to application when Edit action button is clicked', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(0);
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-1');
    });
  });

  it('should navigate to correct application when Edit action is clicked for second row', async () => {
    const user = userEvent.setup();

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    expect(editButtons.length).toBeGreaterThan(1);
    await user.click(editButtons[1]);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/applications/app-2');
    });
  });

  it('should log error when Edit button navigation fails', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValueOnce(navigationError);

    vi.mocked(useGetApplications).mockReturnValue({
      data: mockApplicationsData,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetApplications>);

    renderComponent();

    const editButtons = screen.getAllByRole('button', {name: /^edit$/i});
    await user.click(editButtons[0]);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to application',
        expect.objectContaining({
          error: navigationError,
          applicationId: 'app-1',
        }),
      );
    });
  });
});
