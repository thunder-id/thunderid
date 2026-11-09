// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
import {render, screen, waitFor} from '@thunderid/test-utils';
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

// Mock translations
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => {
      const translations: Record<string, string> = {
        'agents:listing.title': 'Agents',
        'agents:listing.subtitle': 'Manage service identities and machine clients',
        'agents:listing.addAgent': 'Add Agent',
        'agents:listing.search.placeholder': 'Search agents',
      };
      return translations[key] ?? fallback ?? key;
    },
  }),
}));

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('AgentsListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page title and subtitle', () => {
    render(<AgentsListPage />);

    expect(screen.getByRole('heading', {level: 1, name: 'Agents'})).toBeInTheDocument();
    expect(screen.getByText('Manage service identities and machine clients')).toBeInTheDocument();
  });

  it('sets the browser tab title', async () => {
    render(<AgentsListPage />);

    await waitFor(() => expect(document.title).toBe('Agents | ThunderID'));
  });

  it('should render the Add agent button', () => {
    render(<AgentsListPage />);

    expect(screen.getByRole('button', {name: 'Add Agent'})).toBeInTheDocument();
  });

  // Agent types are reached from the sidebar, as user types are, so the list has no Schema button.
  it('should not render a Schema button', () => {
    render(<AgentsListPage />);

    expect(screen.queryByRole('button', {name: 'Schema'})).not.toBeInTheDocument();
    expect(screen.queryByTestId('agent-schema-button')).not.toBeInTheDocument();
  });

  it('hides the Add agent button when the admin may not create agents', () => {
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <AgentsListPage />
      </CapabilitiesProvider>,
    );

    expect(screen.getByTestId('agents-list')).toBeInTheDocument();
    expect(screen.queryByTestId('agent-add-button')).not.toBeInTheDocument();
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

  it('handles navigation errors gracefully when Add agent navigation fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    render(<AgentsListPage />);

    await user.click(screen.getByTestId('agent-add-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/agents/create');
    await vi.waitFor(() => expect(mockLogger.error).toHaveBeenCalled());
  });
});
