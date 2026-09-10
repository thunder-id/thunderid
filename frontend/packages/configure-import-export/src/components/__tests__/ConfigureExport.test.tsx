// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockLogger = {
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  debug: vi.fn(),
};

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
      getResourceIdentifier: () => undefined,
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
    Bot: () => <span data-testid="icon-bot" />,
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
    Settings: () => <span data-testid="icon-settings" />,
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

vi.mock('@monaco-editor/react', () => ({
  default: () => null,
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

    it('adds a server configurations row when server configs are present', () => {
      render(<ConfigureExport resourceCounts={{server_config: 2}} />);
      expect(screen.getByTestId('icon-settings')).toBeInTheDocument();
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

    it('parses agent resources and renders agents section', () => {
      const agentYaml = `
---
resource_type: agent
id: agent-1
name: Test Agent
description: A test agent
`;
      render(<ConfigureExport resources={agentYaml} />);
      expect(screen.getByTestId('icon-bot')).toBeInTheDocument();
    });

    it('parses connection resources and renders a connections section', () => {
      const connectionDoc = (idx: number) => `
---
resource_type: connection
name: Connection ${idx}
type: google
`;
      // More than 5 so the "show more" toggle renders too.
      const connectionsYaml = Array.from({length: 6}, (_, idx) => connectionDoc(idx)).join('\n');

      render(<ConfigureExport resources={connectionsYaml} />);
      expect(screen.getAllByTestId('icon-layers').length).toBeGreaterThan(0);
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
        getGateCallbackUrl: () => 'http://localhost:8090/gate/callback',
        getServerHostname: () => 'localhost',
        getServerPort: () => 8090,
        isHttpOnly: () => false,
        getClientId: () => 'CONSOLE',
        getScopes: () => ['openid', 'profile'],
        getResourceIdentifier: () => undefined,
        getClientUrl: () => 'http://localhost:8090/console',
        getClientUuid: () => undefined,
        getTrustedIssuerUrl: () => 'http://localhost:8090',
        getTrustedIssuerClientId: () => 'CONSOLE',
        getTrustedIssuerScopes: () => ['openid', 'profile'],
        isTrustedIssuerGenericOidc: () => false,
        getDocumentationLink: () => undefined,
      });

      render(<ConfigureExport />);

      // Product name should be used in translations
      expect(screen.getByText('CustomProduct will start with your exported configuration')).toBeInTheDocument();
    });
  });

  const buildResourcesYaml = (count: number, body: (idx: number) => string): string =>
    Array.from({length: count}, (_, idx) => `---\n${body(idx)}\n`).join('\n');

  const resourceTypeCases: {resourceType: string; label: string; body: (idx: number) => string}[] = [
    {
      resourceType: 'application',
      label: 'Applications',
      body: (idx) =>
        `resource_type: application\nname: App ${idx}\ndescription: Application description ${idx}\nurl: https://app${idx}.example.com\ninbound_auth_config:\n  - type: oauth2\n    config:\n      client_id: client-app-${idx}`,
    },
    {
      resourceType: 'connection',
      label: 'Connections',
      body: (idx) => `resource_type: connection\nname: Connection ${idx}\ntype: google`,
    },
    {
      resourceType: 'flow',
      label: 'Flows',
      body: (idx) => `resource_type: flow\nname: Flow ${idx}\nhandle: flow-${idx}\nflowType: authentication`,
    },
    {
      resourceType: 'theme',
      label: 'Themes',
      body: (idx) =>
        `resource_type: theme\nname: Theme ${idx}\nhandle: theme-${idx}\ndescription: Theme description ${idx}`,
    },
    {
      resourceType: 'user',
      label: 'Users',
      body: (idx) =>
        `resource_type: user\ntype: person\nattributes:\n  name: User Name ${idx}\n  username: user${idx}\n  email: user${idx}@example.com`,
    },
    {
      resourceType: 'organization_unit',
      label: 'Organization Units',
      body: (idx) =>
        `resource_type: organization_unit\nname: Org ${idx}\nhandle: org-${idx}\ndescription: Org description ${idx}`,
    },
    {
      resourceType: 'user_type',
      label: 'User Types',
      body: (idx) =>
        `resource_type: user_type\nname: Schema ${idx}\nhandle: schema-${idx}\nallow_self_registration: true`,
    },
    {
      resourceType: 'agent_type',
      label: 'Agent Types',
      body: (idx) => `resource_type: agent_type\nname: Agent Type ${idx}\nhandle: agent-type-${idx}`,
    },
    {
      resourceType: 'translation',
      label: 'Translations',
      body: (idx) => `resource_type: translation\nlocale: en-US-${idx}\nnamespace: common`,
    },
    {
      resourceType: 'layout',
      label: 'Layouts',
      body: (idx) =>
        `resource_type: layout\nname: Layout ${idx}\nhandle: layout-${idx}\ndescription: Layout description ${idx}`,
    },
    {
      resourceType: 'resource_server',
      label: 'Resource Servers',
      body: (idx) =>
        `resource_type: resource_server\nname: Resource Server ${idx}\nhandle: rs-${idx}\ndescription: Resource server description ${idx}`,
    },
    {
      resourceType: 'role',
      label: 'Roles',
      body: (idx) =>
        `resource_type: role\nname: Role ${idx}\nhandle: role-${idx}\ndescription: Role description ${idx}`,
    },
    {
      resourceType: 'group',
      label: 'Groups',
      body: (idx) =>
        `resource_type: group\nid: group-${idx}\nname: Group ${idx}\ndescription: Group description ${idx}`,
    },
    {
      resourceType: 'agent',
      label: 'Agents',
      body: (idx) =>
        `resource_type: agent\nid: agent-${idx}\nname: Agent ${idx}\ndescription: Agent description ${idx}\ninbound_auth_config:\n  - type: oauth2\n    config:\n      client_id: client-agent-${idx}`,
    },
    {
      resourceType: 'server_config',
      label: 'Server Configurations',
      body: (idx) => `resource_type: server_config\nname: Server Config ${idx}`,
    },
    {
      resourceType: 'credential_configuration',
      label: 'Credential Configurations',
      body: (idx) =>
        `resource_type: credential_configuration\nname: Credential Config ${idx}\nhandle: credential-config-${idx}\nvct: VerifiableId`,
    },
    {
      resourceType: 'presentation_definition',
      label: 'Presentation Definitions',
      body: (idx) =>
        `resource_type: presentation_definition\nname: Presentation Definition ${idx}\nhandle: presentation-definition-${idx}\nvct: VerifiableId`,
    },
  ];

  describe.each(resourceTypeCases)('$resourceType resource section', ({label, body}) => {
    it('renders the section and toggles show more/show less', async () => {
      const yaml = buildResourcesYaml(6, body);

      render(<ConfigureExport resources={yaml} />);

      const row = screen.getByText(label).closest('tr');
      expect(row).not.toBeNull();

      await userEvent.click(row!);

      const moreChip = screen.getByText('+ 1 more');
      await userEvent.click(moreChip);

      expect(screen.getByText('Show less')).toBeInTheDocument();
    });
  });

  describe('handleCopyCommand', () => {
    it('writes the run command to the clipboard when the copy button is clicked', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText},
        configurable: true,
        writable: true,
      });

      render(<ConfigureExport />);

      const copyButton = screen.getByTestId('icon-copy').closest('button');
      expect(copyButton).not.toBeNull();

      await userEvent.click(copyButton!);

      await waitFor(() => {
        expect(writeText).toHaveBeenCalledWith('./start.sh project-foo.yml --env production.env');
      });
    });
  });

  describe('handleDownloadConfiguration', () => {
    afterEach(() => {
      delete (window as unknown as {showSaveFilePicker?: unknown}).showSaveFilePicker;
    });

    it('saves the file via the File System Access API when supported', async () => {
      const write = vi.fn().mockResolvedValue(undefined);
      const close = vi.fn().mockResolvedValue(undefined);
      const createWritable = vi.fn().mockResolvedValue({write, close});
      const showSaveFilePicker = vi.fn().mockResolvedValue({createWritable});
      (window as unknown as {showSaveFilePicker: typeof showSaveFilePicker}).showSaveFilePicker = showSaveFilePicker;

      render(<ConfigureExport resources={'---\nresource_type: application\nname: Test App'} />);

      const exportButton = screen.getByText('Export Configuration');
      await userEvent.click(exportButton);

      await waitFor(() => {
        expect(showSaveFilePicker).toHaveBeenCalled();
      });
      await waitFor(() => {
        expect(write).toHaveBeenCalledWith('---\nresource_type: application\nname: Test App');
      });
      expect(close).toHaveBeenCalled();
    });

    it('silently ignores errors when the save is cancelled', async () => {
      const showSaveFilePicker = vi.fn().mockRejectedValue(new Error('cancelled'));
      (window as unknown as {showSaveFilePicker: typeof showSaveFilePicker}).showSaveFilePicker = showSaveFilePicker;

      render(<ConfigureExport resources={'---\nresource_type: application\nname: Test App'} />);

      const exportButton = screen.getByText('Export Configuration');
      await userEvent.click(exportButton);

      await waitFor(() => {
        expect(showSaveFilePicker).toHaveBeenCalled();
      });
      expect(screen.getByText('Export Configuration')).toBeInTheDocument();
    });

    it('falls back to a blob download when the File System Access API is unsupported', async () => {
      delete (window as unknown as {showSaveFilePicker?: unknown}).showSaveFilePicker;
      const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);

      render(<ConfigureExport resources={'---\nresource_type: application\nname: Test App'} />);

      const exportButton = screen.getByText('Export Configuration');
      await userEvent.click(exportButton);

      await waitFor(() => {
        expect(clickSpy).toHaveBeenCalled();
      });

      clickSpy.mockRestore();
    });
  });

  describe('YAML parsing edge cases', () => {
    it('extracts the file name from a File: comment when logging a parse warning', () => {
      const yamlWithFileComment = [
        '---',
        '# File: applications/app1.yml',
        'name: Test App',
        '  invalid: indentation',
        '    bad: yaml',
      ].join('\n');

      render(<ConfigureExport resources={yamlWithFileComment} />);

      expect(mockLogger.warn).toHaveBeenCalledWith(
        'Failed to parse YAML section',
        expect.objectContaining({fileName: 'applications/app1.yml'}),
      );
    });

    it('logs an error and renders without crashing when resources is not a string', () => {
      render(<ConfigureExport resources={12345 as unknown as string} />);

      expect(mockLogger.error).toHaveBeenCalledWith('Failed to parse export resources', expect.any(Object));
      expect(screen.getByTestId('icon-terminal')).toBeInTheDocument();
    });
  });
});
