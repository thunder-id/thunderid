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
import {render, screen, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentEditPage from '../AgentEditPage';

const {mockNavigate, mockRefetch, mockUseGetAgent, mockUseUpdateAgent, mockMutateAsync} = vi.hoisted(() => ({
  mockNavigate: vi.fn(),
  mockRefetch: vi.fn(),
  mockUseGetAgent: vi.fn(),
  mockUseUpdateAgent: vi.fn(),
  mockMutateAsync: vi.fn(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({agentId: 'agent-1'}),
    Link: ({to, children = undefined, ...props}: {to: string; children?: ReactNode; [key: string]: unknown}) => (
      <a
        {...(props as Record<string, unknown>)}
        href={to}
        onClick={(e) => {
          e.preventDefault();
          Promise.resolve(mockNavigate(to)).catch(() => null);
        }}
      >
        {children}
      </a>
    ),
  };
});

vi.mock('../../api/useGetAgent', () => ({
  default: (id: string) => mockUseGetAgent(id),
}));

vi.mock('../../api/useUpdateAgent', () => ({
  default: () => mockUseUpdateAgent(),
}));

// Mock heavy child components — focus on page wiring
vi.mock('../../components/edit-agent/general-settings/EditGeneralSettings', () => ({
  default: ({onDeleteSuccess}: {onDeleteSuccess?: () => void}) => (
    <div data-testid="edit-general">
      <button type="button" onClick={() => onDeleteSuccess?.()}>
        Delete Successful
      </button>
    </div>
  ),
}));

vi.mock('../../components/edit-agent/attributes/EditAgentAttributes', () => ({
  default: ({onSaved}: {onSaved?: () => void}) => (
    <div data-testid="edit-attributes">
      <button type="button" onClick={() => onSaved?.()}>
        Saved
      </button>
    </div>
  ),
}));

vi.mock('../../components/edit-agent/flows-settings/AllowedUserTypesSection', () => ({
  default: () => <div data-testid="allowed-user-types" />,
}));

vi.mock('../../components/edit-agent/advanced-settings/EditAdvancedSettings', () => ({
  default: () => <div data-testid="edit-advanced" />,
}));

vi.mock('../../../applications/components/edit-application/flows-settings/EditFlowsSettings', () => ({
  default: () => <div data-testid="edit-flows" />,
}));

vi.mock('../../../applications/components/edit-application/token-settings/EditTokenSettings', () => ({
  default: () => <div data-testid="edit-token" />,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | {defaultValue?: string}) => {
      if (typeof fallback === 'string') return fallback || key;
      if (fallback && typeof fallback === 'object') return fallback.defaultValue ?? key;
      return key;
    },
  }),
}));

describe('AgentEditPage', () => {
  const baseAgent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    name: 'Test Agent',
    description: 'Test description',
    inboundAuthConfig: [
      {
        type: 'oauth2' as const,
        config: {
          grantTypes: ['client_credentials'],
          responseTypes: [],
          clientId: 'client-id-xyz',
        },
      },
    ],
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetAgent.mockReturnValue({
      data: baseAgent,
      isLoading: false,
      error: null,
      isError: false,
      refetch: mockRefetch,
    });
    mockUseUpdateAgent.mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    });
    mockMutateAsync.mockResolvedValue(undefined);
    mockRefetch.mockResolvedValue({});
  });

  describe('Loading and Error States', () => {
    it('renders a progressbar while loading', () => {
      mockUseGetAgent.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('renders an error alert when fetching fails', () => {
      mockUseGetAgent.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Boom'),
        isError: true,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);

      expect(screen.getByText('Boom')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Back to agents/i})).toBeInTheDocument();
    });

    it('renders a not-found alert when agent is null', () => {
      mockUseGetAgent.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);

      expect(screen.getByText('Agent not found')).toBeInTheDocument();
    });
  });

  describe('Tabs', () => {
    it('renders General and Attributes tabs by default', () => {
      render(<AgentEditPage />);

      expect(screen.getByRole('tab', {name: /General/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Attributes/i})).toBeInTheDocument();
    });

    it('renders OAuth-specific tabs when the agent has an OAuth2 inbound config', () => {
      render(<AgentEditPage />);

      expect(screen.getByRole('tab', {name: /Flows/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Token/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Advanced/i})).toBeInTheDocument();
    });

    it('switches tabs when clicked', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: /Attributes/i}));

      expect(screen.getByTestId('edit-attributes')).toBeInTheDocument();
    });

    it('does not render OAuth tabs when agent has no OAuth config', () => {
      mockUseGetAgent.mockReturnValue({
        data: {...baseAgent, inboundAuthConfig: []},
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);

      expect(screen.queryByRole('tab', {name: /Flows/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: /Token/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: /Advanced/i})).not.toBeInTheDocument();
    });
  });

  describe('Header inline editing', () => {
    it('renders the agent name and description', () => {
      render(<AgentEditPage />);

      expect(screen.getByText('Test Agent')).toBeInTheDocument();
      expect(screen.getByText('Test description')).toBeInTheDocument();
    });

    it('shows the edit name input when its edit icon is clicked', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      // Find and click the first edit icon (next to the name)
      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      // The first edit-pencil button next to the name
      const nameEditButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test Agent'));
      if (!nameEditButton) throw new Error('name edit button not found');
      await user.click(nameEditButton);

      // After clicking, the heading text becomes a text input
      const inputs = screen.getAllByRole('textbox');
      expect(inputs.length).toBeGreaterThan(0);
    });

    it('does not raise an unsaved-changes diff when description editor is opened and closed without changes', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const descEditButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test description'));
      if (!descEditButton) throw new Error('description edit button not found');
      await user.click(descEditButton);

      const descInput = screen
        .getAllByRole('textbox')
        .find((el) => (el as HTMLTextAreaElement).value === 'Test description');
      if (!descInput) throw new Error('description textarea not found');

      // Blur without typing → no diff should be created → no unsaved-changes bar.
      descInput.dispatchEvent(new FocusEvent('blur', {bubbles: true}));

      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });
  });

  describe('Delete success', () => {
    it('navigates back to /agents when EditGeneralSettings reports onDeleteSuccess', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByText('Delete Successful'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/agents');
      });
    });
  });

  describe('Attributes Saved', () => {
    it('refetches the agent when Attributes reports onSaved', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: /Attributes/i}));
      await user.click(screen.getByText('Saved'));

      await waitFor(() => {
        expect(mockRefetch).toHaveBeenCalled();
      });
    });
  });

  describe('Back navigation', () => {
    it('renders the back link to /agents', () => {
      render(<AgentEditPage />);

      const backLink = screen.getByRole('link', {name: /Back to agents/i});
      expect(backLink).toHaveAttribute('href', '/agents');
    });
  });
});
