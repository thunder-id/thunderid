// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {Agent, OAuthAgentConfig} from '../../../../models/agent';
import ConnectAgentGuide from '../ConnectAgentGuide';

const {mockUseGetAgentRoles, mockUseGetFlowById} = vi.hoisted(() => ({
  mockUseGetAgentRoles: vi.fn(),
  mockUseGetFlowById: vi.fn(),
}));

vi.mock('../../../../api/useGetAgentRoles', () => ({
  default: (...args: unknown[]): unknown => mockUseGetAgentRoles(...args) as unknown,
}));

vi.mock('@thunderid/configure-flows', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-flows')>()),
  useGetFlowById: (...args: unknown[]): unknown => mockUseGetFlowById(...args) as unknown,
}));

const mockFetch = vi.fn();

const agent: Agent = {id: 'agent-123', ouId: 'ou-1', type: 'default', authFlowId: 'flow-1'};
const ownConfig: OAuthAgentConfig = {grantTypes: ['client_credentials'], responseTypes: []};
const delegatedConfig: OAuthAgentConfig = {
  grantTypes: ['authorization_code', 'client_credentials'],
  responseTypes: ['code'],
  redirectUris: ['http://localhost:6274/callback'],
  pkceRequired: true,
};

const markdown = `## Register an Agent

Add the agent.

## Create the Project

<!-- variant lang=python -->

\`\`\`bash
pip install langchain
\`\`\`

<!-- /variant -->

<!-- variant lang=typescript -->

\`\`\`bash
npm install langchain
\`\`\`

<!-- /variant -->

\`\`\`dotenv title=".env"
AGENT_CLIENT_ID=replace-with-your-client-id
\`\`\`

## Run the Agent

<!-- variant mode=own -->

The agent lists the courses.

<!-- /variant -->

<!-- variant mode=obo -->

The agent enrolls you. See [delegated access](https://example.com/docs/delegated.md).

<!-- /variant -->
`;

const docsUrl = 'https://example.com/docs/langchain/';

