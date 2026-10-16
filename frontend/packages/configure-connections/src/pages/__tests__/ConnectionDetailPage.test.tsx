// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {type ReactNode, useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {OutboundAuthMethod} from '../../models/connection';
import ConnectionDetailPage from '../ConnectionDetailPage';

const updateMock = vi.fn().mockResolvedValue({});
const updateResetMock = vi.fn();
const refetchMock = vi.fn().mockResolvedValue({});
const deleteMock = vi.fn((_id: string, opts: {onSuccess: () => void}) => opts.onSuccess());
const navigateMock = vi.fn();
const updateMutationState = {isPending: false, isError: false};

const ATTR_CONFIG = {
  userTypeResolution: {default: 'employee'},
  userTypeAttributeMappings: [
    {userType: 'employee', attributes: [{externalAttribute: 'email', localAttribute: 'mail'}]},
  ],
};

const AUTHZ_MAPPINGS = [
  {claim: 'groups', values: [{operator: 'equals', value: 'engineering', targets: [{type: 'role', id: 'role-1'}]}]},
];

const DIRECT_MAPPINGS = [{claim: 'role_name', targetType: 'role'}];

const LINKING = {attributes: ['email']};

const CONNECTION = {
  id: 'g1',
  type: 'google',
  name: 'Google',
  clientId: 'cid',
  clientSecret: '******',
  redirectUri: 'https://id.acme.io/oauth/callback/google',
  scopes: ['openid'],
  attributeConfiguration: {
    ...ATTR_CONFIG,
    authorizationMapping: {rules: AUTHZ_MAPPINGS, direct: DIRECT_MAPPINGS},
    accountLinking: LINKING,
  },
};

const TWILIO_CONNECTION = {
  id: 'tw1',
  type: 'twilio',
  name: 'Twilio',
  accountSid: 'AC00000000000000000000000000000000',
  authToken: '******',
  senderId: '+15005550006',
};

const OIDC_CONNECTION = {
  id: 'oidc1',
  type: 'oidc',
  name: 'Acme Workforce OIDC',
  clientId: 'cid',
  clientSecret: '******',
  authorizationEndpoint: 'https://idp.example.com/authorize',
  tokenEndpoint: 'https://idp.example.com/token',
  redirectUri: 'https://id.acme.io/oauth/callback/oidc',
};

const AUTHZEN_PDP_CONNECTION = {
  id: 'pdp1',
  type: 'authzen-pdp',
  name: 'AuthZEN PDP',
  endpoint: 'https://pdp.example.com/.well-known/authzen-configuration',
  batchEndpoint: 'https://pdp.example.com/access/v1/evaluations',
  timeoutMs: 500,
  retryCount: 1,
  subjectAttributeMappings: [
    {
      entityType: 'employee',
      attributes: [{attribute: 'email'}, {attribute: 'department', pdpAttribute: 'dept'}],
    },
  ],
};

const AUTHZEN_PDP_BEARER_CONNECTION = {
  ...AUTHZEN_PDP_CONNECTION,
  authentication: {
    scheme: 'BEARER',
    bearer: {token: '******'},
  },
};

const AUTHZEN_PDP_BEARER_WITHOUT_TOKEN = {
  ...AUTHZEN_PDP_CONNECTION,
  authentication: {
    scheme: 'BEARER',
    bearer: {token: ''},
  },
};

const AUTHZEN_PDP_API_KEY_WITH_NULL_HEADERS = {
  ...AUTHZEN_PDP_CONNECTION,
  authentication: {
    scheme: 'API_KEY',
    apiKey: {headers: null},
  },
};

const HTTP_CONNECTION = {
  id: 'http1',
  type: 'http',
  name: 'Risk Engine',
  url: 'https://risk.example.com/evaluate',
  timeoutMs: 5000,
  headers: [{name: 'X-Tenant', value: 'acme'}],
  authentication: {type: 'basic', properties: {username: 'svc', password: '******'}},
};

const HTTP_NONE_CONNECTION = {
  ...HTTP_CONNECTION,
  authentication: undefined,
};

