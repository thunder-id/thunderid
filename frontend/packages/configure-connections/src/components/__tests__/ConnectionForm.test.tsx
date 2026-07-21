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
import {describe, expect, it, vi} from 'vitest';
import ConnectionForm from '../ConnectionForm';

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast: vi.fn()}),
}));

function getConnectionField(id: string): HTMLElement {
  const field = document.getElementById(`connection-field-${id}`);
  if (!field) {
    throw new Error(`Expected connection field ${id} to exist`);
  }
  return field;
}

function isFieldMarkedRequired(id: string): boolean {
  const label = document.querySelector(`label[for="connection-field-${id}"]`);
  return Boolean(label?.querySelector('.MuiFormLabel-asterisk'));
}

describe('ConnectionForm', () => {
  const baseProps = {
    type: 'google' as const,
    mode: 'create' as const,
    values: {
      name: '',
      clientId: '',
      clientSecret: '',
      redirectUri: 'https://id.acme.io/oauth/callback/google',
      scopes: '',
    },
    secretReplacing: false,
    hasStoredSecret: false,
    vendorDisplayName: 'Google',
    onFieldChange: vi.fn(),
    onSecretReplacingChange: vi.fn(),
  };

  it('shows field hints by default and replaces them with validation errors after blur', () => {
    render(<ConnectionForm {...baseProps} />);

    expect(screen.getByText('OAuth2 client identifier used for authentication.')).toBeInTheDocument();
    expect(screen.getByText('OAuth2 client secret issued by your identity provider.')).toBeInTheDocument();
    expect(screen.getByText(/Space-separated scopes to request during sign-in\. Defaults to/)).toBeInTheDocument();

    fireEvent.blur(getConnectionField('clientId'));

    expect(screen.getByText('This field is required.')).toBeInTheDocument();
    expect(screen.queryByText('OAuth2 client identifier used for authentication.')).not.toBeInTheDocument();
  });

  it('reports field edits through onFieldChange', () => {
    const onFieldChange = vi.fn();
    render(<ConnectionForm {...baseProps} onFieldChange={onFieldChange} />);

    fireEvent.change(getConnectionField('clientId'), {
      target: {value: 'my-client-id'},
    });

    expect(onFieldChange).toHaveBeenCalledWith('clientId', 'my-client-id');
  });

  it('renders the redirect URI as a read-only copy field with the derived value', () => {
    render(<ConnectionForm {...baseProps} />);

    const field = getConnectionField('redirectUri') as HTMLInputElement;
    expect(field).toHaveValue('https://id.acme.io/oauth/callback/google');
    expect(field).toHaveAttribute('readonly');
    expect(screen.getByTestId('connection-field-redirectUri-copy')).toBeInTheDocument();
    expect(screen.getByText('Add this exact URI to your Google OAuth client.')).toBeInTheDocument();
  });

  describe('OIDC federation fields', () => {
    const oidcProps = {
      ...baseProps,
      type: 'oidc' as const,
      values: {
        name: '',
        clientId: '',
        clientSecret: '',
        authorizationEndpoint: '',
        tokenEndpoint: '',
        issuer: '',
        userInfoEndpoint: '',
        jwksEndpoint: '',
        redirectUri: 'https://id.acme.io/oauth/callback/oidc',
        scopes: '',
        tokenExchangeEnabled: 'false',
        trustedTokenAudience: '',
      },
    };

    it('renders the Federation section heading above the tokenExchangeEnabled field', () => {
      render(<ConnectionForm {...oidcProps} />);

      expect(screen.getByRole('heading', {name: 'Federation'})).toBeInTheDocument();
    });

    it('renders a switch for the tokenExchangeEnabled field', () => {
      render(<ConnectionForm {...oidcProps} />);

      const toggle = screen.getByRole('switch', {name: 'Enable token exchange'});
      expect(toggle).toBeInTheDocument();
      expect(toggle).not.toBeChecked();
    });

    it('reports the switch toggle through onFieldChange as a "true"/"false" string', () => {
      const onFieldChange = vi.fn();
      render(<ConnectionForm {...oidcProps} onFieldChange={onFieldChange} />);

      const toggle = screen.getByRole('switch', {name: 'Enable token exchange'});
      fireEvent.click(toggle);

      expect(onFieldChange).toHaveBeenCalledWith('tokenExchangeEnabled', 'true');
    });

    it('hides trustedTokenAudience when tokenExchangeEnabled is off', () => {
      render(<ConnectionForm {...oidcProps} />);

      expect(document.getElementById('connection-field-trustedTokenAudience')).not.toBeInTheDocument();
    });

    it('shows trustedTokenAudience when tokenExchangeEnabled is on', () => {
      render(<ConnectionForm {...oidcProps} values={{...oidcProps.values, tokenExchangeEnabled: 'true'}} />);

      expect(document.getElementById('connection-field-trustedTokenAudience')).toBeInTheDocument();
    });

    it('does not mark issuer/jwksEndpoint required when tokenExchangeEnabled is off', () => {
      render(<ConnectionForm {...oidcProps} />);

      expect(isFieldMarkedRequired('issuer')).toBe(false);
      expect(isFieldMarkedRequired('jwksEndpoint')).toBe(false);
    });

    it('marks issuer/jwksEndpoint required when tokenExchangeEnabled is on', () => {
      render(<ConnectionForm {...oidcProps} values={{...oidcProps.values, tokenExchangeEnabled: 'true'}} />);

      expect(isFieldMarkedRequired('issuer')).toBe(true);
      expect(isFieldMarkedRequired('jwksEndpoint')).toBe(true);
    });
  });
});
