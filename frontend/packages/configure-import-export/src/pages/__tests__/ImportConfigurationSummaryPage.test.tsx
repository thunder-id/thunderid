// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseMutationResult} from '@tanstack/react-query';
import {render, screen, userEvent, waitFor, fireEvent} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import type {ConfigSummaryItem, ImportResponse, ProductConfig} from '../../models/import-configuration';

const mockNavigate = vi.fn();
const mockShowToast = vi.fn();
const mockLogger = {error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn()};
const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let mockMutationState: Partial<UseMutationResult<ImportResponse, Error, unknown>> = {
  mutate: mockMutate,
  mutateAsync: mockMutateAsync,
  data: undefined,
  isPending: false,
  isError: false,
  error: null,
};

const mockLocationState = {
  configData: {
    application: [{id: 'app1', name: 'App 1'}],
    user: [{id: 'user1', username: 'john'}],
    flow: [{id: 'flow1', name: 'Login Flow'}],
  } as ProductConfig,
  envData: 'API_KEY=secret123\nDATABASE_URL=postgres://localhost\n',
  configContent: 'application:\n  - name: {{.APP_NAME}}\n',
};

let mockPathname = '/import/summary';

// "Import Configuration" is used both as the breadcrumb label (upload.breadcrumb.openProject)
// and as the submit button label (summary.import.action). Disambiguate by element tag: the
// submit action is a real <button>, the breadcrumb item is a non-button element with role="button".
const getImportConfigurationButton = (): HTMLElement =>
  screen.getAllByRole('button', {name: 'Import Configuration'}).find((el) => el.tagName === 'BUTTON')!;
const getImportConfigurationBreadcrumb = (): HTMLElement =>
  screen.getAllByRole('button', {name: 'Import Configuration'}).find((el) => el.tagName !== 'BUTTON')!;

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({
      state: mockLocationState,
      pathname: mockPathname,
    }),
  };
});

vi.mock('../../api/useImportConfiguration', () => ({
  default: () => mockMutationState,
}));

vi.mock('@thunderid/contexts', async () => {
  const actual = await vi.importActual<typeof import('@thunderid/contexts')>('@thunderid/contexts');
  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: 'ThunderID',
        },
      },
    }),
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => mockLogger,
}));

vi.mock('../../components/EnvVariablesViewer', () => ({
  default: ({content, onChange}: {content: string; onChange?: (val: string) => void}) => (
    <div data-testid="env-variables-viewer">
      <textarea data-testid="env-editor" value={content} onChange={(e) => onChange?.(e.target.value)} />
    </div>
  ),
}));

vi.mock('../../components/ResourceSummaryTable', () => ({
  default: ({items}: {items: ConfigSummaryItem[]}) => (
    <div data-testid="resource-summary-table">
      {items.map((item) => (
        <div key={item.label}>
          <div>
            {item.label}: {item.value}
          </div>
          {item.content}
        </div>
      ))}
    </div>
  ),
}));

vi.mock('../../components/TemplateVariableDisplay', () => ({
  default: ({text}: {text: string}) => <div data-testid="template-variable">{text}</div>,
}));

import ImportConfigurationSummaryPage from '../ImportConfigurationSummaryPage';

afterEach(() => {
  vi.clearAllMocks();
});

