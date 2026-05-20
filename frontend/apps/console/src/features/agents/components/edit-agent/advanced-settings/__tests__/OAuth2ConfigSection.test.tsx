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
import {describe, it, expect, vi} from 'vitest';
import type {OAuth2Config} from '../../../../../applications/models/oauth';
import OAuth2ConfigSection from '../OAuth2ConfigSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    discovery: {
      wellKnown: {
        grant_types_supported: ['authorization_code', 'refresh_token', 'client_credentials'],
        response_types_supported: ['code', 'token'],
        token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
      },
    },
  }),
}));

describe('OAuth2ConfigSection (agent)', () => {
  describe('Rendering', () => {
    it('returns null when oauth2Config is undefined', () => {
      const {container} = render(<OAuth2ConfigSection />);
      expect(container.firstChild).toBeNull();
    });

    it('renders the OAuth2 section with header', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
    });

    it('renders configured grant types as chips', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code', 'refresh_token'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.getByText('refresh_token')).toBeInTheDocument();
    });

    it('renders configured response types as chips', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code', 'token'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getByText('token')).toBeInTheDocument();
    });

    it('shows the placeholder for grant types when none are selected', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: [],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.grantTypes.placeholder')).toBeInTheDocument();
    });

    it('shows the placeholder for response types when none are selected', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: [],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.responseTypes.placeholder')).toBeInTheDocument();
    });
  });

  describe('Token Endpoint Auth Method', () => {
    it('shows the placeholder when no method is set', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.tokenEndpointAuthMethod.placeholder')).toBeInTheDocument();
    });

    it('locks the token method when client is public', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: true,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.tokenEndpointAuthMethod.lockedHint')).toBeInTheDocument();
    });
  });

  describe('Read-only Mode', () => {
    it('disables controls when onOAuth2ConfigChange is not provided', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      // The grant-type select should be disabled
      const grantTypesSelect = document.getElementById('grant_types');
      expect(grantTypesSelect).toHaveClass('Mui-disabled');
    });
  });

  describe('Public client toggle', () => {
    it('renders the public-client switch with the current value', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: true,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      const publicClientSwitch = screen.getByLabelText('applications:edit.advanced.labels.publicClient');
      expect(publicClientSwitch).toBeChecked();
    });
  });

  describe('PKCE toggle', () => {
    it('renders the PKCE switch with the configured value', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      const pkceSwitch = screen.getByLabelText('applications:edit.advanced.labels.pkceRequired');
      expect(pkceSwitch).toBeChecked();
    });

    it('forces PKCE on when client is public', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: true,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      const pkceSwitch = screen.getByLabelText('applications:edit.advanced.labels.pkceRequired');
      expect(pkceSwitch).toBeChecked();
    });
  });
});
