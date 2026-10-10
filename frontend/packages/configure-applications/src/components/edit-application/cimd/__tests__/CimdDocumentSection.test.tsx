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
    expect(screen.getByLabelText('removed')).toBeInTheDocument();
    expect(screen.getByLabelText('added')).toBeInTheDocument();

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

  it('disables re-fetch for a read-only application', () => {
    render(
      <CimdDocumentSection application={application} oauth2Config={oauth2Config} onFieldChange={vi.fn()} isReadOnly />,
    );

    expect(screen.getByTestId('cimd-refetch-button')).toBeDisabled();
  });
});
