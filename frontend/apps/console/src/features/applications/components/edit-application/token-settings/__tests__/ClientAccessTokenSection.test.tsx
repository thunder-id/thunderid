// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {InboundAuthConfig, OAuth2Config} from '@thunderid/configure-applications';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ClientAccessTokenSection, {type ClientAccessTokenCopy} from '../ClientAccessTokenSection';

vi.mock('../JwtPreview', () => ({
  default: ({payload}: {payload: Record<string, unknown>}) => (
    <pre data-testid="jwt-preview">{JSON.stringify(payload)}</pre>
  ),
}));

const copy: ClientAccessTokenCopy = {
  attributesTitle: 'Access Token Attributes',
  attributesDescription: 'Extra attributes for the access token.',
  attributesLabel: 'Add or Remove Attributes',
  attributesHint: 'Click an attribute to include it.',
  attributesEmpty: 'No attributes available.',
  validityTitle: 'Token Validity',
  validityDescription: 'How long the token is valid.',
  validityLabel: 'Token Validity',
  validityHint: 'Token validity period in seconds.',
  validityError: 'Enter a validity period of at least 1 second.',
};

describe('ClientAccessTokenSection', () => {
  const onFieldChange = vi.fn();
  const inboundAuthConfig: InboundAuthConfig[] = [
    {type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: []}},
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the supplied claims as selectable chips', () => {
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups', 'roles']}
        copy={copy}
        subjectType="application"
      />,
    );

    expect(screen.getByText('groups')).toBeInTheDocument();
    expect(screen.getByText('roles')).toBeInTheDocument();
  });

  it('adds a claim to clientConfig when its chip is clicked', async () => {
    const user = userEvent.setup();
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups', 'roles']}
        copy={copy}
        subjectType="application"
      />,
    );

    await user.click(screen.getByText('roles'));

    expect(onFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          type: 'oauth2',
          config: expect.objectContaining({
            token: expect.objectContaining({
              accessToken: expect.objectContaining({
                clientConfig: expect.objectContaining({attributes: ['roles']}) as Record<string, unknown>,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  it('defaults the validity to 3600 and commits a valid change', async () => {
    const user = userEvent.setup();
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups']}
        copy={copy}
        subjectType="application"
        inputId="client-access-token-validity"
      />,
    );

    const input = document.getElementById('client-access-token-validity')!;
    expect(input).toHaveValue(3600);

    await user.clear(input);
    await user.type(input, '7200');

    expect(onFieldChange).toHaveBeenLastCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          config: expect.objectContaining({
            token: expect.objectContaining({
              accessToken: expect.objectContaining({
                clientConfig: expect.objectContaining({validityPeriod: 7200}) as Record<string, unknown>,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  it('reports a validation error without committing when the validity is cleared', async () => {
    const user = userEvent.setup();
    const onValidationChange = vi.fn();
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups']}
        copy={copy}
        subjectType="application"
        inputId="client-access-token-validity"
        onValidationChange={onValidationChange}
      />,
    );

    onFieldChange.mockClear();
    await user.clear(document.getElementById('client-access-token-validity')!);

    expect(screen.getByText('Enter a validity period of at least 1 second.')).toBeInTheDocument();
    expect(onValidationChange).toHaveBeenLastCalledWith(true);
    expect(onFieldChange).not.toHaveBeenCalled();
  });

  it('shows the supplied subjectValue as the sub claim in the preview', () => {
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups']}
        copy={copy}
        subjectType="application"
        subjectValue="my-agent-123"
      />,
    );

    const preview = JSON.parse(screen.getByTestId('jwt-preview').textContent ?? '{}') as {sub?: string};
    expect(preview.sub).toBe('my-agent-123');
  });

  // A selected sub_type shows its literal value, not a <placeholder>: the server fixes it.
  it.each([
    ['application', 'application'],
    ['agent', 'agent'],
  ] as const)('shows sub_type=%s in the preview for a %s client', (subjectType, expected) => {
    render(
      <ClientAccessTokenSection
        oauth2Config={
          {
            grantTypes: ['client_credentials'],
            responseTypes: [],
            token: {accessToken: {clientConfig: {attributes: ['sub_type']}}},
          } as unknown as OAuth2Config
        }
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups', 'sub_type']}
        copy={copy}
        subjectType={subjectType}
      />,
    );

    const preview = JSON.parse(screen.getByTestId('jwt-preview').textContent ?? '{}') as {sub_type?: string};
    expect(preview.sub_type).toBe(expected);
  });

  // Creation seeds sub_type, so its chip starts filled and clicking it removes the claim.
  it('removes sub_type from the selection when its chip is clicked', async () => {
    const user = userEvent.setup();
    render(
      <ClientAccessTokenSection
        oauth2Config={
          {
            grantTypes: ['client_credentials'],
            responseTypes: [],
            token: {accessToken: {clientConfig: {attributes: ['sub_type']}}},
          } as unknown as OAuth2Config
        }
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups', 'sub_type']}
        copy={copy}
        subjectType="agent"
      />,
    );

    await user.click(screen.getByText('sub_type'));

    expect(onFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          config: expect.objectContaining({
            token: expect.objectContaining({
              accessToken: expect.objectContaining({
                clientConfig: expect.objectContaining({attributes: []}) as Record<string, unknown>,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  it('adds sub_type back to the selection when its chip is clicked while unselected', async () => {
    const user = userEvent.setup();
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups', 'sub_type']}
        copy={copy}
        subjectType="agent"
      />,
    );

    await user.click(screen.getByText('sub_type'));

    expect(onFieldChange).toHaveBeenCalledWith(
      'inboundAuthConfig',
      expect.arrayContaining([
        expect.objectContaining({
          config: expect.objectContaining({
            token: expect.objectContaining({
              accessToken: expect.objectContaining({
                clientConfig: expect.objectContaining({attributes: ['sub_type']}) as Record<string, unknown>,
              }) as Record<string, unknown>,
            }) as Record<string, unknown>,
          }) as Record<string, unknown>,
        }),
      ]),
    );
  });

  // A client without the claim selected must not see it in the preview.
  it('omits sub_type from the preview when it is not selected', () => {
    render(
      <ClientAccessTokenSection
        oauth2Config={
          {
            grantTypes: ['client_credentials'],
            responseTypes: [],
            token: {accessToken: {clientConfig: {attributes: ['groups']}}},
          } as unknown as OAuth2Config
        }
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups', 'sub_type']}
        copy={copy}
        subjectType="agent"
      />,
    );

    const preview = JSON.parse(screen.getByTestId('jwt-preview').textContent ?? '{}') as Record<string, unknown>;
    expect(preview).not.toHaveProperty('sub_type');
    expect(preview.groups).toBe('<groups>');
  });

  it('falls back to the <sub> placeholder when no subjectValue is supplied', () => {
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups']}
        copy={copy}
        subjectType="application"
      />,
    );

    const preview = JSON.parse(screen.getByTestId('jwt-preview').textContent ?? '{}') as {sub?: string};
    expect(preview.sub).toBe('<sub>');
  });

  it('disables the validity input when disabled', () => {
    render(
      <ClientAccessTokenSection
        oauth2Config={{grantTypes: ['client_credentials'], responseTypes: []}}
        inboundAuthConfig={inboundAuthConfig}
        onFieldChange={onFieldChange}
        availableAttributes={['groups']}
        copy={copy}
        subjectType="application"
        inputId="client-access-token-validity"
        disabled
      />,
    );

    expect(document.getElementById('client-access-token-validity')!).toBeDisabled();
  });
});
