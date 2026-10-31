// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access */
import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import TrustedIssuerCreateForm from '../TrustedIssuerCreateForm';

const {mockMutate, mockReset, mockUseAuthorizationServerMetadata} = vi.hoisted(() => ({
  mockMutate: vi.fn(),
  mockReset: vi.fn(),
  mockUseAuthorizationServerMetadata: vi.fn(),
}));
const mutationState = {isPending: false, isError: false};

vi.mock('../../api/useAuthorizationServerMetadata', () => ({
  default: mockUseAuthorizationServerMetadata,
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});

vi.mock('../../api/useCreateTrustedIssuer', () => ({
  default: () => ({mutate: mockMutate, reset: mockReset, ...mutationState}),
}));

const {useNavigate} = await import('react-router');

const ACCEPTANCE_UNAVAILABLE_HINT = 'This server does not accept identity assertions from any issuer.';

// Stands in for the runtime's authorization server metadata query (data undefined while loading or failed).
const mockServerMetadata = (data: Record<string, string[]> | undefined): void => {
  mockUseAuthorizationServerMetadata.mockReturnValue({data});
};

describe('TrustedIssuerCreateForm', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;
  let onNameConflict: ReturnType<typeof vi.fn<() => void>>;
  let onBack: ReturnType<typeof vi.fn<() => void>>;

  beforeEach(() => {
    mockNavigate = vi.fn();
    onNameConflict = vi.fn<() => void>();
    onBack = vi.fn<() => void>();
    mockMutate.mockReset();
    mockReset.mockReset();
    mutationState.isPending = false;
    mutationState.isError = false;
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    mockServerMetadata(undefined);
  });

  it('should render the form with the ID-JAG switch off and token exchange switch on by default', () => {
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    expect(screen.getByLabelText(/^Issuer URI/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^JWKS endpoint/)).toBeInTheDocument();
    expect(screen.getByRole('switch', {name: /id-jag/i})).not.toBeChecked();
    expect(screen.getByRole('switch', {name: /enable token exchange/i})).toBeChecked();
  });

  it('should not render a name field (collected on the wizard name step)', () => {
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();
  });

  it('should render a Back button that calls onBack, and no Cancel affordance', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    expect(screen.queryByRole('button', {name: /^cancel$/i})).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: /back/i}));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('should disable the submit button until all required fields are valid', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    const submitButton = screen.getByTestId('trusted-issuer-create-submit');
    expect(submitButton).toBeDisabled();

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    expect(submitButton).toBeDisabled();

    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    expect(submitButton).toBeEnabled();
  });

  it('should show a validation error when an issuer URI is not https', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    const issuerField = screen.getByLabelText(/^Issuer URI/);
    await user.type(issuerField, 'http://acme.okta.com');
    await user.tab();

    expect(await screen.findByText('Enter a valid https:// URL.')).toBeInTheDocument();
  });

  it('should show a required error when a field is left blank on blur', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    const issuerField = screen.getByLabelText(/^Issuer URI/);
    await user.click(issuerField);
    await user.tab();

    expect(await screen.findByText('This field is required.')).toBeInTheDocument();
  });

  it('should submit the form with the wizard-collected name and navigate to the detail page on success', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'ti-1',
        name: 'Acme Okta',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/keys',
        idJagEnabled: true,
        ouId: 'ou-1',
      });
    });

    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        ouId: 'ou-1',
        name: 'Acme Okta',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/keys',
        idJagEnabled: false,
        tokenExchangeEnabled: true,
        trustedTokenAudience: undefined,
      },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/trusted-issuers/ti-1');
    });
  });

  it('should report a missing organization unit instead of creating', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(screen.getByText('Organization unit is required')).toBeInTheDocument();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should call onNameConflict on a 409 conflict without navigating', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onError({response: {status: 409}});
    });

    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    await waitFor(() => expect(onNameConflict).toHaveBeenCalledTimes(1));
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('should show a general inline error for a non-conflict create failure, without calling onNameConflict', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onError({response: {status: 500}});
    });

    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(await screen.findByText('Failed to create trusted issuer. Please try again.')).toBeInTheDocument();
    expect(onNameConflict).not.toHaveBeenCalled();
  });

  it('should clear the general create error when a field is edited', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onError({response: {status: 500}});
    });

    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));
    expect(await screen.findByText('Failed to create trusted issuer. Please try again.')).toBeInTheDocument();

    await user.type(screen.getByLabelText(/^Issuer URI/), '2');

    expect(screen.queryByText('Failed to create trusted issuer. Please try again.')).not.toBeInTheDocument();
  });

  it('should not reset a still-pending mutation when a field is edited', async () => {
    const user = userEvent.setup();
    mutationState.isPending = true;
    mutationState.isError = false;

    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');

    expect(mockReset).not.toHaveBeenCalled();
  });

  it('should reset a failed (settled) mutation when a field is edited', async () => {
    const user = userEvent.setup();
    mutationState.isPending = false;
    mutationState.isError = true;

    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');

    expect(mockReset).toHaveBeenCalled();
  });

  it('should turn on ID-JAG when the switch is toggled', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'ti-2',
        name: 'Beta AD',
        issuer: 'https://beta.example.com',
        jwksEndpoint: 'https://beta.example.com/keys',
        idJagEnabled: true,
      });
    });

    render(<TrustedIssuerCreateForm name="Beta AD" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    await user.click(screen.getByRole('switch', {name: /id-jag/i}));
    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://beta.example.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://beta.example.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({idJagEnabled: true}), expect.any(Object));
  });

  it('should not let ID-JAG be turned on and explain why when the server does not accept identity assertions', async () => {
    // A disabled switch has pointer-events: none, so the check is skipped to force the click through.
    const user = userEvent.setup({pointerEventsCheck: 0});
    mockServerMetadata({grant_types_supported: ['authorization_code']});

    render(<TrustedIssuerCreateForm name="Beta AD" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    const idJagSwitch = screen.getByRole('switch', {name: /id-jag/i});
    expect(idJagSwitch).toBeDisabled();
    expect(screen.getByText(ACCEPTANCE_UNAVAILABLE_HINT)).toBeInTheDocument();

    await user.click(idJagSwitch);
    expect(idJagSwitch).not.toBeChecked();

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://beta.example.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://beta.example.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({idJagEnabled: false}), expect.any(Object));
  });

  it('should still let ID-JAG be turned off when it was turned on before the runtime metadata showed it unavailable', async () => {
    const user = userEvent.setup();
    const {rerender} = render(
      <TrustedIssuerCreateForm name="Beta AD" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />,
    );

    await user.click(screen.getByRole('switch', {name: /id-jag/i}));
    mockServerMetadata({grant_types_supported: ['authorization_code']});
    rerender(<TrustedIssuerCreateForm name="Beta AD" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    const idJagSwitch = screen.getByRole('switch', {name: /id-jag/i});
    expect(idJagSwitch).toBeChecked();
    expect(idJagSwitch).not.toBeDisabled();

    await user.click(idJagSwitch);

    expect(idJagSwitch).not.toBeChecked();
    expect(idJagSwitch).toBeDisabled();
  });

  it('should let ID-JAG be turned on with no hint when the server accepts identity assertions', async () => {
    const user = userEvent.setup();
    mockServerMetadata({
      grant_types_supported: ['authorization_code', 'urn:ietf:params:oauth:grant-type:jwt-bearer'],
      authorization_grant_profiles_supported: ['urn:ietf:params:oauth:grant-profile:id-jag'],
    });

    render(<TrustedIssuerCreateForm name="Beta AD" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    const idJagSwitch = screen.getByRole('switch', {name: /id-jag/i});
    expect(idJagSwitch).not.toBeDisabled();
    expect(screen.queryByText(ACCEPTANCE_UNAVAILABLE_HINT)).not.toBeInTheDocument();

    await user.click(idJagSwitch);

    expect(idJagSwitch).toBeChecked();
  });

  it('should let ID-JAG be turned on with no hint while the runtime metadata is loading', () => {
    render(<TrustedIssuerCreateForm name="Beta AD" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    expect(screen.getByRole('switch', {name: /id-jag/i})).not.toBeDisabled();
    expect(screen.queryByText(ACCEPTANCE_UNAVAILABLE_HINT)).not.toBeInTheDocument();
  });

  it('should not render a client id field', () => {
    render(<TrustedIssuerCreateForm name="Acme Okta" ouId="ou-1" onNameConflict={onNameConflict} onBack={onBack} />);

    expect(screen.queryByLabelText(/^Client ID/)).not.toBeInTheDocument();
  });
});
