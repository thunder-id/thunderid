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
import type {OAuth2Config} from '../../../../models/oauth';
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
        grant_types_supported: [
          'authorization_code',
          'refresh_token',
          'client_credentials',
          'urn:openid:params:grant-type:ciba',
        ],
        response_types_supported: ['code', 'token'],
        token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
      },
    },
  }),
}));

describe('OAuth2ConfigSection', () => {
  describe('Rendering', () => {
    it('should return null when oauth2Config is not provided', () => {
      const {container} = render(<OAuth2ConfigSection />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null when oauth2Config is undefined', () => {
      const {container} = render(<OAuth2ConfigSection oauth2Config={undefined} />);

      expect(container.firstChild).toBeNull();
    });

    it('should render OAuth2 config section with all elements', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code', 'refresh_token'],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.oauth2Config.intro')).toBeInTheDocument();
    });
  });

  describe('Grant Types Display', () => {
    it('should display all grant types as chips', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code', 'refresh_token', 'client_credentials'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.grantTypes')).toBeInTheDocument();
      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.getByText('refresh_token')).toBeInTheDocument();
      expect(screen.getByText('client_credentials')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.grantTypes.hint')).toBeInTheDocument();
    });

    it('should handle single grant type', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.queryByText('refresh_token')).not.toBeInTheDocument();
    });

    it('should handle empty grant types array', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: [],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.grantTypes')).toBeInTheDocument();
      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });
  });

  describe('Response Types Display', () => {
    it('should display all response types as chips', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code', 'token'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.responseTypes')).toBeInTheDocument();
      expect(screen.getByText('code')).toBeInTheDocument();
      expect(screen.getByText('token')).toBeInTheDocument();
    });

    it('should handle single response type', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('code')).toBeInTheDocument();
    });

    it('should handle empty response types array', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: [],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.responseTypes')).toBeInTheDocument();
    });
  });

  describe('Public Client Status', () => {
    it('should display public client as yes when true', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: true,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.publicClient')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.publicClient.public')).toBeInTheDocument();
    });

    it('should display public client as no when false', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.publicClient')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.publicClient.confidential')).toBeInTheDocument();
    });

    it('should handle undefined publicClient as false', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.publicClient')).toBeInTheDocument();
    });
  });

  describe('PKCE Requirement Status', () => {
    it('should display PKCE as required when true', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.pkceRequired')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.pkce.enabled')).toBeInTheDocument();
    });

    it('should display PKCE as not required when false', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.pkceRequired')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.pkce.disabled')).toBeInTheDocument();
    });

    it('should handle undefined pkceRequired as false', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.pkceRequired')).toBeInTheDocument();
    });
  });

  describe('Layout and Styling', () => {
    it('should render grant type chips with correct styling', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      const chip = screen.getByText('authorization_code').closest('.MuiChip-root');
      expect(chip).toHaveClass('MuiChip-sizeSmall');
    });

    it('should render in a Stack with proper spacing', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      const {container} = render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      const stack = container.querySelector('.MuiStack-root');
      expect(stack).toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle minimal OAuth2 config', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.getByText('code')).toBeInTheDocument();
    });

    it('should handle multiple grant types correctly', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: [
          'authorization_code',
          'refresh_token',
          'client_credentials',
          'urn:ietf:params:oauth:grant-type:token-exchange',
        ],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.getByText('refresh_token')).toBeInTheDocument();
      expect(screen.getByText('client_credentials')).toBeInTheDocument();
      expect(screen.getByText('urn:ietf:params:oauth:grant-type:token-exchange')).toBeInTheDocument();
    });
  });

  describe('CIBA Grant Type', () => {
    it('renders the friendly label for the CIBA URN in the grant type picker', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        grantTypes: [],
        responseTypes: [],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} onOAuth2ConfigChange={vi.fn()} />);

      const grantTypesSelect = document.getElementById('grant_types')!;
      await user.click(grantTypesSelect);

      expect(screen.getByText('applications:edit.advanced.grantTypes.labels.ciba')).toBeInTheDocument();
    });

    it('calls onOAuth2ConfigChange with the raw CIBA URN when the CIBA option is selected', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      const oauth2Config: OAuth2Config = {
        grantTypes: [],
        responseTypes: [],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      const grantTypesSelect = document.getElementById('grant_types')!;
      await user.click(grantTypesSelect);
      await user.click(screen.getByText('applications:edit.advanced.grantTypes.labels.ciba'));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          grantTypes: expect.arrayContaining(['urn:openid:params:grant-type:ciba']) as unknown,
        }),
      );
    });
  });
});
