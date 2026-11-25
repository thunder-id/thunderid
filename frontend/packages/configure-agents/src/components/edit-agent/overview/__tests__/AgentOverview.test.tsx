// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {
  RuntimeEnvironmentsContext,
  type EnvironmentValue,
  type RuntimeEnvironment,
  type RuntimeEnvironmentsContextType,
} from '@thunderid/contexts';
import {fireEvent, render, screen, within} from '@thunderid/test-utils';
import {afterEach, describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import AgentOverview from '../AgentOverview';

const {mockGetServerUrl, mockGetDocumentationLink, mockUseGetUsers, mockUseGetUserTypes} = vi.hoisted(() => ({
  mockGetServerUrl: vi.fn(() => 'https://localhost:8090'),
  mockGetDocumentationLink: vi.fn((key: string) => documentationLinks[key]),
  mockUseGetUsers: vi.fn(),
  mockUseGetUserTypes: vi.fn(),
}));

const documentationLinks: Record<string, string | undefined> = {
  'agents.quickstarts.langchain.docs': 'https://thunderid.dev/docs/next/getting-started/connect-your-agent/langchain/',
};

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: {brand: {product_name: 'ThunderID'}},
      getServerUrl: mockGetServerUrl,
      getDocumentationLink: mockGetDocumentationLink,
    }),
  };
});

vi.mock('@thunderid/configure-users', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-users')>()),
  useGetUsers: (...args: unknown[]): unknown => mockUseGetUsers(...args) as unknown,
}));

vi.mock('@thunderid/configure-user-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-user-types')>()),
  useGetUserTypes: (): unknown => mockUseGetUserTypes() as unknown,
}));

// The guide's own reads, covered in its tests.
vi.mock('../../../../api/useGetAgentRoles', () => ({default: () => ({data: undefined})}));
vi.mock('@thunderid/configure-flows', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-flows')>()),
  useGetFlowById: () => ({data: undefined}),
}));

vi.mock('../../attributes/AttributesSummarySection', () => ({
  default: ({variant}: {variant?: string}) => <div data-testid="attributes-summary-section">{variant}</div>,
}));

const mockFetch = vi.fn();

const quickstartMarkdown = `## Register an Agent

Add the agent.

## Create the Project

\`\`\`dotenv title=".env"
THUNDERID_BASE_URL=https://localhost:8090
AGENT_CLIENT_ID=replace-with-your-client-id
\`\`\`

## Run the Agent

<!-- variant mode=own -->

Run it on its own.

<!-- /variant -->

<!-- variant mode=obo -->

Run it for a user.

<!-- /variant -->
`;

const baseAgent: Agent = {
  id: 'agent-123',
  ouId: 'ou-1',
  ouHandle: 'engineering',
  type: 'default',
  display: 'Enrollment Agent',
  clientId: 'client-abc',
  owner: {id: 'user-1', idType: 'uuid', isLocal: true},
};

const delegatedOauth2Config: OAuthAgentConfig = {
  clientId: 'client-abc',
  grantTypes: ['authorization_code', 'client_credentials'],
  responseTypes: ['code'],
};