const HTTP_LOCAL_CONNECTION = {
  ...HTTP_NONE_CONNECTION,
  url: 'http://localhost:8090/evaluate',
};

const ORIGIN_CHANGED_HINT = 'The URL origin changed. Enter the credentials again under Authentication before saving.';
const HTTPS_REQUIRED = 'Use an https URL when an authentication method is set, or set the method to None.';
const PASSWORD_FIELD_ID = 'connection-field-authentication.properties.password';
const USERNAME_FIELD_ID = 'connection-field-authentication.properties.username';

const mockParams: {type: string; id: string} = {type: 'google', id: 'g1'};
const mockConn: {data: Record<string, unknown>} = {data: CONNECTION};
const mockMeta: {methods: OutboundAuthMethod[]; isError: boolean} = {methods: [], isError: false};

const SMTP_METHODS: OutboundAuthMethod[] = [
  {type: 'none', displayName: 'None'},
  {
    type: 'basic',
    displayName: 'Username and Password',
    fields: [
      {key: 'username', type: 'string', required: true, displayName: 'Username'},
      {key: 'password', type: 'string', required: true, credential: true, displayName: 'Password'},
    ],
  },
];

function smtpConnection(properties: Record<string, string>): Record<string, unknown> {
  return {
    id: 'sm1',
    type: 'email-smtp',
    name: 'Corp SMTP',
    host: 'smtp.example.com',
    port: 587,
    fromAddress: 'noreply@example.com',
    tls: 'starttls',
    authentication: {type: 'basic', properties},
  };
}

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
  useParams: () => mockParams,
}));
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({
    config: {brand: {product_name: 'ThunderID'}},
    getGateCallbackUrl: () => 'https://id.acme.io/gate/callback',
  }),
  useToast: () => ({showToast: vi.fn()}),
}));
vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  SettingsCard: ({title, children}: {title: string; children: ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
  UnsavedChangesBar: ({
    onSave,
    saveLabel,
    saveDisabled,
  }: {
    onSave: () => void;
    saveLabel: string;
    saveDisabled: boolean;
  }) => (
    <button type="button" data-testid="save-bar" onClick={onSave} disabled={saveDisabled}>
      {saveLabel}
    </button>
  ),
}));

vi.mock('../../api/useConnection', () => ({
  default: () => ({data: mockConn.data, isLoading: false, isError: false, refetch: refetchMock}),
}));
vi.mock('../../api/useConnectionMeta', () => ({
  default: () =>
    mockMeta.isError
      ? {data: undefined, isError: true}
      : {data: {authentication: {methods: mockMeta.methods}}, isError: false},
}));
vi.mock('../../api/useConnectionInstances', () => ({default: () => ({data: [], isLoading: false})}));
vi.mock('../../api/useUpdateConnection', () => ({
  default: () => ({mutateAsync: updateMock, reset: updateResetMock, ...updateMutationState}),
}));
vi.mock('../../api/useDeleteConnection', () => ({default: () => ({mutate: deleteMock, isPending: false})}));
vi.mock('../../api/useGetConnectionUsages', () => ({
  default: () => ({data: {totalResults: 0, count: 0, summary: {}, usages: []}, isLoading: false}),
}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({
    onFieldChange,
    nameError,
  }: {
    onFieldChange: (name: string, value: string) => void;
    nameError?: string | null;
  }) {
    return (
      <div data-testid="stub-connection-form">
        <button type="button" data-testid="edit-client-id" onClick={() => onFieldChange('clientId', 'changed')}>
          edit
        </button>
        <button type="button" data-testid="edit-name" onClick={() => onFieldChange('name', 'Renamed')}>
          edit name
        </button>
        <button
          type="button"
          data-testid="edit-url-path"
          onClick={() => onFieldChange('url', 'https://risk.example.com/v2/evaluate')}
        >
          edit url path
        </button>
        <button
          type="button"
          data-testid="edit-url-host"
          onClick={() => onFieldChange('url', 'https://collector.example.net/evaluate')}
        >
          edit url host
        </button>
        <button
          type="button"
          data-testid="edit-url-back"
          onClick={() => onFieldChange('url', 'https://risk.example.com/evaluate')}
        >
          edit url back
        </button>
        {nameError && <div data-testid="stub-name-error">{nameError}</div>}
      </div>
    );
  },
}));

