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

import {useConfig} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import HowProductRunInHostedIllustration from '../HowProductRunInHostedIllustration';

const mockT = vi.fn((key: string, options?: {productName?: string}) => {
  if (key === 'howSolutionWorksIllustration:runtimeHosted' && options?.productName) {
    return `Runtime (Hosted) - ${String(options.productName)}`;
  }
  if (key === 'howSolutionWorksIllustration:runInProduction' && options?.productName) {
    return `How ${String(options.productName)} Runs in Production`;
  }
  return key;
});

vi.mock('@thunderid/contexts', async () => {
  const actual = await vi.importActual<typeof import('@thunderid/contexts')>('@thunderid/contexts');
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: mockT}),
}));

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
      getServerHostname: () => '',
      getServerPort: () => 0,
      isHttpOnly: () => false,
      getClientId: () => '',
      getScopes: () => [],
      getClientUrl: () => '',
      getClientUuid: () => undefined,
      getTrustedIssuerUrl: () => '',
      getTrustedIssuerClientId: () => '',
      getTrustedIssuerScopes: () => [],
      isTrustedIssuerGenericOidc: () => false,
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

    expect(screen.getByText('howSolutionWorksIllustration:run')).toBeInTheDocument();
    expect(screen.getByText('howSolutionWorksIllustration:projectEnvConfigs')).toBeInTheDocument();
    expect(screen.getByText('howSolutionWorksIllustration:import')).toBeInTheDocument();
    expect(screen.getByText('howSolutionWorksIllustration:runtimeComponentsOnly')).toBeInTheDocument();
    expect(screen.getByText('howSolutionWorksIllustration:adminApp')).toBeInTheDocument();
    expect(screen.getByText('howSolutionWorksIllustration:loginApp')).toBeInTheDocument();
  });

  it('displays product name in title', () => {
    render(<HowProductRunInHostedIllustration />);

    expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:runInProduction', {productName: 'ThunderID'});
    expect(screen.getByText('How ThunderID Runs in Production')).toBeInTheDocument();
  });

  it('displays product name in runtime text', () => {
    render(<HowProductRunInHostedIllustration />);

    expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:runtimeHosted', {productName: 'ThunderID'});
    expect(screen.getByText('Runtime (Hosted) - ThunderID')).toBeInTheDocument();
  });

  it('handles missing product name gracefully', () => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {product_name: '', favicon: {light: '', dark: ''}},
        client: {base: '', client_id: ''},
        server: {hostname: '', port: 0, http_only: false},
      },
      getServerUrl: () => '',
      getServerHostname: () => '',
      getServerPort: () => 0,
      isHttpOnly: () => false,
      getClientId: () => '',
      getScopes: () => [],
      getClientUrl: () => '',
      getClientUuid: () => undefined,
      getTrustedIssuerUrl: () => '',
      getTrustedIssuerClientId: () => '',
      getTrustedIssuerScopes: () => [],
      isTrustedIssuerGenericOidc: () => false,
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
