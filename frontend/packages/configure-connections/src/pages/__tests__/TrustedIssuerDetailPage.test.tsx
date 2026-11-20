// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
import {act, render, screen, waitFor} from '@thunderid/test-utils';
import type {NavigateFunction, Params} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {TrustedIssuer} from '../../models/trusted-issuer';
import TrustedIssuerDetailPage from '../TrustedIssuerDetailPage';

const {mockMutate, mockRefetch, mockDeleteMutate, mockUseAuthorizationServerMetadata} = vi.hoisted(() => ({
  mockMutate: vi.fn(),
  mockRefetch: vi.fn(),
  mockDeleteMutate: vi.fn(),
  mockUseAuthorizationServerMetadata: vi.fn(),
}));

const TRUSTED_ISSUER: TrustedIssuer = {
  id: 'ti-1',
  name: 'Acme Okta',
  ouId: 'ou-1',
  issuer: 'https://acme.okta.com',
  jwksEndpoint: 'https://acme.okta.com/keys',
  idJagEnabled: true,
};

const NEW_DIRECT = [{claim: 'scope', targetType: 'permission'}];
const NEW_RULES = [{claim: 'groups', value: 'admins', targetType: 'role', targetId: 'role-1'}];
const STORED_DIRECT = [{claim: 'groups', targetType: 'role'}];

vi.mock('../../api/useAuthorizationServerMetadata', () => ({
  default: mockUseAuthorizationServerMetadata,
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
    useParams: vi.fn(),
  };
});

vi.mock('../../api/useTrustedIssuer', () => ({
  default: vi.fn(),
}));

vi.mock('../../api/useUpdateTrustedIssuer', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

vi.mock('../../api/useDeleteConnection', () => ({
  default: () => ({mutate: mockDeleteMutate, isPending: false}),
}));

vi.mock('../../components/OrganizationUnitSection', () => ({
  default: ({ouId}: {ouId: string}) => <section aria-label="Organization unit" data-ou-id={ouId} />,
}));

vi.mock('../../components/AuthorizationMappingSection', async () => {
  const {useState} = await import('react');
  return {
    default: function StubAuthorizationMappingSection({
      onRuleChange,
      onDirectChange,
      ouId = undefined,
      initialDirectConfig = undefined,
      description = undefined,
    }: {
      onRuleChange: (c: unknown, v: boolean) => void;
      onDirectChange: (c: unknown, v: boolean) => void;
      ouId?: string;
      initialDirectConfig?: unknown;
      description?: string;
    }) {
      // Like the real section, read the initial mapping only on mount.
      const [initialDirect] = useState(initialDirectConfig);
      // Stands for rows the section holds in its own state, which a remount would lose.
      const [localRows, setLocalRows] = useState(0);
      return (
        <>
          <div data-testid="authz-section" data-ou-id={ouId}>
            {description}
          </div>
          <div data-testid="authz-initial-direct">{JSON.stringify(initialDirect ?? null)}</div>
          <div data-testid="authz-local-rows">{localRows}</div>
          <button type="button" onClick={() => setLocalRows((n) => n + 1)}>
            add local row
          </button>
          <button type="button" onClick={() => onDirectChange(NEW_DIRECT, true)}>
            set direct mapping
          </button>
          <button type="button" onClick={() => onRuleChange(NEW_RULES, true)}>
            set rule mapping
          </button>
          <button type="button" onClick={() => onDirectChange(undefined, true)}>
            clear direct mappings
          </button>
          <button type="button" onClick={() => onDirectChange(undefined, false)}>
            report incomplete row
          </button>
        </>
      );
    },
  };
});

vi.mock('../../components/ConnectionDeleteDialog', () => ({
  default: function StubConnectionDeleteDialog({open, onConfirm}: {open: boolean; onConfirm: () => void}) {
    return open ? (
      <div data-testid="stub-delete-dialog">
        <button type="button" onClick={onConfirm}>
          Simulate delete confirm
        </button>
      </div>
    ) : null;
  },
}));

const {useNavigate, useParams} = await import('react-router');
const {default: useTrustedIssuer} = await import('../../api/useTrustedIssuer');

const ID_JAG_TITLE = 'Identity Assertion JWT Authorization Grant (ID-JAG)';
const ID_JAG_ENABLED_NOTE = 'Identity assertions from this issuer are accepted via the ID-JAG protocol.';
const ACCEPTANCE_UNAVAILABLE_HINT = 'This server does not accept identity assertions from any issuer.';
const ACCEPTING_METADATA = {
  grant_types_supported: ['authorization_code', 'urn:ietf:params:oauth:grant-type:jwt-bearer'],
  authorization_grant_profiles_supported: ['urn:ietf:params:oauth:grant-profile:id-jag'],
};
const NOT_ACCEPTING_METADATA = {grant_types_supported: ['authorization_code']};

// Stands in for the runtime's authorization server metadata query (data undefined while loading or failed).
const mockServerMetadata = (data: Record<string, string[]> | undefined): void => {
  mockUseAuthorizationServerMetadata.mockReturnValue({data});
};

const mockIdJagEnabled = (idJagEnabled: boolean): void => {
  vi.mocked(useTrustedIssuer).mockReturnValue({
    data: {...TRUSTED_ISSUER, idJagEnabled},
    isLoading: false,
    isError: false,
    refetch: mockRefetch,
  } as unknown as ReturnType<typeof useTrustedIssuer>);
};

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  isLoading: false,
};