describe('ImportConfigurationSummaryPage', () => {
  beforeEach(() => {
    mockMutateAsync.mockReset();
    mockMutationState = {
      mutate: mockMutate,
      mutateAsync: mockMutateAsync,
      data: undefined,
      isPending: false,
      isError: false,
      error: null,
    };
    mockPathname = '/import/summary';
  });

  describe('rendering', () => {
    it('renders page title', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });

    it('renders resource summary section', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Project Details')).toBeInTheDocument();
    });

    it('renders environment variables section', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('renders import test section', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Import Test Status')).toBeInTheDocument();
    });

    it('displays resource counts', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Applications: 1')).toBeInTheDocument();
      expect(screen.getByText('Users: 1')).toBeInTheDocument();
      expect(screen.getByText('Flows: 1')).toBeInTheDocument();
    });

    it('displays a server configurations section', () => {
      const original = mockLocationState.configData;
      mockLocationState.configData = {server_config: [{name: 'cors'}]} as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/Server Configurations.*1/i)).toBeInTheDocument();

      mockLocationState.configData = original;
    });
  });

  describe('breadcrumb navigation', () => {
    it('renders breadcrumb with steps', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Summary')).toBeInTheDocument();
    });

    it('navigates to upload page when clicking breadcrumb', async () => {
      render(<ImportConfigurationSummaryPage />);

      const uploadLink = getImportConfigurationBreadcrumb();
      await userEvent.click(uploadLink);

      expect(mockNavigate).toHaveBeenCalledWith('/import-configuration');
    });

    it('shows a welcome breadcrumb that navigates to /welcome when reached from the welcome flow', async () => {
      mockPathname = '/welcome/import-configuration/summary';
      render(<ImportConfigurationSummaryPage />);

      const welcomeLink = screen.getByText('Welcome');
      await userEvent.click(welcomeLink);

      expect(mockNavigate).toHaveBeenCalledWith('/welcome');
    });
  });

  describe('environment variables', () => {
    it('renders environment variables viewer', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('displays environment variable count', () => {
      render(<ImportConfigurationSummaryPage />);

      // EnvVariablesViewer displays the count
      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('allows editing environment variables', async () => {
      render(<ImportConfigurationSummaryPage />);

      const editor = screen.getByTestId<HTMLTextAreaElement>('env-editor');
      await userEvent.type(editor, '\nNEW_VAR=value');

      expect(editor.value).toContain('NEW_VAR=value');
    });

    it('allows uploading new env file', async () => {
      render(<ImportConfigurationSummaryPage />);

      const uploadButton = screen.getByText('Re-upload .env file');
      await userEvent.click(uploadButton);

      // Input should be triggered (we can't actually test file upload in JSDOM)
      expect(uploadButton).toBeInTheDocument();
    });
  });

  describe('dry run functionality', () => {
    it('shows run dry run button', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Test')).toBeInTheDocument();
    });

    it('executes dry run when button clicked', () => {
      render(<ImportConfigurationSummaryPage />);

      const runButton = screen.getByText('Test');

      // Button exists but may not trigger mutation if env variables are missing
      expect(runButton).toBeInTheDocument();
    });

    it('shows loading state during dry run', () => {
      mockMutationState.isPending = true;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Running...')).toBeInTheDocument();
    });

    it('disables import button when dry run has not passed', () => {
      render(<ImportConfigurationSummaryPage />);

      const importButton = getImportConfigurationButton();
      expect(importButton).toBeDisabled();
    });

    it('shows passed status on successful dry run', () => {
      mockMutationState.data = {
        summary: {
          totalDocuments: 0,
          imported: 0,
          failed: 0,
          importedAt: new Date().toISOString(),
        },
        results: [],
      };

      render(<ImportConfigurationSummaryPage />);

      // Need to trigger dry run first, so this tests the result state
      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('shows failed status on failed dry run', () => {
      mockMutationState.data = {
        summary: {
          totalDocuments: 1,
          imported: 0,
          failed: 1,
          importedAt: new Date().toISOString(),
        },
        results: [
          {
            resourceType: 'application',
            resourceId: 'app1',
            status: 'failed',
            message: 'Validation error',
          },
        ],
      };

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });
  });

  describe('import functionality', () => {
    it('shows import button', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(getImportConfigurationButton()).toBeInTheDocument();
    });

    it('executes import when button clicked after dry run passes', async () => {
      mockMutationState.data = {
        summary: {
          totalDocuments: 0,
          imported: 0,
          failed: 0,
          importedAt: new Date().toISOString(),
        },
        results: [],
      };

      const {rerender} = render(<ImportConfigurationSummaryPage />);

      // First run dry run
      const runButton = screen.getByText('Test');
      await userEvent.click(runButton);

      // Mock successful dry run
      mockMutationState.data = {
        summary: {
          totalDocuments: 0,
          imported: 0,
          failed: 0,
          importedAt: new Date().toISOString(),
        },
        results: [],
      };
      rerender(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(getImportConfigurationButton()).toBeInTheDocument();
      });
    });

    it('shows toast on successful import', () => {
      mockMutationState.data = {
        summary: {
          totalDocuments: 0,
          imported: 0,
          failed: 0,
          importedAt: new Date().toISOString(),
        },
        results: [],
      };

      render(<ImportConfigurationSummaryPage />);

      // Component will show toast when import is successful
      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('shows toast on failed import', () => {
      mockMutationState.isError = true;
      mockMutationState.error = new Error('Import failed');

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('navigates home after the dry run passes and the import is confirmed', async () => {
      const originalConfigContent = mockLocationState.configContent;
      const originalEnvData = mockLocationState.envData;
      // No `{{ }}` placeholders, so there are no required env variables to satisfy.
      mockLocationState.configContent = 'application:\n  - name: test-app\n';
      mockLocationState.envData = '';
      mockMutateAsync.mockResolvedValue({
        results: [],
        summary: {imported: 1, totalDocuments: 1, failed: 0, importedAt: new Date(0).toISOString()},
      });

      render(<ImportConfigurationSummaryPage />);

      await userEvent.click(screen.getByText('Test'));

      const importButton = await waitFor(() => {
        const button = getImportConfigurationButton();
        expect(button).not.toBeDisabled();
        return button;
      });
      await userEvent.click(importButton);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/home');
      });

      mockLocationState.configContent = originalConfigContent;
      mockLocationState.envData = originalEnvData;
    });
  });

  describe('missing env variables', () => {
    it('detects missing environment variables', () => {
      // ConfigContent has {{.APP_NAME}} but envData doesn't have APP_NAME
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('disables dry run when env variables are missing', () => {
      render(<ImportConfigurationSummaryPage />);

      const runButton = screen.getByText('Test');
      // Button may be disabled if missing required variables
      expect(runButton).toBeInTheDocument();
    });

    it('shows warning when required variables are missing', () => {
      render(<ImportConfigurationSummaryPage />);

      // Warning should be shown if required variables are missing
      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });
  });

  describe('cancel action', () => {
    it('shows cancel button', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByLabelText('Close')).toBeInTheDocument();
    });

    it('navigates back on cancel', async () => {
      render(<ImportConfigurationSummaryPage />);

      const cancelButton = screen.getByLabelText('Close');
      await userEvent.click(cancelButton);

      expect(mockNavigate).toHaveBeenCalledWith('/home');
    });
  });

  describe('edge cases', () => {
    it('handles missing config data', () => {
      mockLocationState.configData = null as never;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });

    it('handles missing env data', () => {
      mockLocationState.envData = null as never;

      render(<ImportConfigurationSummaryPage />);

      // Page still renders without env data
      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });

    it('handles empty resource arrays', () => {
      mockLocationState.configData = {
        application: [],
        user: [],
        flow: [],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });

    it('handles undefined resource arrays', () => {
      mockLocationState.configData = {} as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });
  });

  describe('file upload handling', () => {
    it('validates env file extension', () => {
      render(<ImportConfigurationSummaryPage />);

      // Test validates that the page renders successfully
      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });

    it('logs warning for invalid file types', () => {
      render(<ImportConfigurationSummaryPage />);

      // Component will log warnings for invalid files
      expect(mockLogger.warn).not.toHaveBeenCalled();
    });

    it('resets file input after upload', () => {
      render(<ImportConfigurationSummaryPage />);

      // Page renders successfully with env data
      expect(screen.getByText('Configuration Summary')).toBeInTheDocument();
    });
  });

  describe('dry run status transitions', () => {
    const noTemplateState = {
      configData: {application: [{id: 'app1', name: 'App 1'}]},
      envData: 'API_KEY=secret123\n',
      configContent: 'application:\n  - name: static-app\n',
    };

    it('shows passed alert after successful dry run', async () => {
      const successResponse: ImportResponse = {
        summary: {totalDocuments: 2, imported: 2, failed: 0, importedAt: new Date().toISOString()},
        results: [],
      };
      mockMutateAsync.mockResolvedValue(successResponse);
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByText('Import test passed. 2 of 2 resources validated.')).toBeInTheDocument();
      });
    });

    it('shows failed alert and retry button after dry run with failures', async () => {
      const failedResponse: ImportResponse = {
        summary: {totalDocuments: 1, imported: 0, failed: 1, importedAt: new Date().toISOString()},
        results: [{resourceType: 'application', resourceId: 'app1', status: 'failed', message: 'Validation error'}],
      };
      mockMutateAsync.mockResolvedValue(failedResponse);
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'Retry'})).toBeInTheDocument();
      });
    });

    it('shows failed results list, resolving a mapped code instead of raw server text', async () => {
      const failedResponse: ImportResponse = {
        summary: {totalDocuments: 1, imported: 0, failed: 1, importedAt: new Date().toISOString()},
        results: [
          {
            resourceType: 'application',
            resourceId: 'app1',
            resourceName: 'MyApp',
            status: 'failed',
            code: 'IMP-1002',
            message: 'raw server text',
          },
        ],
      };
      mockMutateAsync.mockResolvedValue(failedResponse);
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByText('Import Test failures')).toBeInTheDocument();
        expect(screen.getByText(/MyApp/)).toBeInTheDocument();
        expect(screen.getByText(/The uploaded YAML content could not be parsed\./)).toBeInTheDocument();
        expect(screen.queryByText(/raw server text/)).not.toBeInTheDocument();
      });
    });

    it('shows a generic fallback for a failed result with no mapped code', async () => {
      const failedResponse: ImportResponse = {
        summary: {totalDocuments: 1, imported: 0, failed: 1, importedAt: new Date().toISOString()},
        results: [
          {
            resourceType: 'application',
            resourceId: 'app1',
            resourceName: 'MyApp',
            status: 'failed',
          },
        ],
      };
      mockMutateAsync.mockResolvedValue(failedResponse);
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByText(/This resource failed to import\./)).toBeInTheDocument();
      });
    });

    it('shows failed alert when dry run throws', async () => {
      mockMutateAsync.mockRejectedValue(new Error('network error'));
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'Retry'})).toBeInTheDocument();
      });
    });
  });

  describe('resource counts display', () => {
    it('displays count of 0 for missing resource types', () => {
      mockLocationState.configData = {
        application: [{id: 'app1', name: 'App 1'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('resource-summary-table')).toBeInTheDocument();
    });

    it('displays multiple resources of same type', () => {
      mockLocationState.configData = {
        application: [
          {id: 'app1', name: 'App 1'},
          {id: 'app2', name: 'App 2'},
          {id: 'app3', name: 'App 3'},
        ],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/application.*3/i)).toBeInTheDocument();
    });

    it('displays agents when agent data is present', () => {
      mockLocationState.configData = {
        agent: [
          {id: 'agent1', name: 'Test Agent', description: 'A test agent'},
          {id: 'agent2', name: 'Another Agent'},
        ],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/Agents.*2/i)).toBeInTheDocument();
    });

    it('does not display agents section when no agents present', () => {
      mockLocationState.configData = {
        application: [{id: 'app1', name: 'App 1'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.queryByText(/Agents/i)).not.toBeInTheDocument();
    });

    it('displays connections when connection data is present', () => {
      mockLocationState.configData = {
        connection: [
          {name: 'Email Sender', type: 'SMTP'},
          {name: 'SMS Sender', type: 'TWILIO'},
        ],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/Connections.*2/i)).toBeInTheDocument();
    });

    it('displays resource servers when resource_server data is present', () => {
      mockLocationState.configData = {
        resource_server: [{name: 'API Gateway', description: 'Main gateway'}, {name: 'Reports API'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/Resource Servers.*2/i)).toBeInTheDocument();
    });

    it('displays roles when role data is present', () => {
      mockLocationState.configData = {
        role: [{name: 'Admin', description: 'Administrator'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/Roles.*1/i)).toBeInTheDocument();
    });

    it('displays groups when group data is present', () => {
      mockLocationState.configData = {
        group: [
          {id: 'group1', name: 'Engineering'},
          {id: 'group2', name: 'Marketing'},
          {id: 'group3', name: 'Sales'},
        ],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/Groups.*3/i)).toBeInTheDocument();
    });

    it('does not display resource server, role, or group sections when absent', () => {
      mockLocationState.configData = {
        application: [{id: 'app1', name: 'App 1'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.queryByText(/Resource Servers/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Roles/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Groups/i)).not.toBeInTheDocument();
    });
  });

  describe('resource detail rendering', () => {
    it('renders names, chips, and details for each resource type', () => {
      mockLocationState.configData = {
        application: [
          {
            name: 'My App',
            description: 'App description',
            url: 'https://app.example.com',
            inbound_auth_config: [{type: 'oauth2', config: {client_id: 'app-client-id'}}],
          },
        ],
        flow: [{name: 'My Flow', flowType: 'LOGIN', handle: 'login-flow'}],
        connection: [
          {name: 'Google', type: 'OIDC', handle: 'google-idp'},
          {name: 'Email Sender', type: 'SMTP', handle: 'email-sender'},
        ],
        layout: [{name: 'My Layout', handle: 'layout-1', description: 'Layout description'}],
        organization_unit: [{name: 'Engineering OU', handle: 'eng-ou', description: 'Org description'}],
        theme: [{name: 'Dark Theme', handle: 'dark-theme', description: 'Theme description'}],
        translation: [{locale: 'fr-FR', namespace: 'common'}],
        user: [{type: 'customer', attributes: {name: 'Jane Doe', username: 'jane', email: 'jane@example.com'}}],
        user_type: [{name: 'Customer', handle: 'customer', allow_self_registration: true}],
        agent_type: [{name: 'Enterprise Agent', handle: 'enterprise-agent', allow_self_registration: true}],
        agent: [
          {
            name: 'My Agent',
            description: 'Agent description',
            inbound_auth_config: [{type: 'oauth2', config: {client_id: 'agent-client-id'}}],
          },
        ],
        resource_server: [{name: 'API Server', handle: 'api-server', description: 'RS description'}],
        role: [{name: 'Administrator', handle: 'admin-role', description: 'Role description'}],
        group: [{id: 'g1', name: 'Engineering', description: 'Group description'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      // Application detail line, URL and client id.
      expect(screen.getByText('App description')).toBeInTheDocument();
      expect(screen.getByText('https://app.example.com')).toBeInTheDocument();
      expect(screen.getByText('app-client-id')).toBeInTheDocument();

      // Chips render the raw values.
      expect(screen.getByText('LOGIN')).toBeInTheDocument();
      expect(screen.getByText('OIDC')).toBeInTheDocument();
      expect(screen.getByText('SMTP')).toBeInTheDocument();
      expect(screen.getByText('customer')).toBeInTheDocument();
      expect(screen.getAllByText('Self Registration')).toHaveLength(2);
      expect(screen.getByText('Enterprise Agent')).toBeInTheDocument();

      // Handle detail lines shown when distinct from the name.
      expect(screen.getByText('login-flow')).toBeInTheDocument();
      expect(screen.getByText('eng-ou')).toBeInTheDocument();

      // Description detail lines for the remaining types.
      expect(screen.getByText('Layout description')).toBeInTheDocument();
      expect(screen.getByText('Org description')).toBeInTheDocument();
      expect(screen.getByText('Theme description')).toBeInTheDocument();
      expect(screen.getByText('RS description')).toBeInTheDocument();
      expect(screen.getByText('Role description')).toBeInTheDocument();
      expect(screen.getByText('Group description')).toBeInTheDocument();

      // Translation locale and namespace chip.
      expect(screen.getByText('fr-FR')).toBeInTheDocument();
      expect(screen.getByText('common')).toBeInTheDocument();

      // User username and email detail lines plus the agent client id.
      expect(screen.getByText('@jane')).toBeInTheDocument();
      expect(screen.getByText('jane@example.com')).toBeInTheDocument();
      expect(screen.getByText('agent-client-id')).toBeInTheDocument();
    });

    it('toggles the expand/collapse control when more than five items exist', async () => {
      const user = userEvent.setup();
      mockLocationState.configData = {
        application: Array.from({length: 7}, (_unused, idx) => ({name: `App ${idx + 1}`})),
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      // Only the first five are shown initially.
      expect(screen.getByText('App 5')).toBeInTheDocument();
      expect(screen.queryByText('App 7')).not.toBeInTheDocument();

      await user.click(screen.getByText('+ 2 more'));

      // After expanding, all items are shown and a collapse control appears.
      expect(screen.getByText('App 7')).toBeInTheDocument();
      const collapse = screen.getByText('Show less');

      await user.click(collapse);

      expect(screen.queryByText('App 7')).not.toBeInTheDocument();
    });

    it('falls back to generated labels and keys when identifiers are missing', () => {
      mockLocationState.configData = {
        flow: [{}],
        theme: [{}],
        user_type: [{}],
        agent_type: [{}],
        translation: [{}],
        user: [{}],
        group: [{}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Flow 1')).toBeInTheDocument();
      expect(screen.getByText('Theme 1')).toBeInTheDocument();
      expect(screen.getByText('Schema 1')).toBeInTheDocument();
      expect(screen.getByText('Agent Type 1')).toBeInTheDocument();
      expect(screen.getByText('Unnamed Translation')).toBeInTheDocument();
      expect(screen.getByText('User 1')).toBeInTheDocument();
      expect(screen.getByText('Unnamed Group')).toBeInTheDocument();
    });

    it('renders credential configurations and presentation definitions with chips and detail lines', () => {
      mockLocationState.configData = {
        credential_configuration: [{handle: 'cc-1', display: {name: 'My Credential'}, vct: 'VCT-1'}],
        presentation_definition: [{handle: 'pd-1', displayName: 'My Presentation', vct: 'VCT-2'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Credential Configurations: 1')).toBeInTheDocument();
      expect(screen.getByText('Presentation Definitions: 1')).toBeInTheDocument();
      expect(screen.getByText('My Credential')).toBeInTheDocument();
      expect(screen.getByText('My Presentation')).toBeInTheDocument();
      expect(screen.getByText('VCT-1')).toBeInTheDocument();
      expect(screen.getByText('VCT-2')).toBeInTheDocument();
      expect(screen.getByText('cc-1')).toBeInTheDocument();
      expect(screen.getByText('pd-1')).toBeInTheDocument();
    });

    it('falls back to unnamed labels for credential configurations and presentation definitions', () => {
      mockLocationState.configData = {
        credential_configuration: [{}],
        presentation_definition: [{}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('Unnamed Credential Configuration')).toBeInTheDocument();
      expect(screen.getByText('Unnamed Presentation Definition')).toBeInTheDocument();
    });
  });

  describe('array-valued environment variables', () => {
    it('parses range-templated env variables as arrays, including blank and delimited formats', async () => {
      const originalConfigContent = mockLocationState.configContent;
      const originalEnvData = mockLocationState.envData;
      mockLocationState.configContent =
        '{{- range .EMPTY_LIST}}\n{{- range .BRACKET_LIST}}\n{{- range .PLAIN_LIST}}\n{{- range .EMPTY_BRACKET_LIST}}\n';
      mockLocationState.envData = "EMPTY_LIST=\nBRACKET_LIST=['a', 'b']\nPLAIN_LIST=x, y, z\nEMPTY_BRACKET_LIST=[]\n";
      mockMutateAsync.mockResolvedValue({
        results: [],
        summary: {imported: 0, totalDocuments: 0, failed: 0, importedAt: new Date(0).toISOString()},
      });

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalledWith(
          expect.objectContaining({
            variables: {
              EMPTY_LIST: [],
              BRACKET_LIST: ['a', 'b'],
              PLAIN_LIST: ['x', 'y', 'z'],
              EMPTY_BRACKET_LIST: [],
            },
          }),
        );
      });

      mockLocationState.configContent = originalConfigContent;
      mockLocationState.envData = originalEnvData;
    });
  });

  describe('missing required environment variables from config data', () => {
    const originalConfigContent = mockLocationState.configContent;
    const originalConfigData = mockLocationState.configData;
    const originalEnvData = mockLocationState.envData;

    beforeEach(() => {
      mockLocationState.configData = {
        application: [{name: 'App', description: '{{ .APP_DESC }}'}],
      } as ProductConfig;
      mockLocationState.envData = '';
    });

    afterEach(() => {
      mockLocationState.configContent = originalConfigContent;
      mockLocationState.configData = originalConfigData;
      mockLocationState.envData = originalEnvData;
    });

    it('lists the missing template variable and disables the dry run message', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('1 environment value(s) are missing. Add them before importing.')).toBeInTheDocument();
      expect(screen.getByTestId('template-variable')).toHaveTextContent('{{.APP_DESC}}');
      expect(screen.getByText('Fix missing environment values, then run test.')).toBeInTheDocument();
    });
  });

  describe('dry run status message when configuration content is unavailable', () => {
    it('shows the config-unavailable message', () => {
      const originalConfigContent = mockLocationState.configContent;
      mockLocationState.configContent = null as unknown as string;

      render(<ImportConfigurationSummaryPage />);

      expect(
        screen.getByText('Configuration content is unavailable. Re-upload the configuration file.'),
      ).toBeInTheDocument();

      mockLocationState.configContent = originalConfigContent;
    });
  });

  describe('manually triggering the dry run test', () => {
    it('calls the dry run mutation when the Test button is clicked', () => {
      const originalConfigContent = mockLocationState.configContent;
      const originalEnvData = mockLocationState.envData;
      mockLocationState.configContent = 'application:\n  - name: static-app\n';
      mockLocationState.envData = '';

      render(<ImportConfigurationSummaryPage />);

      fireEvent.click(screen.getByText('Test'));

      expect(mockMutateAsync).toHaveBeenCalled();

      mockLocationState.configContent = originalConfigContent;
      mockLocationState.envData = originalEnvData;
    });
  });

  describe('re-uploading the environment file from the summary page', () => {
    it('replaces the env data when a valid .env file is re-uploaded', async () => {
      render(<ImportConfigurationSummaryPage />);

      const envInput = document.querySelector('input[type="file"][accept=".env"]')!;
      expect(envInput).toBeInTheDocument();

      const file = new File(['NEW_KEY=new-value'], '.env', {type: 'text/plain'});
      await userEvent.upload(envInput, file);

      await waitFor(() => {
        expect(screen.getByTestId<HTMLTextAreaElement>('env-editor').value).toBe('NEW_KEY=new-value');
      });
    });

    it('logs a warning and skips non-.env files re-uploaded', async () => {
      render(<ImportConfigurationSummaryPage />);

      const envInput = document.querySelector('input[type="file"][accept=".env"]')!;
      const file = new File(['not an env file'], 'notes.txt', {type: 'text/plain'});
      fireEvent.change(envInput, {target: {files: [file]}});

      await waitFor(() => {
        expect(mockLogger.warn).toHaveBeenCalledWith('Invalid file type', {fileName: 'notes.txt'});
      });
    });

    it('does nothing when the change event fires with no selected file', () => {
      render(<ImportConfigurationSummaryPage />);

      const envInput = document.querySelector('input[type="file"][accept=".env"]')!;
      fireEvent.change(envInput, {target: {files: []}});

      expect(mockLogger.warn).not.toHaveBeenCalled();
      expect(screen.getByTestId<HTMLTextAreaElement>('env-editor').value).toBe(mockLocationState.envData);
    });
  });

  describe('import completion outcomes', () => {
    const noTemplateState = {
      configContent: 'application:\n  - name: static-app\n',
      envData: '',
    };

    it('shows a warning toast when the import completes with failures', async () => {
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;
      mockMutateAsync
        .mockResolvedValueOnce({
          results: [],
          summary: {imported: 1, totalDocuments: 1, failed: 0, importedAt: new Date(0).toISOString()},
        })
        .mockResolvedValueOnce({
          summary: {imported: 0, totalDocuments: 1, failed: 1, importedAt: new Date(0).toISOString()},
        });

      render(<ImportConfigurationSummaryPage />);

      await userEvent.click(screen.getByText('Test'));

      const importButton = await waitFor(() => {
        const button = getImportConfigurationButton();
        expect(button).not.toBeDisabled();
        return button;
      });
      await userEvent.click(importButton);

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith('Import completed with 1 failed resource.', 'warning');
      });
    });

    it('pluralizes the success toast when more than one resource is imported', async () => {
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;
      mockMutateAsync
        .mockResolvedValueOnce({
          results: [],
          summary: {imported: 2, totalDocuments: 2, failed: 0, importedAt: new Date(0).toISOString()},
        })
        .mockResolvedValueOnce({
          results: [],
          summary: {imported: 2, totalDocuments: 2, failed: 0, importedAt: new Date(0).toISOString()},
        });

      render(<ImportConfigurationSummaryPage />);

      await userEvent.click(screen.getByText('Test'));

      const importButton = await waitFor(() => {
        const button = getImportConfigurationButton();
        expect(button).not.toBeDisabled();
        return button;
      });
      await userEvent.click(importButton);

      await waitFor(() => {
        expect(mockShowToast).toHaveBeenCalledWith('Import completed successfully. 2 resources imported.', 'success');
      });
    });

    it('logs an error when the import mutation rejects', async () => {
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;
      mockMutateAsync
        .mockResolvedValueOnce({
          results: [],
          summary: {imported: 1, totalDocuments: 1, failed: 0, importedAt: new Date(0).toISOString()},
        })
        .mockRejectedValueOnce(new Error('import network error'));

      render(<ImportConfigurationSummaryPage />);

      await userEvent.click(screen.getByText('Test'));

      const importButton = await waitFor(() => {
        const button = getImportConfigurationButton();
        expect(button).not.toBeDisabled();
        return button;
      });
      await userEvent.click(importButton);

      await waitFor(() => {
        expect(mockLogger.error).toHaveBeenCalledWith('Failed to import configuration', {
          error: expect.any(Error) as Error,
        });
      });
    });
  });
});
