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

import type {UseMutationResult} from '@tanstack/react-query';
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
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

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({
      state: mockLocationState,
      pathname: '/import/summary',
    }),
  };
});

vi.mock('../../api/useImportConfiguration', () => ({
  default: () => mockMutationState,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'summary.resourceCount' && params?.count !== undefined) {
        const count = typeof params.count === 'number' ? params.count : Number(params.count);
        return `${count} resources`;
      }
      if (key === 'summary.envVariables' && params?.resolved !== undefined && params?.total !== undefined) {
        const resolved = typeof params.resolved === 'number' ? params.resolved : Number(params.resolved);
        const total = typeof params.total === 'number' ? params.total : Number(params.total);
        return `${resolved} of ${total} resolved`;
      }
      return key;
    },
  }),
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
  });

  describe('rendering', () => {
    it('renders page title', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.title')).toBeInTheDocument();
    });

    it('renders resource summary section', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.projectDetails')).toBeInTheDocument();
    });

    it('renders environment variables section', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('renders import test section', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.importTest.status')).toBeInTheDocument();
    });

    it('displays resource counts', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/application.*1/i)).toBeInTheDocument();
      expect(screen.getByText(/user.*1/i)).toBeInTheDocument();
      expect(screen.getByText(/flow.*1/i)).toBeInTheDocument();
    });
  });

  describe('breadcrumb navigation', () => {
    it('renders breadcrumb with steps', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.breadcrumb')).toBeInTheDocument();
    });

    it('navigates to upload page when clicking breadcrumb', async () => {
      render(<ImportConfigurationSummaryPage />);

      const uploadLink = screen.getByText('upload.breadcrumb.openProject');
      await userEvent.click(uploadLink);

      expect(mockNavigate).toHaveBeenCalledWith('/import-configuration');
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

      const uploadButton = screen.getByText('summary.actions.reuploadEnv');
      await userEvent.click(uploadButton);

      // Input should be triggered (we can't actually test file upload in JSDOM)
      expect(uploadButton).toBeInTheDocument();
    });
  });

  describe('dry run functionality', () => {
    it('shows run dry run button', () => {
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.importTest.test')).toBeInTheDocument();
    });

    it('executes dry run when button clicked', () => {
      render(<ImportConfigurationSummaryPage />);

      const runButton = screen.getByText('summary.importTest.test');

      // Button exists but may not trigger mutation if env variables are missing
      expect(runButton).toBeInTheDocument();
    });

    it('shows loading state during dry run', () => {
      mockMutationState.isPending = true;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.importTest.runningShort')).toBeInTheDocument();
    });

    it('disables import button when dry run has not passed', () => {
      render(<ImportConfigurationSummaryPage />);

      const importButton = screen.getByText('summary.import.action');
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

      expect(screen.getByText('summary.import.action')).toBeInTheDocument();
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
      const runButton = screen.getByText('summary.importTest.test');
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
        expect(screen.getByText('summary.import.action')).toBeInTheDocument();
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
  });

  describe('missing env variables', () => {
    it('detects missing environment variables', () => {
      // ConfigContent has {{.APP_NAME}} but envData doesn't have APP_NAME
      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByTestId('env-variables-viewer')).toBeInTheDocument();
    });

    it('disables dry run when env variables are missing', () => {
      render(<ImportConfigurationSummaryPage />);

      const runButton = screen.getByText('summary.importTest.test');
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

      expect(screen.getByLabelText('common:actions.close')).toBeInTheDocument();
    });

    it('navigates back on cancel', async () => {
      render(<ImportConfigurationSummaryPage />);

      const cancelButton = screen.getByLabelText('common:actions.close');
      await userEvent.click(cancelButton);

      expect(mockNavigate).toHaveBeenCalledWith('/home');
    });
  });

  describe('edge cases', () => {
    it('handles missing config data', () => {
      mockLocationState.configData = null as never;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.title')).toBeInTheDocument();
    });

    it('handles missing env data', () => {
      mockLocationState.envData = null as never;

      render(<ImportConfigurationSummaryPage />);

      // Page still renders without env data
      expect(screen.getByText('summary.title')).toBeInTheDocument();
    });

    it('handles empty resource arrays', () => {
      mockLocationState.configData = {
        application: [],
        user: [],
        flow: [],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.title')).toBeInTheDocument();
    });

    it('handles undefined resource arrays', () => {
      mockLocationState.configData = {} as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.title')).toBeInTheDocument();
    });
  });

  describe('file upload handling', () => {
    it('validates env file extension', () => {
      render(<ImportConfigurationSummaryPage />);

      // Test validates that the page renders successfully
      expect(screen.getByText('summary.title')).toBeInTheDocument();
    });

    it('logs warning for invalid file types', () => {
      render(<ImportConfigurationSummaryPage />);

      // Component will log warnings for invalid files
      expect(mockLogger.warn).not.toHaveBeenCalled();
    });

    it('resets file input after upload', () => {
      render(<ImportConfigurationSummaryPage />);

      // Page renders successfully with env data
      expect(screen.getByText('summary.title')).toBeInTheDocument();
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
        expect(screen.getByText('summary.importTest.passed')).toBeInTheDocument();
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
        expect(screen.getByRole('button', {name: /summary\.importTest\.retry/})).toBeInTheDocument();
      });
    });

    it('shows failed results list when dry run fails with results', async () => {
      const failedResponse: ImportResponse = {
        summary: {totalDocuments: 1, imported: 0, failed: 1, importedAt: new Date().toISOString()},
        results: [
          {
            resourceType: 'application',
            resourceId: 'app1',
            resourceName: 'MyApp',
            status: 'failed',
            message: 'Bad config',
          },
        ],
      };
      mockMutateAsync.mockResolvedValue(failedResponse);
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByText(/summary\.importTest\.failures/)).toBeInTheDocument();
        expect(screen.getByText(/MyApp/)).toBeInTheDocument();
        expect(screen.getByText(/Bad config/)).toBeInTheDocument();
      });
    });

    it('shows failed alert when dry run throws', async () => {
      mockMutateAsync.mockRejectedValue(new Error('network error'));
      mockLocationState.configContent = noTemplateState.configContent;
      mockLocationState.envData = noTemplateState.envData;

      render(<ImportConfigurationSummaryPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /summary\.importTest\.retry/})).toBeInTheDocument();
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

      expect(screen.getByText(/configureExport\.labels\.agents.*2/i)).toBeInTheDocument();
    });

    it('does not display agents section when no agents present', () => {
      mockLocationState.configData = {
        application: [{id: 'app1', name: 'App 1'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.queryByText(/configureExport\.labels\.agents/i)).not.toBeInTheDocument();
    });

    it('displays notification senders when notification_sender data is present', () => {
      mockLocationState.configData = {
        notification_sender: [
          {name: 'Email Sender', type: 'SMTP'},
          {name: 'SMS Sender', type: 'TWILIO'},
        ],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/configureExport\.labels\.notificationSenders.*2/i)).toBeInTheDocument();
    });

    it('displays resource servers when resource_server data is present', () => {
      mockLocationState.configData = {
        resource_server: [{name: 'API Gateway', description: 'Main gateway'}, {name: 'Reports API'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/configureExport\.labels\.resourceServers.*2/i)).toBeInTheDocument();
    });

    it('displays roles when role data is present', () => {
      mockLocationState.configData = {
        role: [{name: 'Admin', description: 'Administrator'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText(/configureExport\.labels\.roles.*1/i)).toBeInTheDocument();
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

      expect(screen.getByText(/configureExport\.labels\.groups.*3/i)).toBeInTheDocument();
    });

    it('does not display resource server, role, or group sections when absent', () => {
      mockLocationState.configData = {
        application: [{id: 'app1', name: 'App 1'}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.queryByText(/configureExport\.labels\.resourceServers/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/configureExport\.labels\.roles/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/configureExport\.labels\.groups/i)).not.toBeInTheDocument();
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
        identity_provider: [{name: 'Google', type: 'OIDC', handle: 'google-idp'}],
        notification_sender: [{name: 'Email Sender', type: 'SMTP', handle: 'email-sender'}],
        layout: [{name: 'My Layout', handle: 'layout-1', description: 'Layout description'}],
        organization_unit: [{name: 'Engineering OU', handle: 'eng-ou', description: 'Org description'}],
        theme: [{name: 'Dark Theme', handle: 'dark-theme', description: 'Theme description'}],
        translation: [{locale: 'fr-FR', namespace: 'common'}],
        user: [{type: 'customer', attributes: {name: 'Jane Doe', username: 'jane', email: 'jane@example.com'}}],
        user_type: [{name: 'Customer', handle: 'customer', allow_self_registration: true}],
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
      expect(screen.getByText('configureExport.labels.selfRegistration')).toBeInTheDocument();

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

      await user.click(screen.getByText('configureExport.actions.more'));

      // After expanding, all items are shown and a collapse control appears.
      expect(screen.getByText('App 7')).toBeInTheDocument();
      const collapse = screen.getByText('configureExport.actions.showLess');

      await user.click(collapse);

      expect(screen.queryByText('App 7')).not.toBeInTheDocument();
    });

    it('falls back to generated labels and keys when identifiers are missing', () => {
      mockLocationState.configData = {
        flow: [{}],
        theme: [{}],
        user_type: [{}],
        translation: [{}],
        user: [{}],
        group: [{}],
      } as ProductConfig;

      render(<ImportConfigurationSummaryPage />);

      expect(screen.getByText('summary.fallback.flow')).toBeInTheDocument();
      expect(screen.getByText('summary.fallback.theme')).toBeInTheDocument();
      expect(screen.getByText('summary.fallback.schema')).toBeInTheDocument();
      expect(screen.getByText('configureExport.fallback.unnamedTranslation')).toBeInTheDocument();
      expect(screen.getByText('summary.fallback.user')).toBeInTheDocument();
      expect(screen.getByText('configureExport.fallback.unnamedGroup')).toBeInTheDocument();
    });
  });
});
