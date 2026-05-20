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
import HowSolutionWorksIllustration from '../HowSolutionWorksIllustration';

const mockT = vi.fn((key: string, options?: {productName?: string}) => {
  if (key === 'howSolutionWorksIllustration:runtimeLocal' && options?.productName) {
    return `Runtime (Local) - ${String(options.productName)}`;
  }
  if (key === 'howSolutionWorksIllustration:console' && options?.productName) {
    return `Console - ${String(options.productName)}`;
  }
  if (key === 'howSolutionWorksIllustration:runtimeHosted' && options?.productName) {
    return `Runtime (Hosted) - ${String(options.productName)}`;
  }
  if (key === 'howSolutionWorksIllustration:runInProduction' && options?.productName) {
    return `How ${String(options.productName)} Runs in Production`;
  }
  if (key === 'howSolutionWorksIllustration:designConfigure' && options?.productName) {
    return `Design & Configure ${String(options.productName)}`;
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

describe('HowSolutionWorksIllustration', () => {
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

  describe('rendering', () => {
    it('renders the SVG illustration', () => {
      const {container} = render(<HowSolutionWorksIllustration />);
      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('renders with correct viewBox dimensions', () => {
      const {container} = render(<HowSolutionWorksIllustration />);
      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('viewBox', '0 0 1237 350');
      expect(svg).toHaveAttribute('width', '1237');
      expect(svg).toHaveAttribute('height', '350');
    });

    it('has correct display name', () => {
      expect(HowSolutionWorksIllustration.displayName).toBe('HowSolutionWorksIllustration');
    });
  });

  describe('workflow steps', () => {
    it('displays all workflow step labels', () => {
      render(<HowSolutionWorksIllustration />);

      // Step 1: Design & Configure
      expect(screen.getByText('howSolutionWorksIllustration:configureProject')).toBeInTheDocument();

      // Step 2: Validate/Test
      expect(screen.getByText('howSolutionWorksIllustration:validateTest')).toBeInTheDocument();

      // Step 3: Save/Export
      expect(screen.getByText('howSolutionWorksIllustration:saveExport')).toBeInTheDocument();

      // Step 4: Import
      expect(screen.getByText('howSolutionWorksIllustration:import')).toBeInTheDocument();

      // Step 5: Run
      expect(screen.getByText('howSolutionWorksIllustration:run')).toBeInTheDocument();
    });

    it('displays environment and configuration labels', () => {
      render(<HowSolutionWorksIllustration />);

      expect(screen.getByText('howSolutionWorksIllustration:projectEnvConfigs')).toBeInTheDocument();
      expect(screen.getByText('howSolutionWorksIllustration:runtimeComponentsOnly')).toBeInTheDocument();
      expect(screen.getByText('howSolutionWorksIllustration:designComponents')).toBeInTheDocument();
    });

    it('displays application component labels', () => {
      render(<HowSolutionWorksIllustration />);

      expect(screen.getByText('howSolutionWorksIllustration:adminApp')).toBeInTheDocument();
      expect(screen.getByText('howSolutionWorksIllustration:loginApp')).toBeInTheDocument();
    });

    it('displays command labels', () => {
      render(<HowSolutionWorksIllustration />);

      expect(screen.getByText('howSolutionWorksIllustration:commandProduction')).toBeInTheDocument();
      expect(screen.getByText('howSolutionWorksIllustration:commandStart')).toBeInTheDocument();
    });
  });

  describe('product name interpolation', () => {
    it('displays product name in local runtime text', () => {
      render(<HowSolutionWorksIllustration />);

      expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:runtimeLocal', {productName: 'ThunderID'});
      expect(screen.getByText('Runtime (Local) - ThunderID')).toBeInTheDocument();
    });

    it('displays product name in console text', () => {
      render(<HowSolutionWorksIllustration />);

      expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:console', {productName: 'ThunderID'});
      expect(screen.getByText('Console - ThunderID')).toBeInTheDocument();
    });

    it('displays product name in hosted runtime text', () => {
      render(<HowSolutionWorksIllustration />);

      expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:runtimeHosted', {productName: 'ThunderID'});
      expect(screen.getByText('Runtime (Hosted) - ThunderID')).toBeInTheDocument();
    });

    it('displays product name in title text', () => {
      render(<HowSolutionWorksIllustration />);

      expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:runInProduction', {productName: 'ThunderID'});
      expect(screen.getByText('How ThunderID Runs in Production')).toBeInTheDocument();
    });

    it('displays product name in design & configure text', () => {
      render(<HowSolutionWorksIllustration />);

      expect(mockT).toHaveBeenCalledWith('howSolutionWorksIllustration:designConfigure', {productName: 'ThunderID'});
      expect(screen.getByText('Design & Configure ThunderID')).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
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

      const {container} = render(<HowSolutionWorksIllustration />);
      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('handles missing brand config gracefully', () => {
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

      const {container} = render(<HowSolutionWorksIllustration />);
      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });
  });

  describe('SVG structure', () => {
    it('renders all main SVG elements', () => {
      const {container} = render(<HowSolutionWorksIllustration />);

      // Check for paths (workflow boxes, arrows, icons)
      const paths = container.querySelectorAll('path');
      expect(paths.length).toBeGreaterThan(0);

      // Check for text elements
      const textElements = container.querySelectorAll('text');
      expect(textElements.length).toBeGreaterThan(0);

      // Check for rectangles (command boxes)
      const rects = container.querySelectorAll('rect');
      expect(rects.length).toBeGreaterThan(0);

      // Check for lines (arrows)
      const lines = container.querySelectorAll('line');
      expect(lines.length).toBeGreaterThan(0);
    });

    it('renders command boxes with correct styling', () => {
      const {container} = render(<HowSolutionWorksIllustration />);

      const commandBoxes = container.querySelectorAll('rect[fill="black"]');
      expect(commandBoxes.length).toBe(2); // commandStart and commandProduction
    });

    it('renders text with proper attributes', () => {
      const {container} = render(<HowSolutionWorksIllustration />);

      // Check for text with different fill colors
      const mutedText = container.querySelectorAll('text[fill="muted"]');
      expect(mutedText.length).toBeGreaterThan(0);

      const whiteText = container.querySelectorAll('text[fill="white"]');
      expect(whiteText.length).toBe(2); // Commands in black boxes

      const currentColorText = container.querySelectorAll('text[fill="currentColor"]');
      expect(currentColorText.length).toBeGreaterThan(0);
    });
  });

  describe('workflow sections', () => {
    it('renders configure/design section', () => {
      const {container} = render(<HowSolutionWorksIllustration />);

      // Check for gear/settings icon paths (configure section)
      const paths = container.querySelectorAll('path[stroke="primary"]');
      expect(paths.length).toBeGreaterThan(0);
    });

    it('renders validate/test section', () => {
      render(<HowSolutionWorksIllustration />);

      expect(screen.getByText('howSolutionWorksIllustration:validateTest')).toBeInTheDocument();
    });

    it('renders export/import section', () => {
      render(<HowSolutionWorksIllustration />);

      expect(screen.getByText('howSolutionWorksIllustration:saveExport')).toBeInTheDocument();
      expect(screen.getByText('howSolutionWorksIllustration:import')).toBeInTheDocument();
    });

    it('renders runtime sections', () => {
      render(<HowSolutionWorksIllustration />);

      expect(screen.getByText('Runtime (Local) - ThunderID')).toBeInTheDocument();
      expect(screen.getByText('Runtime (Hosted) - ThunderID')).toBeInTheDocument();
    });
  });
});
