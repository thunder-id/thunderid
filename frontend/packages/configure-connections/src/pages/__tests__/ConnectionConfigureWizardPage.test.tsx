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
import ConnectionConfigureWizardPage from '../ConnectionConfigureWizardPage';

const mutateMock = vi.fn();
const navigateMock = vi.fn();
const mockParams = {type: 'google'};

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
  useParams: () => mockParams,
}));
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getGateCallbackUrl: () => 'https://id.acme.io/gate/callback'}),
  useToast: () => ({showToast: vi.fn()}),
}));
vi.mock('../../api/useCreateConnection', () => ({default: () => ({mutate: mutateMock, isPending: false})}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onFieldChange}: {onFieldChange: (name: string, value: string) => void}) {
    useEffect(() => {
      // Populate both IdP and SMS fields; each type's form only reads the ones it declares.
      onFieldChange('clientId', 'x');
      onFieldChange('clientSecret', 's');
      onFieldChange('accountSid', 'AC00000000000000000000000000000000');
      onFieldChange('authToken', 's');
      onFieldChange('senderId', '+15005550006');
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return <div data-testid="stub-connection-form" />;
  },
}));

describe('ConnectionConfigureWizardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.type = 'google';
  });

  it('shows a single configure step and creates with the fixed vendor name', () => {
    render(<ConnectionConfigureWizardPage />);

    // Single step: the credentials form is shown with a Create button (no attribute-mapping step).
    expect(screen.getByTestId('connection-fullpage-content')).toBeInTheDocument();
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByText('Configure your Google connection')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    const payload = mutateMock.mock.calls[0][0] as {
      name: string;
      clientId: string;
      redirectUri: string;
      scopes?: string[];
      attributeConfiguration?: unknown;
    };
    expect(payload).toMatchObject({
      name: 'Google',
      clientId: 'x',
      clientSecret: 's',
      redirectUri: 'https://id.acme.io/gate/callback',
    });
    expect(payload.scopes).toBeUndefined();
    expect(payload.attributeConfiguration).toBeUndefined();
  });

  it('shows the setup hint with the redirect URI to copy for Google', () => {
    render(<ConnectionConfigureWizardPage />);

    const hint = screen.getByTestId('connection-create-hint');
    expect(hint).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://id.acme.io/gate/callback')).toBeInTheDocument();
  });

  it('navigates to the connection detail page after a successful create', () => {
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('wizard-create'));

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'conn-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/google/conn-1');
  });

  it('SMS vendor: single configure step creates without attribute mapping', () => {
    mockParams.type = 'twilio';
    render(<ConnectionConfigureWizardPage />);

    // Single step: the credentials form with a Create button (no attribute-mapping step).
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    const payload = mutateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'Twilio',
      accountSid: 'AC00000000000000000000000000000000',
      senderId: '+15005550006',
    });
    expect('attributeConfiguration' in payload).toBe(false);
  });
});