vi.mock('../../components/AttributeMappingSection', () => ({
  default: function StubAttributeMappingSection({onChange}: {onChange: (c: unknown, v: boolean) => void}) {
    useEffect(() => {
      onChange(ATTR_CONFIG, true);
    }, [onChange]);
    return <div data-testid="stub-attribute-mapping" />;
  },
}));

// Fires onChange only on an explicit click, so tests that never click it leave the section untouched
// (editedAuthzMappings/editedLinking stay null, falling back to the baseline), matching how a real user
// who never opens the section behaves.
vi.mock('../../components/AuthorizationMappingSection', () => ({
  default: function StubAuthorizationMappingSection({
    onRuleChange,
    onDirectChange,
  }: {
    onRuleChange: (c: unknown, v: boolean) => void;
    onDirectChange: (c: unknown, v: boolean) => void;
  }) {
    return (
      <>
        <button type="button" data-testid="clear-authz-mappings" onClick={() => onRuleChange(undefined, true)}>
          clear authorization mappings
        </button>
        <button type="button" data-testid="clear-direct-mappings" onClick={() => onDirectChange(undefined, true)}>
          clear direct mappings
        </button>
        {/* Simulates an attempted-but-incomplete row: the serialized value is unchanged from baseline
            (nothing to diff), but the section reports itself invalid, the same way an incomplete
            AuthorizationMappingSection row does. */}
        <button
          type="button"
          data-testid="report-incomplete-authz-row"
          onClick={() => onRuleChange(AUTHZ_MAPPINGS, false)}
        >
          report incomplete row
        </button>
      </>
    );
  },
}));

vi.mock('../../components/AccountLinkingSection', () => ({
  default: function StubAccountLinkingSection({onChange}: {onChange: (c: unknown) => void}) {
    return (
      <button type="button" data-testid="clear-linking" onClick={() => onChange(undefined)}>
        clear account linking
      </button>
    );
  },
}));

vi.mock('../../components/SubjectMappingSection', () => ({
  default: function StubSubjectMappingSection({
    values,
    onChange,
  }: {
    values: {
      subjectAttributeMappings?: {
        entityType: string;
        attributes: {attribute: string}[];
      }[];
    };
    onChange: (field: 'subjectAttributeMappings', value: unknown[]) => void;
  }) {
    return (
      <div data-testid="stub-subject-mapping">
        <span>{values.subjectAttributeMappings?.[0]?.attributes.map(({attribute}) => attribute).join(' ')}</span>
        <button
          type="button"
          data-testid="edit-subject-mapping"
          onClick={() =>
            onChange('subjectAttributeMappings', [
              {
                entityType: 'employee',
                attributes: [{attribute: 'email'}, {attribute: 'riskScore'}],
              },
            ])
          }
        >
          edit subject mapping
        </button>
        <button
          type="button"
          data-testid="duplicate-subject-mapping"
          onClick={() =>
            onChange('subjectAttributeMappings', [
              {
                entityType: 'employee',
                attributes: [{attribute: 'email'}, {attribute: 'email'}],
              },
            ])
          }
        >
          duplicate subject mapping
        </button>
        <button
          type="button"
          data-testid="restore-subject-mapping"
          onClick={() =>
            onChange('subjectAttributeMappings', [
              {
                entityType: 'employee',
                attributes: [{attribute: 'email'}, {attribute: 'department', pdpAttribute: 'dept'}],
              },
            ])
          }
        >
          restore subject mapping
        </button>
      </div>
    );
  },
}));

