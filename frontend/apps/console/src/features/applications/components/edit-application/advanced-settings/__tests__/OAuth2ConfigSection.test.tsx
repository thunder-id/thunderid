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
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import type {OAuth2Config} from '../../../../models/oauth';
import OAuth2ConfigSection from '../OAuth2ConfigSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const {mockUseThunderID} = vi.hoisted(() => ({
  mockUseThunderID: vi.fn().mockReturnValue({
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

vi.mock('@thunderid/react', () => ({
  useThunderID: mockUseThunderID,
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
      expect(screen.queryByText('authorization_code')).not.toBeInTheDocument();
      expect(screen.queryByText('refresh_token')).not.toBeInTheDocument();
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

  describe('Token Endpoint Auth Method', () => {
    it('should render the token endpoint auth method select with label', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
        tokenEndpointAuthMethod: 'client_secret_basic',
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.tokenEndpointAuthMethod')).toBeInTheDocument();
      expect(screen.getByText('client_secret_basic')).toBeInTheDocument();
    });

    it('should lock token endpoint auth method to none when publicClient is true', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: true,
        tokenEndpointAuthMethod: 'client_secret_basic',
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} onOAuth2ConfigChange={vi.fn()} />);

      const select = document.getElementById('token_endpoint_auth_method')!;
      expect(select).toHaveAttribute('aria-disabled', 'true');
    });

    it('should call onOAuth2ConfigChange when token endpoint auth method is changed', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
        tokenEndpointAuthMethod: 'client_secret_basic',
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      const select = document.getElementById('token_endpoint_auth_method')!;
      await user.click(select);
      await user.click(screen.getByText('client_secret_post'));

      expect(onOAuth2ConfigChange).toHaveBeenCalled();
    });
  });

  describe('PAR Toggle', () => {
    it('should render the PAR toggle with correct label', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.requirePAR')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.par.hint')).toBeInTheDocument();
    });

    it('should render PAR toggle with requirePushedAuthorizationRequests true', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
        requirePushedAuthorizationRequests: true,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} />);

      expect(screen.getByText('applications:edit.advanced.labels.requirePAR')).toBeInTheDocument();
    });
  });

  describe('ACR Values', () => {
    const baseConfig: OAuth2Config = {
      grantTypes: ['authorization_code'],
      responseTypes: ['code'],
      pkceRequired: false,
      publicClient: false,
    };

    beforeEach(() => {
      mockUseThunderID.mockReturnValue({
        discovery: {
          wellKnown: {
            grant_types_supported: ['authorization_code', 'refresh_token', 'client_credentials'],
            response_types_supported: ['code', 'token'],
            token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
            acr_values_supported: ['urn:acr:loa1', 'urn:acr:loa2', 'urn:acr:loa3'],
          },
        },
      });
    });

    afterEach(() => {
      mockUseThunderID.mockReturnValue({
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
      });
    });

    it('should not render ACR values field when acr_values_supported is absent', () => {
      mockUseThunderID.mockReturnValue({discovery: null});

      render(<OAuth2ConfigSection oauth2Config={baseConfig} />);

      expect(screen.queryByText('applications:edit.advanced.labels.acrValues')).not.toBeInTheDocument();
    });

    it('should not render ACR values field when acr_values_supported is empty', () => {
      mockUseThunderID.mockReturnValue({
        discovery: {wellKnown: {acr_values_supported: []}},
      });

      render(<OAuth2ConfigSection oauth2Config={baseConfig} />);

      expect(screen.queryByText('applications:edit.advanced.labels.acrValues')).not.toBeInTheDocument();
    });

    it('should render ACR values field when acr_values_supported is present', () => {
      render(<OAuth2ConfigSection oauth2Config={baseConfig} />);

      expect(screen.getByText('applications:edit.advanced.labels.acrValues')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.acrValues.hint')).toBeInTheDocument();
    });

    it('should display selected ACR values as chips', () => {
      render(<OAuth2ConfigSection oauth2Config={{...baseConfig, acrValues: ['urn:acr:loa1']}} />);

      expect(screen.getByText('urn:acr:loa1')).toBeInTheDocument();
    });

    it('should show placeholder text when no ACR values are selected', () => {
      render(<OAuth2ConfigSection oauth2Config={{...baseConfig, acrValues: []}} />);

      expect(screen.getByText('applications:edit.advanced.acrValues.placeholder')).toBeInTheDocument();
    });

    it('should render available ACR values in the dropdown when clicked', async () => {
      const user = userEvent.setup();

      render(<OAuth2ConfigSection oauth2Config={baseConfig} onOAuth2ConfigChange={vi.fn()} />);

      await user.click(document.getElementById('acr_values')!);

      expect(screen.getByText('urn:acr:loa1')).toBeInTheDocument();
      expect(screen.getByText('urn:acr:loa2')).toBeInTheDocument();
      expect(screen.getByText('urn:acr:loa3')).toBeInTheDocument();
    });

    it('should call onOAuth2ConfigChange with acrValues when selection changes', async () => {
      const user = userEvent.setup();
      const onOAuth2ConfigChange = vi.fn();

      render(<OAuth2ConfigSection oauth2Config={baseConfig} onOAuth2ConfigChange={onOAuth2ConfigChange} />);

      await user.click(document.getElementById('acr_values')!);
      await user.click(screen.getByText('urn:acr:loa1'));

      expect(onOAuth2ConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({acrValues: expect.any(Array) as unknown}),
      );
    });

    it('should disable the ACR values select when disabled prop is true', () => {
      render(<OAuth2ConfigSection oauth2Config={baseConfig} disabled />);

      expect(document.getElementById('acr_values')).toHaveAttribute('aria-disabled', 'true');
    });

    it('should display multiple selected ACR values as chips', () => {
      render(
        <OAuth2ConfigSection
          oauth2Config={{...baseConfig, acrValues: ['urn:acr:loa1', 'urn:acr:loa2', 'urn:acr:loa3']}}
        />,
      );

      expect(screen.getByText('urn:acr:loa1')).toBeInTheDocument();
      expect(screen.getByText('urn:acr:loa2')).toBeInTheDocument();
      expect(screen.getByText('urn:acr:loa3')).toBeInTheDocument();
    });
  });

  describe('Allowed Grant Types Limiting', () => {
    beforeEach(() => {
      mockUseThunderID.mockReturnValue({
        discovery: {
          wellKnown: {
            grant_types_supported: [
              'authorization_code',
              'refresh_token',
              'client_credentials',
              'implicit',
              'password',
            ],
            response_types_supported: ['code', 'token'],
            token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
          },
        },
      });
    });

    afterEach(() => {
      mockUseThunderID.mockReturnValue({
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
      });
    });

    it('should offer only the intersection of allowedGrantTypes and discovery grants when allowedGrantTypes is set', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: true,
      };

      render(
        <OAuth2ConfigSection
          oauth2Config={oauth2Config}
          onOAuth2ConfigChange={vi.fn()}
          allowedGrantTypes={['authorization_code', 'refresh_token', 'client_credentials']}
        />,
      );

      const grantTypesSelect = document.getElementById('grant_types')!;
      await user.click(grantTypesSelect);

      expect(screen.getByText('refresh_token')).toBeInTheDocument();
      expect(screen.getByText('client_credentials')).toBeInTheDocument();
      expect(screen.queryByText('implicit')).not.toBeInTheDocument();
      expect(screen.queryByText('password')).not.toBeInTheDocument();
    });

    it('should offer all discovery grant types when allowedGrantTypes is not provided', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code'],
        responseTypes: ['code'],
        pkceRequired: false,
        publicClient: false,
      };

      render(<OAuth2ConfigSection oauth2Config={oauth2Config} onOAuth2ConfigChange={vi.fn()} />);

      const grantTypesSelect = document.getElementById('grant_types')!;
      await user.click(grantTypesSelect);

      expect(screen.getByText('refresh_token')).toBeInTheDocument();
      expect(screen.getByText('client_credentials')).toBeInTheDocument();
      expect(screen.getByText('implicit')).toBeInTheDocument();
      expect(screen.getByText('password')).toBeInTheDocument();
    });
  });

  describe('Template-Locked PKCE (mcp-client constraints)', () => {
    it('should render the PKCE switch as locked and disabled when oauth2Constraints marks pkceRequired read-only', () => {
      const oauth2Config: OAuth2Config = {
        grantTypes: ['authorization_code', 'refresh_token'],
        responseTypes: ['code'],
        pkceRequired: true,
        publicClient: true,
      };

      render(
        <OAuth2ConfigSection
          oauth2Config={oauth2Config}
          onOAuth2ConfigChange={vi.fn()}
          oauth2Constraints={{pkceRequired: {readOnly: true, value: true}}}
        />,
      );

      const pkceSwitch = screen.getByLabelText('applications:edit.advanced.labels.pkceRequired');
      expect(pkceSwitch).toBeDisabled();
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
