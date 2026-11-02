// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {fireEvent, render, screen, waitFor, within} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import EditTokenSettings from '../EditTokenSettings';

// Stable mock references, created via vi.hoisted so the hoisted vi.mock factories can use them.
// New identities on every render would re-fire the schema-fetch effect in a loop.
const {mockHttp, mockGetServerUrl, mockLogger} = vi.hoisted(() => ({
  mockHttp: {
    request: vi.fn().mockResolvedValue({
      data: {id: 'schema-1', handle: 'default', displayName: 'Default', schema: {email: {type: 'string'}}},
    }),
  },
  mockGetServerUrl: vi.fn().mockReturnValue('https://api.example.com'),
  mockLogger: {error: vi.fn(), info: vi.fn(), debug: vi.fn()},
}));

vi.mock('@thunderid/configure-user-types', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-user-types')>()),
  useGetUserTypes: () => ({
    data: {types: [{id: 'schema-1', handle: 'default', displayName: 'Default'}]},
    isLoading: false,
  }),
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

// JwtPreview renders through the Monaco editor; a stand-in keeps the preview observable.
vi.mock('../JwtPreview', () => ({
  default: () => <div data-testid="jwt-preview" />,
}));

// Hoisted so its identity is stable across renders; a fresh allowedUserTypes array each render
// would re-fire the schema-fetch effect.
const application = {
  id: 'agent-1',
  name: 'Agent',
  allowedUserTypes: ['default'],
  inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: []}}],
} as unknown as Application;

/** An OAuth config shaped like one loaded from the API, with every token block materialized. */
const oauth2Config = (idTokenValidity = 3600): OAuth2Config => ({
  grantTypes: ['client_credentials', 'urn:ietf:params:oauth:grant-type:token-exchange'],
  responseTypes: [],
  token: {
    accessToken: {userConfig: {validityPeriod: 1800, attributes: ['email']}},
    idToken: {validityPeriod: idTokenValidity, userAttributes: ['email'], responseType: 'JWT'},
    refreshToken: {validityPeriod: 86400},
  },
  scopeClaims: {profile: ['email']},
});

/** The OAuth config written by the last onFieldChange('inboundAuthConfig', ...) call. */
const lastWrittenConfig = (onFieldChange: ReturnType<typeof vi.fn>): OAuth2Config => {
  const calls = onFieldChange.mock.calls.filter(([field]) => field === 'inboundAuthConfig');
  const inboundAuth = calls[calls.length - 1][1] as {type: string; config: OAuth2Config}[];
  return inboundAuth.find((auth) => auth.type === 'oauth2')!.config;
};

