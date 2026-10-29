// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import CimdDocumentSection from '../CimdDocumentSection';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: vi.fn(),
}));

const CLIENT_ID = 'https://vscode.dev/oauth/client-metadata.json';

const oauth2Config: OAuth2Config = {
  clientId: CLIENT_ID,
  clientIdMetadataDocument: true,
  redirectUris: ['http://127.0.0.1/callback', 'http://127.0.0.1/old-callback'],
  grantTypes: ['authorization_code', 'refresh_token'],
  responseTypes: ['code'],
  tokenEndpointAuthMethod: 'none',
  publicClient: true,
  pkceRequired: true,
  scopes: ['openid'],
};

const application = {
  id: 'app-1',
  name: 'Visual Studio Code',
  inboundAuthConfig: [{type: 'oauth2', config: oauth2Config}],
} as unknown as Application;

const request = vi.fn();

describe('CimdDocumentSection', () => {
  beforeEach(() => {
    request.mockReset();
    vi.mocked(useThunderID).mockReturnValue({http: {request}} as unknown as ReturnType<typeof useThunderID>);
    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://localhost:8090',
    } as unknown as ReturnType<typeof useConfig>);
  });

  it('shows the stored values read-only', () => {
    render(
      <CimdDocumentSection
        application={application}
        oauth2Config={oauth2Config}
        onFieldChange={vi.fn()}
        isReadOnly={false}
      />,
    );

    expect(screen.getByText(CLIENT_ID)).toBeInTheDocument();
    expect(screen.getByText('http://127.0.0.1/old-callback')).toBeInTheDocument();
    expect(screen.getByText('Public client, no secret')).toBeInTheDocument();
  });

  it('previews the changed document and applies the returned values', async () => {
    request.mockResolvedValue({
      data: {
        name: 'Visual Studio Code',
        inboundAuthConfig: [
          {
            type: 'oauth2',
            config: {
              ...oauth2Config,
              scopes: undefined,
              redirectUris: ['http://127.0.0.1/callback', 'http://[::1]/callback'],
            },
          },
        ],
      },
    });
    const onFieldChange = vi.fn();
    render(
      <CimdDocumentSection
        application={application}
        oauth2Config={oauth2Config}
        onFieldChange={onFieldChange}
        isReadOnly={false}
      />,
    );

    await userEvent.click(screen.getByTestId('cimd-refetch-button'));

    expect(await screen.findByText(/The document has changed/)).toBeInTheDocument();
    expect(request).toHaveBeenCalledWith(expect.objectContaining({data: {clientId: CLIENT_ID}}));
    expect(screen.getByLabelText('Removed')).toBeInTheDocument();
    expect(screen.getByLabelText('Added')).toBeInTheDocument();

    await userEvent.click(screen.getByTestId('cimd-apply-button'));

    await waitFor(() => expect(onFieldChange).toHaveBeenCalledWith('inboundAuthConfig', expect.any(Array)));
    const [[, inboundAuthConfig]] = onFieldChange.mock.calls as [[string, {config: OAuth2Config}[]]];
    expect(inboundAuthConfig[0].config.redirectUris).toEqual(['http://127.0.0.1/callback', 'http://[::1]/callback']);
    expect(inboundAuthConfig[0].config.clientIdMetadataDocument).toBe(true);
    expect(onFieldChange).not.toHaveBeenCalledWith('name', expect.anything());
  });

  it('shows why the document could not be re-fetched', async () => {
    request.mockRejectedValue(Object.assign(new Error('Bad Request'), {response: {data: {code: 'CIMD-1003'}}}));
    render(
      <CimdDocumentSection
        application={application}
        oauth2Config={oauth2Config}
        onFieldChange={vi.fn()}
        isReadOnly={false}
      />,
    );

    await userEvent.click(screen.getByTestId('cimd-refetch-button'));

    expect(await screen.findByText(/Couldn't retrieve a metadata document from this URL/)).toBeInTheDocument();
    expect(screen.getByTestId('cimd-apply-button')).toBeDisabled();
  });

  describe('a private key JWT client with identity assertions', () => {
    const TOKEN_EXCHANGE = 'urn:ietf:params:oauth:grant-type:token-exchange';
    const JWT_BEARER = 'urn:ietf:params:oauth:grant-type:jwt-bearer';
    const documentConfig: OAuth2Config = {
      ...oauth2Config,
      clientId: 'https://chatgpt.com/oauth/client.json',
      redirectUris: ['https://chatgpt.com/connector_platform_oauth_redirect'],
      tokenEndpointAuthMethod: 'private_key_jwt',
      publicClient: false,
      certificate: {type: 'JWKS_URI', value: 'https://chatgpt.com/oauth/jwks.json'},
      requirePushedAuthorizationRequests: false,
    };
    const confidentialConfig = {
      ...documentConfig,
      grantTypes: ['authorization_code', 'refresh_token', TOKEN_EXCHANGE, JWT_BEARER],
      requirePushedAuthorizationRequests: true,
      token: {idJag: {enabled: true, allowedAudiences: ['https://as.example.com']}},
    } as OAuth2Config;
    const confidentialApplication = {
      ...application,
      inboundAuthConfig: [{type: 'oauth2', config: confidentialConfig}],
    } as unknown as Application;
    const previewResponse = (config: OAuth2Config) => ({
      data: {name: 'ChatGPT', inboundAuthConfig: [{type: 'oauth2', config}]},
    });
    const appliedConfig = (onFieldChange: ReturnType<typeof vi.fn>): OAuth2Config => {
      const call = onFieldChange.mock.calls.find(([field]) => field === 'inboundAuthConfig') as [
        string,
        {config: OAuth2Config}[],
      ];
      return call[1][0].config;
    };

    it('treats the grants the administrator added as unchanged', async () => {
      request.mockResolvedValue(previewResponse(documentConfig));
      render(
        <CimdDocumentSection
          application={confidentialApplication}
          oauth2Config={confidentialConfig}
          onFieldChange={vi.fn()}
          isReadOnly={false}
        />,
      );

      await userEvent.click(screen.getByTestId('cimd-refetch-button'));

      expect(await screen.findByText('The document matches what this application uses.')).toBeInTheDocument();
      expect(screen.getByText('Token exchange')).toBeInTheDocument();
      expect(screen.getByText('JWT bearer')).toBeInTheDocument();
      expect(screen.queryByLabelText('Removed')).not.toBeInTheDocument();
    });

    it('keeps those grants and the other settings when the document changes', async () => {
      request.mockResolvedValue(
        previewResponse({...documentConfig, redirectUris: ['https://chatgpt.com/connector_platform_oauth_redirect2']}),
      );
      const onFieldChange = vi.fn();
      render(
        <CimdDocumentSection
          application={confidentialApplication}
          oauth2Config={confidentialConfig}
          onFieldChange={onFieldChange}
          isReadOnly={false}
        />,
      );

      await userEvent.click(screen.getByTestId('cimd-refetch-button'));
      await screen.findByText(/The document has changed/);
      await userEvent.click(screen.getByTestId('cimd-apply-button'));

      const config = appliedConfig(onFieldChange);
      expect(config.redirectUris).toEqual(['https://chatgpt.com/connector_platform_oauth_redirect2']);
      expect(config.grantTypes).toEqual(['authorization_code', 'refresh_token', TOKEN_EXCHANGE, JWT_BEARER]);
      expect(config.token?.idJag?.enabled).toBe(true);
      expect(config.requirePushedAuthorizationRequests).toBe(true);
    });

    it('removes those grants and turns identity assertions off when the document becomes public', async () => {
      request.mockResolvedValue(
        previewResponse({
          ...documentConfig,
          tokenEndpointAuthMethod: 'none',
          publicClient: true,
          certificate: undefined,
        }),
      );
      const onFieldChange = vi.fn();
      render(
        <CimdDocumentSection
          application={confidentialApplication}
          oauth2Config={confidentialConfig}
          onFieldChange={onFieldChange}
          isReadOnly={false}
        />,
      );

      await userEvent.click(screen.getByTestId('cimd-refetch-button'));

      expect(await screen.findByText(/Identity assertions \(ID-JAG\) will be turned off/)).toBeInTheDocument();
      expect(screen.getAllByLabelText('Removed')).toHaveLength(2);

      await userEvent.click(screen.getByTestId('cimd-apply-button'));

      const config = appliedConfig(onFieldChange);
      expect(config.grantTypes).toEqual(['authorization_code', 'refresh_token']);
      expect(config.tokenEndpointAuthMethod).toBe('none');
      expect(config.certificate).toBeNull();
      expect(config.token?.idJag).toEqual({enabled: false, allowedAudiences: ['https://as.example.com']});
    });

    it('marks a grant the document now declares as added', async () => {
      const withoutRefresh = {...confidentialConfig, grantTypes: ['authorization_code', TOKEN_EXCHANGE, JWT_BEARER]};
      request.mockResolvedValue(previewResponse(documentConfig));
      render(
        <CimdDocumentSection
          application={
            {...confidentialApplication, inboundAuthConfig: [{type: 'oauth2', config: withoutRefresh}]} as Application
          }
          oauth2Config={withoutRefresh}
          onFieldChange={vi.fn()}
          isReadOnly={false}
        />,
      );

      await userEvent.click(screen.getByTestId('cimd-refetch-button'));

      expect(await screen.findByText(/The document has changed/)).toBeInTheDocument();
      expect(screen.getByLabelText('Added')).toBeInTheDocument();
      expect(screen.queryByLabelText('Removed')).not.toBeInTheDocument();
    });

    it('removes JWT bearer without a warning when identity assertions are off and the document becomes public', async () => {
      const redeemOnly = {...documentConfig, grantTypes: ['authorization_code', 'refresh_token', JWT_BEARER]};
      request.mockResolvedValue(
        previewResponse({
          ...documentConfig,
          tokenEndpointAuthMethod: 'none',
          publicClient: true,
          certificate: undefined,
        }),
      );
      const onFieldChange = vi.fn();
      render(
        <CimdDocumentSection
          application={
            {...confidentialApplication, inboundAuthConfig: [{type: 'oauth2', config: redeemOnly}]} as Application
          }
          oauth2Config={redeemOnly}
          onFieldChange={onFieldChange}
          isReadOnly={false}
        />,
      );

      await userEvent.click(screen.getByTestId('cimd-refetch-button'));

      expect(await screen.findByText(/The document has changed/)).toBeInTheDocument();
      expect(screen.getAllByLabelText('Removed')).toHaveLength(1);
      expect(screen.queryByText(/Identity assertions \(ID-JAG\) will be turned off/)).not.toBeInTheDocument();

      await userEvent.click(screen.getByTestId('cimd-apply-button'));

      expect(appliedConfig(onFieldChange).grantTypes).toEqual(['authorization_code', 'refresh_token']);
    });
  });

  it('disables re-fetch for a read-only application', () => {
    render(
      <CimdDocumentSection application={application} oauth2Config={oauth2Config} onFieldChange={vi.fn()} isReadOnly />,
    );

    expect(screen.getByTestId('cimd-refetch-button')).toBeDisabled();
  });
});
