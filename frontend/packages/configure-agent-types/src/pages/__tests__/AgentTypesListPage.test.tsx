// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentTypesListPage from '../AgentTypesListPage';

const mockNavigate = vi.fn();
const mockLoggerError = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

vi.mock('../../components/AgentTypesList', () => ({
  default: () => <div data-testid="agent-types-list" />,
}));

describe('AgentTypesListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
  });

  it('should render the page title and description', () => {
    render(<AgentTypesListPage />);

    expect(screen.getByRole('heading', {level: 1, name: 'Agent types'})).toBeInTheDocument();
    expect(screen.getByText('Define and manage agent types with custom schemas')).toBeInTheDocument();
  });

  it('should set the document title', async () => {
    render(<AgentTypesListPage />);

    await waitFor(() => expect(document.title).toBe('Agent types | ThunderID'));
  });

  it('should render the agent types list', () => {
    render(<AgentTypesListPage />);

    expect(screen.getByTestId('agent-types-list')).toBeInTheDocument();
  });

  it('should navigate to the create page when Create agent type is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentTypesListPage />);

    await user.click(screen.getByRole('button', {name: 'Create agent type'}));

    expect(mockNavigate).toHaveBeenCalledWith('/agent-types/create');
  });

  it('should log a failed navigation to the create page', async () => {
    const user = userEvent.setup();
    const testError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(testError);

    render(<AgentTypesListPage />);

    await user.click(screen.getByRole('button', {name: 'Create agent type'}));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to create agent type page',
        expect.objectContaining({error: testError}),
      );
    });
  });
});
