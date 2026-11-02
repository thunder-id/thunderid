// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {CimdPreview, CimdPreviewResponse} from '../../../../models/cimd';
import toCimdPreview from '../../../../utils/toCimdPreview';
import ConfigureMetadataDocument from '../ConfigureMetadataDocument';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: vi.fn(),
}));

const CLIENT_ID = 'https://vscode.dev/oauth/client-metadata.json';

const VSCODE_RESPONSE: CimdPreviewResponse = {
  name: 'Visual Studio Code',
  inboundAuthConfig: [
    {
      type: 'oauth2',
      config: {
        clientId: CLIENT_ID,
        clientIdMetadataDocument: true,
        redirectUris: ['http://127.0.0.1/callback'],
        grantTypes: ['authorization_code', 'refresh_token'],
        responseTypes: ['code'],
        tokenEndpointAuthMethod: 'none',
        publicClient: true,
        pkceRequired: true,
      },
    },
  ],
};

const VSCODE_PREVIEW: CimdPreview = toCimdPreview(VSCODE_RESPONSE);

const request = vi.fn();

describe('ConfigureMetadataDocument', () => {
  beforeEach(() => {
    request.mockReset();
    vi.mocked(useThunderID).mockReturnValue({http: {request}} as unknown as ReturnType<typeof useThunderID>);
    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://localhost:8090',
    } as unknown as ReturnType<typeof useConfig>);
  });

  it('is not ready until a metadata document has been fetched', async () => {
    const onReadyChange = vi.fn();
    render(<ConfigureMetadataDocument preview={null} onPreviewChange={vi.fn()} onReadyChange={onReadyChange} />);

    await waitFor(() => expect(onReadyChange).toHaveBeenLastCalledWith(false));
  });

  it('is ready once a document has been approved', async () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureMetadataDocument preview={VSCODE_PREVIEW} onPreviewChange={vi.fn()} onReadyChange={onReadyChange} />,
    );

    await waitFor(() => expect(onReadyChange).toHaveBeenLastCalledWith(true));
  });

  it('offers no quick picks when the template lists no known clients', () => {
    render(<ConfigureMetadataDocument preview={null} onPreviewChange={vi.fn()} />);

    expect(screen.queryByText('Known clients:')).not.toBeInTheDocument();
  });

  it('previews the document through the server and reports the returned values', async () => {
    request.mockResolvedValue({data: VSCODE_RESPONSE});
    const onPreviewChange = vi.fn();
    render(
      <ConfigureMetadataDocument
        preview={null}
        onPreviewChange={onPreviewChange}
        knownClients={[{name: 'Visual Studio Code', clientId: CLIENT_ID}]}
      />,
    );

    await userEvent.click(screen.getByText('Visual Studio Code'));

    await waitFor(() => expect(onPreviewChange).toHaveBeenLastCalledWith(VSCODE_PREVIEW));
    expect(request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/cimd/preview',
        method: 'POST',
        data: {clientId: CLIENT_ID},
      }),
    );
  });

  it('renders the preview with the loopback warning', () => {
    render(<ConfigureMetadataDocument preview={VSCODE_PREVIEW} onPreviewChange={vi.fn()} />);

    expect(screen.getByTestId('cimd-preview-card')).toBeInTheDocument();
    expect(screen.getByText('Published by vscode.dev')).toBeInTheDocument();
    expect(screen.getByText(/only returns to the user's device/)).toBeInTheDocument();
  });

  it('clears the approved document when the URL is edited', async () => {
    const onPreviewChange = vi.fn();
    render(<ConfigureMetadataDocument preview={VSCODE_PREVIEW} onPreviewChange={onPreviewChange} />);

    await userEvent.type(screen.getByTestId('cimd-document-url-input'), 'x');

    expect(onPreviewChange).toHaveBeenLastCalledWith(null);
  });

  it('shows the rule a rejected document broke', async () => {
    request.mockRejectedValue(Object.assign(new Error('Bad Request'), {response: {data: {code: 'CIMD-1005'}}}));
    render(<ConfigureMetadataDocument preview={null} onPreviewChange={vi.fn()} />);

    await userEvent.type(screen.getByTestId('cimd-document-url-input'), 'https://example.com/client.json');
    await userEvent.click(screen.getByTestId('cimd-fetch-document-button'));

    expect(await screen.findByTestId('cimd-preview-error')).toHaveTextContent(
      "Each redirect URI must be on the client's own domain",
    );
  });

  it('falls back to a generic message for an unknown error', async () => {
    request.mockRejectedValue(new Error('Network Error'));
    render(<ConfigureMetadataDocument preview={null} onPreviewChange={vi.fn()} />);

    await userEvent.type(screen.getByTestId('cimd-document-url-input'), 'https://example.com/client.json');
    await userEvent.click(screen.getByTestId('cimd-fetch-document-button'));

    expect(await screen.findByTestId('cimd-preview-error')).toHaveTextContent("This metadata document can't be used.");
  });
});
