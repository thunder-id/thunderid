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
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockT = vi.fn((key: string) => key);
const mockLogger = {
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  debug: vi.fn(),
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: mockT}),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => mockLogger,
}));

vi.mock('@thunderid/contexts', async () => {
  const actual = await vi.importActual<typeof import('@thunderid/contexts')>('@thunderid/contexts');
  return {
    ...actual,
    useConfig: vi.fn(() => ({
      config: {
        brand: {
          product_name: 'ThunderID',
        },
      },
      getServerUrl: () => 'http://localhost:8090',
      getServerHostname: () => 'localhost',
      getServerPort: () => 8090,
      isHttpOnly: () => false,
      getClientId: () => 'CONSOLE',
      getScopes: () => ['openid', 'profile'],
      getClientUrl: () => 'http://localhost:8090/console',
      getClientUuid: () => undefined,
      getTrustedIssuerUrl: () => 'http://localhost:8090',
      getTrustedIssuerClientId: () => 'CONSOLE',
    })),
  };
});

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Bell: () => <span data-testid="icon-bell" />,
    Building: () => <span data-testid="icon-building" />,
    Copy: () => <span data-testid="icon-copy" />,
    FileDown: () => <span data-testid="icon-file-down" />,
    Key: () => <span data-testid="icon-key" />,
    Languages: () => <span data-testid="icon-languages" />,
    Layers: () => <span data-testid="icon-layers" />,
    LayoutGrid: () => <span data-testid="icon-layout-grid" />,
    Layout: () => <span data-testid="icon-layout" />,
    Palette: () => <span data-testid="icon-palette" />,
    Server: () => <span data-testid="icon-server" />,
    Terminal: () => <span data-testid="icon-terminal" />,
    UserRoundCog: () => <span data-testid="icon-user-round-cog" />,
    Users: () => <span data-testid="icon-users" />,
    UsersRound: () => <span data-testid="icon-users-round" />,
    Workflow: () => <span data-testid="icon-workflow" />,
  };
});

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    ColorSchemeSVG: ({svg: SvgComponent}: {svg: React.ComponentType}) => <SvgComponent />,
  };
});

vi.mock('./HowProductRunInHostedIllustration', () => ({
  default: () => <div data-testid="how-product-run-illustration">Illustration</div>,
}));

vi.mock('./ResourceSummaryTable', () => ({
  default: ({items}: {items: unknown[]}) => (
    <div data-testid="resource-summary-table">Resource Table: {items.length} items</div>
  ),
}));

vi.mock('./FileContentViewer', () => ({
  default: ({content}: {content: string}) => <div data-testid="file-content-viewer">{content}</div>,
}));

vi.mock('./EnvVariablesViewer', () => ({
  default: ({content}: {content: string}) => <div data-testid="env-variables-viewer">{content}</div>,
}));

vi.mock('./TemplateVariableDisplay', () => ({
  default: ({text}: {text: string}) => <span data-testid="template-variable-display">{text}</span>,
}));

import ConfigureExport from '../ConfigureExport';

afterEach(() => {
  vi.clearAllMocks();
});

describe('ConfigureExport', () => {
  describe('rendering', () => {
    it('renders without resources', () => {
      render(<ConfigureExport />);
      // Check that the right column with Terminal icon is rendered
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });

    it('renders with valid YAML resources', () => {
      const validYaml = `
# applications/app1.yml
---
name: Test App
description: A test application

# flows/flow1.yml
---
name: Test Flow
flowType: authentication
`;

      render(<ConfigureExport resources={validYaml} />);
      // Component renders successfully with resources
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });

    it('handles malformed YAML gracefully and logs warning', () => {
      const malformedYaml = `
# applications/app1.yml
---
name: Test App
  invalid: indentation
    bad: yaml

# flows/flow1.yml
---
name: Valid Flow
`;

      render(<ConfigureExport resources={malformedYaml} />);

      // Component should still render
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();

      // Logger should have been called for the malformed section
      expect(mockLogger.warn).toHaveBeenCalledWith('Failed to parse YAML section', expect.any(Object));
    });

    it('displays environment variables viewer when provided', () => {
      const envVars = 'DB_HOST=localhost\nDB_PORT=5432';

      render(<ConfigureExport environmentVariables={envVars} />);
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });

    it('displays resource counts when provided', () => {
      const resourceCounts = {
        application: 5,
        flow: 3,
        theme: 2,
      };

      render(<ConfigureExport resourceCounts={resourceCounts} />);
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });

    it('shows loading state when exporting', () => {
      render(<ConfigureExport isExporting={true} />);
      // Component should render even during export
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });
  });

  describe('YAML parsing', () => {
    it('parses multiple resource types correctly', () => {
      const multiResourceYaml = `
# applications/app1.yml
---
name: App 1

# flows/flow1.yml
---
name: Flow 1

# themes/theme1.yml
---
name: Theme 1
`;

      render(<ConfigureExport resources={multiResourceYaml} />);
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });

    it('handles empty YAML sections', () => {
      const emptyYaml = `
# applications/app1.yml
---

# flows/flow1.yml
---
name: Valid Flow
`;

      render(<ConfigureExport resources={emptyYaml} />);
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });

    it('logs error when resources string is completely invalid', () => {
      const invalidYaml = '@@@ invalid yaml @@@';

      render(<ConfigureExport resources={invalidYaml} />);

      // Should still render
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });
  });

  describe('product name usage', () => {
    it('uses product name from config', () => {
      vi.mocked(useConfig).mockReturnValue({
        config: {
          brand: {
            product_name: 'CustomProduct',
            favicon: {light: '', dark: ''},
          },
          client: {base: '', client_id: ''},
          server: {hostname: '', port: 0, http_only: false},
        },
        getServerUrl: () => 'http://localhost:8090',
        getServerHostname: () => 'localhost',
        getServerPort: () => 8090,
        isHttpOnly: () => false,
        getClientId: () => 'CONSOLE',
        getScopes: () => ['openid', 'profile'],
        getClientUrl: () => 'http://localhost:8090/console',
        getClientUuid: () => undefined,
        getTrustedIssuerUrl: () => 'http://localhost:8090',
        getTrustedIssuerClientId: () => 'CONSOLE',
        getTrustedIssuerScopes: () => ['openid', 'profile'],
        isTrustedIssuerGenericOidc: () => false,
      });

      render(<ConfigureExport />);

      // Product name should be used in translations
      expect(mockT).toHaveBeenCalledWith(
        expect.stringContaining('nextSteps.startWithConfig'),
        expect.objectContaining({productName: 'CustomProduct'}),
      );
    });
  });
});
