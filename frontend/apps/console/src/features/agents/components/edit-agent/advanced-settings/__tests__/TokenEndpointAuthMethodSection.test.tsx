/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi} from 'vitest';
import type {OAuth2Config} from '../../../../../applications/models/oauth';
import TokenEndpointAuthMethodSection from '../TokenEndpointAuthMethodSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    discovery: {
      wellKnown: {
        token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
      },
    },
  }),
}));

describe('TokenEndpointAuthMethodSection', () => {
  it('returns null when oauth2Config is undefined', () => {
    const {container} = render(<TokenEndpointAuthMethodSection />);
    expect(container.firstChild).toBeNull();
  });

  it('shows the placeholder when no method is set', () => {
    const oauth2Config: OAuth2Config = {grantTypes: [], responseTypes: []};
    render(<TokenEndpointAuthMethodSection oauth2Config={oauth2Config} />);

    expect(screen.getByText('agents:edit.credentials.tokenEndpointAuthMethod.placeholder')).toBeInTheDocument();
  });

  it('locks the token method when client is public', () => {
    const oauth2Config: OAuth2Config = {grantTypes: [], responseTypes: [], publicClient: true};
    render(<TokenEndpointAuthMethodSection oauth2Config={oauth2Config} />);

    expect(screen.getByText('agents:edit.credentials.tokenEndpointAuthMethod.lockedHint')).toBeInTheDocument();
    const select = document.getElementById('agent_token_endpoint_auth_method');
    expect(select).toHaveClass('Mui-disabled');
  });

  it('renders the currently selected method', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: [],
      responseTypes: [],
      tokenEndpointAuthMethod: 'client_secret_basic',
    };
    render(<TokenEndpointAuthMethodSection oauth2Config={oauth2Config} />);

    expect(screen.getByText('client_secret_basic')).toBeInTheDocument();
  });

  it('calls onOAuth2ConfigChange when a new method is selected', async () => {
    const user = userEvent.setup();
    const onOAuth2ConfigChange = vi.fn();
    const oauth2Config: OAuth2Config = {
      grantTypes: [],
      responseTypes: [],
      tokenEndpointAuthMethod: 'client_secret_basic',
    };
    render(<TokenEndpointAuthMethodSection oauth2Config={oauth2Config} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'client_secret_post'}));

    expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
      expect.objectContaining({tokenEndpointAuthMethod: 'client_secret_post'}) as Partial<OAuth2Config>,
    );
  });
});
