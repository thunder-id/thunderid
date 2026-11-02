// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {fireEvent, render, screen, waitFor, within} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../../models/agent';
import EditTokensSettings from '../EditTokensSettings';

// Stable references, so the user-type schema fetch in the token settings does not re-fire on every
// render. Two user types: the agent allows person only, so employee_id is offered only when the
// attributes come from every user type. The default source reads the list through the mocked
// useGetUserTypes below; the every-user-type source goes through useGetUserTypeAttributes, which
// requests the list and each schema over http.
const {mockHttp, userTypesList} = vi.hoisted(() => {
  const list = {
    totalResults: 2,
    types: [
      {id: 'ut-1', handle: 'person', displayName: 'Person'},
      {id: 'ut-2', handle: 'employee', displayName: 'Employee'},
    ],
  };
  const schemas: Record<string, {handle: string; schema: Record<string, unknown>}> = {
    'ut-1': {handle: 'person', schema: {email: {type: 'string'}, given_name: {type: 'string'}}},
    'ut-2': {handle: 'employee', schema: {email: {type: 'string'}, employee_id: {type: 'string'}}},
  };
  return {
    mockHttp: {
      request: vi.fn(({url}: {url: string}) => {
        const id = /\/user-types\/([^?]+)/.exec(url)?.[1];
        return Promise.resolve({data: id ? {id, displayName: schemas[id].handle, ...schemas[id]} : list});
      }),
    },
    userTypesList: {data: list, isLoading: false},
  };
});

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: mockHttp}),
}));

vi.mock('@thunderid/configure-user-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-user-types')>()),
  useGetUserTypes: () => userTypesList,
}));

// JwtPreview renders the token payload through the Monaco editor; stub it to print the JSON.
vi.mock('@monaco-editor/react', () => ({
  default: ({value}: {value: string}) => <pre data-testid="jwt-preview">{value}</pre>,
}));

vi.mock('../AgentAccessTokenSection', () => ({
  default: () => <div data-testid="agent-access-token" />,
}));

const AUTHORIZATION_CODE = 'authorization_code';
const CIBA = 'urn:openid:params:grant-type:ciba';
const TOKEN_EXCHANGE = 'urn:ietf:params:oauth:grant-type:token-exchange';
const JWT_BEARER = 'urn:ietf:params:oauth:grant-type:jwt-bearer';

const tokenExchangeNote = 'Token exchange keeps only the selected attributes that the incoming token already carries.';
const jwtBearerNote = 'Tokens from a redeemed ID-JAG carry no user attributes.';

/** An agent OAuth config shaped like one loaded from the API, with every token block materialized. */
const configFor = (grants: string[]): OAuthAgentConfig => ({
  grantTypes: ['client_credentials', ...grants],
  responseTypes: [],
  token: {
    accessToken: {userConfig: {validityPeriod: 1800, attributes: ['email']}},
    idToken: {validityPeriod: 3600, userAttributes: ['email']},
    refreshToken: {validityPeriod: 86400},
  },
  scopeClaims: {profile: ['email']},
});

const agentFor = (config: OAuthAgentConfig): Agent => ({
  id: 'agent-1',
  ouId: 'ou-1',
  type: 'default',
  display: 'Test Agent',
  allowedUserTypes: ['person'],
  inboundAuthConfig: [{type: 'oauth2', config}],
});

const renderUserAudience = async (
  grants: string[],
  onFieldChange = vi.fn(),
): Promise<{config: OAuthAgentConfig; onFieldChange: ReturnType<typeof vi.fn>}> => {
  const user = userEvent.setup();
  const config = configFor(grants);
  render(
    <EditTokensSettings
      agent={agentFor(config)}
      editedAgent={{}}
      oauth2Config={config}
      onFieldChange={onFieldChange}
    />,
  );
  await user.click(screen.getByRole('tab', {name: 'User'}));
  await screen.findAllByText('Token validity');
  return {config, onFieldChange};
};

/** The chips offered by the access token attribute picker. */
const pickerAttributes = (): string[] =>
  within(screen.getByText('Add or remove attributes').parentElement!)
    .queryAllByRole('button')
    .map((chip) => chip.textContent ?? '');

const tabNames = (): string[] =>
  within(screen.getByRole('tabpanel'))
    .queryAllByRole('tab')
    .map((tab) => tab.textContent ?? '');