describe('ConnectionDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.type = 'google';
    mockParams.id = 'g1';
    mockConn.data = CONNECTION;
    updateMutationState.isPending = false;
    updateMutationState.isError = false;
    mockMeta.methods = [];
    mockMeta.isError = false;
  });

  it('renders the general tab with quick-copy and the credentials form', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByTestId('connection-id-copy')).toBeInTheDocument();
    expect(screen.getByDisplayValue('g1')).toBeInTheDocument();
    expect(screen.getByText('Unique identifier for this connection.')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
  });

  it('renders the connection form in the Credentials card', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByRole('region', {name: 'Credentials'})).toContainElement(
      screen.getByTestId('stub-connection-form'),
    );
  });

  it('omits the Authentication card when the vendor advertises no authentication methods', () => {
    render(<ConnectionDetailPage />);
    expect(screen.queryByRole('region', {name: 'Authentication'})).not.toBeInTheDocument();
  });

  it('renders the authentication section in its own card when the vendor advertises methods', () => {
    mockMeta.methods = [{type: 'none', displayName: 'None'}];
    render(<ConnectionDetailPage />);
    expect(screen.getByRole('region', {name: 'Authentication'})).toContainElement(
      screen.getByTestId('connection-authentication-section'),
    );
  });

  // A provider saved with a credential comes back masked, so the password stays locked until the
  // user chooses to replace it.
  it('keeps the stored password of an email provider locked', () => {
    mockParams.type = 'email-smtp';
    mockParams.id = 'sm1';
    mockConn.data = smtpConnection({username: 'mailer', password: '******'});
    mockMeta.methods = SMTP_METHODS;
    render(<ConnectionDetailPage />);
    expect(document.getElementById('connection-field-authentication.properties.password')).toBeDisabled();
    expect(screen.getByTestId('connection-field-authentication.properties.password-replace')).toBeInTheDocument();
  });

  // Nothing stored for the selected method means the password has to be entered, not shown as kept.
  it('asks for an email provider password that is not stored', () => {
    mockParams.type = 'email-smtp';
    mockParams.id = 'sm1';
    mockConn.data = smtpConnection({username: 'mailer'});
    mockMeta.methods = SMTP_METHODS;
    render(<ConnectionDetailPage />);
    expect(document.getElementById('connection-field-authentication.properties.password')).not.toBeDisabled();
    expect(screen.queryByTestId('connection-field-authentication.properties.password-replace')).not.toBeInTheDocument();
  });

  // The authentication card reports a failed metadata read instead of disappearing.
  it('reports a failed metadata read in the Authentication card', () => {
    mockMeta.isError = true;
    render(<ConnectionDetailPage />);
    expect(screen.getByRole('region', {name: 'Authentication'})).toContainElement(screen.getByRole('alert'));
    expect(screen.queryByTestId('connection-authentication-section')).not.toBeInTheDocument();
  });

  it('renders the danger-zone delete on the advanced tab', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-advanced'));
    expect(screen.getByTestId('connection-delete-button')).toBeInTheDocument();
  });

  it('shows the Configured status under the name in the card style, with no redundant subtitle', () => {
    render(<ConnectionDetailPage />);
    expect(screen.getByText('Google')).toBeInTheDocument();
    expect(screen.getByText('Configured')).toBeInTheDocument();
    expect(screen.queryByText('Google connection')).not.toBeInTheDocument();
    expect(document.querySelector('.MuiChip-root')).not.toBeInTheDocument();
  });

  it('hides the save bar until a field is edited', () => {
    render(<ConnectionDetailPage />);
    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId('edit-client-id'));
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();
  });

  it('saves the merged payload, preserves stored attribute mappings, refetches, then clears the dirty bar', async () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as {name: string; clientId: string; attributeConfiguration?: unknown};
    expect(payload).toMatchObject({name: 'Google', clientId: 'changed'});
    // General-tab-only edit must not wipe the stored attribute configuration, including both halves
    // of the nested authorization mapping (rule-based and direct) and account linking, none of which
    // this edit touched.
    expect(payload.attributeConfiguration).toEqual({
      ...ATTR_CONFIG,
      authorizationMapping: {rules: AUTHZ_MAPPINGS, direct: DIRECT_MAPPINGS},
      accountLinking: LINKING,
    });

    await waitFor(() => expect(refetchMock).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument());
  });

  it('omits authorization mappings and account linking from the save payload once cleared, instead of reverting to the stored value', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-attributes'));
    fireEvent.click(screen.getByTestId('clear-authz-mappings'));
    fireEvent.click(screen.getByTestId('clear-direct-mappings'));
    fireEvent.click(screen.getByTestId('clear-linking'));
    fireEvent.click(screen.getByTestId('save-bar'));

    const payload = updateMock.mock.calls[0][0] as {
      attributeConfiguration?: {
        authorizationMapping?: unknown;
        accountLinking?: unknown;
      };
    };
    expect(payload.attributeConfiguration?.authorizationMapping).toBeUndefined();
    expect(payload.attributeConfiguration?.accountLinking).toBeUndefined();
  });

  it('clearing only the rule-based half keeps the stored direct mappings nested alongside it', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-attributes'));
    fireEvent.click(screen.getByTestId('clear-authz-mappings'));
    fireEvent.click(screen.getByTestId('save-bar'));

    const payload = updateMock.mock.calls[0][0] as {
      attributeConfiguration?: {authorizationMapping?: {rules?: unknown; direct?: unknown}};
    };
    expect(payload.attributeConfiguration?.authorizationMapping).toEqual({direct: DIRECT_MAPPINGS});
  });

  it('shows the save bar for an attempted-but-incomplete authorization mapping row, and still blocks the save', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-attributes'));

    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('report-incomplete-authz-row'));

    // Nothing in the serialized payload changed (the incomplete row contributes nothing to it), so
    // this only shows up as an invalid section, not a diff - the save bar must still appear for it.
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('save-bar'));
    expect(updateMock).not.toHaveBeenCalled();
  });

  it('shows an inline name error on a 409 update conflict, and clears it when the name is edited', async () => {
    mockParams.type = 'oidc';
    mockParams.id = 'oidc1';
    mockConn.data = OIDC_CONNECTION;
    updateMock.mockRejectedValueOnce({response: {status: 409}});
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    await waitFor(() =>
      expect(screen.getByTestId('stub-name-error')).toHaveTextContent('A connection with this name already exists.'),
    );

    fireEvent.click(screen.getByTestId('edit-name'));
    expect(screen.queryByTestId('stub-name-error')).not.toBeInTheDocument();
  });

  it('resets a failed (settled) update mutation as soon as a field is edited', () => {
    updateMutationState.isError = true;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(updateResetMock).toHaveBeenCalled();
  });

  it('does not reset a still-pending update mutation when a field is edited', () => {
    updateMutationState.isPending = true;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(updateResetMock).not.toHaveBeenCalled();
  });

  it('deletes the connection and returns to the list', () => {
    render(<ConnectionDetailPage />);
    fireEvent.click(screen.getByTestId('connection-tab-advanced'));
    fireEvent.click(screen.getByTestId('connection-delete-button'));
    fireEvent.click(screen.getByTestId('connection-delete-confirm'));
    expect(deleteMock).toHaveBeenCalledWith('g1', expect.anything());
    expect(navigateMock).toHaveBeenCalledWith('/connections');
  });

  it('SMS vendor: hides the attribute-mapping tab and save omits attributeConfiguration', () => {
    mockParams.type = 'twilio';
    mockParams.id = 'tw1';
    mockConn.data = TWILIO_CONNECTION;
    render(<ConnectionDetailPage />);

    expect(screen.getByTestId('connection-tab-general')).toBeInTheDocument();
    expect(screen.queryByTestId('connection-tab-attributes')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('edit-client-id'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'Twilio',
      accountSid: 'AC00000000000000000000000000000000',
      senderId: '+15005550006',
    });
    expect('attributeConfiguration' in payload).toBe(false);
  });

  it('AuthZEN PDP: shows subject mapping in its own tab and saves it with the connection', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_CONNECTION;
    render(<ConnectionDetailPage />);

    expect(screen.getByTestId('connection-tab-subject-mapping')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('connection-tab-subject-mapping'));
    expect(screen.getByTestId('stub-subject-mapping')).toHaveTextContent('email department');

    fireEvent.click(screen.getByTestId('edit-subject-mapping'));
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock).toHaveBeenCalledTimes(1);
    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'AuthZEN PDP',
      endpoint: 'https://pdp.example.com/.well-known/authzen-configuration',
      batchEndpoint: 'https://pdp.example.com/access/v1/evaluations',
      subjectAttributeMappings: [
        {
          entityType: 'employee',
          attributes: [{attribute: 'email'}, {attribute: 'riskScore'}],
        },
      ],
    });
    expect('resourceType' in payload).toBe(false);
    expect('resourceId' in payload).toBe(false);
    expect('authentication' in payload).toBe(false);
  });

  it('AuthZEN PDP: disables save while one entity type has duplicate attributes', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_CONNECTION;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-subject-mapping'));
    fireEvent.click(screen.getByTestId('duplicate-subject-mapping'));

    expect(screen.getByTestId('save-bar')).toBeDisabled();
  });

  it('AuthZEN PDP: becomes clean after a subject mapping edit is undone', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = {
      ...AUTHZEN_PDP_CONNECTION,
      subjectAttributeMappings: [
        ...AUTHZEN_PDP_CONNECTION.subjectAttributeMappings,
        {entityType: 'assistant', attributes: []},
      ],
    };
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-subject-mapping'));
    fireEvent.click(screen.getByTestId('edit-subject-mapping'));
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('restore-subject-mapping'));
    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();
  });

  it('AuthZEN PDP: displays authentication separately from connection configuration', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_CONNECTION;
    render(<ConnectionDetailPage />);

    expect(screen.getByLabelText('Connection Configuration')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    expect(screen.getByText('Authentication method')).toBeInTheDocument();
    expect(screen.getByText('None')).toBeInTheDocument();
  });

  it('AuthZEN PDP: becomes clean after API-key headers are entered and the scheme returns to None', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_CONNECTION;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'API key'}));
    fireEvent.change(document.getElementById('connection-field-httpHeaders-name-1')!, {
      target: {value: 'X-API-Key'},
    });
    fireEvent.change(document.getElementById('connection-field-httpHeaders-value-1')!, {
      target: {value: 'secret'},
    });
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'None'}));

    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();
  });

  it('AuthZEN PDP: handles a stored API key response with null headers', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_API_KEY_WITH_NULL_HEADERS;

    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('edit-name'));
    expect(screen.getByTestId('save-bar')).toBeDisabled();
    expect(screen.getByTestId('connection-tab-authentication')).toHaveAttribute('aria-invalid', 'true');

    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    expect(screen.getByText('This field is required.')).toBeInTheDocument();
  });

  it('AuthZEN PDP: preserves stored authentication when saving unrelated changes', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_BEARER_CONNECTION;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-subject-mapping'));
    fireEvent.click(screen.getByTestId('edit-subject-mapping'));
    fireEvent.click(screen.getByTestId('save-bar'));

    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty('authentication');
  });

  it('AuthZEN PDP: submits a structured Bearer credential replacement', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_BEARER_CONNECTION;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    fireEvent.click(screen.getByRole('button', {name: 'Update'}));
    fireEvent.change(document.getElementById('connection-field-bearerToken')!, {target: {value: 'new-token'}});
    fireEvent.click(screen.getByTestId('save-bar'));

    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({authentication: {scheme: 'BEARER', bearer: {token: 'new-token'}}});
    expect(payload).not.toHaveProperty('authenticationScheme');
    expect(payload).not.toHaveProperty('bearerToken');
  });

  it('AuthZEN PDP: becomes clean after entering and clearing an unstored Bearer token', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_BEARER_WITHOUT_TOKEN;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    const bearerToken = document.getElementById('connection-field-bearerToken')!;
    fireEvent.change(bearerToken, {target: {value: 'temporary-token'}});
    expect(screen.getByTestId('save-bar')).toBeInTheDocument();

    fireEvent.change(bearerToken, {target: {value: ''}});
    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();
  });

  it('AuthZEN PDP: submits an entered Bearer token when no credential is stored', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_BEARER_WITHOUT_TOKEN;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    fireEvent.change(document.getElementById('connection-field-bearerToken')!, {target: {value: 'new-token'}});
    fireEvent.click(screen.getByTestId('save-bar'));

    expect(updateMock.mock.calls[0][0]).toMatchObject({
      authentication: {scheme: 'BEARER', bearer: {token: 'new-token'}},
    });
  });

  it('AuthZEN PDP: submits NONE when stored authentication is removed', () => {
    mockParams.type = 'authzen-pdp';
    mockParams.id = 'pdp1';
    mockConn.data = AUTHZEN_PDP_BEARER_CONNECTION;
    render(<ConnectionDetailPage />);

    fireEvent.click(screen.getByTestId('connection-tab-authentication'));
    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'None'}));
    fireEvent.click(screen.getByTestId('save-bar'));

    const payload = updateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({authentication: {scheme: 'NONE'}});
  });

  describe('HTTP endpoint', () => {
    beforeEach(() => {
      mockParams.type = 'http';
      mockParams.id = 'http1';
      mockConn.data = HTTP_CONNECTION;
      mockMeta.methods = SMTP_METHODS;
    });

    function selectMethod(name: string): void {
      fireEvent.mouseDown(screen.getByRole('combobox'));
      fireEvent.click(screen.getByRole('option', {name}));
    }

    function field(id: string): HTMLElement {
      return document.getElementById(id)!;
    }

    it('edits authentication in the General tab from the vendor metadata', () => {
      render(<ConnectionDetailPage />);

      expect(screen.getByLabelText('Connection Configuration')).toBeInTheDocument();
      expect(screen.getByRole('region', {name: 'Authentication'})).toContainElement(
        screen.getByTestId('connection-authentication-section'),
      );
      expect(screen.queryByTestId('connection-tab-authentication')).not.toBeInTheDocument();
      expect(screen.queryByTestId('connection-tab-subject-mapping')).not.toBeInTheDocument();
      expect(field(PASSWORD_FIELD_ID)).toBeDisabled();
    });

    it('keeps the stored password by omitting it when saving unrelated changes', () => {
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-name'));
      fireEvent.click(screen.getByTestId('save-bar'));

      expect(updateMock.mock.calls[0][0]).toEqual({
        name: 'Renamed',
        url: 'https://risk.example.com/evaluate',
        timeoutMs: 5000,
        headers: [{name: 'X-Tenant', value: 'acme'}],
        authentication: {type: 'basic', properties: {username: 'svc'}},
      });
    });

    it('keeps the stored password when only the URL path changes', () => {
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-path'));

      expect(screen.queryByText(ORIGIN_CHANGED_HINT)).not.toBeInTheDocument();
      expect(field(PASSWORD_FIELD_ID)).toBeDisabled();
      fireEvent.click(screen.getByTestId('save-bar'));
      expect(updateMock.mock.calls[0][0]).toMatchObject({
        url: 'https://risk.example.com/v2/evaluate',
        authentication: {type: 'basic', properties: {username: 'svc'}},
      });
    });

    it('blocks Save after a URL origin change until the password is entered again', () => {
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-host'));

      expect(screen.getByTestId('save-bar')).toBeDisabled();
      expect(screen.getByText(ORIGIN_CHANGED_HINT)).toBeInTheDocument();
      expect(field(PASSWORD_FIELD_ID)).not.toBeDisabled();

      fireEvent.change(field(PASSWORD_FIELD_ID), {target: {value: 'new-pw'}});
      expect(screen.getByTestId('save-bar')).not.toBeDisabled();
      fireEvent.click(screen.getByTestId('save-bar'));

      expect(updateMock.mock.calls[0][0]).toMatchObject({
        url: 'https://collector.example.net/evaluate',
        authentication: {type: 'basic', properties: {username: 'svc', password: 'new-pw'}},
      });
    });

    it('keeps a password typed during the origin change editable after the URL is changed back', () => {
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-host'));
      fireEvent.change(field(PASSWORD_FIELD_ID), {target: {value: 'new-pw'}});
      fireEvent.click(screen.getByTestId('edit-url-back'));

      expect(screen.queryByText(ORIGIN_CHANGED_HINT)).not.toBeInTheDocument();
      expect(field(PASSWORD_FIELD_ID)).not.toBeDisabled();
      expect(field(PASSWORD_FIELD_ID)).toHaveValue('new-pw');

      fireEvent.change(field(PASSWORD_FIELD_ID), {target: {value: 'newer-pw'}});
      fireEvent.click(screen.getByTestId('save-bar'));
      expect(updateMock.mock.calls[0][0]).toMatchObject({
        url: 'https://risk.example.com/evaluate',
        authentication: {type: 'basic', properties: {username: 'svc', password: 'newer-pw'}},
      });
    });

    it('asks for the password again when the method is switched away and back after an origin change', () => {
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-host'));
      selectMethod('None');
      expect(screen.queryByText(ORIGIN_CHANGED_HINT)).not.toBeInTheDocument();
      expect(screen.getByTestId('save-bar')).not.toBeDisabled();

      selectMethod('Username and Password');
      expect(screen.getByText(ORIGIN_CHANGED_HINT)).toBeInTheDocument();
      expect(screen.getByTestId('save-bar')).toBeDisabled();

      fireEvent.change(field(PASSWORD_FIELD_ID), {target: {value: 'new-pw'}});
      expect(screen.getByTestId('save-bar')).not.toBeDisabled();
    });

    it('drops authentication when an origin change comes with a switch to None', () => {
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-host'));
      selectMethod('None');
      fireEvent.click(screen.getByTestId('save-bar'));

      expect(updateMock.mock.calls[0][0]).toMatchObject({
        url: 'https://collector.example.net/evaluate',
        authentication: {type: 'none'},
      });
    });

    it('requires the new credentials when an origin change comes with a switch to a method', () => {
      mockConn.data = HTTP_NONE_CONNECTION;
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-host'));
      selectMethod('Username and Password');
      expect(screen.getByTestId('save-bar')).toBeDisabled();

      fireEvent.change(field(USERNAME_FIELD_ID), {target: {value: 'svc'}});
      expect(screen.getByTestId('save-bar')).toBeDisabled();
      fireEvent.change(field(PASSWORD_FIELD_ID), {target: {value: 'pw'}});
      fireEvent.click(screen.getByTestId('save-bar'));

      expect(updateMock.mock.calls[0][0]).toMatchObject({
        url: 'https://collector.example.net/evaluate',
        authentication: {type: 'basic', properties: {username: 'svc', password: 'pw'}},
      });
    });

    it('allows a URL origin change without credentials when no authentication is set', () => {
      mockConn.data = HTTP_LOCAL_CONNECTION;
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-host'));

      expect(screen.queryByText(ORIGIN_CHANGED_HINT)).not.toBeInTheDocument();
      expect(screen.getByTestId('save-bar')).not.toBeDisabled();
    });

    it('blocks Save when authentication is set on an http URL', () => {
      mockConn.data = HTTP_LOCAL_CONNECTION;
      render(<ConnectionDetailPage />);

      selectMethod('Username and Password');
      fireEvent.change(field(USERNAME_FIELD_ID), {target: {value: 'svc'}});
      fireEvent.change(field(PASSWORD_FIELD_ID), {target: {value: 'pw'}});

      expect(screen.getByText(HTTPS_REQUIRED)).toBeInTheDocument();
      expect(screen.getByTestId('save-bar')).toBeDisabled();

      selectMethod('None');
      expect(screen.queryByText(HTTPS_REQUIRED)).not.toBeInTheDocument();
    });

    it('shows the localized message for a rejected URL', async () => {
      updateMock.mockRejectedValueOnce({response: {status: 400, data: {code: 'CON-1016'}}});
      render(<ConnectionDetailPage />);

      fireEvent.click(screen.getByTestId('edit-url-path'));
      fireEvent.click(screen.getByTestId('save-bar'));

      expect(
        await screen.findByText(
          'The URL must be an absolute http or https URL without a user name or password, and must use https when authentication is set.',
        ),
      ).toBeInTheDocument();
    });
  });
});
