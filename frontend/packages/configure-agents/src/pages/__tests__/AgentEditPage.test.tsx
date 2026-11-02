// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import userEvent from '@testing-library/user-event';
import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import type {ComponentProps, ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
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
  advancedSettings,
} = vi.hoisted(() => ({
  // Switches the Advanced tab from its stub to the real component for the tests that need it.
  advancedSettings: {real: false},
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

vi.mock('@thunderid/configure-agent-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-agent-types')>()),
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
    AccountStatusAction: ({listQueryKey, onChanged}: {listQueryKey: unknown; onChanged: () => void}) => (
      <button
        type="button"
        data-testid="account-status-action"
        data-list-query-key={JSON.stringify(listQueryKey)}
        onClick={onChanged}
      >
        status action
      </button>
    ),
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

vi.mock('../../components/edit-agent/advanced-settings/EditAdvancedSettings', async (importOriginal) => {
  const {default: RealEditAdvancedSettings} =
    await importOriginal<typeof import('../../components/edit-agent/advanced-settings/EditAdvancedSettings')>();
  return {
    default: (props: ComponentProps<typeof RealEditAdvancedSettings>) => {
      if (advancedSettings.real) return <RealEditAdvancedSettings {...props} />;
      const {onDeleteSuccess, onBackchannelLogoutUriValidationChange, onIdentityAssertionsValidationChange} = props;
      return (
        <div data-testid="edit-advanced">
          <button type="button" onClick={() => onDeleteSuccess?.()}>
            Delete Successful
          </button>
          <button type="button" onClick={() => onBackchannelLogoutUriValidationChange?.(true)}>
            Type an invalid back-channel logout URI
          </button>
          <button type="button" onClick={() => onIdentityAssertionsValidationChange?.(true)}>
            Type an invalid ID-JAG validity period
          </button>
        </div>
      );
    },
  };
});

// The real Advanced tab's other sections fetch data the back-channel logout tests do not need.
vi.mock('../../components/edit-agent/advanced-settings/OwnerSection', () => ({default: () => null}));
vi.mock('../../components/edit-agent/advanced-settings/AllowedUserTypesSection', () => ({default: () => null}));
vi.mock('../../components/edit-agent/advanced-settings/AgentSignInSection', () => ({default: () => null}));
vi.mock('../../components/AgentDeleteDialog', () => ({default: () => null}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({
    discovery: {wellKnown: {token_endpoint_auth_methods_supported: ['client_secret_basic', 'none']}},
  }),
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
  Trans: ({defaults = ''}: {defaults?: string}) => <span>{defaults}</span>,
}));

// The real Identity assertions card reads the runtime's authorization server metadata. Answer that
// one request from `serverMetadata` (a 404, which is unknown and blocks nothing, unless a test sets
// a body) rather than reaching whatever listens on the test server URL; other requests pass through.
const serverMetadata: {body: Record<string, string[]> | undefined} = {body: undefined};
const realFetch = globalThis.fetch;
const fetchWithStubbedServerMetadata = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
  let url: string;
  if (typeof input === 'string') url = input;
  else if (input instanceof URL) url = input.href;
  else url = input.url;
  if (!url.endsWith('/.well-known/oauth-authorization-server')) return realFetch(input, init);
  return Promise.resolve(
    serverMetadata.body
      ? new Response(JSON.stringify(serverMetadata.body), {status: 200, headers: {'Content-Type': 'application/json'}})
      : new Response(null, {status: 404}),
  );
};

