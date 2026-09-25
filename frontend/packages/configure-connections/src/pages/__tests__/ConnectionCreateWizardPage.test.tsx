// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionCreateWizardPage from '../ConnectionCreateWizardPage';

const mutateMock = vi.fn();
const resetMock = vi.fn();
const navigateMock = vi.fn();
const showToastMock = vi.fn();
const mutationState = {isPending: false, isError: false};

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
}));
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({
    getGateCallbackUrl: () => 'https://id.acme.io/gate/callback',
    config: {brand: {product_name: 'ThunderID'}},
  }),
  useToast: () => ({showToast: showToastMock}),
}));
vi.mock('../../api/useCreateConnection', () => ({
  default: () => ({mutate: mutateMock, reset: resetMock, ...mutationState}),
}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onFieldChange}: {onFieldChange: (name: string, value: string) => void}) {
    useEffect(() => {
      // Populate the fields required by every connection type used in these tests
      // (oidc, oauth, sms-gateway, email-smtp).
      onFieldChange('clientId', 'x');
      onFieldChange('clientSecret', 's');
      onFieldChange('authorizationEndpoint', 'https://idp.example.com/authorize');
      onFieldChange('tokenEndpoint', 'https://idp.example.com/token');
      onFieldChange('userInfoEndpoint', 'https://idp.example.com/userinfo');
      onFieldChange('url', 'https://sms.example.com/send');
      onFieldChange('host', 'smtp.example.com');
      onFieldChange('fromAddress', 'noreply@example.com');
      onFieldChange('fromName', 'Acme Support');
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return <div data-testid="stub-connection-form" />;
  },
}));

vi.mock('../../components/TrustedIssuerCreateForm', () => ({
  default: function StubTrustedIssuerCreateForm({
    name,
    onNameConflict,
    onBack,
  }: {
    name: string;
    onNameConflict: () => void;
    onBack: () => void;
  }) {
    return (
      <div data-testid="custom-step">
        {name}
        <button type="button" data-testid="custom-step-conflict" onClick={onNameConflict}>
          trigger conflict
        </button>
        <button type="button" data-testid="custom-step-back" onClick={onBack}>
          back
        </button>
      </div>
    );
  },
}));

/** Drives the wizard from the type step through the name step, entering the given name. */
function selectTypeAndName(typeTestId: string, name = 'Acme Connection'): void {
  fireEvent.click(screen.getByTestId(typeTestId));
  fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: name}});
  fireEvent.click(screen.getByTestId('wizard-continue'));
}

