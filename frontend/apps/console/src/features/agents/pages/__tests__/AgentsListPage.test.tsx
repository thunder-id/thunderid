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

/* eslint-disable @typescript-eslint/no-unsafe-return */
import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentsListPage from '../AgentsListPage';

// Mock the AgentsList component so we can focus on the page wiring.
vi.mock('../../components/AgentsList', () => ({
  default: () => <div data-testid="agents-list">Agents List</div>,
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
vi.mock('@thunderid/configure-agent-types', () => ({
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
      data: {types: [{id: 'schema-1', name: 'default', ouId: 'ou-1'}]},
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

  it('renders the search field', () => {
    render(<AgentsListPage />);

    const searchInput = screen.getByPlaceholderText('Search agents');
    expect(searchInput).toBeInTheDocument();
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
  });

  it('handles navigation errors gracefully when Schema navigation fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValueOnce(new Error('Navigation failed'));

    render(<AgentsListPage />);

    await user.click(screen.getByTestId('agent-schema-button'));

    expect(mockNavigate).toHaveBeenCalledWith('/agent-types/schema-1');
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