describe('AgentEditPage', () => {
  const baseAgent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    display: 'Test Agent',
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

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    serverMetadata.body = undefined;
    vi.stubGlobal('fetch', fetchWithStubbedServerMetadata);
    stagingCallbackIdentities.clear();
    advancedSettings.real = false;
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
      data: {types: [{id: 'default-type', handle: 'default', displayName: 'Default'}]},
      isLoading: false,
      error: null,
    });
    mockUseGetAgentType.mockReturnValue({
      data: {id: 'default-type', handle: 'default', displayName: 'Default', schema: {}},
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

    it('falls back to the agent ID when the agent has no display value', () => {
      mockUseGetAgent.mockReturnValue({data: {...baseAgent, display: undefined}, isLoading: false, error: null});
      render(<AgentEditPage />);

      expect(screen.getAllByRole('heading', {name: 'agent-1'}).length).toBeGreaterThan(0);
      expect(screen.queryByText('Test Agent')).not.toBeInTheDocument();
    });

    it('does not offer an inline rename, since the name is a schema attribute', () => {
      render(<AgentEditPage />);

      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      expect(editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test Agent'))).toBeUndefined();
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

  // The server holds a delegated agent to the delegated grant rules and does not store the flag, so a
  // save has to say whether the agent is delegated. Delegated mode is any delegation grant.
  describe('Delegated flag on save', () => {
    // A delegated agent has to pass the page's own checks before it can be saved: a callback and an
    // allowed user type.
    const saveWithGrants = async (grantTypes?: string[]): Promise<{delegated?: boolean}> => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['employee'],
          inboundAuthConfig:
            grantTypes === undefined
              ? []
              : [
                  {
                    type: 'oauth2' as const,
                    config: {
                      ...baseAgent.inboundAuthConfig[0].config,
                      grantTypes,
                      redirectUris: ['https://app.example.com/callback'],
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

      await user.click(screen.getByRole('button', {name: 'Pick logo B'}));
      await user.click(screen.getByRole('button', {name: 'Save logo'}));

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalled();
      });
      return (mockMutateAsync.mock.calls[0][0] as {data: {delegated?: boolean}}).data;
    };

    it('sends delegated when the agent holds the authorization code grant', async () => {
      const data = await saveWithGrants(['client_credentials', 'authorization_code', 'refresh_token']);

      expect(data.delegated).toBe(true);
    });

    it('sends delegated when the agent holds CIBA or token exchange without the authorization code grant', async () => {
      const data = await saveWithGrants(['client_credentials', 'urn:openid:params:grant-type:ciba']);

      expect(data.delegated).toBe(true);
    });

    it('should send delegated when the agent holds JWT bearer without the authorization code grant', async () => {
      const data = await saveWithGrants(['client_credentials', 'urn:ietf:params:oauth:grant-type:jwt-bearer']);

      expect(data.delegated).toBe(true);
    });

    it('sends not delegated when the agent has client credentials only', async () => {
      const data = await saveWithGrants(['client_credentials']);

      expect(data.delegated).toBe(false);
    });

    it('sends not delegated when the agent has no OAuth configuration', async () => {
      const data = await saveWithGrants();

      expect(data.delegated).toBe(false);
    });
  });

  describe('Access status', () => {
    const renderAgent = (agent: Record<string, unknown>): void => {
      mockUseGetAgent.mockReturnValue({
        data: {...baseAgent, ...agent},
        isLoading: false,
        error: null,
        isError: false,
        refetch: mockRefetch,
      });
      render(<AgentEditPage />);
    };

    it('shows the suspension banner for a suspended agent', () => {
      renderAgent({status: {value: 'SUSPENDED'}});

      expect(screen.getByRole('status')).toHaveTextContent('Suspended');
      expect(screen.getByRole('status')).toHaveTextContent('The agent is suspended');
    });

    it('shows no banner for an active agent', () => {
      renderAgent({});

      expect(screen.queryByRole('status')).not.toBeInTheDocument();
    });

    it('offers the lifecycle action on a writable agent', () => {
      renderAgent({});

      expect(screen.getByTestId('account-status-action')).toBeInTheDocument();
    });

    // The lifecycle flows change only runtime state, which a declared record keeps too.
    it('offers the lifecycle action on a read-only agent', () => {
      renderAgent({isReadOnly: true});

      expect(screen.getByTestId('account-status-action')).toBeInTheDocument();
    });

    it('shows the banner below the read-only notice and above the header', () => {
      renderAgent({isReadOnly: true, status: {value: 'SUSPENDED'}});

      const notice = screen.getByText('This resource is read-only and cannot be modified.');
      const banner = screen.getByRole('status');
      expect(banner).toHaveTextContent('Suspended');
      expect(notice.compareDocumentPosition(banner) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(
        banner.compareDocumentPosition(screen.getByTestId('account-status-action')) & Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    });

    it('hands the agents list query to the lifecycle action', () => {
      renderAgent({});

      expect(screen.getByTestId('account-status-action')).toHaveAttribute('data-list-query-key', '["agents"]');
    });

    it('refetches the agent after a lifecycle operation', async () => {
      const user = userEvent.setup();
      renderAgent({});

      await user.click(screen.getByTestId('account-status-action'));

      expect(mockRefetch).toHaveBeenCalled();
    });

    it('keeps a suspended agent editable', async () => {
      const user = userEvent.setup();
      renderAgent({allowedUserTypes: [], status: {value: 'SUSPENDED'}});

      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const editButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test description'));
      if (!editButton) throw new Error('description edit button not found');
      await user.click(editButton);
      const input = screen.getAllByRole('textbox')[0];
      await user.type(input, ' updated');
      fireEvent.blur(input);

      expect(screen.getByRole('button', {name: 'Save'})).toBeEnabled();
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

      expect(screen.getByRole('button', {name: 'Update logo'})).toBeInTheDocument();
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

      expect(screen.queryByRole('button', {name: 'Update logo'})).not.toBeInTheDocument();
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
    const editDescription = async (
      user: ReturnType<typeof userEvent.setup>,
      from: string,
      to: string,
    ): Promise<void> => {
      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const editButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes(from));
      if (!editButton) throw new Error(`description edit button for "${from}" not found`);
      await user.click(editButton);
      const input = screen.getAllByRole('textbox').find((el) => (el as HTMLTextAreaElement).value === from);
      if (!input) throw new Error(`description textarea for "${from}" not found`);
      await user.clear(input);
      await user.type(input, to);
      fireEvent.blur(input);
    };

    it('hides the bar when a field is manually retyped back to its original value', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await editDescription(user, 'Test description', 'Changed description');
      expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();

      await editDescription(user, 'Changed description', 'Test description');
      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });

    it('keeps the bar visible when only one of two edited fields is reverted', async () => {
      const user = userEvent.setup();
      render(<AgentEditPage />);

      await editDescription(user, 'Test description', 'Changed description');
      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));
      await editDescription(user, 'Changed description', 'Test description');

      // The staged attribute edit is still pending, so the bar must stay visible
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

    it('names the back-channel logout URI field when the server refuses a private address', async () => {
      const user = userEvent.setup();
      mockUseUpdateAgent.mockReturnValue({
        mutateAsync: mockMutateAsync,
        isPending: false,
        error: {
          response: {
            data: {
              code: 'AGT-1021',
              description: {key: 'error.agentservice.backchannel_logout_uri_private_host_description'},
            },
          },
        } as unknown as Error,
        isError: true,
        reset: vi.fn(),
      });

      render(<AgentEditPage />);

      await user.click(screen.getByRole('tab', {name: 'Attributes'}));
      await user.click(screen.getByText('Edit an attribute'));

      expect(screen.getByText(/The server refused the back-channel logout URI/)).toBeInTheDocument();
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
    // Any field edit is enough to surface the Save bar — editing the description is the simplest
    // one available without depending on any of the (mocked) tab content components.
    const triggerAChange = async (user: ReturnType<typeof userEvent.setup>) => {
      const editIcons = screen.getAllByRole('button').filter((b) => b.querySelector('svg'));
      const editButton = editIcons.find((btn) => btn.parentElement?.textContent?.includes('Test description'));
      if (!editButton) throw new Error('description edit button not found');
      await user.click(editButton);
      const input = screen.getAllByRole('textbox')[0];
      await user.type(input, ' updated');
      fireEvent.blur(input);
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

    it('disables Save when the back-channel logout URI is http on a public client, even without visiting the advanced tab', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['person'],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['client_credentials', 'authorization_code'],
                responseTypes: ['code'],
                redirectUris: ['https://agent.example.com/cb'],
                publicClient: true,
                backchannelLogoutUri: 'http://agent.example.com/bcl',
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

      expect(screen.getByText('Before saving, fix the back-channel logout URI.')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('disables Save while the back-channel logout URI field holds an invalid typed value', async () => {
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['person'],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['client_credentials', 'authorization_code'],
                responseTypes: ['code'],
                redirectUris: ['https://agent.example.com/cb'],
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

      await user.click(screen.getByRole('tab', {name: /Advanced/i}));
      await user.click(screen.getByText('Type an invalid back-channel logout URI'));

      expect(screen.getByText('Before saving, fix the back-channel logout URI.')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    it('blocks Save, without a render loop, when an invalid back-channel logout URI is typed in the real Advanced tab', async () => {
      advancedSettings.real = true;
      // Silenced, so a loop cannot flood the output, and restored in finally, so it cannot leak.
      const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['person'],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['client_credentials', 'authorization_code'],
                responseTypes: ['code'],
                redirectUris: ['https://agent.example.com/cb'],
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

      try {
        render(<AgentEditPage />);
        await triggerAChange(user);
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));
        await user.type(screen.getByPlaceholderText('https://example.com/backchannel-logout'), 'x');
        await user.tab();

        expect(screen.getByText('Before saving, fix the back-channel logout URI.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
        expect(consoleError.mock.calls.flat().join(' ')).not.toContain('Maximum update depth exceeded');
      } finally {
        consoleError.mockRestore();
      }
    });

    it('stops blocking Save once the Advanced tab, and the invalid value typed in it, is left behind', async () => {
      advancedSettings.real = true;
      const user = userEvent.setup();
      mockUseGetAgent.mockReturnValue({
        data: {
          ...baseAgent,
          allowedUserTypes: ['person'],
          inboundAuthConfig: [
            {
              type: 'oauth2' as const,
              config: {
                grantTypes: ['client_credentials', 'authorization_code'],
                responseTypes: ['code'],
                redirectUris: ['https://agent.example.com/cb'],
                backchannelLogoutUri: 'https://agent.example.com/bcl',
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
      await user.click(screen.getByRole('tab', {name: /Advanced/i}));
      await user.type(screen.getByDisplayValue('https://agent.example.com/bcl'), '#frag');
      await user.tab();
      expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();

      // The typed value was never committed and is dropped with the tab, so the saved URI stands.
      await user.click(screen.getByRole('tab', {name: 'Overview'}));
      expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();

      await user.click(screen.getByRole('tab', {name: /Advanced/i}));
      expect(screen.getByDisplayValue('https://agent.example.com/bcl')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
    });

    describe('identity assertions (ID-JAG)', () => {
      const renderWithIdJag = (
        idJag: Record<string, unknown>,
        grantTypes = ['client_credentials', 'urn:ietf:params:oauth:grant-type:token-exchange'],
      ): void => {
        mockUseGetAgent.mockReturnValue({
          data: {
            ...baseAgent,
            inboundAuthConfig: [
              {
                type: 'oauth2' as const,
                config: {grantTypes, responseTypes: [], clientId: 'client-id-xyz', token: {idJag}},
              },
            ],
          },
          isLoading: false,
          error: null,
          isError: false,
          refetch: mockRefetch,
        });
        render(<AgentEditPage />);
      };

      it('should disable Save when ID-JAG is on with no allowed audience, even without visiting the advanced tab', async () => {
        const user = userEvent.setup();
        renderWithIdJag({enabled: true, allowedAudiences: []});
        await triggerAChange(user);

        expect(screen.getByText('Before saving, fix the identity assertions (ID-JAG) settings.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
      });

      it('should enable Save when ID-JAG is on with an allowed audience', async () => {
        const user = userEvent.setup();
        renderWithIdJag({enabled: true, allowedAudiences: ['https://mcp.example.com']});
        await triggerAChange(user);

        expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
      });

      it('should not block Save on an ID-JAG config that is turned off', async () => {
        const user = userEvent.setup();
        renderWithIdJag({enabled: false, allowedAudiences: []});
        await triggerAChange(user);

        expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
      });

      it('should disable Save while the card reports an invalid typed value', async () => {
        const user = userEvent.setup();
        renderWithIdJag({enabled: true, allowedAudiences: ['https://mcp.example.com']});
        await triggerAChange(user);
        expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();

        await user.click(screen.getByRole('tab', {name: /Advanced/i}));
        await user.click(screen.getByText('Type an invalid ID-JAG validity period'));

        expect(screen.getByText('Before saving, fix the identity assertions (ID-JAG) settings.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
      });

      it('should block Save on an invalid validity period typed in the real card, until the tab is left', async () => {
        advancedSettings.real = true;
        const user = userEvent.setup();
        renderWithIdJag({enabled: true, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 300});
        await triggerAChange(user);
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));

        fireEvent.change(screen.getByDisplayValue('300'), {target: {value: '0'}});

        expect(screen.getByText('Enter a value of at least 1 second.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();

        // The typed value was never committed and is dropped with the tab, so the saved one stands.
        await user.click(screen.getByRole('tab', {name: 'Overview'}));
        expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
      });

      it('should save token exchange and ID-JAG together when ID-JAG is turned on in the real card', async () => {
        advancedSettings.real = true;
        const user = userEvent.setup();
        mockUseGetAgent.mockReturnValue({
          data: {
            ...baseAgent,
            allowedUserTypes: ['person'],
            inboundAuthConfig: [
              {
                type: 'oauth2' as const,
                config: {
                  grantTypes: ['client_credentials', 'authorization_code'],
                  responseTypes: ['code'],
                  redirectUris: ['https://agent.example.com/cb'],
                  pkceRequired: true,
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
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));

        await user.click(screen.getByRole('switch', {name: 'Identity assertions (ID-JAG)'}));
        await user.type(
          screen.getByPlaceholderText('Type an audience and press Enter'),
          'https://mcp.example.com{Enter}',
        );
        await user.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => {
          expect(mockMutateAsync).toHaveBeenCalledTimes(1);
        });
        const {data} = mockMutateAsync.mock.calls[0][0] as {
          data: {inboundAuthConfig: {config: Record<string, unknown>}[]};
        };
        expect(data.inboundAuthConfig[0].config).toEqual(
          expect.objectContaining({
            grantTypes: ['client_credentials', 'authorization_code', 'urn:ietf:params:oauth:grant-type:token-exchange'],
            pkceRequired: true,
            token: {idJag: {enabled: true, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 300}},
          }),
        );
      });

      it('should enable Save once Delegated mode is turned off while the real card holds an invalid validity period', async () => {
        advancedSettings.real = true;
        const user = userEvent.setup();
        renderWithIdJag({enabled: true, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 300});
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));

        fireEvent.change(screen.getByDisplayValue('300'), {target: {value: '0'}});
        await triggerAChange(user);
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();

        await user.click(screen.getByRole('switch', {name: 'Delegated mode'}));

        expect(screen.queryByRole('switch', {name: 'Identity assertions (ID-JAG)'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).not.toBeDisabled();
      });

      it('should not let ID-JAG be turned on in the real card when the runtime does not serve token exchange', async () => {
        advancedSettings.real = true;
        serverMetadata.body = {grant_types_supported: ['client_credentials', 'authorization_code']};
        const user = userEvent.setup();
        renderWithIdJag({enabled: false});
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));

        expect(
          await screen.findByText(
            'This server does not allow the token exchange grant, so identity assertions cannot be issued.',
          ),
        ).toBeInTheDocument();
        expect(screen.getByRole('switch', {name: 'Identity assertions (ID-JAG)'})).toBeDisabled();
      });

      it('should save ID-JAG off without the token exchange grant when the runtime does not serve it', async () => {
        advancedSettings.real = true;
        serverMetadata.body = {grant_types_supported: ['client_credentials', 'authorization_code']};
        const user = userEvent.setup();
        renderWithIdJag({enabled: true, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 300});
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));
        await screen.findByText(
          'This server does not allow the token exchange grant, so identity assertions cannot be issued.',
        );

        await user.click(screen.getByRole('switch', {name: 'Identity assertions (ID-JAG)'}));
        await user.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => {
          expect(mockMutateAsync).toHaveBeenCalledTimes(1);
        });
        const {data} = mockMutateAsync.mock.calls[0][0] as {
          data: {inboundAuthConfig: {config: {grantTypes: string[]; token: Record<string, unknown>}}[]};
        };
        expect(data.inboundAuthConfig[0].config.grantTypes).not.toContain(
          'urn:ietf:params:oauth:grant-type:token-exchange',
        );
        expect(data.inboundAuthConfig[0].config.token).toEqual({
          idJag: {enabled: false, allowedAudiences: ['https://mcp.example.com'], validityPeriod: 300},
        });
      });

      it('should block Save when ID-JAG is turned on in the real card without an allowed audience', async () => {
        advancedSettings.real = true;
        const user = userEvent.setup();
        renderWithIdJag({enabled: false});
        await triggerAChange(user);
        await user.click(screen.getByRole('tab', {name: /Advanced/i}));

        await user.click(screen.getByRole('switch', {name: 'Identity assertions (ID-JAG)'}));

        expect(screen.getByText('Add at least one audience.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
      });
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

  describe('Client secret Popup (just created)', () => {
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