describe('ConnectAgentGuide', () => {
  const onOpenLink = vi.fn();
  const onGoToAdvanced = vi.fn();
  const onGoToAccess = vi.fn();
  const onGoToFlows = vi.fn();
  const renderGuide = (props: {oauth2Config?: OAuthAgentConfig; isDelegated?: boolean; hasTabs?: boolean} = {}) =>
    render(
      <ConnectAgentGuide
        docsUrl={docsUrl}
        codeValues={{'replace-with-your-client-id': 'client-abc'}}
        agent={agent}
        oauth2Config={props.oauth2Config}
        isDelegated={props.isDelegated ?? false}
        onGoToAdvanced={props.hasTabs === false ? undefined : onGoToAdvanced}
        onGoToAccess={props.hasTabs === false ? undefined : onGoToAccess}
        onGoToFlows={props.hasTabs === false ? undefined : onGoToFlows}
        onOpenLink={onOpenLink}
      />,
    );
  /** One check of the "Before you run" list, by its title. */
  const check = (title: string): HTMLElement => screen.getByText(title).closest('li')!;
  // The install command is the guide's only untitled sample, so its label is the generic one.
  const installCommand = (): HTMLElement => screen.getByLabelText('Code sample');

  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockReset().mockResolvedValue({ok: true, status: 200, text: () => Promise.resolve(markdown)});
    vi.stubGlobal('fetch', mockFetch);
    mockUseGetAgentRoles.mockReturnValue({data: {totalResults: 0, startIndex: 1, count: 0, roles: []}});
    mockUseGetFlowById.mockReturnValue({
      data: {
        name: 'Basic Login Flow',
        nodes: [{id: 'auth', type: 'TASK_EXECUTION', executor: {name: 'BasicAuthExecutor'}}],
      },
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows the steps for an agent acting on its own, in Python, until another choice is made', async () => {
    renderGuide();

    expect(await screen.findByRole('heading', {name: 'Create the Project'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Acting on its own'})).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', {name: 'Python'})).toHaveAttribute('aria-haspopup', 'menu');
    expect(installCommand()).toHaveTextContent('pip install langchain');
    expect(screen.getByText('The agent lists the courses.')).toBeInTheDocument();
    expect(screen.getByText(/No user signs in/)).toBeInTheDocument();
    expect(screen.getByLabelText('.env')).toHaveTextContent('AGENT_CLIENT_ID=client-abc');
  });

  it('says the steps build a sample use case, before what the sample needs', () => {
    renderGuide({oauth2Config: ownConfig});

    const heading = screen.getByRole('heading', {name: 'Try it with a sample use case'});
    const checks = screen.getByRole('heading', {name: 'Before you run the sample'});
    expect(heading.compareDocumentPosition(checks) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('keeps the mode chosen when it is selected again', async () => {
    const user = userEvent.setup();
    renderGuide();
    await screen.findByText('The agent lists the courses.');

    await user.click(screen.getByRole('button', {name: 'Acting on its own'}));

    expect(screen.getByRole('button', {name: 'Acting on its own'})).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('The agent lists the courses.')).toBeInTheDocument();
  });

  it('shows the code of the language chosen', async () => {
    const user = userEvent.setup();
    renderGuide();
    await screen.findByRole('heading', {name: 'Create the Project'});

    await user.click(screen.getByRole('button', {name: 'Python'}));
    const languages = within(screen.getByRole('menu', {name: 'Language'}));
    expect(languages.getByRole('menuitemradio', {name: 'Python'})).toHaveAttribute('aria-checked', 'true');
    await user.click(languages.getByRole('menuitemradio', {name: 'TypeScript'}));

    expect(screen.getByRole('button', {name: 'TypeScript'})).toBeInTheDocument();

    await waitFor(() => expect(installCommand()).toHaveTextContent('npm install langchain'));
  });

  it('shows the steps for acting on behalf of a user, with links opened through the host page', async () => {
    const user = userEvent.setup();
    renderGuide();
    await screen.findByText('The agent lists the courses.');

    await user.click(screen.getByRole('button', {name: 'On behalf of a user'}));

    expect(await screen.findByText(/The agent enrolls you/)).toBeInTheDocument();
    expect(screen.queryByText('The agent lists the courses.')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'delegated access'}));
    expect(onOpenLink).toHaveBeenCalledWith('https://example.com/docs/delegated');
  });

  describe('before you run', () => {
    it('says what an agent acting on its own still needs, and the way to settle it', async () => {
      const user = userEvent.setup();
      renderGuide({oauth2Config: ownConfig});

      expect(screen.getByRole('list', {name: 'Before you run the sample'})).toBeInTheDocument();
      expect(within(check('This agent can act on its own')).getByRole('img', {name: 'Ready'})).toBeInTheDocument();
      const roles = check('No role assigned to this agent');
      expect(within(roles).getByRole('img', {name: 'Needs attention'})).toBeInTheDocument();
      expect(mockUseGetAgentRoles).toHaveBeenCalledWith('agent-123', {limit: 100, offset: 0});

      await user.click(within(roles).getByRole('button', {name: 'Assign in Access'}));
      expect(onGoToAccess).toHaveBeenCalledTimes(1);
    });

    it('says when an agent acting on its own has all it needs', () => {
      mockUseGetAgentRoles.mockReturnValue({data: {totalResults: 1, startIndex: 1, count: 1, roles: ['Reader']}});
      renderGuide({oauth2Config: ownConfig});

      expect(within(check('A role is assigned to this agent')).getByRole('img', {name: 'Ready'})).toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Assign in Access'})).not.toBeInTheDocument();
    });

    it('says when the agent may not act on its own', async () => {
      const user = userEvent.setup();
      renderGuide({oauth2Config: {grantTypes: ['authorization_code'], responseTypes: ['code']}});

      await user.click(
        within(check('This agent cannot act on its own')).getByRole('button', {name: 'Turn on in Advanced'}),
      );
      expect(onGoToAdvanced).toHaveBeenCalledTimes(1);
    });

    it('leaves out a check whose answer is not known', () => {
      mockUseGetAgentRoles.mockReturnValue({data: undefined});
      renderGuide();

      expect(screen.queryByText('Before you run the sample')).not.toBeInTheDocument();
    });

    it('says what acting on behalf of a user still needs, and the way to settle each', async () => {
      const user = userEvent.setup();
      renderGuide({oauth2Config: ownConfig});
      await user.click(screen.getByRole('button', {name: 'On behalf of a user'}));

      expect(mockUseGetFlowById).toHaveBeenLastCalledWith('flow-1', true);
      await user.click(within(check('Delegated mode is off')).getByRole('button', {name: 'Turn on in Advanced'}));
      const redirect = check('Redirect URI not added');
      expect(redirect).toHaveTextContent('The sample listens on http://localhost:6274/callback.');
      await user.click(within(redirect).getByRole('button', {name: 'Add in Advanced'}));
      expect(onGoToAdvanced).toHaveBeenCalledTimes(2);
      const consent = check('Sign-in flow has no consent step');
      expect(consent).toHaveTextContent('Basic Login Flow. The user must be asked to allow the agent.');
      await user.click(within(consent).getByRole('button', {name: 'Open Flows'}));
      expect(onGoToFlows).toHaveBeenCalledTimes(1);
      expect(screen.queryByText('PKCE is required')).not.toBeInTheDocument();
      expect(screen.queryByText('No role assigned to this agent')).not.toBeInTheDocument();
    });

    it('says when acting on behalf of a user has all it needs', async () => {
      const user = userEvent.setup();
      mockUseGetFlowById.mockReturnValue({
        data: {
          name: 'Enrollment Sign-in',
          nodes: [{id: 'consent', type: 'TASK_EXECUTION', executor: {name: 'ConsentExecutor'}}],
        },
      });
      renderGuide({oauth2Config: delegatedConfig, isDelegated: true});
      await user.click(screen.getByRole('button', {name: 'On behalf of a user'}));

      ['Delegated mode is on', 'Redirect URI added', 'Sign-in flow asks for consent', 'PKCE is required'].forEach(
        (title) => expect(within(check(title)).getByRole('img', {name: 'Ready'})).toBeInTheDocument(),
      );
      expect(screen.queryByRole('img', {name: 'Needs attention'})).not.toBeInTheDocument();
    });

    it('says when a delegated agent lacks the grant the sample signs the user in with', async () => {
      const user = userEvent.setup();
      renderGuide({
        oauth2Config: {grantTypes: ['urn:openid:params:grant-type:ciba'], responseTypes: []},
        isDelegated: true,
      });
      await user.click(screen.getByRole('button', {name: 'On behalf of a user'}));

      const grant = check('The authorization code grant is off');
      expect(within(grant).getByRole('img', {name: 'Needs attention'})).toBeInTheDocument();
      expect(screen.queryByText('Delegated mode is on')).not.toBeInTheDocument();
      await user.click(within(grant).getByRole('button', {name: 'Turn on in Advanced'}));
      expect(onGoToAdvanced).toHaveBeenCalledTimes(1);
    });

    it('offers no way to a tab the page does not have', async () => {
      const user = userEvent.setup();
      renderGuide({oauth2Config: ownConfig, hasTabs: false});
      expect(screen.queryByRole('button', {name: 'Assign in Access'})).not.toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'On behalf of a user'}));

      expect(screen.getByText('Delegated mode is off')).toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Turn on in Advanced'})).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Open Flows'})).not.toBeInTheDocument();
    });
  });

  it('points to the docs site when its page does not mark the choices', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      text: () => Promise.resolve('## Register an Agent\n\nAdd it.\n\n## Run the Agent\n\nRun it.'),
    });
    renderGuide();

    expect(await screen.findByText('Follow the step-by-step guide on the docs site.')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Read the full guide'})).toHaveAttribute('href', docsUrl);
  });
});
