// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
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

    await user.click(screen.getByTestId('trusted-issuer-delete-button'));
    await user.click(screen.getByRole('button', {name: /simulate delete confirm/i}));

    expect(mockDeleteMutate).toHaveBeenCalledWith('ti-1', expect.any(Object));
    expect(mockNavigate).toHaveBeenCalledWith('/connections');
  });
});
