// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ProjectScopeProvider} from '@thunderid/contexts';
import {render, screen, waitFor, userEvent, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {userEvent as browserUser} from 'vitest/browser';
import AgentAddFormPage from '../AgentAddFormPage';

const h = vi.hoisted(() => ({
  http: {request: vi.fn<(config: unknown) => Promise<{data: unknown}>>()},
  navigate: vi.fn(),
  created: {current: {id: 'agent-1', name: 'Calendar Agent'} as Record<string, unknown>},
  // The signed-in user as the SDK holds them: the ID token's claims, which carry no ID, merged with
  // the profile /users/me returns, which does, when the caller is a user here.
  signedIn: {current: {id: 'user-1', username: 'alice'} as Record<string, unknown> | null},
}));

// Translations are shared across the suite, so keys are asserted rather than English copy.
vi.mock('react-i18next', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useTranslation: () => ({t: (key: string) => key}),
  };
});

vi.mock('react-router', async () => ({
  ...(await vi.importActual<typeof import('react-router')>('react-router')),
  useNavigate: () => h.navigate,
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: h.http, user: h.signedIn.current}),
}));

vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  OrganizationUnitTreePicker: ({
    onChange,
    rootOuId = undefined,
    helperText = undefined,
  }: {
    onChange: (ouId: string) => void;
    rootOuId?: string;
    helperText?: string;
  }) => (
    <>
      <button type="button" data-testid="ou-picker" data-root={rootOuId} onClick={() => onChange('ou-child')}>
        pick ou
      </button>
      {helperText && <span>{helperText}</span>}
    </>
  ),
}));

interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

const agentTypes = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  types: [{id: 'type-1', handle: 'default', displayName: 'Default', ouId: 'ou-root'}],
};

const users = {
  totalResults: 1,
  startIndex: 1,
  count: 1,
  users: [{id: 'user-1', ouId: 'ou-root', type: 'person', display: 'alice'}],
};

const defaultType = {
  id: 'type-1',
  handle: 'default',
  displayName: 'Default',
  ouId: 'ou-root',
  schema: {
    name: {type: 'string', required: true},
    llmModel: {type: 'string', required: true},
    environment: {type: 'string'},
  },
};

const ISSUED_CREDENTIALS = {
  type: 'oauth2',
  config: {grantTypes: ['client_credentials'], responseTypes: [], tokenEndpointAuthMethod: 'client_secret_basic'},
};

// A read of one user, as against the list of users.
function lookedUpUser(url: string): string | undefined {
  return /\/users\/([^/?]+)/.exec(url)?.[1];
}

// Answers a read of one user as the server does: the user when they exist here, and 404 otherwise.
function answerUserLookup(userId: string): Promise<{data: unknown}> {
  const found = users.users.find((candidate) => candidate.id === userId);
  return found
    ? Promise.resolve({data: found})
    : Promise.reject(Object.assign(new Error('not found'), {response: {status: 404}}));
}

function requests(): RecordedRequest[] {
  return h.http.request.mock.calls.map(([config]) => config as RecordedRequest);
}

// The agent type select comes first in the form and the owner select second.
async function chooseOption(user: ReturnType<typeof userEvent.setup>, index: number, label: string): Promise<void> {
  await user.click((await screen.findAllByRole('combobox'))[index]);
  await user.click(within(await screen.findByRole('listbox')).getByText(label));
}

// The first step: the agent type and the organization unit.
async function fillSetup(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await chooseOption(user, 0, 'Default');
  await user.click(await screen.findByTestId('ou-picker'));
}

// The first step's Continue turns on once the type's schema has loaded, and opens the attributes step.
async function continueToAttributes(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  const next = await screen.findByRole('button', {name: 'common:actions.continue'});
  await waitFor(() => expect(next).toBeEnabled());
  await user.click(next);
}

async function fillRequired(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await fillSetup(user);
  await continueToAttributes(user);
  await user.type(await screen.findByRole('textbox', {name: /^name/}), 'Calendar Agent');
}