describe('TrustedIssuerDetailPage', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockMutate.mockReset();
    mockRefetch.mockReset();
    mockDeleteMutate.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useParams).mockReturnValue({id: 'ti-1'} as unknown as Params);
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: TRUSTED_ISSUER,
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    mockServerMetadata(undefined);
  });

  it('should not render a client id field', () => {
    render(<TrustedIssuerDetailPage />);

    expect(screen.queryByLabelText(/^Client ID/)).not.toBeInTheDocument();
  });

  it('should render the ID-JAG card title and enabled note when assertions are accepted', () => {
    render(<TrustedIssuerDetailPage />);

    expect(
      screen.getByRole('heading', {name: 'Identity Assertion JWT Authorization Grant (ID-JAG)'}),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Identity assertions from this issuer are accepted via the ID-JAG protocol.'),
    ).toBeInTheDocument();
  });

  it('should show the organization unit the trusted issuer belongs to', () => {
    render(<TrustedIssuerDetailPage />);

    expect(screen.getByRole('region', {name: 'Organization unit'})).toHaveAttribute('data-ou-id', 'ou-1');
  });

  it('should not render capability chips next to the page title', () => {
    render(<TrustedIssuerDetailPage />);

    // The only "Token exchange" text is the settings card heading, not a chip beside the title.
    expect(screen.getAllByText('Token exchange')).toEqual([screen.getByRole('heading', {name: 'Token exchange'})]);
    expect(screen.queryByText('Inactive')).not.toBeInTheDocument();
    expect(screen.queryByText('ID-JAG')).not.toBeInTheDocument();
  });

  it('should hide the ID-JAG enabled note when assertions are not accepted', () => {
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {...TRUSTED_ISSUER, idJagEnabled: false},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);

    render(<TrustedIssuerDetailPage />);

    expect(
      screen.queryByText('Identity assertions from this issuer are accepted via the ID-JAG protocol.'),
    ).not.toBeInTheDocument();
  });

  describe('ID-JAG acceptance availability', () => {
    it('should not let ID-JAG be turned on and explain why when the server does not accept identity assertions', async () => {
      const user = userEvent.setup();
      mockIdJagEnabled(false);
      mockServerMetadata(NOT_ACCEPTING_METADATA);

      render(<TrustedIssuerDetailPage />);

      const idJagSwitch = screen.getByRole('switch', {name: ID_JAG_TITLE});
      expect(idJagSwitch).not.toBeChecked();
      expect(idJagSwitch).toBeDisabled();
      // The reason is visible in the card body, not only in the disabled switch's tooltip.
      expect(screen.getByText(ACCEPTANCE_UNAVAILABLE_HINT)).toBeInTheDocument();

      await user.hover(idJagSwitch.closest('.MuiSwitch-root')!.parentElement!);

      expect(await screen.findByRole('tooltip')).toHaveTextContent(ACCEPTANCE_UNAVAILABLE_HINT);
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    it('should not change the setting when the disabled switch is clicked', async () => {
      // A disabled switch has pointer-events: none, so the check is skipped to force the click through.
      const user = userEvent.setup({pointerEventsCheck: 0});
      mockIdJagEnabled(false);
      mockServerMetadata(NOT_ACCEPTING_METADATA);

      render(<TrustedIssuerDetailPage />);

      const idJagSwitch = screen.getByRole('switch', {name: ID_JAG_TITLE});
      await user.click(idJagSwitch);

      expect(idJagSwitch).not.toBeChecked();
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    it('should let an issuer that already accepts ID-JAG turn it off and save', async () => {
      const user = userEvent.setup();
      mockIdJagEnabled(true);
      mockServerMetadata(NOT_ACCEPTING_METADATA);

      render(<TrustedIssuerDetailPage />);

      const idJagSwitch = screen.getByRole('switch', {name: ID_JAG_TITLE});
      expect(idJagSwitch).toBeChecked();
      expect(idJagSwitch).not.toBeDisabled();

      await user.click(idJagSwitch);

      expect(idJagSwitch).not.toBeChecked();
      expect(idJagSwitch).toBeDisabled();
      // The pointer still rests on the switch, so its tooltip is open too; this is the card body copy.
      expect(screen.getByText(ACCEPTANCE_UNAVAILABLE_HINT, {selector: 'p'})).toBeInTheDocument();

      await user.click(await screen.findByText('Save changes'));

      expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({idJagEnabled: false}), expect.any(Object));
    });

    it('should show the hint instead of the enabled note while the server does not accept identity assertions', () => {
      mockIdJagEnabled(true);
      mockServerMetadata(NOT_ACCEPTING_METADATA);

      render(<TrustedIssuerDetailPage />);

      expect(screen.getByText(ACCEPTANCE_UNAVAILABLE_HINT)).toBeInTheDocument();
      expect(screen.queryByText(ID_JAG_ENABLED_NOTE)).not.toBeInTheDocument();
    });

    it('should let ID-JAG be turned on with no hint when the server accepts identity assertions', async () => {
      const user = userEvent.setup();
      mockIdJagEnabled(false);
      mockServerMetadata(ACCEPTING_METADATA);

      render(<TrustedIssuerDetailPage />);

      const idJagSwitch = screen.getByRole('switch', {name: ID_JAG_TITLE});
      expect(idJagSwitch).not.toBeDisabled();
      expect(screen.queryByText(ACCEPTANCE_UNAVAILABLE_HINT)).not.toBeInTheDocument();

      await user.click(idJagSwitch);

      expect(idJagSwitch).toBeChecked();
      expect(screen.getByText(ID_JAG_ENABLED_NOTE)).toBeInTheDocument();
      expect(await screen.findByText('You have unsaved changes')).toBeInTheDocument();
    });

    it('should let ID-JAG be turned on with no hint while the runtime metadata is loading', async () => {
      const user = userEvent.setup();
      mockIdJagEnabled(false);

      render(<TrustedIssuerDetailPage />);

      const idJagSwitch = screen.getByRole('switch', {name: ID_JAG_TITLE});
      expect(idJagSwitch).not.toBeDisabled();

      await user.click(idJagSwitch);

      expect(idJagSwitch).toBeChecked();
      expect(screen.queryByText(ACCEPTANCE_UNAVAILABLE_HINT)).not.toBeInTheDocument();
    });
  });

  it('should show the unsaved changes bar and save the updated name', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();

    const nameField = screen.getByLabelText(/^Name/);
    await user.clear(nameField);
    await user.type(nameField, 'Updated Acme Okta');

    expect(await screen.findByText('You have unsaved changes')).toBeInTheDocument();

    await user.click(screen.getByText('Save changes'));

    expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({name: 'Updated Acme Okta'}), expect.any(Object));
    expect(mockMutate.mock.calls[0]?.[0]).not.toHaveProperty('ouId');
  });

  it('should hide delete and disable save when the admin may only view connections', async () => {
    const user = userEvent.setup();
    render(
      <CapabilitiesProvider capabilities={viewOnly}>
        <TrustedIssuerDetailPage />
      </CapabilitiesProvider>,
    );

    expect(screen.queryByTestId('trusted-issuer-delete-button')).not.toBeInTheDocument();
    expect(screen.queryByTestId('trusted-issuer-tab-advanced')).not.toBeInTheDocument();

    const nameField = screen.getByLabelText(/^Name/);
    await user.clear(nameField);
    await user.type(nameField, 'Updated Acme Okta');

    expect(await screen.findByRole('button', {name: 'Save changes'})).toBeDisabled();
  });

  it('should navigate back to connections when Back is clicked', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    await user.click(screen.getByRole('button', {name: /back to connections/i}));

    expect(mockNavigate).toHaveBeenCalledWith('/connections');
  });

  it('should navigate to connections when the trusted issuer is deleted', async () => {
    const user = userEvent.setup();
    mockDeleteMutate.mockImplementation((_id: string, opts: {onSuccess: () => void}) => {
      opts.onSuccess();
    });
    render(<TrustedIssuerDetailPage />);

    await user.click(screen.getByTestId('trusted-issuer-tab-advanced'));
    await user.click(screen.getByTestId('trusted-issuer-delete-button'));
    await user.click(screen.getByRole('button', {name: /simulate delete confirm/i}));

    expect(mockDeleteMutate).toHaveBeenCalledWith('ti-1', expect.any(Object));
    expect(mockNavigate).toHaveBeenCalledWith('/connections');
  });

  it('should show General first and the authorization mapping and danger zone on their own tabs', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
    expect(screen.getByTestId('authz-section')).not.toBeVisible();
    expect(screen.queryByTestId('trusted-issuer-delete-button')).not.toBeInTheDocument();

    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));
    expect(screen.getByTestId('authz-section')).toBeVisible();
    expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();

    await user.click(screen.getByTestId('trusted-issuer-tab-advanced'));
    expect(screen.getByTestId('trusted-issuer-delete-button')).toBeInTheDocument();
    expect(screen.getByTestId('authz-section')).not.toBeVisible();
  });

  it('should name each tab panel by its tab', () => {
    render(<TrustedIssuerDetailPage />);

    const panels = screen.getAllByRole('tabpanel', {hidden: true});
    expect(panels).toHaveLength(3);
    ['general', 'attributes', 'advanced'].forEach((name, index) => {
      const tab = screen.getByTestId(`trusted-issuer-tab-${name}`);
      expect(tab).toHaveAttribute('aria-controls', panels[index].id);
      expect(panels[index]).toHaveAttribute('aria-labelledby', tab.id);
    });
    expect(screen.getByRole('tabpanel', {name: 'General'})).toHaveAttribute('id', 'trusted-issuer-tabpanel-0');
    expect(screen.getByRole('tablist', {name: 'Trusted issuer settings'})).toBeInTheDocument();
  });

  it('should show a name conflict on General when the save is made from another tab', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_body: unknown, opts: {onError: (error: unknown) => void}) =>
      opts.onError({response: {status: 409}}),
    );
    render(<TrustedIssuerDetailPage />);
    const nameField = screen.getByLabelText(/^Name/);
    await user.clear(nameField);
    await user.type(nameField, 'Taken name');
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    await user.click(screen.getByText('Save changes'));

    expect(screen.getByTestId('trusted-issuer-tab-general')).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('A trusted issuer with this name already exists.')).toBeInTheDocument();
  });

  it('should show the load error when the trusted issuer cannot be loaded', () => {
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: undefined,
      error: new Error('Network Error'),
      isLoading: false,
      isError: true,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    render(<TrustedIssuerDetailPage />);

    expect(screen.getByText('Failed to load trusted issuer.')).toBeInTheDocument();
    expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();
  });

  it('should load again when Refresh is clicked on the load error', async () => {
    const user = userEvent.setup();
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: undefined,
      error: new Error('Network Error'),
      isLoading: false,
      isError: true,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    render(<TrustedIssuerDetailPage />);

    await user.click(screen.getByRole('button', {name: 'Refresh'}));

    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it('should keep the page and its edits when a refresh fails', async () => {
    const user = userEvent.setup();
    const {rerender} = render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));
    await user.click(screen.getByRole('button', {name: 'add local row'}));
    await user.click(screen.getByRole('button', {name: 'set direct mapping'}));

    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: TRUSTED_ISSUER,
      error: new Error('Network Error'),
      isLoading: false,
      isError: true,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    rerender(<TrustedIssuerDetailPage />);

    expect(screen.getByTestId('authz-local-rows')).toHaveTextContent('1');
    expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it('should show a refreshed stored mapping while the mapping is not being edited', () => {
    const {rerender} = render(<TrustedIssuerDetailPage />);
    expect(screen.getByTestId('authz-initial-direct')).toHaveTextContent('null');

    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {...TRUSTED_ISSUER, attributeConfiguration: {authorizationMapping: {direct: STORED_DIRECT}}},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    rerender(<TrustedIssuerDetailPage />);

    expect(screen.getByTestId('authz-initial-direct')).toHaveTextContent(JSON.stringify(STORED_DIRECT));
  });

  it('should send the edited rule mapping when saved', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    await user.click(screen.getByRole('button', {name: 'set rule mapping'}));
    await user.click(await screen.findByText('Save changes'));

    const body = mockMutate.mock.calls[0]?.[0] as {attributeConfiguration?: unknown};
    expect(body.attributeConfiguration).toEqual({
      userTypeResolution: {default: ''},
      authorizationMapping: {rules: NEW_RULES, direct: undefined},
    });
  });

  it('should require https:// URLs for the issuer and the JWKS endpoint', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    const issuerField = screen.getByLabelText(/^Issuer URI/);
    await user.clear(issuerField);
    await user.type(issuerField, 'http://acme.okta.com');
    const jwksField = screen.getByLabelText(/^JWKS endpoint/);
    await user.clear(jwksField);
    await user.type(jwksField, 'http://acme.okta.com/keys');
    await user.tab();

    expect(screen.getAllByText('Enter a valid https:// URL.')).toHaveLength(2);
    expect(screen.getByText('Save changes').closest('button')).toBeDisabled();
  });

  it('should save token exchange and its trusted audience', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    await user.click(screen.getByRole('switch', {name: 'Toggle Token exchange'}));
    await user.type(screen.getByLabelText(/^Trusted token audience/), 'api://acme');
    await user.click(await screen.findByText('Save changes'));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({tokenExchangeEnabled: true, trustedTokenAudience: 'api://acme'}),
      expect.any(Object),
    );
  });

  it('should keep the mapping being edited when a refresh brings a different stored mapping', async () => {
    const user = userEvent.setup();
    const {rerender} = render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));
    await user.click(screen.getByRole('button', {name: 'add local row'}));
    await user.click(screen.getByRole('button', {name: 'set direct mapping'}));

    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {...TRUSTED_ISSUER, attributeConfiguration: {authorizationMapping: {direct: STORED_DIRECT}}},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    rerender(<TrustedIssuerDetailPage />);

    expect(screen.getByTestId('authz-local-rows')).toHaveTextContent('1');
    expect(screen.getByTestId('authz-initial-direct')).toHaveTextContent('null');
  });

  it('should keep the mapping rows being edited when switching tabs', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));
    await user.click(screen.getByRole('button', {name: 'add local row'}));

    await user.click(screen.getByTestId('trusted-issuer-tab-advanced'));
    await user.click(screen.getByTestId('trusted-issuer-tab-general'));
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    expect(screen.getByTestId('authz-local-rows')).toHaveTextContent('1');
  });

  it("should render the authorization mapping section for the issuer's organization unit", async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    const section = screen.getByTestId('authz-section');
    expect(section).toHaveAttribute('data-ou-id', 'ou-1');
    expect(section).toHaveTextContent('When a mapping is set, these tokens carry only the permissions it grants.');
  });

  it('should mark dirty and send the edited direct mapping when saved', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    await user.click(screen.getByRole('button', {name: 'set direct mapping'}));
    expect(await screen.findByText('You have unsaved changes')).toBeInTheDocument();
    await user.click(screen.getByText('Save changes'));

    const body = mockMutate.mock.calls[0]?.[0] as {attributeConfiguration?: unknown};
    expect(body.attributeConfiguration).toEqual({
      userTypeResolution: {default: ''},
      authorizationMapping: {rules: undefined, direct: NEW_DIRECT},
    });
  });

  it('should show the saved mapping after a save', async () => {
    const user = userEvent.setup();
    const saved = {
      data: {...TRUSTED_ISSUER, attributeConfiguration: {authorizationMapping: {direct: NEW_DIRECT}}},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>;
    mockMutate.mockImplementation((_body: unknown, opts: {onSuccess: () => void}) => opts.onSuccess());
    // The query returns the saved issuer only once the refetch has finished.
    mockRefetch.mockImplementation(async () => {
      await Promise.resolve();
      vi.mocked(useTrustedIssuer).mockReturnValue(saved);
      return {};
    });
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    await user.click(screen.getByRole('button', {name: 'set direct mapping'}));
    await user.click(await screen.findByText('Save changes'));

    await waitFor(() =>
      expect(screen.getByTestId('authz-initial-direct')).toHaveTextContent(JSON.stringify(NEW_DIRECT)),
    );
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });

  it('should keep the saved mapping when the post-save refetch fails', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_body: unknown, opts: {onSuccess: () => void}) => opts.onSuccess());
    mockRefetch.mockResolvedValue({isError: true});
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));
    await user.click(screen.getByRole('button', {name: 'set direct mapping'}));
    await user.click(await screen.findByText('Save changes'));
    await act(async () => {
      await Promise.resolve();
    });

    // The edits stay, so the next save sends the saved mapping, not the stale cached one.
    await user.click(screen.getByText('Save changes'));
    const body = mockMutate.mock.calls[1]?.[0] as {attributeConfiguration?: {authorizationMapping?: unknown}};
    expect(body.attributeConfiguration?.authorizationMapping).toEqual({rules: undefined, direct: NEW_DIRECT});
  });

  it('should keep an edit made while the post-save refetch runs', async () => {
    const user = userEvent.setup();
    const saved = {
      data: {...TRUSTED_ISSUER, attributeConfiguration: {authorizationMapping: {direct: NEW_DIRECT}}},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>;
    let finishRefetch: () => void = () => undefined;
    mockMutate.mockImplementation((_body: unknown, opts: {onSuccess: () => void}) => opts.onSuccess());
    mockRefetch.mockImplementation(
      () =>
        new Promise((resolve) => {
          finishRefetch = () => resolve({});
        }),
    );
    const {rerender} = render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));
    await user.click(screen.getByRole('button', {name: 'set direct mapping'}));
    await user.click(await screen.findByText('Save changes'));

    await user.click(screen.getByRole('button', {name: 'add local row'}));
    await user.click(screen.getByRole('button', {name: 'clear direct mappings'}));
    // The query returns the saved issuer once the refetch has finished.
    vi.mocked(useTrustedIssuer).mockReturnValue(saved);
    await act(async () => {
      finishRefetch();
      await Promise.resolve();
    });
    rerender(<TrustedIssuerDetailPage />);

    expect(screen.getByTestId('authz-local-rows')).toHaveTextContent('1');
    expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
  });

  it('should keep the stored attribute configuration when another field is saved', async () => {
    const user = userEvent.setup();
    const attributeConfiguration = {
      userTypeResolution: {default: 'person'},
      authorizationMapping: {direct: STORED_DIRECT},
    };
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {...TRUSTED_ISSUER, attributeConfiguration},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    render(<TrustedIssuerDetailPage />);

    await user.type(screen.getByLabelText(/^Name/), ' 2');
    await user.click(screen.getByText('Save changes'));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'Acme Okta 2',
        attributeConfiguration: {
          userTypeResolution: {default: 'person'},
          authorizationMapping: {rules: undefined, direct: STORED_DIRECT},
        },
      }),
      expect.any(Object),
    );
  });

  it('should not send an attribute configuration when none is stored or set', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    await user.type(screen.getByLabelText(/^Name/), ' 2');
    await user.click(screen.getByText('Save changes'));

    expect(mockMutate.mock.calls[0]?.[0]).not.toHaveProperty('attributeConfiguration');
  });

  it('should drop the authorization mapping when it is cleared', async () => {
    const user = userEvent.setup();
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {
        ...TRUSTED_ISSUER,
        attributeConfiguration: {userTypeResolution: {default: ''}, authorizationMapping: {direct: STORED_DIRECT}},
      },
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    await user.click(screen.getByRole('button', {name: 'clear direct mappings'}));
    await user.click(await screen.findByText('Save changes'));

    const body = mockMutate.mock.calls[0]?.[0] as {attributeConfiguration: {authorizationMapping?: unknown}};
    expect(body.attributeConfiguration.authorizationMapping).toBeUndefined();
  });

  it('should block saving while a mapping row is incomplete and restore on reset', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);
    await user.click(screen.getByTestId('trusted-issuer-tab-attributes'));

    await user.click(screen.getByRole('button', {name: 'report incomplete row'}));
    expect(await screen.findByRole('button', {name: 'Save changes'})).toBeDisabled();

    await user.click(screen.getByRole('button', {name: 'Reset'}));
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });

  it('should keep stored attribute mappings when the issuer has no authorization mapping', async () => {
    const user = userEvent.setup();
    const userTypeAttributeMappings = [
      {userType: 'person', attributes: [{externalAttribute: 'e', localAttribute: 'l'}]},
    ];
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {
        ...TRUSTED_ISSUER,
        attributeConfiguration: {userTypeResolution: {default: 'person'}, userTypeAttributeMappings},
      },
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
    render(<TrustedIssuerDetailPage />);

    await user.type(screen.getByLabelText(/^Name/), ' 2');
    await user.click(screen.getByText('Save changes'));

    const body = mockMutate.mock.calls[0]?.[0] as {attributeConfiguration: Record<string, unknown>};
    expect(body.attributeConfiguration['userTypeAttributeMappings']).toEqual(userTypeAttributeMappings);
    expect(body.attributeConfiguration['userTypeResolution']).toEqual({default: 'person'});
  });
});