describe('AgentOverview', () => {
  // The guide repeats the client ID and names the modes too, so the cards beside it are read on their own.
  const card = (title: string) => within(screen.getByText(title).closest<HTMLElement>('.MuiPaper-root')!);
  const details = () => card('Agent details');
  const accessMode = () => card('Access mode');

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
    mockGetDocumentationLink.mockImplementation((key: string) => documentationLinks[key]);
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', display: 'Alice'}]},
      isLoading: false,
    });
    mockUseGetUserTypes.mockReturnValue({
      data: {types: [{id: 'ut-1', handle: 'person', displayName: 'Person'}]},
      isLoading: false,
    });
    mockFetch.mockReset().mockResolvedValue({ok: true, status: 200, text: () => Promise.resolve(quickstartMarkdown)});
    vi.stubGlobal('fetch', mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('always shows agent details, including owner, organization unit id and handle', () => {
    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('Agent details')).toBeInTheDocument();
    expect(screen.getByText('agent-123')).toBeInTheDocument();
    expect(screen.getByText('client-abc')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('ou-1')).toBeInTheDocument();
    expect(screen.getByText('engineering')).toBeInTheDocument();
  });

  it('renders the attributes summary in its own Overview-styled card, using the bare variant', () => {
    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('Attributes')).toBeInTheDocument();
    expect(screen.getByTestId('attributes-summary-section')).toHaveTextContent('bare');
  });

  it('always shows the useful endpoints, built from the server URL', () => {
    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('https://localhost:8090/oauth2/token')).toBeInTheDocument();
    expect(screen.getByText('https://localhost:8090/oauth2/authorize')).toBeInTheDocument();
  });

  it('shows own identity as enabled and delegated as disabled when there is no authorization_code grant', () => {
    render(<AgentOverview agent={baseAgent} oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}} />);

    expect(screen.getByText('On its own behalf')).toBeInTheDocument();
    expect(accessMode().getByText('On behalf of a user')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  it('shows delegated mode as enabled and lists allowed user types when authorization_code is on', () => {
    render(<AgentOverview agent={{...baseAgent, allowedUserTypes: ['person']}} oauth2Config={delegatedOauth2Config} />);

    expect(screen.getByText('Allowed user types')).toBeInTheDocument();
    expect(screen.getByText('Person')).toBeInTheDocument();
  });

  it('shows delegated mode as enabled for an agent that holds CIBA but no authorization_code', () => {
    render(
      <AgentOverview
        agent={baseAgent}
        oauth2Config={{grantTypes: ['client_credentials', 'urn:openid:params:grant-type:ciba'], responseTypes: []}}
      />,
    );

    // Own identity and delegated mode are both on, so neither row reads Disabled.
    expect(screen.getAllByText('Enabled')).toHaveLength(2);
    expect(screen.queryByText('Disabled')).not.toBeInTheDocument();
  });

  // Allowed user types apply only to users signing in through the agent, so only authorization
  // code shows the row, even though every one of these grants turns Delegated mode on.
  it.each([
    ['CIBA', 'urn:openid:params:grant-type:ciba'],
    ['token exchange', 'urn:ietf:params:oauth:grant-type:token-exchange'],
    ['JWT bearer', 'urn:ietf:params:oauth:grant-type:jwt-bearer'],
  ])('should hide the allowed user types row for an agent delegated through %s alone', (_name, grant) => {
    render(
      <AgentOverview
        agent={{...baseAgent, allowedUserTypes: ['person']}}
        oauth2Config={{grantTypes: ['client_credentials', grant], responseTypes: []}}
      />,
    );

    expect(screen.getAllByText('Enabled')).toHaveLength(2);
    expect(screen.queryByText('Allowed user types')).not.toBeInTheDocument();
  });

  it('should hide the allowed user types row when Delegated mode is off', () => {
    render(
      <AgentOverview
        agent={{...baseAgent, allowedUserTypes: ['person']}}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
      />,
    );

    expect(screen.queryByText('Allowed user types')).not.toBeInTheDocument();
  });

  it('falls back to the handle when an allowed user type is not found', () => {
    render(
      <AgentOverview agent={{...baseAgent, allowedUserTypes: ['unknown']}} oauth2Config={delegatedOauth2Config} />,
    );

    expect(screen.getByText('unknown')).toBeInTheDocument();
  });

  it('calls onGoToAdvanced when the Edit in Advanced action is clicked', async () => {
    const user = userEvent.setup();
    const onGoToAdvanced = vi.fn();
    render(<AgentOverview agent={baseAgent} onGoToAdvanced={onGoToAdvanced} />);

    await user.click(screen.getByRole('button', {name: 'Edit in advanced'}));

    expect(onGoToAdvanced).toHaveBeenCalledTimes(1);
  });

  it('shows the LangChain quickstart card when the docs link is configured', () => {
    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('Connect with LangChain')).toBeInTheDocument();
  });

  it('hides the quickstart card when the docs link is not configured', () => {
    mockGetDocumentationLink.mockReturnValue(undefined);
    render(<AgentOverview agent={baseAgent} />);

    expect(screen.queryByText('Connect with LangChain')).not.toBeInTheDocument();
  });

  it('leads from the guide to the tabs that settle what the agent still needs', async () => {
    const user = userEvent.setup();
    const onGoToAdvanced = vi.fn();
    render(
      <AgentOverview
        agent={baseAgent}
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        onGoToAdvanced={onGoToAdvanced}
      />,
    );
    await user.click(screen.getByRole('button', {name: 'On behalf of a user'}));

    await user.click(screen.getByRole('button', {name: 'Turn on in Advanced'}));

    expect(onGoToAdvanced).toHaveBeenCalledTimes(1);
  });

  it('links to the full quickstart on the docs site', () => {
    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByRole('link', {name: 'Read the full guide'})).toHaveAttribute(
      'href',
      'https://thunderid.dev/docs/next/getting-started/connect-your-agent/langchain/',
    );
  });

  it("shows the quickstart's steps with this agent's values filled into the code", async () => {
    render(<AgentOverview agent={baseAgent} />);

    expect(await screen.findByRole('heading', {name: 'Create the Project'})).toBeInTheDocument();
    expect(mockFetch).toHaveBeenCalledWith(
      'https://thunderid.dev/docs/next/getting-started/connect-your-agent/langchain.md',
    );
    const sample = screen.getByLabelText('.env');
    expect(sample).toHaveTextContent('AGENT_CLIENT_ID=client-abc');
    expect(sample).not.toHaveTextContent('replace-with-your-client-id');
    expect(screen.getByText('Run it on its own.')).toBeInTheDocument();
    expect(screen.queryByText('Run it for a user.')).not.toBeInTheDocument();
  });

  it('puts the access mode and attributes beside the guide, and in its place when there is none', () => {
    const {unmount} = render(<AgentOverview agent={baseAgent} />);
    const guide = screen.getByRole('region', {name: 'Connect with LangChain'});
    expect(guide.parentElement).not.toContainElement(screen.getByText('Access mode'));
    expect(screen.getByText('Agent details').closest('.MuiStack-root')).toContainElement(
      screen.getByText('Access mode'),
    );
    unmount();

    mockGetDocumentationLink.mockReturnValue(undefined);
    render(<AgentOverview agent={baseAgent} />);
    expect(screen.getByText('Agent details').closest('.MuiStack-root')).not.toContainElement(
      screen.getByText('Access mode'),
    );
  });

  describe('personal agent', () => {
    const personalAgent = {...baseAgent, kind: 'personal-assistant', allowedUserTypes: ['person']};

    it('shows its own behalf as disabled, and only its owner as allowed to sign in', () => {
      render(<AgentOverview agent={personalAgent} oauth2Config={delegatedOauth2Config} />);

      const ownRow = screen.getByText('On its own behalf').parentElement!;
      expect(within(ownRow).getByText('Disabled')).toBeInTheDocument();
      const delegatedRow = accessMode().getByText('On behalf of a user').parentElement!;
      expect(within(delegatedRow).getByText('Enabled')).toBeInTheDocument();
      expect(screen.getByText('Allowed users')).toBeInTheDocument();
      expect(screen.getByText('Owner only')).toBeInTheDocument();
      expect(screen.queryByText('Allowed user types')).not.toBeInTheDocument();
    });

    it('does not offer the LangChain quickstart, since the owner connects the agent', () => {
      render(<AgentOverview agent={personalAgent} oauth2Config={delegatedOauth2Config} />);

      expect(screen.queryByText('Connect with LangChain')).not.toBeInTheDocument();
    });
  });

  it('falls back to the username attribute for the owner label when the matched user has no display name', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {username: 'alice.doe', email: 'alice@example.com'}}]},
      isLoading: false,
    });

    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('alice.doe')).toBeInTheDocument();
  });

  it('falls back to the email attribute for the owner label when there is no display name or username', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {email: 'alice@example.com'}}]},
      isLoading: false,
    });

    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });

  it('falls back to the user id for the owner label when the matched user has no display name or attributes', () => {
    mockUseGetUsers.mockReturnValue({data: {users: [{id: 'user-1'}]}, isLoading: false});

    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
  });

  it('shows an external owner ID without matching it to a local user', () => {
    mockUseGetUsers.mockReturnValue({data: {users: [{id: 'user-1', display: 'Alice'}]}, isLoading: false});

    render(<AgentOverview agent={{...baseAgent, owner: {id: 'user-1', idType: 'uuid', isLocal: false}}} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
    expect(screen.queryByText('Alice')).not.toBeInTheDocument();
  });

  it('shows the raw owner id when the owner is not among the fetched users', () => {
    mockUseGetUsers.mockReturnValue({data: {users: [{id: 'someone-else', display: 'Bob'}]}, isLoading: false});

    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
    expect(screen.queryByText('Bob')).not.toBeInTheDocument();
  });

  describe('environments', () => {
    const development: RuntimeEnvironment = {id: 'env-dev', name: 'Development', runtimeUrl: 'https://dev.example.com'};
    const production: RuntimeEnvironment = {id: 'env-prod', name: 'Production', runtimeUrl: 'https://prod.example.com'};
    const getValue = vi.fn<RuntimeEnvironmentsContextType['getValue']>();
    let clientIds: Record<string, EnvironmentValue>;

    const renderWithEnvironments = (environments: RuntimeEnvironment[]) =>
      render(
        <RuntimeEnvironmentsContext.Provider value={{environments, getValue, deployPath: '/projects/default/pipeline'}}>
          <AgentOverview agent={baseAgent} />
        </RuntimeEnvironmentsContext.Provider>,
      );

    const selectEnvironment = (name: string): void => {
      fireEvent.mouseDown(screen.getByRole('combobox', {name: 'Environment'}));
      fireEvent.click(within(screen.getByRole('listbox')).getByRole('option', {name: new RegExp(`^${name}`)}));
    };

    beforeEach(() => {
      clientIds = {
        'env-dev': {status: 'set', value: 'client-dev'},
        'env-prod': {status: 'set', value: 'client-prod'},
      };
      getValue.mockReset().mockImplementation((environmentId: string) => Promise.resolve(clientIds[environmentId]));
    });

    it('shows no environment choice when the console provides no environments', () => {
      render(<AgentOverview agent={baseAgent} />);

      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(screen.getByText('client-abc')).toBeInTheDocument();
      expect(screen.getByText('https://localhost:8090/oauth2/token')).toBeInTheDocument();
    });

    it("shows the first environment's client ID and endpoints, read for the agent", async () => {
      renderWithEnvironments([development, production]);

      expect(screen.getByRole('combobox', {name: 'Environment'})).toHaveTextContent('Development');
      expect(screen.getByRole('link', {name: 'Deployment pipeline'})).toHaveAttribute(
        'href',
        '/projects/default/pipeline',
      );
      expect(await details().findByText('client-dev')).toBeInTheDocument();
      expect(screen.queryByText('client-abc')).not.toBeInTheDocument();
      expect(screen.getByText('https://dev.example.com/oauth2/token')).toBeInTheDocument();
      expect(getValue).toHaveBeenCalledWith('env-dev', {
        resourceType: 'agent',
        resourceId: 'agent-123',
        field: 'clientId',
      });
    });

    it('shows the client ID and endpoints of the environment chosen, and fills them into the guide', async () => {
      renderWithEnvironments([development, production]);
      await details().findByText('client-dev');

      selectEnvironment('Production');

      expect(await details().findByText('client-prod')).toBeInTheDocument();
      expect(await screen.findByLabelText('.env')).toHaveTextContent('THUNDERID_BASE_URL=https://prod.example.com');
      expect(screen.getByLabelText('.env')).toHaveTextContent('AGENT_CLIENT_ID=client-prod');
      expect(screen.getByText('https://prod.example.com/oauth2/token')).toBeInTheDocument();
      expect(screen.queryByText('https://dev.example.com/oauth2/token')).not.toBeInTheDocument();
    });

    it('shows no client ID or endpoints for an environment the agent is not in, and the way to get it there', async () => {
      clientIds['env-prod'] = {status: 'absent'};
      renderWithEnvironments([development, production]);
      await details().findByText('client-dev');

      selectEnvironment('Production');

      expect(await screen.findByText('Not promoted to Production yet')).toBeInTheDocument();
      expect(
        screen.getByText(
          'There is no client ID or endpoints for Enrollment Agent here. Promote it from Development to get them.',
        ),
      ).toBeInTheDocument();
      expect(screen.getByRole('link', {name: 'Open deployment pipeline'})).toHaveAttribute(
        'href',
        '/projects/default/pipeline',
      );
      expect(screen.queryByText('Client ID')).not.toBeInTheDocument();
      expect(screen.queryByText('Useful endpoints')).not.toBeInTheDocument();
      expect(screen.getByText('agent-123')).toBeInTheDocument();
    });

    it("shows a single environment's client ID and endpoints, with no environment choice", async () => {
      renderWithEnvironments([development]);

      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(await details().findByText('client-dev')).toBeInTheDocument();
      expect(screen.queryByText('client-abc')).not.toBeInTheDocument();
      expect(screen.getByText('https://dev.example.com/oauth2/token')).toBeInTheDocument();
    });

    it('says where the only environment does not have the agent', async () => {
      clientIds['env-dev'] = {status: 'absent'};
      renderWithEnvironments([development]);

      expect(await screen.findByRole('status')).toHaveTextContent('Not deployed to Development yet');
      expect(screen.queryByText('Environment')).not.toBeInTheDocument();
      expect(screen.queryByText('client-abc')).not.toBeInTheDocument();
      expect(screen.queryByText('https://dev.example.com/oauth2/token')).not.toBeInTheDocument();
    });

    it('says where an environment runs the agent without a client ID', async () => {
      clientIds['env-dev'] = {status: 'unset'};
      renderWithEnvironments([development, production]);

      expect(await screen.findByText('No client ID is set in Development yet.')).toBeInTheDocument();
    });
  });
});