// The second step's required attributes, once the first step is done.
async function fillAttributes(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await continueToAttributes(user);
  await user.type(await screen.findByRole('textbox', {name: /^name/}), 'Calendar Agent');
  await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
}

describe('AgentAddFormPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    h.created.current = {id: 'agent-1', name: 'Calendar Agent'};
    h.signedIn.current = {id: 'user-1', username: 'alice'};
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST' && url.endsWith('/agents')) {
        return Promise.resolve({data: h.created.current});
      }
      const userId = lookedUpUser(url);
      if (userId) return answerUserLookup(userId);
      if (url.includes('/users')) {
        return Promise.resolve({data: users});
      }
      if (url.includes('/agent-types/type-1')) {
        return Promise.resolve({data: defaultType});
      }
      if (url.includes('/agent-types')) {
        return Promise.resolve({data: agentTypes});
      }
      return Promise.reject(new Error(`unexpected request: ${url}`));
    });
  });

  it('posts the description, type, organization unit and schema attributes, the name among them, to /agents', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    expect(screen.getByTestId('ou-picker')).toHaveAttribute('data-root', 'ou-root');
    await user.type(screen.getByRole('textbox', {name: 'agents:addForm.description'}), 'Exposes calendar tools');
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().some((request) => request.method === 'POST')).toBe(true);
    });

    const post = requests().find((request) => request.method === 'POST');
    expect(post?.url).toMatch(/\/agents$/);
    // The optional attribute left empty is omitted; OAuth client credentials are asked for by default,
    // naming only the grant and the authentication so the API generates the client ID and secret.
    expect(post?.data).toEqual({
      description: 'Exposes calendar tools',
      ouId: 'ou-child',
      type: 'default',
      attributes: {name: 'Calendar Agent', llmModel: 'claude'},
      inboundAuthConfig: [ISSUED_CREDENTIALS],
    });

    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });

    // No flow is involved anywhere along the way.
    expect(requests().some((request) => request.url.includes('/flow/execute'))).toBe(false);
    expect(requests().some((request) => request.url.includes('/server-config/flow'))).toBe(false);
  });

  it('creates the agent in the selected project, with no organization unit to pick', async () => {
    const user = userEvent.setup();
    render(
      <ProjectScopeProvider project={{id: 'project-1'}}>
        <AgentAddFormPage />
      </ProjectScopeProvider>,
    );

    await chooseOption(user, 0, 'Default');
    expect(screen.queryByTestId('ou-picker')).not.toBeInTheDocument();
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().some((request) => request.method === 'POST')).toBe(true);
    });
    const post = requests().find((request) => request.method === 'POST');
    expect(post?.data).toEqual({
      projectId: 'project-1',
      type: 'default',
      attributes: {name: 'Calendar Agent', llmModel: 'claude'},
      inboundAuthConfig: [ISSUED_CREDENTIALS],
    });
  });

  it('posts the chosen owner, and none when no owner is chosen', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await chooseOption(user, 1, 'alice');
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toMatchObject({
        owner: {id: 'user-1', idType: 'uuid', isLocal: true},
      });
    });
  });

  it('posts an external owner from the type and ID entered', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await user.click(screen.getByRole('radio', {name: 'agents:owner.kind.external'}));
    await user.type(screen.getByRole('textbox', {name: 'agents:owner.idType.email'}), 'jane@example.com');
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() =>
      expect(posted()).toMatchObject({owner: {id: 'jane@example.com', idType: 'email', isLocal: false}}),
    );
  });

  it('lets a signed-in user found nowhere here create the agent for an external owner with a valid ID', async () => {
    h.signedIn.current = {id: 'operator-elsewhere', username: 'admin'};
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    const next = screen.getByRole('button', {name: 'common:actions.continue'});
    await waitFor(() => expect(next).toBeDisabled());

    await user.click(screen.getByRole('radio', {name: 'agents:owner.kind.external'}));
    const externalId = screen.getByRole('textbox', {name: 'agents:owner.idType.email'});
    await user.type(externalId, 'jane');
    expect(next).toBeDisabled();
    await user.type(externalId, '@example.com');
    expect(next).toBeEnabled();
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() =>
      expect(posted()).toMatchObject({owner: {id: 'jane@example.com', idType: 'email', isLocal: false}}),
    );
  });

  it('posts the chosen user again once Internal is chosen after External', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await chooseOption(user, 1, 'alice');
    await user.click(screen.getByRole('radio', {name: 'agents:owner.kind.external'}));
    await user.click(screen.getByRole('radio', {name: 'agents:owner.kind.internal'}));
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => expect(posted()).toMatchObject({owner: {id: 'user-1', idType: 'uuid', isLocal: true}}));
  });

  // Answers the request for the list of users with the given outcome and everything else as the other
  // tests do.
  function answerUsersWith(outcome: () => Promise<{data: unknown}>): void {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    h.http.request.mockImplementation((config: unknown) => {
      const {url} = config as RecordedRequest;
      return url.includes('/users') && !lookedUpUser(url) ? outcome() : base(config);
    });
  }

  // Answers a read of one user with the given outcome and everything else as the other tests do.
  function answerLookupWith(outcome: () => Promise<{data: unknown}>): void {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    h.http.request.mockImplementation((config: unknown) =>
      lookedUpUser((config as RecordedRequest).url) ? outcome() : base(config),
    );
  }

  function posted(): unknown {
    return requests().find((request) => request.method === 'POST')?.data;
  }

  it('offers the signed-in user, found here, as the owner it defaults to, and posts none', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    expect(await screen.findByText('agents:addForm.ownerHint')).toBeInTheDocument();
    expect(screen.getByText('agents:addForm.ownerDefault')).toBeInTheDocument();
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => expect(posted()).toBeDefined());
    expect(posted()).not.toHaveProperty('owner');
    expect(requests().some((request) => lookedUpUser(request.url) === 'user-1')).toBe(true);
  });

  it.each([
    ['carries only the claims of their ID token', {username: 'admin'}],
    ['names no user at all', null],
  ])('does not offer a signed-in user whose session %s, and asks for an owner before creating', async (_, signedIn) => {
    h.signedIn.current = signedIn;
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    expect(await screen.findByText('agents:addForm.ownerRequiredHint')).toBeInTheDocument();
    const next = screen.getByRole('button', {name: 'common:actions.continue'});
    expect(next).toBeDisabled();

    await user.click(screen.getAllByRole('combobox')[1]);
    const listbox = await screen.findByRole('listbox');
    expect(within(listbox).getByText('agents:addForm.selectOwner')).toBeInTheDocument();
    expect(within(listbox).queryByText('agents:addForm.ownerDefault')).not.toBeInTheDocument();
    await user.click(within(listbox).getByText('alice'));
    expect(next).toBeEnabled();
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => expect(posted()).toMatchObject({owner: {id: 'user-1', idType: 'uuid', isLocal: true}}));
    expect(requests().some((request) => lookedUpUser(request.url))).toBe(false);
  });

  it('does not offer a signed-in user this deployment does not find', async () => {
    h.signedIn.current = {id: 'operator-elsewhere', username: 'admin'};
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);

    expect(await screen.findByText('agents:addForm.ownerRequiredHint')).toBeInTheDocument();
    expect(screen.getByText('agents:addForm.selectOwner')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeDisabled();
    expect(requests().some((request) => lookedUpUser(request.url) === 'operator-elsewhere')).toBe(true);
  });

  it('still offers the signed-in user when looking them up is refused for another reason', async () => {
    answerLookupWith(() => Promise.reject(Object.assign(new Error('forbidden'), {response: {status: 403}})));
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await waitFor(() => expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeEnabled());
    expect(screen.getByText('agents:addForm.ownerDefault')).toBeInTheDocument();
    expect(screen.getByText('agents:addForm.ownerHint')).toBeInTheDocument();
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => expect(posted()).toBeDefined());
    expect(posted()).not.toHaveProperty('owner');
  });

  it('creates nothing by default while the signed-in user is being looked up, but takes a chosen owner', async () => {
    answerLookupWith(() => new Promise(() => undefined));
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    const next = screen.getByRole('button', {name: 'common:actions.continue'});
    expect(next).toBeDisabled();
    expect(screen.queryByText('agents:addForm.ownerRequiredHint')).not.toBeInTheDocument();

    await chooseOption(user, 1, 'alice');
    expect(next).toBeEnabled();
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => expect(posted()).toMatchObject({owner: {id: 'user-1', idType: 'uuid', isLocal: true}}));
  });

  it('says there are no users to own the agent when one is needed and none exist', async () => {
    h.signedIn.current = {username: 'admin'};
    answerUsersWith(() => Promise.resolve({data: {totalResults: 0, startIndex: 1, count: 0, users: []}}));
    render(<AgentAddFormPage />);

    expect(await screen.findByText('agents:addForm.ownerNoUsers')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeDisabled();
  });

  it('tells two users with the same display name apart by their IDs', async () => {
    answerUsersWith(() =>
      Promise.resolve({
        data: {
          totalResults: 3,
          startIndex: 1,
          count: 3,
          users: [
            {id: 'user-1', ouId: 'ou-root', type: 'person', display: 'sam'},
            {id: 'user-2', ouId: 'ou-root', type: 'person', display: 'sam'},
            {id: 'user-3', ouId: 'ou-root', type: 'person', display: 'alice'},
          ],
        },
      }),
    );
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await user.click((await screen.findAllByRole('combobox'))[1]);
    const listbox = await screen.findByRole('listbox');

    expect(within(listbox).getByText('sam (user-1)')).toBeInTheDocument();
    expect(within(listbox).getByText('sam (user-2)')).toBeInTheDocument();
    expect(within(listbox).getByText('alice')).toBeInTheDocument();
  });

  it('says so when the users to choose an owner from cannot be loaded, and offers a retry', async () => {
    let attempts = 0;
    answerUsersWith(() => {
      attempts += 1;
      return attempts === 1 ? Promise.reject(new Error('network down')) : Promise.resolve({data: users});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await user.click(await screen.findByRole('button', {name: 'common:actions.refresh'}));

    // Only the agent type select is left until the owners load, then the owner select is back.
    await waitFor(() => {
      expect(screen.getAllByRole('combobox')).toHaveLength(2);
    });
    expect(attempts).toBeGreaterThan(1);
  });

  it('says when only the first users are offered as owners', async () => {
    answerUsersWith(() => Promise.resolve({data: {...users, totalResults: 250}}));
    render(<AgentAddFormPage />);

    expect(await screen.findByText('agents:addForm.ownerTruncated')).toBeInTheDocument();
  });

  it('shows the issued client secret once, then opens the agent', async () => {
    h.created.current = {
      id: 'agent-1',
      inboundAuthConfig: [{type: 'oauth2', config: {clientId: 'agent-client', clientSecret: 's3cret-value'}}],
    };
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('agents:addForm.secretIssued.title')).toBeInTheDocument();
    expect(h.navigate).not.toHaveBeenCalled();

    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });
  });

  it('creates the agent without OAuth credentials when switched off, and opens it at once', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await user.click(screen.getByRole('switch', {name: 'agents:addForm.issueCredentials'}));
    await fillAttributes(user);
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(h.navigate).toHaveBeenCalledWith('/agents/agent-1');
    });
    const post = requests().find((request) => request.method === 'POST');
    expect(post?.data).not.toHaveProperty('inboundAuthConfig');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('omits the description when none is given', async () => {
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST') return Promise.resolve({data: {id: 'agent-2'}});
      const userId = lookedUpUser(url);
      if (userId) return answerUserLookup(userId);
      if (url.includes('/agent-types/type-1')) {
        return Promise.resolve({data: {...defaultType, schema: {name: {type: 'string'}}}});
      }
      return Promise.resolve({data: agentTypes});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    const submit = screen.getByRole('button', {name: 'agents:addForm.submit'});
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(submit);

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toEqual({
        ouId: 'ou-child',
        type: 'default',
        attributes: {name: 'Calendar Agent'},
        inboundAuthConfig: [ISSUED_CREDENTIALS],
      });
    });
  });

  it('asks for a full page of types and posts nothing a previously chosen type was given', async () => {
    const twoTypes = {
      ...agentTypes,
      totalResults: 2,
      count: 2,
      types: [...agentTypes.types, {id: 'type-2', handle: 'assistant', displayName: 'Assistant', ouId: 'ou-root'}],
    };
    const assistantType = {...defaultType, id: 'type-2', handle: 'assistant', displayName: 'Assistant'};
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST') return Promise.resolve({data: {id: 'agent-3'}});
      const userId = lookedUpUser(url);
      if (userId) return answerUserLookup(userId);
      if (url.includes('/agent-types/type-1')) return Promise.resolve({data: defaultType});
      if (url.includes('/agent-types/type-2')) return Promise.resolve({data: assistantType});
      return Promise.resolve({data: twoTypes});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    const choose = async (type: string): Promise<void> => {
      await chooseOption(user, 0, type);
      await user.click(await screen.findByTestId('ou-picker'));
      await continueToAttributes(user);
    };
    const back = (): Promise<void> => user.click(screen.getByRole('button', {name: 'common:actions.back'}));
    await screen.findAllByRole('combobox');
    await choose('Default');
    await screen.findByRole('textbox', {name: /environment/});
    await back();
    await choose('Assistant');
    await user.type(await screen.findByRole('textbox', {name: /environment/}), 'staging');
    await back();
    // Back to a type whose schema is already loaded, so its fields stay mounted across the change.
    await choose('Default');
    await waitFor(() => expect(screen.getByRole('textbox', {name: /environment/})).toHaveValue(''));
    await user.type(screen.getByRole('textbox', {name: /^name/}), 'Helper');
    await user.type(screen.getByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => {
      expect(requests().find((request) => request.method === 'POST')?.data).toEqual({
        ouId: 'ou-child',
        type: 'default',
        attributes: {name: 'Helper', llmModel: 'claude'},
        inboundAuthConfig: [ISSUED_CREDENTIALS],
      });
    });
    expect(requests().some((request) => /\/agent-types\?.*limit=100/.test(request.url))).toBe(true);
  });

  // An agent must sit in its type's organization unit or one of its descendants, so an organization unit picked
  // under one type is never posted with another.
  it('should clear the organization unit and scope the picker to the new type when the type changes', async () => {
    const twoTypes = {
      ...agentTypes,
      totalResults: 2,
      count: 2,
      types: [...agentTypes.types, {id: 'type-2', handle: 'assistant', displayName: 'Assistant', ouId: 'ou-other'}],
    };
    const assistantType = {
      ...defaultType,
      id: 'type-2',
      handle: 'assistant',
      displayName: 'Assistant',
      ouId: 'ou-other',
    };
    h.http.request.mockImplementation((config: unknown) => {
      const {url} = config as RecordedRequest;
      if (url.includes('/users')) return Promise.resolve({data: users});
      if (url.includes('/agent-types/type-1')) return Promise.resolve({data: defaultType});
      if (url.includes('/agent-types/type-2')) return Promise.resolve({data: assistantType});
      return Promise.resolve({data: twoTypes});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    const next = screen.getByRole('button', {name: 'common:actions.continue'});
    await waitFor(() => expect(next).toBeEnabled());

    await chooseOption(user, 0, 'Assistant');

    expect(await screen.findByTestId('ou-picker')).toHaveAttribute('data-root', 'ou-other');
    expect(next).toBeDisabled();
  });

  it('should explain which organization units an agent of the chosen type can go in', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await chooseOption(user, 0, 'Default');

    expect(await screen.findByText('agents:addForm.organizationUnitHint')).toBeInTheDocument();
  });

  it('should explain an organization unit outside the agent type when the server refuses it', async () => {
    h.http.request.mockImplementation((config: unknown) => {
      const {url, method} = config as RecordedRequest;
      if (method === 'POST') {
        return Promise.reject(Object.assign(new Error('Bad request'), {response: {data: {code: 'AGT-1053'}}}));
      }
      if (url.includes('/users')) return Promise.resolve({data: users});
      if (url.includes('/agent-types/type-1')) return Promise.resolve({data: defaultType});
      return Promise.resolve({data: agentTypes});
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await user.type(await screen.findByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    expect(await screen.findByText('agents:errors.AGT-1053')).toBeInTheDocument();
    expect(h.navigate).not.toHaveBeenCalled();
  });

  it('keeps Continue disabled until a type and an organization unit are given', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    const next = await screen.findByRole('button', {name: 'common:actions.continue'});
    expect(next).toBeDisabled();
    expect(screen.queryByRole('button', {name: 'agents:addForm.submit'})).not.toBeInTheDocument();

    await chooseOption(user, 0, 'Default');
    expect(next).toBeDisabled();
    await user.click(await screen.findByTestId('ou-picker'));
    await waitFor(() => expect(next).toBeEnabled());
  });

  it('asks for the type and placement first, and the schema attributes on the next step', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    // The type's schema has loaded, but its attributes wait for the next step.
    await waitFor(() => expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeEnabled());
    expect(screen.queryByRole('textbox', {name: /^name/})).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', {name: /llmModel/})).not.toBeInTheDocument();
    expect(screen.queryByText('agents:addForm.attributesStep.breadcrumb')).not.toBeInTheDocument();

    await continueToAttributes(user);

    expect(screen.getByRole('heading', {name: 'agents:addForm.attributesStep.title'})).toBeInTheDocument();
    expect(screen.getByText('agents:addForm.attributesStep.breadcrumb')).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /^name/})).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /llmModel/})).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /environment/})).toBeInTheDocument();
    // What the first step asked for is out of view.
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', {name: 'agents:addForm.description'})).not.toBeInTheDocument();
    expect(screen.queryByRole('switch', {name: 'agents:addForm.issueCredentials'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'common:actions.continue'})).not.toBeInTheDocument();
  });

  it('keeps what was entered on both steps when going back and forth', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await user.click(screen.getByRole('radio', {name: 'agents:owner.kind.external'}));
    await user.type(screen.getByRole('textbox', {name: 'agents:owner.idType.email'}), 'jane@example.com');
    await fillAttributes(user);

    await user.click(screen.getByRole('button', {name: 'common:actions.back'}));
    expect(screen.getByRole('radio', {name: 'agents:owner.kind.external'})).toBeChecked();
    expect(screen.getByRole('textbox', {name: 'agents:owner.idType.email'})).toHaveValue('jane@example.com');

    await user.click(screen.getByRole('button', {name: 'common:actions.continue'}));
    expect(screen.getByRole('textbox', {name: /^name/})).toHaveValue('Calendar Agent');
    expect(screen.getByRole('textbox', {name: /llmModel/})).toHaveValue('claude');
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() =>
      expect(posted()).toMatchObject({
        attributes: {name: 'Calendar Agent', llmModel: 'claude'},
        owner: {id: 'jane@example.com', idType: 'email', isLocal: false},
      }),
    );
  });

  it('keeps the way back closed while the agent is being created', async () => {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    h.http.request.mockImplementation((config: unknown) =>
      (config as RecordedRequest).method === 'POST' ? new Promise(() => undefined) : base(config),
    );
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await fillAttributes(user);
    expect(screen.getByText('agents:addAgent', {selector: 'button, a, [role="button"]'})).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await waitFor(() => expect(screen.getByRole('button', {name: 'common:actions.back'})).toBeDisabled());
    expect(screen.queryByText('agents:addAgent', {selector: 'button, a, [role="button"]'})).not.toBeInTheDocument();
  });

  it('does not create the agent when Continue is clicked, even with every attribute filled', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await continueToAttributes(user);
    expect(screen.queryByText(/is required/)).not.toBeInTheDocument();
    await user.type(screen.getByRole('textbox', {name: /^name/}), 'Calendar Agent');
    await user.type(screen.getByRole('textbox', {name: /llmModel/}), 'claude');
    await user.click(screen.getByRole('button', {name: 'common:actions.back'}));
    // A real click, not a dispatched one: the browser's default action for it then runs after
    // React has re-rendered, so it acts on whatever button sits in Continue's place by then.
    await browserUser.click(screen.getByRole('button', {name: 'common:actions.continue'}));

    expect(await screen.findByRole('button', {name: 'agents:addForm.submit'})).toBeEnabled();
    await new Promise((resolve) => {
      setTimeout(resolve, 200);
    });
    expect(requests().some((request) => request.method === 'POST')).toBe(false);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it("asks for the attributes in the agent type's order: display attribute, required, optional, credentials", async () => {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    h.http.request.mockImplementation((config: unknown) =>
      (config as RecordedRequest).url.includes('/agent-types/type-1')
        ? Promise.resolve({
            data: {
              ...defaultType,
              systemAttributes: {display: 'label'},
              schema: {
                model: {type: 'string'},
                environment: {type: 'string', required: true},
                apiKey: {type: 'string', required: true, credential: true},
                function: {type: 'string'},
                label: {type: 'string', required: true},
              },
            },
          })
        : base(config),
    );
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await continueToAttributes(user);

    // The credential is a password input, with no textbox role, so every input of the form is read.
    const fields = document.querySelectorAll<HTMLInputElement>('#agent-attributes-form input[id]');
    expect(Array.from(fields, (element) => element.id)).toEqual([
      'label',
      'environment',
      'model',
      'function',
      'apiKey',
    ]);
  });

  it('returns to the first step from its breadcrumb', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillSetup(user);
    await continueToAttributes(user);
    await user.click(screen.getByText('agents:addAgent', {selector: 'button, a, [role="button"]'}));

    expect(await screen.findByRole('button', {name: 'common:actions.continue'})).toBeEnabled();
    expect(screen.queryByRole('textbox', {name: /^name/})).not.toBeInTheDocument();
  });

  it('does not post when a required attribute is missing', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await fillRequired(user);
    await screen.findByRole('textbox', {name: /llmModel/});
    await user.click(screen.getByRole('button', {name: 'agents:addForm.submit'}));

    await screen.findByText(/llmModel is required/);
    expect(requests().some((request) => request.method === 'POST')).toBe(false);
  });

  it('says so when the agent types cannot be loaded, and offers a retry', async () => {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    let attempts = 0;
    h.http.request.mockImplementation((config: unknown) => {
      if ((config as RecordedRequest).url.includes('/agent-types?')) {
        attempts += 1;
        if (attempts === 1) return Promise.reject(new Error('network down'));
      }
      return base(config);
    });
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    // The failure takes the agent type select's place, and is not read as there being no agent types.
    expect(await screen.findByText('agents:addForm.agentTypesError')).toBeInTheDocument();
    expect(screen.queryByText('agents:addForm.noAgentTypes')).not.toBeInTheDocument();
    expect(screen.getAllByRole('combobox')).toHaveLength(1);

    await user.click(screen.getByRole('button', {name: 'common:actions.refresh'}));

    await waitFor(() => {
      expect(screen.getAllByRole('combobox')).toHaveLength(2);
    });
    expect(attempts).toBe(2);
  });

  it('says so when the attributes of the chosen agent type cannot be loaded, and keeps Continue off', async () => {
    const base = h.http.request.getMockImplementation() as (config: unknown) => Promise<{data: unknown}>;
    h.http.request.mockImplementation((config: unknown) =>
      (config as RecordedRequest).url.includes('/agent-types/type-1')
        ? Promise.reject(new Error('network down'))
        : base(config),
    );
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await chooseOption(user, 0, 'Default');

    expect(await screen.findByText('agents:addForm.agentTypeSchemaError')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', {name: /^name/})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeDisabled();
  });

  it('says so when no agent types exist', async () => {
    h.http.request.mockResolvedValue({data: {totalResults: 0, startIndex: 1, count: 0, types: []}});
    render(<AgentAddFormPage />);

    expect(await screen.findByText('agents:addForm.noAgentTypes')).toBeInTheDocument();
  });

  it('returns to the agent list on close', async () => {
    const user = userEvent.setup();
    render(<AgentAddFormPage />);

    await user.click(await screen.findByRole('button', {name: 'actions.close'}));

    expect(h.navigate).toHaveBeenCalledWith('/agents');
  });
});