describe('ConnectionCreateWizardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mutationState.isPending = false;
    mutationState.isError = false;
  });

  it('shows the type heading without the redundant step label', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.getByText('What kind of connection do you want to add?')).toBeInTheDocument();
    expect(screen.getAllByText('Connection type')).toHaveLength(1);
  });

  it('carries no footer on the type step, advancing on the card click alone', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.queryByTestId('wizard-continue')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('connection-type-option-oidc'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
  });

  it('shows the name step after selecting a type, gated on a non-empty name', () => {
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-oidc'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-continue')).toBeDisabled();

    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'Acme Connection'}});
    expect(screen.getByTestId('wizard-continue')).toBeEnabled();
  });

  it('shows the configure heading without the redundant step label after continuing', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');

    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByText('Configure your connection')).toBeInTheDocument();
    // Assert on the raw step-label key: its real translation ("Configure") collides with the heading text.
    expect(screen.queryByText('wizard.steps.configure')).not.toBeInTheDocument();
  });

  it('shows the generic redirect-URI hint on the configure step', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');

    expect(screen.getByTestId('connection-create-hint')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://id.acme.io/gate/callback')).toBeInTheDocument();
  });

  it('supports selecting the Custom OAuth2 type and configuring it', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oauth');

    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByText('Configure your connection')).toBeInTheDocument();
  });

  it('creates the connection from the configure step with the step-collected name and navigates to its detail page', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc', 'Acme Workforce OIDC');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    const payload = mutateMock.mock.calls[0][0] as {
      name: string;
      redirectUri: string;
      attributeConfiguration?: unknown;
    };
    expect(payload.name).toBe('Acme Workforce OIDC');
    expect(payload.redirectUri).toBe('https://id.acme.io/gate/callback');
    expect('attributeConfiguration' in payload).toBe(false);

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'conn-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/oidc/conn-1');
  });

  it('creates an SMS gateway connection with the selected transport defaults', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-sms-gateway', 'Custom SMS Sender');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Custom SMS Sender',
      url: 'https://sms.example.com/send',
      httpMethod: 'POST',
      contentType: 'JSON',
    });

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'sms-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/sms-gateway/sms-1');
  });

  it('creates an SMTP connection with the transport defaults and a numeric port', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-email-smtp', 'Corp SMTP');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Corp SMTP',
      host: 'smtp.example.com',
      port: 587,
      fromAddress: 'noreply@example.com',
      fromName: 'Acme Support',
      tls: 'starttls',
    });

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'smtp-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/email-smtp/smtp-1');
  });

  it('hides the redirect-URI hint for connection types that do not use one', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-sms-gateway');

    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.queryByTestId('connection-create-hint')).not.toBeInTheDocument();
  });

  it('returns to the name step with a duplicate-name error on a 409 create conflict, without a redundant toast', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 409}});
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
    expect(screen.getByText('A connection with this name already exists.')).toBeInTheDocument();
    expect(showToastMock).not.toHaveBeenCalled();
  });

  it('shows a general inline error on the configure step for a non-conflict create failure', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 500}});
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(screen.getByText('Failed to create connection.')).toBeInTheDocument();
    expect(showToastMock).not.toHaveBeenCalled();
  });

  it('clears the general create error when the connection name is edited on a prior step', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 500}});
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');
    fireEvent.click(screen.getByTestId('wizard-create'));
    expect(screen.getByText('Failed to create connection.')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Back'));
    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'Renamed Connection'}});
    fireEvent.click(screen.getByTestId('wizard-continue'));

    expect(screen.queryByText('Failed to create connection.')).not.toBeInTheDocument();
  });

  it('does not reset a still-pending mutation when a field is populated on the configure step', () => {
    mutationState.isPending = true;
    mutationState.isError = false;
    render(<ConnectionCreateWizardPage />);

    // The stubbed ConnectionForm calls onFieldChange (which runs the same clear-error path as a
    // real field edit) as soon as it mounts on the configure step.
    selectTypeAndName('connection-type-option-oidc');

    expect(resetMock).not.toHaveBeenCalled();
  });

  it('resets a failed (settled) mutation when a field is populated on the configure step', () => {
    mutationState.isPending = false;
    mutationState.isError = true;
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');

    expect(resetMock).toHaveBeenCalled();
  });

  it('renders the baked-in trusted-idp step instead of the generic ConnectionForm', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp', 'My Trusted Issuer');

    expect(screen.getByTestId('custom-step')).toHaveTextContent('My Trusted Issuer');
    expect(screen.queryByTestId('stub-connection-form')).not.toBeInTheDocument();
    expect(screen.queryByTestId('wizard-create')).not.toBeInTheDocument();
  });

  it('lets the trusted-idp step bounce back to the name step via onNameConflict, without a redundant toast', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp');
    fireEvent.click(screen.getByTestId('custom-step-conflict'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
    expect(screen.getByText('A connection with this name already exists.')).toBeInTheDocument();
    expect(showToastMock).not.toHaveBeenCalled();
  });

  it('returns to the name step when Back is clicked from the trusted-idp step', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp');
    fireEvent.click(screen.getByTestId('custom-step-back'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
  });

  it('renders the generic ConnectionForm for a type with no custom configure step', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');

    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.queryByTestId('custom-step')).not.toBeInTheDocument();
  });

  it('shows five type cards including trusted-idp and email-smtp', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.getAllByTestId(/^connection-type-option-/)).toHaveLength(5);
    expect(screen.getByTestId('connection-type-option-trusted-idp')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-email-smtp')).toBeInTheDocument();
  });
});
