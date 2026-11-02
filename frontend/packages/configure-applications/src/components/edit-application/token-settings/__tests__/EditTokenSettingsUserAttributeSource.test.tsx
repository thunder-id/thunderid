// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {AggregatedUserTypeAttribute, UseGetUserTypeAttributesResult} from '@thunderid/configure-user-types';
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import EditTokenSettings from '../EditTokenSettings';

// Stable mock references, created via vi.hoisted so the hoisted vi.mock factories can use them.
// New identities on every render would re-fire the schema-fetch effect in a loop.
const {mockHttp, mockGetServerUrl, mockLogger, mockUseGetUserTypes, mockUseGetUserTypeAttributes, allTypes} =
  vi.hoisted(() => {
    const list = {
      data: {
        totalResults: 2,
        types: [
          {id: 'ut-person', handle: 'person', displayName: 'Person'},
          {id: 'ut-employee', handle: 'employee', displayName: 'Employee'},
        ],
      },
      isLoading: false,
    };
    return {
      mockHttp: {request: vi.fn()},
      mockGetServerUrl: vi.fn().mockReturnValue('https://api.example.com'),
      mockLogger: {error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn()},
      mockUseGetUserTypes: vi.fn<(...args: unknown[]) => unknown>(() => list),
      mockUseGetUserTypeAttributes: vi.fn<(options?: {enabled?: boolean}) => unknown>(),
      // What the every-user-type hook returns while enabled; each test sets it.
      allTypes: {current: undefined as unknown as UseGetUserTypeAttributesResult},
    };
  });

const disabledAttributes: UseGetUserTypeAttributesResult = {
  attributes: [],
  isLoading: false,
  error: null,
  refetch: vi.fn(),
};

vi.mock('@thunderid/configure-user-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-user-types')>()),
  useGetUserTypes: (...args: unknown[]) => mockUseGetUserTypes(...args),
  useGetUserTypeAttributes: (options?: {enabled?: boolean}) => mockUseGetUserTypeAttributes(options),
}));

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: mockHttp}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
    useRuntimeUrl: () => 'https://rt.example.com',
  };
});

vi.mock('@thunderid/logger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger')>();
  return {...actual, useLogger: () => mockLogger};
});

// JwtPreview renders through the Monaco editor; a stand-in keeps the tests independent of it.
vi.mock('../JwtPreview', () => ({
  default: () => <div data-testid="jwt-preview" />,
}));

const SCHEMAS: Record<string, Record<string, unknown>> = {
  'ut-person': {email: {type: 'email'}, given_name: {type: 'string'}, password: {type: 'string', credential: true}},
  'ut-employee': {email: {type: 'email'}, employee_id: {type: 'string'}},
};

const attribute = (name: string, credential = false): AggregatedUserTypeAttribute => ({
  attribute: name,
  credential,
  userTypes: ['Person'],
});

// Hoisted so their identities are stable across renders; a fresh allowedUserTypes array each render
// would re-fire the schema-fetch effect.
const allowingPerson = {id: 'app-1', name: 'App', allowedUserTypes: ['person']} as Application;
const allowingNone = {id: 'app-1', name: 'App', allowedUserTypes: []} as unknown as Application;

const oauth2Config: OAuth2Config = {
  grantTypes: ['client_credentials', 'urn:ietf:params:oauth:grant-type:token-exchange'],
  responseTypes: [],
  token: {
    accessToken: {userConfig: {validityPeriod: 3600, attributes: []}},
    idToken: {validityPeriod: 3600, userAttributes: []},
  },
  // No scopes, so the mapping lists no attributes of its own next to the access token picker.
  scopeClaims: {},
};

/** The chips offered by the access token attribute picker. */
const pickerAttributes = (): string[] =>
  within(screen.getByText('Add or remove attributes').parentElement!)
    .queryAllByRole('button')
    .map((chip) => chip.textContent ?? '');

const renderAllUserTypes = (application: Application = allowingNone) =>
  render(
    <EditTokenSettings
      application={application}
      oauth2Config={oauth2Config}
      onFieldChange={vi.fn()}
      accessTokenOnly
      userAttributeSource="allUserTypes"
    />,
  );

