// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import userEvent from '@testing-library/user-event';
import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentConstants from '../../constants/agent-constants';
import AgentEditPage from '../AgentEditPage';

const {
  mockNavigate,
  mockRefetch,
  mockUseGetAgent,
  mockUseUpdateAgent,
  mockMutateAsync,
  mockUseGetAgentTypes,
  mockUseGetAgentType,
  mockUseLocation,
  stagingCallbackIdentities,
} = vi.hoisted(() => ({
  // Every distinct onFieldChange the Attributes tab is handed.
  stagingCallbackIdentities: new Set<unknown>(),
  mockNavigate: vi.fn(),
  mockRefetch: vi.fn(),
  mockUseGetAgent: vi.fn(),
  mockUseUpdateAgent: vi.fn(),
  mockMutateAsync: vi.fn(),
  mockUseGetAgentTypes: vi.fn(),
  mockUseGetAgentType: vi.fn(),
  mockUseLocation: vi.fn(
    (): {
      state: {justCreatedSecret: {agentName: string; clientId?: string; clientSecret: string}} | null;
    } => ({state: null}),
  ),
}));

vi.mock('@thunderid/configure-agent-types', () => ({
  useGetAgentTypes: () => mockUseGetAgentTypes(),
  useGetAgentType: () => mockUseGetAgentType(),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({agentId: 'agent-1'}),
    useLocation: () => mockUseLocation(),
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

const {logoA, logoB} = vi.hoisted(() => ({
  logoA: 'https://example.com/logo-a.png',
  logoB: 'emoji:🚀',
}));

// Only ResourceAvatar is stubbed; LogoPicker's own behavior is covered by its own tests.
vi.mock('@thunderid/components', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    ResourceAvatar: ({
      value = undefined,
      fallback = undefined,
      editable = false,
      onSelect = undefined,
      onSave = undefined,
      editAriaLabel = undefined,
    }: {
      value?: string;
      fallback?: string;
      editable?: boolean;
      onSelect?: (value: string) => void;
      onSave?: () => void | Promise<void>;
      editAriaLabel?: string;
    }) => (
      // Each affordance is keyed to the prop that actually enables it in ResourceAvatar: the
      // pencil to `editable`, the picker to `onSelect`, and persisting to `onSave`.
      <div data-testid="resource-avatar" data-value={value ?? ''} data-fallback={fallback ?? ''}>
        {editable && onSelect && <button type="button" aria-label={editAriaLabel} />}
        {onSelect && (
          <>
            <button type="button" onClick={() => onSelect(logoA)}>
              Pick logo A
            </button>
            <button type="button" onClick={() => onSelect(logoB)}>
              Pick logo B
            </button>
          </>
        )}
        {onSave && (
          <button type="button" onClick={() => void onSave()}>
            Save logo
          </button>
        )}
      </div>
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
vi.mock('../../components/edit-agent/overview/AgentOverview', () => ({
  default: () => <div data-testid="agent-overview" />,
}));

vi.mock('../../components/edit-agent/attributes/EditAgentAttributes', () => ({
  default: ({onFieldChange}: {onFieldChange: (field: string, value: unknown) => void}) => {
    stagingCallbackIdentities.add(onFieldChange);
    return (
      <div data-testid="edit-attributes">
        <button type="button" onClick={() => onFieldChange('attributes', {department: 'sales'})}>
          Edit an attribute
        </button>
      </div>
    );
  },
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
  default: ({onDeleteSuccess}: {onDeleteSuccess?: () => void}) => (
    <div data-testid="edit-advanced">
      <button type="button" onClick={() => onDeleteSuccess?.()}>
        Delete Successful
      </button>
    </div>
  ),
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
    stagingCallbackIdentities.clear();
    mockUseGetAgent.mockReturnValue({
      data: baseAgent,
      isLoading: false,
      error: null,
      isError: false,
      refetch: mockRefetch,
    });
    // A fresh object per call, like useMutation. A stable stand-in hides identity churn.
    mockUseUpdateAgent.mockImplementation(() => ({
      mutateAsync: mockMutateAsync,
      isPending: false,
    }));
    mockMutateAsync.mockResolvedValue(undefined);
    mockRefetch.mockResolvedValue({});
    mockUseGetAgentTypes.mockReturnValue({
      data: {types: [{id: 'default-type', name: 'default'}]},
      isLoading: false,
      error: null,
    });
    mockUseGetAgentType.mockReturnValue({
      data: {id: 'default-type', name: 'default', schema: {}},
      isLoading: false,
      error: null,
    });
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

    it('renders a progressbar while the type schema is still resolving', () => {
      mockUseGetAgentType.mockReturnValue({data: undefined, isLoading: true, error: null});

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

      expect(screen.getByText('Failed to load agent')).toBeInTheDocument();
      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
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
    it('renders Overview, Attributes, Access, and Advanced tabs by default', () => {
      render(<AgentEditPage />);

      expect(screen.getByRole('tab', {name: 'Overview'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Attributes'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Access/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Advanced/i})).toBeInTheDocument();
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
    });

    it('orders tabs as Overview, Attributes, Credentials, Access, Flows, Tokens, Advanced', () => {
      render(<AgentEditPage />);

      const tabNames = screen.getAllByRole('tab').map((tab) => tab.textContent);
      expect(tabNames).toEqual(['Overview', 'Attributes', 'Credentials', 'Access', 'Flows', 'Tokens', 'Advanced']);
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
      // Access and Advanced still render — groups/roles and owner/danger-zone apply regardless of OAuth.
      expect(screen.getByRole('tab', {name: /Access/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /Advanced/i})).toBeInTheDocument();
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

  describe('Logo', () => {
    const renderWithLogo = (logoUrl?: string): void => {
      mockUseGetAgent.mockReturnValue({
        data: logoUrl === undefined ? baseAgent : {...baseAgent, logoUrl},
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });
      render(<AgentEditPage />);
    };

    it('offers the logo picker for a writable agent', () => {
      renderWithLogo();

      expect(screen.getByRole('button', {name: 'Update Logo'})).toBeInTheDocument();
    });

    it('does not offer the logo picker for a read-only agent', () => {
      mockUseGetAgent.mockReturnValue({
        data: {...baseAgent, isReadOnly: true},
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });

      render(<AgentEditPage />);

      expect(screen.queryByRole('button', {name: 'Update Logo'})).not.toBeInTheDocument();
      // ResourceAvatar opens its picker from the avatar itself whenever onSelect is set, so the
      // callback has to be withheld rather than just hiding the pencil.
      expect(screen.queryByRole('button', {name: 'Pick logo B'})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Save logo'})).not.toBeInTheDocument();
    });

    it("shows the agent's current logo as the initial value", () => {
      renderWithLogo(logoA);

      expect(screen.getByTestId('resource-avatar')).toHaveAttribute('data-value', logoA);
    });

    it('falls back to the default agent avatar when no logo is set', () => {
      renderWithLogo();

      const avatar = screen.getByTestId('resource-avatar');
      expect(avatar).toHaveAttribute('data-value', '');
      expect(avatar).toHaveAttribute('data-fallback', AgentConstants.DEFAULT_AVATAR);
    });

    it('stages a picked logo and surfaces the unsaved-changes bar', async () => {
      const user = userEvent.setup();
      renderWithLogo();

      await user.click(screen.getByRole('button', {name: 'Pick logo B'}));

      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      expect(screen.getByTestId('resource-avatar')).toHaveAttribute('data-value', logoB);
    });

    it('hides the bar when the logo is picked back to its original value', async () => {
      const user = userEvent.setup();
      renderWithLogo(logoA);

      await user.click(screen.getByRole('button', {name: 'Pick logo B'}));
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Pick logo A'}));

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
      expect(screen.getByTestId('resource-avatar')).toHaveAttribute('data-value', logoA);
    });

    it('sends the picked logo when the picker saves', async () => {
      const user = userEvent.setup();
      renderWithLogo();

      await user.click(screen.getByRole('button', {name: 'Pick logo B'}));
      await user.click(screen.getByRole('button', {name: 'Save logo'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            agentId: 'agent-1',
            data: expect.objectContaining({logoUrl: logoB}) as Record<string, unknown>,
          }),
        );
      });
    });

    it('cannot save from the picker while the agent fails page validation', async () => {
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

      // The picker still stages a pick, but persisting it would send the whole invalid payload.
      expect(screen.queryByRole('button', {name: 'Save logo'})).not.toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Pick logo B'}));

      expect(mockMutateAsync).not.toHaveBeenCalled();
      expect(
        screen.getByText('Before saving, add a redirect URI and select at least one allowed user type.'),
      ).toBeInTheDocument();
    });

    it('resets a stale save error when the logo changes', async () => {
      const user = userEvent.setup();
      const mockReset = vi.fn();
      mockUseUpdateAgent.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: new Error('Boom'),
        isError: true,
        reset: mockReset,
      });
      renderWithLogo();

      await user.click(screen.getByRole('button', {name: 'Pick logo B'}));

      expect(mockReset).toHaveBeenCalled();
    });
  });

  describe('Unsaved-changes bar', () => {
    const editName = async (user: ReturnType<typeof userEvent.setup>, from: string, to: string): Promise<void> => {
      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const nameEditButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes(from));
      if (!nameEditButton) throw new Error(`name edit button for "${from}" not found`);
      await user.click(nameEditButton);
      const input = screen.getByRole('textbox');
      await user.clear(input);
      await user.type(input, `${to}{Enter}`);
    };

    it('hides the bar when a field is manually retyped back to its original value', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await editName(user, 'Test Agent', 'Renamed Agent');
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();

      await editName(user, 'Renamed Agent', 'Test Agent');
      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });

    it('discards a rename that exceeds the maximum length', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      const editIcons = screen.getAllByRole('button').filter((button) => button.querySelector('svg'));
      const nameEditButton = editIcons.find((button) => button.parentElement?.textContent?.includes('Test Agent'));
      if (!nameEditButton) throw new Error('name edit button for "Test Agent" not found');
      await user.click(nameEditButton);
      const input = screen.getByRole('textbox');
      fireEvent.change(input, {target: {value: 'a'.repeat(AgentConstants.NAME_MAX_LENGTH + 1)}});
      fireEvent.keyDown(input, {key: 'Enter'});

      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      expect(screen.getByText('Test Agent')).toBeInTheDocument();
    });

    it('keeps the bar visible when only one of two edited fields is reverted', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      // Edit description
      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const descEditButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test description'));
      if (!descEditButton) throw new Error('description edit button not found');
      await user.click(descEditButton);
      const descInput = screen
        .getAllByRole('textbox')
        .find((el) => (el as HTMLTextAreaElement).value === 'Test description');
      if (!descInput) throw new Error('description textarea not found');
      await user.clear(descInput);
      await user.type(descInput, 'Changed description');
      descInput.dispatchEvent(new FocusEvent('blur', {bubbles: true}));

      // Edit name, then revert only the name
      await editName(user, 'Test Agent', 'Renamed Agent');
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      await editName(user, 'Renamed Agent', 'Test Agent');

      // Description is still changed, so the bar must stay visible
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });
  });

  describe('Delete success', () => {
    it('navigates back to /agents when EditAdvancedSettings reports onDeleteSuccess', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: /Advanced/i}));
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

    it('keeps the unsaved-changes bar and edited state when saving fails', async () => {
      const user = userEvent.setup();
      mockMutateAsync.mockRejectedValueOnce(new Error('Boom'));
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await user.click(screen.getByRole('button', {name: 'Save'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalled();
      });
      expect(mockRefetch).not.toHaveBeenCalled();
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    });

    it('hands the Attributes tab one stable onFieldChange across re-renders', async () => {
      // A new callback per render refired the tab's staging effect after any real edit, until
      // React stopped committing renders at all.
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(stagingCallbackIdentities.size).toBe(1);
    });

    it('surfaces the resolved save error inline on the unsaved-changes bar', async () => {
      const user = userEvent.setup();
      mockUseUpdateAgent.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: new Error('raw backend update failure detail'),
        isError: true,
        reset: vi.fn(),
      });

      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(screen.getByText('Failed to update agent. Please try again.')).toBeInTheDocument();
      expect(screen.queryByText('raw backend update failure detail')).not.toBeInTheDocument();
    });

    it('resets a failed save mutation as soon as another field changes', async () => {
      const user = userEvent.setup();
      const mockReset = vi.fn();
      mockUseUpdateAgent.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: new Error('Boom'),
        isError: true,
        reset: mockReset,
      });

      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(mockReset).toHaveBeenCalled();
    });
  });

  describe('Reset', () => {
    it('clears edited fields and resets tab content when Reset is clicked', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Reset'}));

      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
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
                redirectUris: ['http://localhost:3000/cb'],
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

    it('disables Save when authorization_code includes a remote HTTP redirect URI', async () => {
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
                redirectUris: ['https://example.com/cb', 'http://example.com/cb'],
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
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('disables Save when a remote HTTP redirect URI remains without authorization_code', async () => {
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
                redirectUris: ['http://example.com/cb'],
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
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
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

  describe('Client Secret Popup (just created)', () => {
    afterEach(() => {
      mockUseLocation.mockReturnValue({state: null});
    });

    it('does not render the secret dialog when there is no justCreatedSecret navigation state', () => {
      render(<AgentEditPage />);

      expect(screen.queryByTestId('agent-show-client-secret')).not.toBeInTheDocument();
    });

    it('renders the secret dialog when justCreatedSecret is present in location state', () => {
      mockUseLocation.mockReturnValue({
        state: {
          justCreatedSecret: {
            agentName: 'My New Agent',
            clientId: 'new-agent-client-id',
            clientSecret: 'brand-new-agent-secret',
          },
        },
      });

      render(<AgentEditPage />);

      expect(screen.getByTestId('agent-show-client-secret')).toBeInTheDocument();
      expect(screen.getByDisplayValue('brand-new-agent-secret')).toBeInTheDocument();
    });

    it('closes the secret dialog when Continue is clicked', async () => {
      const user = userEvent.setup();
      mockUseLocation.mockReturnValue({
        state: {
          justCreatedSecret: {
            agentName: 'My New Agent',
            clientSecret: 'brand-new-agent-secret',
          },
        },
      });

      render(<AgentEditPage />);

      await user.click(screen.getByTestId('agent-client-secret-continue'));

      await waitFor(() => {
        expect(screen.queryByTestId('agent-show-client-secret')).not.toBeInTheDocument();
      });
    });
  });
});
