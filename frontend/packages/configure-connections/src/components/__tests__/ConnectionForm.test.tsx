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

  it('renders the redirect URI as an editable field that reports edits', () => {
    const onFieldChange = vi.fn();
    render(<ConnectionForm {...baseProps} onFieldChange={onFieldChange} />);

    const input = screen.getByPlaceholderText('https://your-gate-host/gate/callback');
    fireEvent.change(input, {target: {value: 'https://gate.example.com/gate/callback'}});

    expect(onFieldChange).toHaveBeenCalledWith('redirectUri', 'https://gate.example.com/gate/callback');
  });
});
