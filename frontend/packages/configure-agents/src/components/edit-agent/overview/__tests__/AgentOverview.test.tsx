// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
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

vi.mock('../../attributes/AttributesSummarySection', () => ({
  default: ({variant}: {variant?: string}) => <div data-testid="attributes-summary-section">{variant}</div>,
}));

const baseAgent: Agent = {
  id: 'agent-123',
  ouId: 'ou-1',
  ouHandle: 'engineering',
  type: 'default',
  display: 'Enrollment Agent',
  clientId: 'client-abc',
  owner: 'user-1',
};

const delegatedOauth2Config: OAuthAgentConfig = {
  clientId: 'client-abc',
  grantTypes: ['authorization_code', 'client_credentials'],
  responseTypes: ['code'],
};

describe('AgentOverview', () => {
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
    expect(screen.getByText('On behalf of a user')).toBeInTheDocument();
    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  it('shows delegated mode as enabled and lists allowed user types when authorization_code is on', () => {
    render(<AgentOverview agent={{...baseAgent, allowedUserTypes: ['person']}} oauth2Config={delegatedOauth2Config} />);

    expect(screen.getByText('Allowed user types')).toBeInTheDocument();
    expect(screen.getByText('Person')).toBeInTheDocument();
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

    await user.click(screen.getByRole('button', {name: 'Edit in Advanced'}));

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

  it('requests navigation confirmation when the quickstart link is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentOverview agent={baseAgent} />);

    await user.click(screen.getByRole('button', {name: /Open quickstart/}));

    expect(
      screen.getByDisplayValue('https://thunderid.dev/docs/next/getting-started/connect-your-agent/langchain/'),
    ).toBeInTheDocument();
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

  it('shows the raw owner id when the owner is not among the fetched users', () => {
    mockUseGetUsers.mockReturnValue({data: {users: [{id: 'someone-else', display: 'Bob'}]}, isLoading: false});

    render(<AgentOverview agent={baseAgent} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
    expect(screen.queryByText('Bob')).not.toBeInTheDocument();
  });
});