describe('EditTokenSettings userAttributeSource', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}')));
    mockHttp.request.mockImplementation(({url}: {url: string}) => {
      const id = url.split('/').pop()!;
      return Promise.resolve({data: {id, handle: id.replace('ut-', ''), displayName: id, schema: SCHEMAS[id] ?? {}}});
    });
    allTypes.current = {
      attributes: [attribute('email'), attribute('employee_id'), attribute('given_name'), attribute('password', true)],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    };
    mockUseGetUserTypeAttributes.mockImplementation((options) =>
      options?.enabled === false ? disabledAttributes : allTypes.current,
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it('should take the attributes from the allowed user types only by default, with the same requests as before', async () => {
    render(<EditTokenSettings application={allowingPerson} oauth2Config={oauth2Config} onFieldChange={vi.fn()} />);

    await waitFor(() => {
      expect(pickerAttributes()).toContain('given_name');
    });
    expect(pickerAttributes()).not.toContain('employee_id');
    // The list keeps its usual key (no params); only an enabled flag is passed.
    expect(mockUseGetUserTypes).toHaveBeenCalledWith(undefined, {enabled: true});
    expect(mockHttp.request).toHaveBeenCalledTimes(1);
    expect(mockUseGetUserTypeAttributes).toHaveBeenCalledWith({enabled: false});
  });

  it('should offer every user type attribute from the shared hook with allUserTypes, without credentials', () => {
    // An allowed user type is set, so the default source would request its schema if it ran.
    renderAllUserTypes(allowingPerson);

    const attributes = pickerAttributes();
    expect(attributes).toEqual(expect.arrayContaining(['email', 'employee_id', 'given_name', 'groups']));
    expect(attributes).not.toContain('password');
    expect(mockUseGetUserTypeAttributes).toHaveBeenCalledWith({enabled: true});
    // The default source's list is disabled and its schema requests are not made.
    expect(mockUseGetUserTypes).toHaveBeenCalledWith(undefined, {enabled: false});
    expect(mockHttp.request).not.toHaveBeenCalled();
    expect(screen.queryByText(/Configure allowed user types/)).not.toBeInTheDocument();
  });

  it('should show the loading text, and no hint, while the user type attributes load', () => {
    allTypes.current = {...allTypes.current, attributes: [], isLoading: true};
    renderAllUserTypes();

    expect(screen.getByText('Loading user attributes...')).toBeInTheDocument();
    expect(screen.queryByText(/No user type defines any attributes yet/)).not.toBeInTheDocument();
  });

  it('should show the read failure with a retry, in place of the hint', async () => {
    const user = userEvent.setup();
    allTypes.current = {...allTypes.current, attributes: [], error: new Error('boom')};
    renderAllUserTypes();

    const notice = screen.getByRole('alert');
    expect(notice).toHaveTextContent('Failed to load the user attributes.');
    expect(notice).not.toHaveTextContent('boom');
    expect(screen.queryByText(/No user type defines any attributes yet/)).not.toBeInTheDocument();

    await user.click(within(notice).getByRole('button', {name: 'Refresh'}));
    expect(allTypes.current.refetch).toHaveBeenCalledTimes(1);
  });

  it('should resolve a user type error code through the user types catalog', () => {
    const deleted = Object.assign(new Error('Request failed'), {response: {data: {code: 'USRS-1002'}}});
    allTypes.current = {...allTypes.current, attributes: [], error: deleted};
    renderAllUserTypes();

    expect(screen.getByRole('alert')).toHaveTextContent(
      'This user type no longer exists. It may have already been deleted.',
    );
  });

  it('should keep offering the attributes that loaded alongside a partial failure', () => {
    allTypes.current = {...allTypes.current, error: new Error('one schema failed')};
    renderAllUserTypes();

    expect(pickerAttributes()).toContain('employee_id');
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to load the user attributes.');
  });

  it('should say no user type defines attributes when everything loaded and there are none', () => {
    allTypes.current = {...allTypes.current, attributes: []};
    renderAllUserTypes();

    expect(screen.getByText(/No user type defines any attributes yet/)).toBeInTheDocument();
    expect(screen.queryByText(/Configure allowed user types/)).not.toBeInTheDocument();
  });

  it('should keep the allowed user types hint by default when there are none', async () => {
    render(<EditTokenSettings application={allowingNone} oauth2Config={oauth2Config} onFieldChange={vi.fn()} />);

    expect(await screen.findByText(/Configure allowed user types for this application/)).toBeInTheDocument();
    expect(screen.queryByText(/No user type defines any attributes yet/)).not.toBeInTheDocument();
  });

  it('should stop offering every user type once the source flips back to the allowed user types', async () => {
    const onFieldChange = vi.fn();
    const {rerender} = render(
      <EditTokenSettings
        application={allowingNone}
        oauth2Config={oauth2Config}
        onFieldChange={onFieldChange}
        accessTokenOnly
        userAttributeSource="allUserTypes"
      />,
    );
    expect(pickerAttributes()).toContain('employee_id');

    rerender(
      <EditTokenSettings application={allowingNone} oauth2Config={oauth2Config} onFieldChange={onFieldChange} />,
    );

    expect(await screen.findByText(/Configure allowed user types for this application/)).toBeInTheDocument();
    expect(pickerAttributes()).not.toContain('employee_id');
  });

  it('should not let an older schema response overwrite a newer one', async () => {
    let resolvePerson: (value: unknown) => void = () => undefined;
    mockHttp.request.mockImplementation(({url}: {url: string}) => {
      const id = url.split('/').pop()!;
      const response = {data: {id, handle: id.replace('ut-', ''), displayName: id, schema: SCHEMAS[id]}};
      // The person schema answers late, after the allowed user types have moved on to employee.
      return id === 'ut-person'
        ? new Promise((resolve) => {
            resolvePerson = () => resolve(response);
          })
        : Promise.resolve(response);
    });
    const allowingEmployee = {id: 'app-1', name: 'App', allowedUserTypes: ['employee']} as Application;
    const {rerender} = render(
      <EditTokenSettings application={allowingPerson} oauth2Config={oauth2Config} onFieldChange={vi.fn()} />,
    );

    rerender(<EditTokenSettings application={allowingEmployee} oauth2Config={oauth2Config} onFieldChange={vi.fn()} />);
    await waitFor(() => {
      expect(pickerAttributes()).toContain('employee_id');
    });
    resolvePerson(undefined);

    await new Promise((resolve) => {
      setTimeout(resolve, 50);
    });
    expect(pickerAttributes()).toContain('employee_id');
    expect(pickerAttributes()).not.toContain('given_name');
  });
});