describe('EditTokensSettings User audience', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}')));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  // Both sign a user in through the agent, so both issue ID and refresh tokens from the profile.
  it.each([
    ['authorization code', [AUTHORIZATION_CODE]],
    ['CIBA only', [CIBA]],
  ])('should show every user token setting for an agent holding %s', async (_name, grants) => {
    await renderUserAudience(grants);

    expect(screen.getByText('Token attributes & response')).toBeInTheDocument();
    expect(tabNames()).toEqual(['Access token', 'ID token', 'Access token', 'ID token', 'Refresh token']);
    expect(screen.queryByText(tokenExchangeNote)).not.toBeInTheDocument();
    expect(screen.queryByText(jwtBearerNote)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Access token validity/)).not.toBeInTheDocument();
  });

  it('should show the access token attributes with the token exchange note for token exchange only', async () => {
    await renderUserAudience([TOKEN_EXCHANGE]);

    expect(screen.getByText('Access token attributes')).toBeInTheDocument();
    expect(screen.getByText(tokenExchangeNote)).toBeInTheDocument();
    expect(screen.queryByText(jwtBearerNote)).not.toBeInTheDocument();
    expect(screen.getByText('Add or remove attributes')).toBeInTheDocument();
    expect(screen.getByTestId('jwt-preview')).toBeInTheDocument();
    expect(screen.getByText(/A custom scope listed here/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Access token validity/)).toHaveValue(1800);
    expect(tabNames()).toEqual([]);
    expect(screen.queryByText('Response format')).not.toBeInTheDocument();
  });

  // The incoming token's user can be of any type, and allowed user types cannot be set without
  // authorization code, so token exchange offers every user type's attributes.
  it.each([
    ['token exchange only', [TOKEN_EXCHANGE]],
    ['token exchange and JWT bearer', [TOKEN_EXCHANGE, JWT_BEARER]],
  ])('should offer the attributes of every user type for %s', async (_name, grants) => {
    await renderUserAudience(grants);

    await waitFor(() => {
      expect(pickerAttributes()).toContain('employee_id');
    });
    expect(pickerAttributes()).toContain('given_name');
    expect(pickerAttributes().filter((attr) => attr === 'email')).toHaveLength(1);
  });

  it('should keep offering the allowed user types attributes for an agent with authorization code', async () => {
    await renderUserAudience([AUTHORIZATION_CODE, TOKEN_EXCHANGE]);

    await waitFor(() => {
      expect(pickerAttributes()).toContain('given_name');
    });
    expect(pickerAttributes()).not.toContain('employee_id');
    // Only the allowed type's schema is requested; every user type's list and schemas are not.
    expect(mockHttp.request).toHaveBeenCalledTimes(1);
  });

  it('should hide the attribute picker and show the ID-JAG note for JWT bearer only', async () => {
    await renderUserAudience([JWT_BEARER]);

    // No attributes to choose, so the card is titled for the access token itself.
    expect(screen.getByText('Access token')).toBeInTheDocument();
    expect(screen.getByText('The access token issued to this agent on behalf of a user.')).toBeInTheDocument();
    expect(screen.queryByText('Access token attributes')).not.toBeInTheDocument();
    expect(screen.getByText(jwtBearerNote)).toBeInTheDocument();
    expect(screen.queryByText(tokenExchangeNote)).not.toBeInTheDocument();
    expect(screen.queryByText('Add or remove attributes')).not.toBeInTheDocument();
    expect(screen.queryByTestId('jwt-preview')).not.toBeInTheDocument();
    expect(screen.getByText(/A custom scope listed here/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Access token validity/)).toHaveValue(1800);
    expect(tabNames()).toEqual([]);
  });

  it('should show the attribute picker with both notes for token exchange and JWT bearer', async () => {
    await renderUserAudience([TOKEN_EXCHANGE, JWT_BEARER]);

    // Each note is its own line in the one alert, not two sentences joined into one string.
    const teLine = screen.getByText(tokenExchangeNote);
    const jwtLine = screen.getByText(jwtBearerNote);
    expect(teLine).not.toBe(jwtLine);
    expect(teLine.closest('[role="alert"]')).toBe(jwtLine.closest('[role="alert"]'));
    expect(screen.getByText('Add or remove attributes')).toBeInTheDocument();
    expect(screen.getByLabelText(/Access token validity/)).toBeInTheDocument();
    expect(tabNames()).toEqual([]);
  });

  it('should keep every user token setting when a sign-in grant is held next to token exchange', async () => {
    await renderUserAudience([AUTHORIZATION_CODE, TOKEN_EXCHANGE, JWT_BEARER]);

    expect(screen.getByText('Token attributes & response')).toBeInTheDocument();
    expect(screen.queryByText(tokenExchangeNote)).not.toBeInTheDocument();
    expect(screen.queryByText(jwtBearerNote)).not.toBeInTheDocument();
    expect(tabNames()).toEqual(['Access token', 'ID token', 'Access token', 'ID token', 'Refresh token']);
  });

  it('should write the access token validity, and leave the hidden token settings alone, for token exchange only', async () => {
    const {config, onFieldChange} = await renderUserAudience([TOKEN_EXCHANGE]);

    fireEvent.change(screen.getByLabelText(/Access token validity/), {target: {value: '900'}});

    await waitFor(() => {
      expect(onFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.anything());
    });
    const inboundAuth = onFieldChange.mock.calls.find(
      ([field]) => field === 'inboundAuthConfig',
    )![1] as AgentInboundAuthConfig[];
    const written = inboundAuth[0].config!;
    expect(written.token?.accessToken?.userConfig?.validityPeriod).toBe(900);
    expect(written.token?.idToken).toEqual(config.token?.idToken);
    expect(written.token?.refreshToken).toEqual(config.token?.refreshToken);
  });
});
