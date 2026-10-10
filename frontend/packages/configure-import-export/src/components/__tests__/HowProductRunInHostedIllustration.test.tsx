// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import HowProductRunInHostedIllustration from '../HowProductRunInHostedIllustration';

vi.mock('@thunderid/contexts', async () => {
  const actual = await vi.importActual<typeof import('@thunderid/contexts')>('@thunderid/contexts');
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const mockUseConfig = vi.mocked(useConfig);

afterEach(() => {
  vi.clearAllMocks();
});

describe('HowProductRunInHostedIllustration', () => {
  beforeEach(() => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          product_name: 'ThunderID',
          favicon: {light: '', dark: ''},
        },
        client: {base: '', client_id: ''},
        server: {hostname: '', port: 0, http_only: false},
      },
      getServerUrl: () => '',
      getGateCallbackUrl: () => '',
      getServerHostname: () => '',
      getServerPort: () => 0,
      isHttpOnly: () => false,
      getClientId: () => '',
      getScopes: () => [],
      getResourceIdentifier: () => undefined,
      getClientUrl: () => '',
      getClientUuid: () => undefined,
      getTrustedIssuerUrl: () => '',
      getTrustedIssuerClientId: () => '',
      getTrustedIssuerScopes: () => [],
      isTrustedIssuerGenericOidc: () => false,
      isControlPlane: () => false,
      getDocumentationLink: () => undefined,
    });
  });

  it('renders the SVG illustration', () => {
    const {container} = render(<HowProductRunInHostedIllustration />);
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
  });

  it('renders with correct viewBox dimensions', () => {
    const {container} = render(<HowProductRunInHostedIllustration />);
    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('viewBox', '0 0 548 279');
  });

  it('displays translated text elements', () => {
    render(<HowProductRunInHostedIllustration />);

    expect(screen.getByText('Run')).toBeInTheDocument();
    expect(screen.getByText('Project + ENV Configs')).toBeInTheDocument();
    expect(screen.getByText('Attach')).toBeInTheDocument();
    expect(screen.getByText('(with required runtime components only)')).toBeInTheDocument();
    expect(screen.getByText('Admin App')).toBeInTheDocument();
    expect(screen.getByText('Login App')).toBeInTheDocument();
  });

  it('displays product name in title', () => {
    render(<HowProductRunInHostedIllustration />);

    expect(screen.getByText('Run ThunderID in Production')).toBeInTheDocument();
  });

  it('displays product name in runtime text', () => {
    render(<HowProductRunInHostedIllustration />);

    expect(screen.getByText('ThunderID Runtime Hosted')).toBeInTheDocument();
  });

  it('handles missing product name gracefully', () => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {product_name: '', favicon: {light: '', dark: ''}},
        client: {base: '', client_id: ''},
        server: {hostname: '', port: 0, http_only: false},
      },
      getServerUrl: () => '',
      getGateCallbackUrl: () => '',
      getServerHostname: () => '',
      getServerPort: () => 0,
      isHttpOnly: () => false,
      getClientId: () => '',
      getScopes: () => [],
      getResourceIdentifier: () => undefined,
      getClientUrl: () => '',
      getClientUuid: () => undefined,
      getTrustedIssuerUrl: () => '',
      getTrustedIssuerClientId: () => '',
      getTrustedIssuerScopes: () => [],
      isTrustedIssuerGenericOidc: () => false,
      isControlPlane: () => false,
      getDocumentationLink: () => undefined,
    });

    const {container} = render(<HowProductRunInHostedIllustration />);
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
  });

  it('has correct display name', () => {
    expect(HowProductRunInHostedIllustration.displayName).toBe('HowProductRunInHostedIllustration');
  });

  it('renders all SVG paths and graphics', () => {
    const {container} = render(<HowProductRunInHostedIllustration />);

    // Check for main server paths
    const paths = container.querySelectorAll('path');
    expect(paths.length).toBeGreaterThan(0);

    // Check for arrow lines
    const lines = container.querySelectorAll('line');
    expect(lines.length).toBeGreaterThan(0);
  });
});
