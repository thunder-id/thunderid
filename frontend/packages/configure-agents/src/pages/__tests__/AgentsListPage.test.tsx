// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentsListPage from '../AgentsListPage';

// Mock the AgentsList component so we can focus on the page wiring.
vi.mock('../../components/AgentsList', () => ({
  default: () => <div data-testid="agents-list">Agents List</div>,
}));

const mockLogger = {debug: vi.fn(), error: vi.fn(), info: vi.fn(), warn: vi.fn()};
vi.mock('@thunderid/logger/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/logger/react')>()),
  useLogger: () => mockLogger,
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

const mockUseGetAgentTypes = vi.fn();
vi.mock('@thunderid/configure-agent-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-agent-types')>()),
  useGetAgentTypes: () => mockUseGetAgentTypes(),
}));

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => {
      const translations: Record<string, string> = {
        'agents:listing.title': 'Agents',
        'agents:listing.subtitle': 'Manage service identities and machine clients',
        'agents:listing.addAgent': 'Add Agent',
        'agents:listing.schema': 'Schema',
        'agents:listing.search.placeholder': 'Search agents',
      };
      return translations[key] ?? fallback ?? key;
    },
  }),
}));

describe('AgentsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAgentTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', handle: 'default', displayName: 'Default', ouId: 'ou-1'}]},
      isLoading: false,
    });
  });

  it('renders the page title and subtitle', () => {
    render(<AgentsListPage />);

    expect(screen.getByRole('heading', {level: 1, name: 'Agents'})).toBeInTheDocument();
    expect(screen.getByText('Manage service identities and machine clients')).toBeInTheDocument();
  });

  it('renders the Schema and Add agent buttons', () => {
    render(<AgentsListPage />);

    expect(screen.getByTestId('agent-schema-button')).toBeInTheDocument();
    expect(screen.getByTestId('agent-add-button')).toBeInTheDocument();
  });

  it('renders the AgentsList component', () => {
    render(<AgentsListPage />);

    expect(screen.getByTestId('agents-list')).toBeInTheDocument();
  });

  it('navigates to the create page when Add agent is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentsListPage />);

    await user.click(screen.getByTestId('agent-add-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/agents/create');
  });

  it('navigates to the schema page when Schema is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentsListPage />);

    await user.click(screen.getByTestId('agent-schema-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/agent-types/schema-1');
  });

  it('disables the Schema button while agent types are loading', () => {
    mockUseGetAgentTypes.mockReturnValue({data: undefined, isLoading: true});
    render(<AgentsListPage />);

    expect(screen.getByTestId('agent-schema-button')).toBeDisabled();
  });

  it('disables the Schema button when no default agent type exists', () => {
    mockUseGetAgentTypes.mockReturnValue({data: {types: []}, isLoading: false});
    render(<AgentsListPage />);

    expect(screen.getByTestId('agent-schema-button')).toBeDisabled();
  });

  it('handles navigation errors gracefully when Add agent navigation fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    render(<AgentsListPage />);

    await user.click(screen.getByTestId('agent-add-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/agents/create');
    await vi.waitFor(() => expect(mockLogger.error).toHaveBeenCalled());
  });

  it('handles navigation errors gracefully when Schema navigation fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    render(<AgentsListPage />);

    await user.click(screen.getByTestId('agent-schema-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/agent-types/schema-1');
    await vi.waitFor(() => expect(mockLogger.error).toHaveBeenCalled());
  });

  it('does not navigate when Schema is clicked but no default type exists', async () => {
    mockUseGetAgentTypes.mockReturnValue({data: {types: []}, isLoading: false});
    const user = userEvent.setup();

    render(<AgentsListPage />);

    // Button is disabled but force-click via direct invocation in handler not possible
    // Verify navigation is not called when button is disabled
    expect(screen.getByTestId('agent-schema-button')).toBeDisabled();
    await user.click(screen.getByTestId('agent-schema-button')).catch(() => null);
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
