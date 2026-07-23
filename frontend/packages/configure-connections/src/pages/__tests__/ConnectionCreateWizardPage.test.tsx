/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionCreateWizardPage from '../ConnectionCreateWizardPage';

const mutateMock = vi.fn();
const navigateMock = vi.fn();

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
  useToast: () => ({showToast: vi.fn()}),
}));
vi.mock('../../api/useCreateConnection', () => ({default: () => ({mutate: mutateMock, isPending: false})}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onFieldChange}: {onFieldChange: (name: string, value: string) => void}) {
    useEffect(() => {
      // Populate the fields required by every connection type used in these tests (oidc, oauth).
      onFieldChange('clientId', 'x');
      onFieldChange('clientSecret', 's');
      onFieldChange('authorizationEndpoint', 'https://idp.example.com/authorize');
      onFieldChange('tokenEndpoint', 'https://idp.example.com/token');
      onFieldChange('userInfoEndpoint', 'https://idp.example.com/userinfo');
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
  fireEvent.click(screen.getByTestId('wizard-continue'));
  fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: name}});
  fireEvent.click(screen.getByTestId('wizard-continue'));
}

describe('ConnectionCreateWizardPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('shows the type heading without the redundant step label', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
    expect(screen.getByText('What kind of connection do you want to add?')).toBeInTheDocument();
    expect(screen.getAllByText('Connection type')).toHaveLength(1);
  });

  it('shows the name step after selecting a type, gated on a non-empty name', () => {
    render(<ConnectionCreateWizardPage />);

    fireEvent.click(screen.getByTestId('connection-type-option-oidc'));
    fireEvent.click(screen.getByTestId('wizard-continue'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
    expect(screen.getByTestId('wizard-continue')).toBeDisabled();

    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'Acme Connection'}});
    expect(screen.getByTestId('wizard-continue')).toBeEnabled();
  });

  it('shows the configure heading without the redundant step label after continuing', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');

    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
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

    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
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

  it('returns to the name step with a duplicate-name error on a 409 create conflict', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 409}});
    });
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-oidc');
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
    expect(screen.getByText('A connection with this name already exists.')).toBeInTheDocument();
  });

  it('renders the baked-in trusted-idp step instead of the generic ConnectionForm', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp', 'My Trusted Issuer');

    expect(screen.getByTestId('custom-step')).toHaveTextContent('My Trusted Issuer');
    expect(screen.queryByTestId('stub-connection-form')).not.toBeInTheDocument();
    expect(screen.queryByTestId('wizard-create')).not.toBeInTheDocument();
  });

  it('lets the trusted-idp step bounce back to the name step via onNameConflict', () => {
    render(<ConnectionCreateWizardPage />);

    selectTypeAndName('connection-type-option-trusted-idp');
    fireEvent.click(screen.getByTestId('custom-step-conflict'));

    expect(screen.getByTestId('connection-name-step')).toBeInTheDocument();
    expect(screen.getByText('A connection with this name already exists.')).toBeInTheDocument();
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

  it('shows four type cards including trusted-idp', () => {
    render(<ConnectionCreateWizardPage />);

    expect(screen.getAllByTestId(/^connection-type-option-/)).toHaveLength(4);
    expect(screen.getByTestId('connection-type-option-trusted-idp')).toBeInTheDocument();
  });
});
