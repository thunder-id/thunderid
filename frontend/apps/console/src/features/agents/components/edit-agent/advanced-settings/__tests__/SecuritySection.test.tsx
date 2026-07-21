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
import SecuritySection from '../SecuritySection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({defaults = ''}: {defaults?: string}) => <span>{defaults}</span>,
}));

describe('SecuritySection (agent)', () => {
  it('returns null when oauth2Config is undefined', () => {
    const {container} = render(<SecuritySection />);
    expect(container.firstChild).toBeNull();
  });

  it('checks PKCE automatically when authorization_code is selected', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      pkceRequired: false,
      publicClient: false,
    };

    render(<SecuritySection oauth2Config={oauth2Config} />);

    const pkceSwitch = screen.getByLabelText('agents:edit.advanced.security.pkce.label');
    expect(pkceSwitch).toBeChecked();
  });

  it('unchecks PKCE when authorization_code is not selected', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: ['client_credentials'],
      responseTypes: [],
      pkceRequired: false,
      publicClient: false,
    };

    render(<SecuritySection oauth2Config={oauth2Config} />);

    const pkceSwitch = screen.getByLabelText('agents:edit.advanced.security.pkce.label');
    expect(pkceSwitch).not.toBeChecked();
  });

  it('checks PKCE when the client is public even without authorization_code', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: ['client_credentials'],
      responseTypes: [],
      pkceRequired: false,
      publicClient: true,
    };

    render(<SecuritySection oauth2Config={oauth2Config} />);

    const pkceSwitch = screen.getByLabelText('agents:edit.advanced.security.pkce.label');
    expect(pkceSwitch).toBeChecked();
  });

  it('is never directly editable by the user', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      pkceRequired: true,
      publicClient: false,
    };

    render(<SecuritySection oauth2Config={oauth2Config} />);

    expect(screen.getByLabelText('agents:edit.advanced.security.pkce.label')).toBeDisabled();
  });

  it('shows the not-applicable caption with the authorization_code grant called out when the grant is off', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: ['client_credentials'],
      responseTypes: [],
      pkceRequired: false,
      publicClient: false,
    };

    render(<SecuritySection oauth2Config={oauth2Config} />);

    expect(screen.getByText(/authorization_code/)).toBeInTheDocument();
    const pkceSwitch = screen.getByLabelText('agents:edit.advanced.security.pkce.label');
    expect(pkceSwitch).toBeDisabled();
  });

  describe('Pushed Authorization Requests', () => {
    const oauth2Config: OAuth2Config = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      pkceRequired: true,
      publicClient: false,
    };

    it('reflects the configured value', () => {
      render(<SecuritySection oauth2Config={{...oauth2Config, requirePushedAuthorizationRequests: true}} />);

      expect(screen.getByLabelText('agents:edit.advanced.security.par.label')).toBeChecked();
    });

    it('treats an unset value as unchecked', () => {
      render(<SecuritySection oauth2Config={oauth2Config} />);

      expect(screen.getByLabelText('agents:edit.advanced.security.par.label')).not.toBeChecked();
    });

    it('calls onOAuth2ConfigChange when toggled', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      render(<SecuritySection oauth2Config={oauth2Config} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      await user.click(screen.getByLabelText('agents:edit.advanced.security.par.label'));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith({requirePushedAuthorizationRequests: true});
    });

    it('is disabled when the section is read-only', () => {
      render(<SecuritySection oauth2Config={oauth2Config} onOAuth2ConfigChange={vi.fn()} disabled />);

      expect(screen.getByLabelText('agents:edit.advanced.security.par.label')).toBeDisabled();
    });

    it('is disabled when there is no onOAuth2ConfigChange handler', () => {
      render(<SecuritySection oauth2Config={oauth2Config} />);

      expect(screen.getByLabelText('agents:edit.advanced.security.par.label')).toBeDisabled();
    });
  });
});