describe('EditTokenSettings with accessTokenOnly', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}')));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it('should show only the access token attributes, the scope mapping and the access token validity', async () => {
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={oauth2Config()}
        onFieldChange={vi.fn()}
        accessTokenOnly
      />,
    );

    expect(await screen.findByText('Access token attributes')).toBeInTheDocument();
    expect(screen.getByText('Add or remove attributes')).toBeInTheDocument();
    expect(screen.getByText(/A custom scope listed here/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Access token validity/)).toHaveValue(1800);
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByText('Response format')).not.toBeInTheDocument();
  });

  it('should keep the tabbed ID token and refresh token settings without accessTokenOnly', async () => {
    render(<EditTokenSettings application={application} oauth2Config={oauth2Config()} onFieldChange={vi.fn()} />);

    expect(await screen.findByRole('tab', {name: /^ID token & user info$/})).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'Refresh token'})).toBeInTheDocument();
    expect(screen.queryByText(/A custom scope listed here/)).not.toBeInTheDocument();
  });

  it('should write only the access token validity when it is changed', async () => {
    const onFieldChange = vi.fn();
    const config = oauth2Config();
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        accessTokenOnly
      />,
    );

    fireEvent.change(await screen.findByLabelText(/Access token validity/), {target: {value: '900'}});

    await waitFor(() => {
      expect(onFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.anything());
    });
    const written = lastWrittenConfig(onFieldChange);
    expect(written.token?.accessToken?.userConfig?.validityPeriod).toBe(900);
    expect(written.token?.accessToken?.userConfig?.attributes).toEqual(['email']);
    expect(written.token?.idToken).toEqual(config.token?.idToken);
    expect(written.token?.refreshToken).toEqual(config.token?.refreshToken);
  });

  it('should not let a hidden ID token validity block saving', async () => {
    const onFieldChange = vi.fn();
    const onValidationChange = vi.fn();
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={oauth2Config(0)}
        onFieldChange={onFieldChange}
        onValidationChange={onValidationChange}
        accessTokenOnly
      />,
    );

    fireEvent.change(await screen.findByLabelText(/Access token validity/), {target: {value: '900'}});

    await waitFor(() => {
      expect(onFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.anything());
    });
    expect(lastWrittenConfig(onFieldChange).token?.idToken?.validityPeriod).toBe(0);
    expect(onValidationChange).toHaveBeenLastCalledWith(false);
  });

  it('should still block saving on an invalid ID token validity without accessTokenOnly', async () => {
    const onValidationChange = vi.fn();
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={oauth2Config(0)}
        onFieldChange={vi.fn()}
        onValidationChange={onValidationChange}
      />,
    );

    await waitFor(() => {
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
  });

  it('should block saving on an invalid access token validity', async () => {
    const onFieldChange = vi.fn();
    const onValidationChange = vi.fn();
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={oauth2Config()}
        onFieldChange={onFieldChange}
        onValidationChange={onValidationChange}
        accessTokenOnly
      />,
    );

    fireEvent.change(await screen.findByLabelText(/Access token validity/), {target: {value: '0'}});

    await waitFor(() => {
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
    expect(onFieldChange).not.toHaveBeenCalledWith('inboundAuthConfig', expect.anything());
  });

  it('should show the note and hide the attribute picker when showAccessTokenAttributes is false', async () => {
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={oauth2Config()}
        onFieldChange={vi.fn()}
        accessTokenOnly
        showAccessTokenAttributes={false}
        accessTokenNote="No user attributes here."
      />,
    );

    expect(await screen.findByText('No user attributes here.')).toBeInTheDocument();
    expect(screen.getByText('Access token')).toBeInTheDocument();
    expect(screen.queryByText('Access token attributes')).not.toBeInTheDocument();
    expect(screen.queryByText('Add or remove attributes')).not.toBeInTheDocument();
    expect(screen.queryByTestId('jwt-preview')).not.toBeInTheDocument();
  });

  it('should stop blocking saving when the mode switches to accessTokenOnly while mounted', async () => {
    const onValidationChange = vi.fn();
    // Same props across the rerender, so only the mode changes.
    const onFieldChange = vi.fn();
    const config = oauth2Config(0);
    const {rerender} = render(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        onValidationChange={onValidationChange}
      />,
    );
    await waitFor(() => {
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });

    rerender(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        onValidationChange={onValidationChange}
        accessTokenOnly
      />,
    );

    await waitFor(() => {
      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });
  });

  it('should start blocking saving, and show why, when the mode switches back to the full settings while mounted', async () => {
    const user = userEvent.setup();
    const onValidationChange = vi.fn();
    // Same props across the rerender, so only the mode changes.
    const onFieldChange = vi.fn();
    const config = oauth2Config(0);
    const {rerender} = render(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        onValidationChange={onValidationChange}
        accessTokenOnly
      />,
    );
    await waitFor(() => {
      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });

    rerender(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        onValidationChange={onValidationChange}
      />,
    );

    await waitFor(() => {
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
    // The now-visible ID token validity carries its error, so the blocked Save has a visible cause.
    await user.click(screen.getByRole('tab', {name: /^ID token$/}));
    expect(screen.getByText('Validity period must be at least 1 second')).toBeInTheDocument();
  });

  it('should edit the access token when the mode switches to accessTokenOnly with the ID token tab open', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();
    const config = oauth2Config();
    const {rerender} = render(
      <EditTokenSettings application={application} oauth2Config={config} onFieldChange={onFieldChange} />,
    );
    await user.click(await screen.findByRole('tab', {name: /^ID token & user info$/}));

    rerender(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        accessTokenOnly
      />,
    );
    // The access token picker renders before the scope mapping, which offers the same attribute.
    const rolesChip = (): Element => screen.getAllByText('roles')[0].closest('.MuiChip-root')!;
    await user.click(rolesChip());

    // The click is staged against the access token, and the chip shows it as selected.
    expect(lastWrittenConfig(onFieldChange).token?.accessToken?.userConfig?.attributes).toEqual(['email', 'roles']);
    expect(rolesChip()).toHaveClass('MuiChip-filled');
  });

  it('should keep the hidden ID token and user info attribute lists in step with a scope mapping change', async () => {
    const user = userEvent.setup();
    const onFieldChange = vi.fn();
    render(
      <EditTokenSettings
        application={application}
        oauth2Config={oauth2Config()}
        onFieldChange={onFieldChange}
        accessTokenOnly
      />,
    );

    // profile is the selected scope; map one more attribute to it.
    const availableSection = (await screen.findByText('Available attributes')).closest('div')!.parentElement!;
    await user.click(within(availableSection).getByText('groups'));

    const written = lastWrittenConfig(onFieldChange);
    expect(written.scopeClaims).toEqual({profile: ['email', 'groups']});
    expect(written.token?.idToken?.userAttributes).toEqual(['email', 'groups']);
    expect(written.userInfo?.userAttributes).toEqual(['email', 'groups']);
    expect(written.token?.idToken?.validityPeriod).toBe(3600);
  });

  it('should keep the reset values after a Reset that changes the mode', async () => {
    const onFieldChange = vi.fn();
    const config = oauth2Config();
    const {rerender} = render(
      <EditTokenSettings application={application} oauth2Config={config} onFieldChange={onFieldChange} />,
    );
    fireEvent.change(document.getElementById('accessTokenValidity-input')!, {target: {value: '900'}});
    await waitFor(() => {
      expect(onFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.anything());
    });
    onFieldChange.mockClear();

    // A Reset drops the staged edit, bumps the reset key and, with other grants restored, the mode.
    rerender(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        sectionResetKey={1}
        accessTokenOnly
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/Access token validity/)).toHaveValue(1800);
    });
    await new Promise((resolve) => {
      setTimeout(resolve, 50);
    });
    expect(onFieldChange).not.toHaveBeenCalled();
  });

  // A mode change can arrive while the form still holds a staged value and no reset is in the same
  // commit. Saving on the agent edit page does that: it clears the edits, refetches, then bumps the
  // reset key, each in its own commit, so the grants (and the mode) can revert before the form
  // resets. A write effect that ran on the mode change would then write the staged value back. The
  // Reset test above cannot show it, since act() renders a Reset's follow-up before the write
  // effect's async validation settles. This pins the cause: a mode change alone writes nothing.
  it('should not run the validity write on a mode change alone', async () => {
    const onFieldChange = vi.fn();
    const config = oauth2Config();
    const {rerender} = render(
      <EditTokenSettings application={application} oauth2Config={config} onFieldChange={onFieldChange} />,
    );
    fireEvent.change(document.getElementById('accessTokenValidity-input')!, {target: {value: '900'}});
    await waitFor(() => {
      expect(onFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.anything());
    });
    onFieldChange.mockClear();

    rerender(
      <EditTokenSettings
        application={application}
        oauth2Config={config}
        onFieldChange={onFieldChange}
        accessTokenOnly
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/Access token validity/)).toHaveValue(900);
    });
    await new Promise((resolve) => {
      setTimeout(resolve, 50);
    });
    expect(onFieldChange).not.toHaveBeenCalled();
  });

  it('should ignore accessTokenOnly in native mode', async () => {
    render(<EditTokenSettings application={application} onFieldChange={vi.fn()} accessTokenOnly />);

    expect(await screen.findByText('Token attributes')).toBeInTheDocument();
    expect(screen.queryByLabelText(/Access token validity/)).not.toBeInTheDocument();
  });
});
