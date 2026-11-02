// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {NotificationConfigResponse} from '../../models/connection';
import ConnectionCreateWizardPage from '../ConnectionCreateWizardPage';

const mutateMock = vi.fn();
const resetMock = vi.fn();
const navigateMock = vi.fn();
const showToastMock = vi.fn();
const mutationState = {isPending: false, isError: false, isSuccess: false};
/** Extra form values the stubbed form enters after the defaults, e.g. authentication fields. */
const formExtraValues: Record<string, string> = {};
const ouSelection = {ouId: 'ou-root', setOuId: vi.fn(), needsPicker: false, isLoading: false};

vi.mock('../../hooks/useConnectionOuSelection', () => ({default: () => ouSelection}));
vi.mock('../../components/create-connection/ConnectionOuPickerStep', () => ({
  default: ({
    value,
    onChange,
    onContinue,
  }: {
    value: string;
    onChange: (ouId: string) => void;
    onContinue: () => void;
  }) => (
    <div data-testid="ou-picker-step" data-value={value}>
      <button type="button" data-testid="ou-picker-select" onClick={() => onChange('ou-picked')} />
      <button type="button" data-testid="ou-picker-continue" onClick={onContinue} />
    </div>
  ),
}));

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
const HTTP_METHODS = [
  {type: 'none', displayName: 'None'},
  {
    type: 'basic',
    displayName: 'Username and Password',
    fields: [
      {key: 'username', type: 'string', required: true, displayName: 'Username'},
      {key: 'password', type: 'string', required: true, credential: true, displayName: 'Password'},
    ],
  },
  {
    type: 'bearer',
    displayName: 'Bearer token',
    fields: [{key: 'token', type: 'string', required: true, credential: true, displayName: 'Bearer token'}],
  },
  {
    type: 'api_key',
    displayName: 'API key',
    fields: [],
    additionalProperties: {key: '', type: 'string', required: true, credential: true, displayName: 'API key headers'},
  },
];
vi.mock('../../api/useConnectionMeta', () => ({
  default: () => ({data: {authentication: {methods: HTTP_METHODS}}, isError: false}),
}));
vi.mock('../../api/useCreateConnection', () => ({
  default: () => ({mutate: mutateMock, reset: resetMock, ...mutationState}),
}));
const emailProviders: {isSuccess: boolean; data: {id: string}[] | undefined} = {isSuccess: true, data: [{id: 'sm0'}]};
vi.mock('../../api/useEmailProviders', () => ({default: () => emailProviders}));
const notificationConfig: {isSuccess: boolean; data: NotificationConfigResponse | undefined} = {
  isSuccess: true,
  data: {readOnly: {}, writable: {}, merged: {}},
};
vi.mock('../../api/useGetNotificationConfig', () => ({default: () => notificationConfig}));
const setDefaultSenderMock = vi.fn();
const setDefaultSenderState = {isPending: false};
vi.mock('../../api/useSetDefaultEmailSender', () => ({
  default: () => ({mutate: setDefaultSenderMock, ...setDefaultSenderState}),
}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({
    onFieldChange,
    showAuthentication,
  }: {
    onFieldChange: (name: string, value: string) => void;
    showAuthentication?: boolean;
  }) {
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
      for (const [name, value] of Object.entries(formExtraValues)) {
        onFieldChange(name, value);
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return <div data-testid="stub-connection-form" data-show-authentication={String(showAuthentication)} />;
  },
}));

