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

// Mock heavy child components — focus on page wiring.
vi.mock('../../components/edit-agent/general/EditGeneralSettings', () => ({
  default: ({onDeleteSuccess}: {onDeleteSuccess?: () => void}) => (
    <div data-testid="edit-general">
      <button type="button" onClick={() => onDeleteSuccess?.()}>
        Delete Successful
      </button>
    </div>
  ),
}));

vi.mock('../../components/edit-agent/attributes/EditAgentAttributes', () => ({
  default: ({onFieldChange}: {onFieldChange: (field: string, value: unknown) => void}) => (
    <div data-testid="edit-attributes">
      <button type="button" onClick={() => onFieldChange('attributes', {department: 'sales'})}>
        Edit an attribute
      </button>
    </div>
  ),
}));

vi.mock('../../components/edit-agent/credentials/EditCredentialsSettings', () => ({
  default: () => <div data-testid="edit-credentials" />,
}));

vi.mock('../../components/edit-agent/flows/EditFlowsSettings', () => ({
  default: () => <div data-testid="edit-flows" />,
}));

vi.mock('../../components/edit-agent/tokens/EditTokensSettings', () => ({
  default: () => <div data-testid="edit-tokens" />,
}));

vi.mock('../../components/edit-agent/access/EditAccessSettings', () => ({
  default: () => <div data-testid="edit-access" />,
}));

vi.mock('../../components/edit-agent/advanced-settings/EditAdvancedSettings', () => ({
  default: () => <div data-testid="edit-advanced" />,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | {defaultValue?: string}, options?: Record<string, unknown>) => {
      let result: string;
      if (typeof fallback === 'string') result = fallback || key;
      else if (fallback && typeof fallback === 'object') result = fallback.defaultValue ?? key;
      else result = key;
      if (options) {
        Object.entries(options).forEach(([optionKey, value]) => {
          result = result.replace(`{{${optionKey}}}`, String(value));
        });
      }
      return result;
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
    it('renders General, Attributes, and Access tabs by default', () => {
      render(<AgentEditPage />);

      expect(screen.getByRole('tab', {name: 'General'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Attributes'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Access/i})).toBeInTheDocument();
    });

    it('does not render icons on any tab', () => {
      render(<AgentEditPage />);

      screen.getAllByRole('tab').forEach((tab) => {
        expect(tab.querySelector('svg')).not.toBeInTheDocument();
      });
    });

    it('renders OAuth-specific tabs when the agent has an OAuth2 inbound config', () => {
      render(<AgentEditPage />);

      expect(screen.getByRole('tab', {name: /Credentials/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Flows'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Tokens'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Advanced/i})).toBeInTheDocument();
    });

    it('orders tabs as General, Attributes, Credentials, Access, Flows, Tokens, Advanced', () => {
      render(<AgentEditPage />);

      const tabNames = screen.getAllByRole('tab').map((tab) => tab.textContent);
      expect(tabNames).toEqual(['General', 'Attributes', 'Credentials', 'Access', 'Flows', 'Tokens', 'Advanced']);
    });

    it('switches tabs when clicked', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: /Access/i}));

      expect(screen.getByTestId('edit-access')).toBeInTheDocument();
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

      expect(screen.queryByRole('tab', {name: /Credentials/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: 'Flows'})).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: 'Tokens'})).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: /Advanced/i})).not.toBeInTheDocument();
      // Access still renders — groups/roles apply regardless of OAuth.
      expect(screen.getByRole('tab', {name: /Access/i})).toBeInTheDocument();
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

  describe('Attribute edits', () => {
    it('surfaces the page-level unsaved-changes bar when an attribute is edited', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    it('includes staged attribute edits when the page-level Save button is clicked', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            data: expect.objectContaining({attributes: {department: 'sales'}}) as Record<string, unknown>,
          }),
        );
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

  describe('Save validation', () => {
    // Any field edit is enough to surface the Save bar — renaming is the simplest one available
    // without depending on any of the (mocked) tab content components.
    const triggerAChange = async (user: ReturnType<typeof userEvent.setup>) => {
      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const nameEditButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test Agent'));
      if (!nameEditButton) throw new Error('name edit button not found');
      await user.click(nameEditButton);
      const input = screen.getAllByRole('textbox')[0];
      await user.type(input, ' Renamed');
      await user.keyboard('{Enter}');
    };

    it('disables Save when authorization_code is selected but no redirect URI or allowed user type is set, even without visiting those tabs', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: [],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['authorization_code'],
                responseTypes: ['code'],
                redirectUris: [],
                clientId: 'client-id-xyz',
              },
            },
          ],
        },
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);
      await triggerAChange(user);

      expect(
        screen.getByText('Before saving, add a redirect URI and select at least one allowed user type.'),
      ).toBeInTheDocument();
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('names only the single failing check when just one is missing', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['employee'],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['authorization_code'],
                responseTypes: ['code'],
                redirectUris: [],
                clientId: 'client-id-xyz',
              },
            },
          ],
        },
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);
      await triggerAChange(user);

      expect(screen.getByText('Before saving, add a redirect URI.')).toBeInTheDocument();
    });

    it('enables Save once a redirect URI and an allowed user type are both set', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['employee'],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['authorization_code'],
                responseTypes: ['code'],
                redirectUris: ['https://example.com/cb'],
                clientId: 'client-id-xyz',
              },
            },
          ],
        },
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);
      await triggerAChange(user);

      expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    it('does not block Save when authorization_code is not selected', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {...baseAgent, allowedUserTypes: []},
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);
      await triggerAChange(user);

      expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
    });

    it('disables Save when private_key_jwt is selected but no certificate is configured, even without visiting the credentials/advanced tabs', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['client_credentials'],
                responseTypes: [],
                tokenEndpointAuthMethod: 'private_key_jwt',
                clientId: 'client-id-xyz',
              },
            },
          ],
        },
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);
      await triggerAChange(user);

      expect(screen.getByText('Before saving, add a certificate.')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('enables Save once a certificate is configured for private_key_jwt', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['client_credentials'],
                responseTypes: [],
                tokenEndpointAuthMethod: 'private_key_jwt',
                certificate: {type: 'JWKS', value: '{"keys":[]}'},
                clientId: 'client-id-xyz',
              },
            },
          ],
        },
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);
      await triggerAChange(user);

      expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
    });
  });
});
