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

import {fireEvent, render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import CertificateTypes from '../../../../constants/certificate-types';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import EditAdvancedSettings from '../EditAdvancedSettings';

const {mockUseThunderID} = vi.hoisted(() => ({
  mockUseThunderID: vi.fn(() => ({discovery: null}) as unknown),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: mockUseThunderID,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('EditAdvancedSettings', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test Description',
    template: 'custom',
    certificate: {
      type: CertificateTypes.NONE,
      value: '',
    },
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-15T00:00:00Z',
  } as Application;

  const mockOAuth2Config: OAuth2Config = {
    grantTypes: ['authorization_code', 'refresh_token'],
    responseTypes: ['code'],
    pkceRequired: true,
    publicClient: false,
  };

  const mockOnFieldChange = vi.fn();

  describe('Rendering', () => {
    it('should render all three sections', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
    });

    it('should not render the attestation section by default', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText('applications:edit.advanced.labels.attestation')).not.toBeInTheDocument();
    });

    it('should render the attestation section when the template supports it', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          showAttestation
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.attestation')).toBeInTheDocument();
    });

    it('should render without OAuth2 config when not provided', () => {
      render(<EditAdvancedSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByText('applications:edit.advanced.labels.oauth2Config')).not.toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.metadata')).toBeInTheDocument();
    });

    it('should render without metadata section when timestamps are missing', () => {
      const appWithoutMetadata = {...mockApplication};
      delete (appWithoutMetadata as Partial<Application>).createdAt;
      delete (appWithoutMetadata as Partial<Application>).updatedAt;

      render(
        <EditAdvancedSettings
          application={appWithoutMetadata}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.oauth2Config')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
      expect(screen.queryByText('applications:edit.advanced.labels.metadata')).not.toBeInTheDocument();
    });
  });

  describe('Section Integration', () => {
    it('should pass correct props to OAuth2ConfigSection', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('authorization_code')).toBeInTheDocument();
      expect(screen.getByText('refresh_token')).toBeInTheDocument();
      expect(screen.getByText('code')).toBeInTheDocument();
    });

    it('should pass correct props to CertificateSection', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByLabelText('applications:edit.advanced.labels.certificateType')).toBeInTheDocument();
    });

    it('should pass correct props to MetadataSection', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.createdAt')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.advanced.labels.updatedAt')).toBeInTheDocument();
    });
  });

  describe('Layout', () => {
    it('should render sections in a Stack with spacing', () => {
      const {container} = render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const stack = container.firstChild;
      expect(stack).toHaveClass('MuiStack-root');
    });
  });

  describe('Edge Cases', () => {
    it('should handle undefined oauth2Config', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={undefined}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText('applications:edit.advanced.labels.oauth2Config')).not.toBeInTheDocument();
    });

    it('should handle empty editedApp', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
    });

    it('should render with minimal application data', () => {
      const minimalApp = {
        id: 'minimal-id',
        name: 'Minimal App',
        template: 'custom',
      } as Application;

      render(<EditAdvancedSettings application={minimalApp} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.advanced.labels.certificate')).toBeInTheDocument();
    });
  });

  describe('AcrValuesSection Integration', () => {
    beforeEach(() => {
      mockUseThunderID.mockReturnValue({
        discovery: {
          wellKnown: {
            acr_values_supported: ['urn:acr:loa1', 'urn:acr:loa2'],
            grant_types_supported: ['authorization_code', 'refresh_token', 'client_credentials'],
            response_types_supported: ['code', 'token'],
            token_endpoint_auth_methods_supported: ['client_secret_basic', 'client_secret_post', 'none'],
          },
        },
      });
    });

    afterEach(() => {
      mockUseThunderID.mockReturnValue({discovery: null});
    });

    it('should render AcrValuesSection when discovery has acr_values_supported', () => {
      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getAllByText('applications:edit.advanced.labels.acrValues').length).toBeGreaterThan(0);
    });

    it('should not render AcrValuesSection when discovery has no acr_values_supported', () => {
      mockUseThunderID.mockReturnValue({discovery: null});

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText('applications:edit.advanced.labels.acrValues')).not.toBeInTheDocument();
    });

    it('should propagate AcrValues changes to onFieldChange as inboundAuthConfig update', () => {
      const appWithInboundAuth = {
        ...mockApplication,
        inboundAuthConfig: [{type: 'oauth2', config: {...mockOAuth2Config}}],
      } as Application;

      render(
        <EditAdvancedSettings
          application={appWithInboundAuth}
          editedApp={{}}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Open the MUI Select dropdown
      const selectButton = document.getElementById('acr_values')!;
      fireEvent.mouseDown(selectButton);

      // Click an ACR value option in the dropdown
      const option = screen.getByText('urn:acr:loa1');
      fireEvent.click(option);

      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({acrValues: ['urn:acr:loa1']}) as unknown,
          }),
        ]),
      );
    });
  });

  describe('ID-JAG Integration', () => {
    it('should apply the idJag enable and the added token-exchange grant type in a single onFieldChange call on the first click', () => {
      const oauth2ConfigWithoutTokenExchange: OAuth2Config = {
        ...mockOAuth2Config,
        grantTypes: ['authorization_code'],
      };
      const appWithInboundAuth = {
        ...mockApplication,
        inboundAuthConfig: [{type: 'oauth2', config: {...oauth2ConfigWithoutTokenExchange}}],
      } as Application;

      mockOnFieldChange.mockClear();

      render(
        <EditAdvancedSettings
          application={appWithInboundAuth}
          editedApp={{}}
          oauth2Config={oauth2ConfigWithoutTokenExchange}
          onFieldChange={mockOnFieldChange}
        />,
      );

      const toggle = screen.getByLabelText('applications:edit.advanced.idJag.title');
      fireEvent.click(toggle);

      expect(mockOnFieldChange).toHaveBeenCalledTimes(1);
      expect(mockOnFieldChange).toHaveBeenCalledWith(
        'inboundAuthConfig',
        expect.arrayContaining([
          expect.objectContaining({
            type: 'oauth2',
            config: expect.objectContaining({
              grantTypes: expect.arrayContaining([
                'authorization_code',
                'urn:ietf:params:oauth:grant-type:token-exchange',
              ]) as unknown,
              token: expect.objectContaining({
                idJag: expect.objectContaining({enabled: true}) as unknown,
              }) as unknown,
            }) as unknown,
          }),
        ]),
      );
    });

    it('should forward onValidationChange to IdentityAssertionsSection', () => {
      const onValidationChange = vi.fn();
      const oauth2ConfigWithIdJagError: OAuth2Config = {
        ...mockOAuth2Config,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
        },
      };

      render(
        <EditAdvancedSettings
          application={mockApplication}
          editedApp={{}}
          oauth2Config={oauth2ConfigWithIdJagError}
          onFieldChange={mockOnFieldChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
  });
});