vi.mock('../../components/TrustedIssuerCreateForm', () => ({
  default: function StubTrustedIssuerCreateForm({
    name,
    ouId,
    onNameConflict,
    onBack,
  }: {
    name: string;
    ouId: string;
    onNameConflict: () => void;
    onBack: () => void;
  }) {
    return (
      <div data-testid="custom-step" data-ou-id={ouId}>
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
    for (const name of Object.keys(formExtraValues)) {
      delete formExtraValues[name];
    }
    mutationState.isPending = false;
    mutationState.isError = false;
    ouSelection.ouId = 'ou-root';
    ouSelection.needsPicker = false;
    ouSelection.isLoading = false;
    mutationState.isSuccess = false;
    setDefaultSenderState.isPending = false;
    emailProviders.isSuccess = true;
    emailProviders.data = [{id: 'sm0'}];
    notificationConfig.isSuccess = true;
    notificationConfig.data = {readOnly: {}, writable: {}, merged: {}};
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

  it('navigates from Policy Decision Point through AuthZEN to the name step', () => {
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-policy-decision-point'));

    expect(screen.getByTestId('select-pdp-protocol')).toBeInTheDocument();
    expect(screen.getByTestId('pdp-protocol-option-authzen')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('pdp-protocol-option-authzen'));
    fireEvent.click(screen.getByTestId('wizard-continue'));

    expect(screen.getByTestId('connection-name-input')).toHaveValue('');
    expect(screen.getByTestId('wizard-continue')).toBeDisabled();

    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'Booking PDP'}});
    fireEvent.click(screen.getByTestId('wizard-continue'));

    expect(screen.getByRole('heading', {name: 'Connection configuration', level: 1})).toBeInTheDocument();
    expect(screen.getByText('Add custom connection')).toBeInTheDocument();
    expect(screen.getByText('Policy decision point (PDP)')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Show hidden breadcrumbs'}));
    expect(screen.getByText('AuthZEN')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toHaveAttribute('data-show-authentication', 'false');
  });

  it('preserves the PDP wizard path when navigating back', () => {
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-policy-decision-point'));
    fireEvent.click(screen.getByTestId('pdp-protocol-option-authzen'));
    fireEvent.click(screen.getByTestId('wizard-continue'));

    fireEvent.click(screen.getByText('Back'));
    expect(screen.getByTestId('select-pdp-protocol')).toBeInTheDocument();
    expect(screen.getByTestId('pdp-protocol-option-authzen')).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(screen.getByText('Back'));
    expect(screen.getByTestId('connection-type-option-policy-decision-point')).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(screen.getByTestId('connection-type-option-policy-decision-point'));
    expect(screen.getByTestId('select-pdp-protocol')).toBeInTheDocument();
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
    expect(payload).toMatchObject({ouId: 'ou-root'});
    expect(payload.redirectUri).toBe('https://id.acme.io/gate/callback');
    expect('attributeConfiguration' in payload).toBe(false);

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'conn-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/oidc/conn-1');
  });

  it('creates an SMS gateway connection with the selected transport defaults', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-sms-gateway', 'Custom SMS Sender');
    expect(screen.getByTestId('stub-connection-form')).toHaveAttribute('data-show-authentication', 'false');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Custom SMS Sender',
      ouId: 'ou-root',
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
      ouId: 'ou-root',
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

  it('creates an HTTP endpoint connection from its name and URL alone', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Risk Engine',
      ouId: 'ou-root',
      url: 'https://sms.example.com/send',
    });

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'http-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/http/http-1');
  });

  it('creates an HTTP endpoint connection with its authentication', () => {
    Object.assign(formExtraValues, {
      'authentication.type': 'basic',
      'authentication.properties.username': 'risk-client',
      'authentication.properties.password': 's3cret',
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Risk Engine',
      ouId: 'ou-root',
      url: 'https://sms.example.com/send',
      authentication: {type: 'basic', properties: {username: 'risk-client', password: 's3cret'}},
    });
  });

  it('creates an HTTP endpoint connection with a bearer token', () => {
    Object.assign(formExtraValues, {
      'authentication.type': 'bearer',
      'authentication.properties.username': 'left-from-basic',
      'authentication.properties.token': 'tok-1',
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Risk Engine',
      ouId: 'ou-root',
      url: 'https://sms.example.com/send',
      authentication: {type: 'bearer', properties: {token: 'tok-1'}},
    });
  });

  it('creates an HTTP endpoint connection with API-key headers', () => {
    Object.assign(formExtraValues, {
      'authentication.type': 'api_key',
      'authentication.additionalProperties': JSON.stringify([
        {name: 'X-Api-Key', value: 'k-1'},
        {name: 'X-Client', value: 'c-1'},
      ]),
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock.mock.calls[0][0]).toEqual({
      name: 'Risk Engine',
      ouId: 'ou-root',
      url: 'https://sms.example.com/send',
      authentication: {type: 'api_key', properties: {'X-Api-Key': 'k-1', 'X-Client': 'c-1'}},
    });
  });

  it('blocks creating an HTTP endpoint with a half-filled or missing API-key header', () => {
    Object.assign(formExtraValues, {
      'authentication.type': 'api_key',
      'authentication.additionalProperties': JSON.stringify([{name: 'X-Api-Key', value: ''}]),
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');

    expect(screen.getByTestId('wizard-create')).toBeDisabled();
  });

  it('blocks creating an HTTP endpoint while a required authentication field is empty', () => {
    Object.assign(formExtraValues, {
      'authentication.type': 'basic',
      'authentication.properties.username': 'risk-client',
      'authentication.properties.password': '  ',
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');

    expect(screen.getByTestId('wizard-create')).toBeDisabled();
  });

  it('blocks creating an HTTP endpoint with bearer authentication and no token', () => {
    Object.assign(formExtraValues, {'authentication.type': 'bearer'});
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');

    expect(screen.getByTestId('wizard-create')).toBeDisabled();
  });

  it('blocks creating an HTTP endpoint with API-key authentication over http', () => {
    Object.assign(formExtraValues, {
      url: 'http://risk.example.com/check',
      'authentication.type': 'api_key',
      'authentication.additionalProperties': JSON.stringify([{name: 'X-Api-Key', value: 'k-1'}]),
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');

    expect(screen.getByTestId('wizard-create')).toBeDisabled();
  });

  it('blocks creating an HTTP endpoint with authentication over http', () => {
    Object.assign(formExtraValues, {
      url: 'http://risk.example.com/check',
      'authentication.type': 'basic',
      'authentication.properties.username': 'risk-client',
      'authentication.properties.password': 's3cret',
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');

    expect(screen.getByTestId('wizard-create')).toBeDisabled();
  });

  it('explains and blocks an HTTP endpoint name longer than 255 characters', () => {
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-http'));
    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'n'.repeat(256)}});

    expect(screen.getByText('Enter a name of at most 255 characters.')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-continue')).toBeDisabled();

    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'n'.repeat(255)}});
    expect(screen.queryByText('Enter a name of at most 255 characters.')).not.toBeInTheDocument();
    expect(screen.getByTestId('wizard-continue')).not.toBeDisabled();
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

  it('creates an OAuth 2 connection owned by the selected organization unit', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oauth', 'Acme OAuth');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock.mock.calls[0][0]).toMatchObject({name: 'Acme OAuth', ouId: 'ou-root'});
  });

  it('creates an AuthZEN PDP connection owned by the selected organization unit', () => {
    Object.assign(formExtraValues, {endpoint: 'https://pdp.example.com/access/v1/evaluation'});
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-policy-decision-point'));
    fireEvent.click(screen.getByTestId('pdp-protocol-option-authzen'));
    fireEvent.click(screen.getByTestId('wizard-continue'));
    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'Booking PDP'}});
    fireEvent.click(screen.getByTestId('wizard-continue'));
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock.mock.calls[0][0]).toMatchObject({name: 'Booking PDP', ouId: 'ou-root'});
  });

  it('passes the default organization unit to the trusted issuer step', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp');

    expect(screen.getByTestId('custom-step')).toHaveAttribute('data-ou-id', 'ou-root');
  });

  it('sends the selected organization unit as ouId', () => {
    ouSelection.ouId = 'ou-acme';
    render(<ConnectionCreateWizardPage />);

    expect(screen.queryByTestId('ou-picker-step')).not.toBeInTheDocument();
    expect(screen.getByText('What kind of connection do you want to add?')).toBeInTheDocument();
    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock.mock.calls[0][0]).toMatchObject({name: 'Risk Engine', ouId: 'ou-acme'});
  });

  it('asks for an organization unit before the wizard when there are several', () => {
    ouSelection.ouId = '';
    ouSelection.needsPicker = true;
    render(<ConnectionCreateWizardPage />);

    expect(screen.getByTestId('ou-picker-step')).toBeInTheDocument();
    expect(screen.queryByText('What kind of connection do you want to add?')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('ou-picker-select'));
    expect(ouSelection.setOuId).toHaveBeenCalledWith('ou-picked');

    ouSelection.ouId = 'ou-picked';
    fireEvent.click(screen.getByTestId('ou-picker-continue'));

    expect(screen.getByText('What kind of connection do you want to add?')).toBeInTheDocument();
    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));
    expect(mutateMock.mock.calls[0][0]).toMatchObject({name: 'Risk Engine', ouId: 'ou-picked'});
  });

  it('passes the selected organization unit to the trusted issuer step', () => {
    ouSelection.ouId = 'ou-acme';
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp');

    expect(screen.getByTestId('custom-step')).toHaveAttribute('data-ou-id', 'ou-acme');
  });

  it('reports a missing organization unit instead of creating', () => {
    ouSelection.ouId = '';
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-http', 'Risk Engine');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(screen.getByText('Organization unit is required')).toBeInTheDocument();
    expect(mutateMock).not.toHaveBeenCalled();
  });

  it('shows no wizard while organization units load', () => {
    ouSelection.ouId = '';
    ouSelection.isLoading = true;
    render(<ConnectionCreateWizardPage />);

    expect(screen.queryByText('What kind of connection do you want to add?')).not.toBeInTheDocument();
    expect(screen.getByRole('progressbar', {name: 'Loading...'})).toBeInTheDocument();
  });

  it('shows seven type cards including trusted-idp, policy decision point, email-smtp and http', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.getAllByTestId(/^connection-type-option-/)).toHaveLength(7);
    expect(screen.getByTestId('connection-type-option-http')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-trusted-idp')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-policy-decision-point')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-email-smtp')).toBeInTheDocument();
  });

  describe('first email sender', () => {
    function createSender(type = 'email-smtp'): void {
      render(<ConnectionCreateWizardPage />);
      selectTypeAndName(`connection-type-option-${type}`, 'Corp Sender');
      fireEvent.click(screen.getByTestId('wizard-create'));
      const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
      onSuccess({id: 'new-1'});
    }

    beforeEach(() => {
      emailProviders.data = [];
    });

    it('makes the only email sender the default, then opens it', () => {
      createSender();

      expect(setDefaultSenderMock).toHaveBeenCalledWith({defaultEmailSenderId: 'new-1'}, expect.any(Object));
      expect(navigateMock).not.toHaveBeenCalled();

      const {onSuccess} = setDefaultSenderMock.mock.calls[0][1] as {onSuccess: () => void};
      onSuccess();
      expect(navigateMock).toHaveBeenCalledWith('/connections/email-smtp/new-1');
    });

    it('warns and still opens the sender when it cannot be made the default', () => {
      createSender();

      const {onError} = setDefaultSenderMock.mock.calls[0][1] as {onError: () => void};
      onError();
      expect(showToastMock).toHaveBeenCalledWith(
        'Connection created, but it could not be made the default email sender.',
        'warning',
      );
      expect(navigateMock).toHaveBeenCalledWith('/connections/email-smtp/new-1');
    });

    it('leaves the default alone when another email sender exists', () => {
      emailProviders.data = [{id: 'sm0'}];
      createSender();

      expect(setDefaultSenderMock).not.toHaveBeenCalled();
      expect(navigateMock).toHaveBeenCalledWith('/connections/email-smtp/new-1');
    });

    it('leaves the default alone when the deployment configuration fixes it', () => {
      notificationConfig.data = {
        readOnly: {defaultEmailSenderId: 'file'},
        writable: {},
        merged: {defaultEmailSenderId: 'file'},
      };
      createSender();

      expect(setDefaultSenderMock).not.toHaveBeenCalled();
    });

    it('leaves the default alone when the existing senders are not known', () => {
      emailProviders.isSuccess = false;
      emailProviders.data = undefined;
      createSender();

      expect(setDefaultSenderMock).not.toHaveBeenCalled();
    });

    it('leaves the default alone when the notification section is not known', () => {
      notificationConfig.isSuccess = false;
      notificationConfig.data = undefined;
      createSender();

      expect(setDefaultSenderMock).not.toHaveBeenCalled();
    });

    // The create succeeded, so a second click would create a duplicate.
    it('keeps create disabled after the connection is created', () => {
      mutationState.isSuccess = true;
      render(<ConnectionCreateWizardPage />);
      selectTypeAndName('connection-type-option-email-smtp', 'Corp Sender');

      expect(screen.getByTestId('wizard-create')).toBeDisabled();
    });

    it('keeps create disabled while the new sender is being made the default', () => {
      setDefaultSenderState.isPending = true;
      render(<ConnectionCreateWizardPage />);
      selectTypeAndName('connection-type-option-email-smtp', 'Corp Sender');

      expect(screen.getByTestId('wizard-create')).toBeDisabled();
    });

    it('never makes a connection that is not an email sender the default', () => {
      createSender('http');

      expect(setDefaultSenderMock).not.toHaveBeenCalled();
      expect(navigateMock).toHaveBeenCalledWith('/connections/http/new-1');
    });
  });
});
