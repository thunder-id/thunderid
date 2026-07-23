/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import userEvent from '@testing-library/user-event';
import type {Theme} from '@thunderid/design';
import {render, screen, waitFor, within} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ApplicationCreateProvider from '../../contexts/ApplicationCreate/ApplicationCreateProvider';
import useApplicationCreateContext from '../../hooks/useApplicationCreateContext';
import type {Application} from '../../models/application';
import ApplicationCreatePage from '../ApplicationCreatePage';

// Mock functions
const mockCreateApplication = vi.fn();
const mockNavigate = vi.fn();
let mockPathname = '/';

// Mock logger
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    withComponent: vi.fn().mockReturnThis(),
  }),
}));

// Mock react-router
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({pathname: mockPathname}),
  };
});

// Mock design hooks
vi.mock('@thunderid/design', () => ({
  useGetThemes: () => ({
    data: {themes: [{id: 'theme-1', displayName: 'Default Theme', theme: {}}]},
    isLoading: false,
  }),
  useGetTheme: () => ({
    data: null,
    isLoading: false,
  }),
}));

// Mock application API
vi.mock('../../api/useCreateApplication', () => ({
  default: () => ({
    mutate: mockCreateApplication,
    isPending: false,
  }),
}));

// Mock user types API
vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypes: () => ({
    data: {
      types: [
        {name: 'customer', displayName: 'Customer'},
        {name: 'employee', displayName: 'Employee'},
      ],
    },
    isLoading: false,
    error: null,
  }),
}));

// Mock integrations API
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useIdentityProviders: () => ({
    data: [
      {id: 'google', name: 'Google', type: 'social'},
      {id: 'github', name: 'GitHub', type: 'social'},
    ],
    isLoading: false,
    error: null,
  }),
}));

// Mock flows API
const {mockCreateFlow, mockGenerateFlowGraph} = vi.hoisted(() => ({
  mockCreateFlow: vi.fn(),
  mockGenerateFlowGraph: vi.fn(),
}));

vi.mock('../../../flows/api/useCreateFlow', () => ({
  default: () => ({
    mutate: mockCreateFlow,
    isPending: false,
  }),
}));

vi.mock('../../../flows/utils/generateFlowGraph', () => ({
  default: mockGenerateFlowGraph,
}));

vi.mock('../../../flows/api/useGetFlows', () => ({
  default: () => ({
    data: {
      flows: [
        {id: 'flow1', name: 'Basic Auth Flow', handle: 'basic-auth'},
        {id: 'flow2', name: 'Google Flow', handle: 'google-flow'},
      ],
    },
    isLoading: false,
    error: null,
  }),
}));

// Mock configuration type utility
vi.mock('../../utils/getConfigurationTypeFromTemplate', () => ({
  default: vi.fn(() => 'URL'),
}));

vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => ({
    hasMultipleOUs: false,
    isLoading: false,
    ouList: [],
  }),
}));

// Mock child components
vi.mock('../../components/create-application/ConfigureName', () => ({
  default: ({
    appName,
    onAppNameChange,
    onReadyChange,
  }: {
    appName: string;
    onAppNameChange: (name: string) => void;
    onReadyChange: (ready: boolean) => void;
  }) => (
    <div data-testid="application-configure-name">
      <input
        data-testid="app-name-input"
        value={appName}
        onChange={(e) => {
          onAppNameChange(e.target.value);
          onReadyChange(e.target.value.length > 0);
        }}
        placeholder="Enter app name"
      />
    </div>
  ),
}));

vi.mock('../../components/create-application/ConfigureDesign', () => ({
  default: ({
    appLogo,
    onLogoSelect,
    onThemeSelect,
  }: {
    appLogo: string | null;
    selectedTheme: Theme | null;
    onLogoSelect: (logo: string) => void;
    onInitialLogoLoad: (logo: string) => void;
    onReadyChange: (ready: boolean) => void;
    onThemeSelect?: (themeId: string, themeConfig: Theme) => void;
  }) => (
    <div data-testid="application-configure-design">
      {appLogo ? <span data-testid="preview-logo">{appLogo}</span> : null}
      <button type="button" data-testid="logo-select-btn" onClick={() => onLogoSelect('test-logo.png')}>
        Select Logo
      </button>
      <button type="button" data-testid="select-theme-btn" onClick={() => onThemeSelect?.('theme-1', {} as Theme)}>
        Select Theme
      </button>
    </div>
  ),
}));

vi.mock('../../components/create-application/configure-signin-options/ConfigureSignInOptions', async () => {
  const useApplicationCreateContextModule = await import('../../hooks/useApplicationCreateContext');

  return {
    default: vi.fn(
      ({
        integrations,
        onIntegrationToggle,
        onReadyChange,
      }: {
        integrations: Record<string, boolean>;
        onIntegrationToggle: (id: string) => void;
        onReadyChange: (ready: boolean) => void;
      }) => {
        const {setSelectedAuthFlow} = useApplicationCreateContextModule.default();

        setTimeout(() => {
          setSelectedAuthFlow({
            id: 'test-flow-id',
            name: 'Test Flow',
            flowType: 'AUTHENTICATION',
            handle: 'test-flow',
            activeVersion: 1,
            createdAt: '2024-01-01T00:00:00Z',
            updatedAt: '2024-01-01T00:00:00Z',
          });
          const hasSelection = Object.values(integrations).some((enabled: boolean) => enabled);
          onReadyChange(hasSelection);
        }, 0);

        return (
          <div data-testid="application-configure-sign-in">
            <button
              type="button"
              data-testid="toggle-integration"
              onClick={() => onIntegrationToggle('credentials_auth')}
            >
              Toggle Integration
            </button>
          </div>
        );
      },
    ),
  };
});

vi.mock('../../components/create-application/ConfigureExperience', () => ({
  default: ({
    onReadyChange,
    onApproachChange,
    selectedApproach,
    allowEmbeddedApproach,
  }: {
    onReadyChange: (ready: boolean) => void;
    onApproachChange: (approach: string) => void;
    selectedApproach: string;
    allowEmbeddedApproach: boolean;
    userTypes: {name: string}[];
    selectedUserTypes: string[];
    onUserTypesChange: (types: string[]) => void;
  }) => {
    setTimeout(() => onReadyChange(true), 0);
    return (
      <div data-testid="application-configure-experience">
        <span data-testid="current-approach">{selectedApproach}</span>
        <span data-testid="allow-embedded-approach">{String(allowEmbeddedApproach)}</span>
        <button type="button" data-testid="select-embedded-approach" onClick={() => onApproachChange('EMBEDDED')}>
          Select Embedded
        </button>
        <button type="button" data-testid="select-inbuilt-approach" onClick={() => onApproachChange('INBUILT')}>
          Select Inbuilt
        </button>
      </div>
    );
  },
}));

vi.mock('../../components/create-application/ConfigureDetails', () => ({
  default: ({
    onReadyChange,
    onCallbackUrlChange,
    onHostingUrlChange,
  }: {
    onReadyChange: (ready: boolean) => void;
    onCallbackUrlChange: (url: string) => void;
    technology?: string;
    platform?: string;
    onHostingUrlChange: (url: string) => void;
  }) => {
    setTimeout(() => onReadyChange(true), 0);
    return (
      <div data-testid="application-configure-details">
        <input
          data-testid="hosting-url-input"
          onChange={(e) => onHostingUrlChange(e.target.value)}
          placeholder="Hosting URL"
        />
        <input
          data-testid="callback-url-input"
          onChange={(e) => onCallbackUrlChange(e.target.value)}
          placeholder="Callback URL"
        />
      </div>
    );
  },
}));

vi.mock('../../../../components/GatePreview/GatePreview', () => ({
  default: () => <div data-testid="preview" />,
}));

vi.mock('../../components/create-application/ShowClientSecret', () => ({
  default: ({
    appName,
    clientSecret,
    onContinue,
  }: {
    appName: string;
    clientSecret: string;
    onCopySecret: () => void;
    onContinue: () => void;
  }) => (
    <div data-testid="application-show-client-secret">
      <div data-testid="client-secret-app-name">{appName}</div>
      <div data-testid="application-client-secret-value">{clientSecret}</div>
      <button type="button" data-testid="application-client-secret-continue" onClick={onContinue}>
        Continue
      </button>
    </div>
  ),
}));

vi.mock('../../components/create-application/mcp/McpConnectComplete', () => ({
  default: ({
    appName,
    clientId,
    clientSecret,
    redirectUris,
    clientType,
    onContinue,
  }: {
    appName?: string;
    clientId?: string;
    clientSecret?: string;
    redirectUris: string[];
    clientType: string;
    onContinue: () => void;
  }) => (
    <div data-testid="application-mcp-connect-complete">
      <div data-testid="mcp-connect-complete-app-name">{appName}</div>
      <div data-testid="mcp-connect-complete-client-id">{clientId}</div>
      <div data-testid="mcp-connect-complete-client-secret">{clientSecret}</div>
      <div data-testid="mcp-connect-complete-redirect-uris">{redirectUris.join(',')}</div>
      <div data-testid="mcp-connect-complete-client-type">{clientType}</div>
      <button type="button" data-testid="mcp-connect-complete-continue" onClick={onContinue}>
        Continue
      </button>
    </div>
  ),
}));

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    AppBreadcrumbs: ({items}: {items: {key: string; label: string; onClick?: () => void}[]}) => (
      <nav>
        {items.map((item) => (
          <span
            key={item.key}
            onClick={item.onClick}
            onKeyDown={(e: React.KeyboardEvent) => (e.key === 'Enter' || e.key === ' ') && item.onClick?.()}
            role={item.onClick ? 'button' : undefined}
            tabIndex={item.onClick ? 0 : undefined}
          >
            {item.label}
          </span>
        ))}
      </nav>
    ),
  };
});

// Template selection now happens on a separate page before the wizard mounts. This helper stands in
// for that page: its buttons seed the same context state (template config + first wizard step) the
// selection page would set, so the wizard behaves as if a template was chosen.
function TemplateSeeder(): JSX.Element {
  const {setSelectedTechnology, setSelectedPlatform, setSelectedTemplateConfig, setCurrentStep} =
    useApplicationCreateContext();

  const seed = (technology: unknown, platform: unknown, template: unknown): void => {
    setSelectedTechnology(technology as never);
    setSelectedPlatform(platform as never);
    setSelectedTemplateConfig(template as never);
    setCurrentStep('NAME');
  };

  return (
    <div>
      <button
        type="button"
        aria-label="seed server template"
        data-testid="select-backend-platform"
        onClick={() =>
          seed(null, 'BACKEND', {id: 'backend', creationFlow: {steps: ['NAME', 'ORGANIZATION_UNIT', 'COMPLETE']}})
        }
      >
        Select Backend
      </button>
      <button
        type="button"
        aria-label="seed wallet template"
        data-testid="select-wallet-platform"
        onClick={() =>
          seed(null, 'WALLET', {
            id: 'wallet',
            creationFlow: {
              steps: ['NAME', 'ORGANIZATION_UNIT', 'CONFIGURE', 'DESIGN', 'OPTIONS', 'EXPERIENCE', 'COMPLETE'],
            },
            defaults: {
              inboundAuthConfig: [
                {
                  type: 'oauth2',
                  config: {grantTypes: ['authorization_code'], responseTypes: ['code'], publicClient: true},
                },
              ],
            },
          })
        }
      >
        Select Wallet
      </button>
      <button
        type="button"
        aria-label="seed spa template"
        data-testid="select-browser-platform"
        onClick={() =>
          seed(null, 'BROWSER', {
            id: 'browser',
            defaults: {
              inboundAuthConfig: [
                {
                  type: 'oauth2',
                  config: {grantTypes: ['authorization_code'], responseTypes: ['code'], publicClient: true},
                },
              ],
            },
          })
        }
      >
        Select Browser
      </button>
      <button
        type="button"
        aria-label="seed mcp template"
        data-testid="select-mcp-client-template"
        onClick={() =>
          seed(null, null, {
            id: 'mcp-client',
            creationFlow: {steps: ['NAME', 'ORGANIZATION_UNIT', 'CLIENT_TYPE', 'COMPLETE']},
            defaults: {
              inboundAuthConfig: [
                {
                  type: 'oauth2',
                  config: {
                    grantTypes: ['authorization_code', 'refresh_token'],
                    responseTypes: ['code'],
                    redirectUris: [],
                    pkceRequired: true,
                    tokenEndpointAuthMethod: 'none',
                    publicClient: true,
                  },
                },
              ],
            },
          })
        }
      >
        Select MCP Client
      </button>
    </div>
  );
}

describe('ApplicationCreatePage', () => {
  let user: ReturnType<typeof userEvent.setup>;

  const renderWithProviders = () =>
    render(
      <ApplicationCreateProvider>
        <ApplicationCreatePage />
        <TemplateSeeder />
      </ApplicationCreateProvider>,
    );

  beforeEach(async () => {
    user = userEvent.setup();

    window.history.replaceState({}, '', '/');
    mockPathname = '/';

    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);

    const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
    vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('URL');
  });

  describe('Initial Rendering', () => {
    it('should render the name step by default', () => {
      renderWithProviders();

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
    });

    it('should not show preview on first step', () => {
      renderWithProviders();

      expect(screen.queryByTestId('preview')).not.toBeInTheDocument();
    });

    it('should render close button', () => {
      const {container} = renderWithProviders();

      const buttons = container.querySelectorAll('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should show breadcrumb with current step', () => {
      renderWithProviders();

      expect(screen.getByText('Create an Application')).toBeInTheDocument();
    });
  });

  describe('Step Navigation', () => {
    it('should show Continue on non-last steps and Finish on the last step', async () => {
      renderWithProviders();

      // The default flow spans several steps, so NAME is not the last — button reads Continue.
      expect(screen.getByTestId('application-wizard-next-button')).toHaveTextContent(/continue/i);

      // The backend flow collapses to a single visible step (NAME), so the button reads Finish.
      await user.click(screen.getByTestId('select-backend-platform'));
      expect(screen.getByTestId('application-wizard-next-button')).toHaveTextContent(/finish/i);
    });

    it('should disable Continue button when name is empty', () => {
      renderWithProviders();

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      const continueButton = screen.getByRole('button', {name: /continue/i});
      expect(continueButton).toBeDisabled();
    });

    it('should enable Continue button when name is entered', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      const continueButton = screen.getByRole('button', {name: /continue/i});
      expect(continueButton).toBeEnabled();
    });

    it('should navigate to design step from name step', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-name')).not.toBeInTheDocument();
    });

    it('should show preview from design step onwards', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('preview')).toBeInTheDocument();
    });

    it('should navigate through all steps', async () => {
      renderWithProviders();

      // Step 1: Name
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 2: Design
      expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 3: Sign In Options
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 4: Experience
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 5: Configure Details
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
    });

    it('should show Back button from the design step onwards', async () => {
      renderWithProviders();

      // NAME is the first step, so there is no Back button yet.
      expect(screen.queryByRole('button', {name: /back/i})).not.toBeInTheDocument();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByRole('button', {name: /back/i})).toBeInTheDocument();
    });

    it('should navigate back to previous step', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();

      // DESIGN → NAME (back)
      await user.click(screen.getByRole('button', {name: /back/i}));

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
    });
  });

  describe('Breadcrumb Navigation', () => {
    it('should update breadcrumb as user progresses', async () => {
      renderWithProviders();

      expect(screen.getByText('Create an Application')).toBeInTheDocument();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByText('Design')).toBeInTheDocument();

      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByText('Sign In Options')).toBeInTheDocument();
    });

    it('should allow clicking on previous breadcrumb steps', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const firstBreadcrumb = screen.getByText('Create an Application');
      await user.click(firstBreadcrumb);

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
    });
  });

  describe('Welcome flow breadcrumbs', () => {
    it('shows welcome prefix breadcrumbs when in welcome flow', () => {
      mockPathname = '/welcome/get-started';
      renderWithProviders();

      expect(screen.getByText('Welcome')).toBeInTheDocument();
      expect(screen.getByText('New')).toBeInTheDocument();
      expect(screen.getByText('Get started')).toBeInTheDocument();
    });

    it('navigates to /welcome when welcome breadcrumb is clicked', async () => {
      mockPathname = '/welcome/get-started';
      renderWithProviders();

      await user.click(screen.getByRole('button', {name: 'Welcome'}));

      expect(mockNavigate).toHaveBeenCalledWith('/welcome');
    });

    it('navigates to /welcome/create-project when create project breadcrumb is clicked', async () => {
      mockPathname = '/welcome/get-started';
      renderWithProviders();

      await user.click(screen.getByRole('button', {name: 'New'}));

      expect(mockNavigate).toHaveBeenCalledWith('/welcome/create-project');
    });

    it('navigates to /welcome/get-started when get-started breadcrumb is clicked', async () => {
      mockPathname = '/welcome/get-started';
      renderWithProviders();

      await user.click(screen.getByRole('button', {name: 'Get started'}));

      expect(mockNavigate).toHaveBeenCalledWith('/welcome/get-started');
    });
  });

  describe('Default breadcrumbs', () => {
    it('navigates to /applications when the default breadcrumb is clicked outside the welcome flow', async () => {
      mockPathname = '/';
      const {container} = renderWithProviders();

      const breadcrumbItem = container.querySelector('nav [role="button"]');
      expect(breadcrumbItem).toBeInTheDocument();
      await user.click(breadcrumbItem!);

      expect(mockNavigate).toHaveBeenCalledWith('/applications');
    });
  });

  describe('Close Functionality', () => {
    it('should navigate to applications list when close button is clicked', async () => {
      const {container} = renderWithProviders();

      const closeButton = container.querySelector('button');
      expect(closeButton).toBeInTheDocument();
      await user.click(closeButton!);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications');
      });
    });
  });

  describe('Form State Management', () => {
    it('should update app name state', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'Test App');

      expect(nameInput).toHaveValue('Test App');
    });

    it('should preserve app name when navigating between steps', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → NAME (back)
      await user.click(screen.getByRole('button', {name: /back/i}));

      expect(screen.getByTestId('app-name-input')).toHaveValue('My App');
    });

    it('should update logo in state', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const logoButton = screen.getByTestId('logo-select-btn');
      await user.click(logoButton);

      expect(screen.getByTestId('preview-logo')).toHaveTextContent('test-logo.png');
    });
  });

  describe('Application Creation - Inbuilt Approach', () => {
    it('should create application with OAuth config for inbuilt approach', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      // Navigate through all steps
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // Verify OAuth config was included
      const createAppCall = mockCreateApplication.mock.calls[0][0] as Application;
      expect(createAppCall.inboundAuthConfig).toBeDefined();
      expect(createAppCall.inboundAuthConfig?.[0]).toBeDefined();
      expect(createAppCall.inboundAuthConfig?.[0]?.type).toBe('oauth2');
    });

    it('should navigate to application details page after creation', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      // Navigate through all steps
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123');
      });
    });
  });

  describe('Application Creation - Embedded Approach', () => {
    it('should create application without OAuth config for embedded approach', async () => {
      const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
      vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('NONE');

      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Select embedded approach
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      const selectEmbeddedBtn = screen.getByTestId('select-embedded-approach');
      await user.click(selectEmbeddedBtn);
      // EXPERIENCE → Create (embedded skips configure)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // Verify OAuth config was NOT included
      const createAppCall = mockCreateApplication.mock.calls[0][0] as Application;
      expect(createAppCall.inboundAuthConfig).toBeUndefined();
    });

    it('should skip configure step for embedded approach', async () => {
      const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
      vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('NONE');

      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      await user.click(screen.getByTestId('select-embedded-approach'));
      // EXPERIENCE → Create (embedded skips configure)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should NOT show configure details step
      await waitFor(() => {
        expect(screen.queryByTestId('application-configure-details')).not.toBeInTheDocument();
        expect(mockCreateApplication).toHaveBeenCalled();
      });
    });
  });

  describe('Embedded Approach Availability', () => {
    const goToExperienceStep = async () => {
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → CONFIGURE (wallet platform only) or DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // The wallet platform's creation flow orders CONFIGURE right after NAME.
      if (screen.queryByTestId('application-configure-details')) {
        // CONFIGURE → DESIGN
        await user.click(screen.getByRole('button', {name: /continue/i}));
      }

      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
    };

    it('should offer the embedded approach for the wallet platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-wallet-platform'));
      await goToExperienceStep();

      expect(screen.getByTestId('allow-embedded-approach')).toHaveTextContent('true');
    });

    it('should show the CONFIGURE step right after NAME for the wallet platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-wallet-platform'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
      // No branding has been configured yet at this point in the wallet flow, so the preview is hidden.
      expect(screen.queryByTestId('preview')).not.toBeInTheDocument();
    });

    it('should not offer the embedded approach for browser-based SPAs', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await goToExperienceStep();

      expect(screen.getByTestId('allow-embedded-approach')).toHaveTextContent('false');
    });
  });

  describe('Error Handling', () => {
    it('should show error when application creation fails', async () => {
      mockCreateApplication.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(new Error('Failed to create application'));
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(
        () => {
          expect(screen.getByText(/failed to create application/i)).toBeInTheDocument();
        },
        {timeout: 10000},
      );
    });

    it('should allow dismissing error message', async () => {
      mockCreateApplication.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(new Error('Failed to create application'));
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(
        () => {
          expect(screen.getByText(/failed to create application/i)).toBeInTheDocument();
        },
        {timeout: 10000},
      );

      const closeButton = screen.getByLabelText(/close/i);
      await user.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByText(/failed to create application/i)).not.toBeInTheDocument();
      });
    });
  });

  describe('Theme Selection', () => {
    it('should allow selecting a theme', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Select a theme
      const selectThemeBtn = screen.getByTestId('select-theme-btn');
      await user.click(selectThemeBtn);

      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // Verify themeId was included in the application creation
      const createAppCall = mockCreateApplication.mock.calls[0][0] as Application;
      expect(createAppCall.themeId).toBe('theme-1');
    });
  });

  describe('Integration Toggle', () => {
    it('should allow toggling integrations', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });

      const toggleButton = screen.getByTestId('toggle-integration');
      await user.click(toggleButton);

      expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
    });
  });

  describe('Callback URL Configuration', () => {
    it('should update OAuth config when callback URL changes', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });

      const callbackInput = screen.getByTestId('callback-url-input');
      await user.type(callbackInput, 'https://example.com/callback');

      expect(callbackInput).toHaveValue('https://example.com/callback');
    });
  });

  describe('Client Secret Display (COMPLETE Step)', () => {
    it('should show COMPLETE step when application is created with clientSecret', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'app-123',
          name: 'My App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'test-client-id',
                clientSecret: 'test_secret_12345',
                redirectUris: ['https://example.com/callback'],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should show COMPLETE step with client secret
      await waitFor(() => {
        expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      });

      expect(screen.getByTestId('client-secret-app-name')).toHaveTextContent('My App');
      expect(screen.getByTestId('application-client-secret-value')).toHaveTextContent('test_secret_12345');
    });

    it('should not show COMPLETE step when application is created without clientSecret', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'app-123',
          name: 'My App',
          inboundAuthConfig: [],
        } as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should navigate directly to application details page
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123');
      });

      // Should not show COMPLETE step
      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
    });

    it('should navigate to application details when continue is clicked on COMPLETE step', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'app-456',
          name: 'My App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'test-client-id',
                clientSecret: 'test_secret_12345',
                redirectUris: ['https://example.com/callback'],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should show COMPLETE step
      await waitFor(() => {
        expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      });

      // Click continue on COMPLETE step
      const continueButton = screen.getByTestId('application-client-secret-continue');
      await user.click(continueButton);

      // Should navigate to application details page
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-456');
      });
    });

    it('should not show back button on COMPLETE step', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'app-123',
          name: 'My App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'test-client-id',
                clientSecret: 'test_secret_12345',
                redirectUris: ['https://example.com/callback'],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should show COMPLETE step
      await waitFor(() => {
        expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      });

      // Back button should not be present
      expect(screen.queryByRole('button', {name: /back/i})).not.toBeInTheDocument();
    });

    it('should not show preview panel on COMPLETE step', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'app-123',
          name: 'My App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'test-client-id',
                clientSecret: 'test_secret_12345',
                redirectUris: ['https://example.com/callback'],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Preview should be visible on DESIGN step
      await waitFor(() => {
        expect(screen.getByTestId('preview')).toBeInTheDocument();
      });

      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should show COMPLETE step
      await waitFor(() => {
        expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      });

      // Preview should not be visible on COMPLETE step
      expect(screen.queryByTestId('preview')).not.toBeInTheDocument();
    });
  });

  describe('Backend Platform (BACKEND / M2M) Flow', () => {
    it('should skip DESIGN, OPTIONS, EXPERIENCE and create app directly from NAME step', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'backend-app-1', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      // Select the BACKEND template
      await user.click(screen.getByTestId('select-backend-platform'));

      // Enter app name
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → [create immediately]
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // Should not have visited DESIGN, OPTIONS or EXPERIENCE
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-sign-in')).not.toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-experience')).not.toBeInTheDocument();
    });

    it('should create backend app without userAttributes, isRegistrationFlowEnabled, or themeId', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'backend-app-2', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const createAppCall = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
      expect(createAppCall.userAttributes).toBeUndefined();
      expect(createAppCall.isRegistrationFlowEnabled).toBeUndefined();
      expect(createAppCall.themeId).toBeUndefined();
      expect(createAppCall.logoUrl).toBeUndefined();
    });

    it('should include the backend template id in the create request', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'backend-app-3', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const createAppCall = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
      expect(createAppCall.template).toBe('backend');
    });

    it('should include inboundAuthConfig (OAuth) in the backend create request', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'backend-app-4', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const createAppCall = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
      expect(createAppCall.inboundAuthConfig).toBeDefined();
    });

    it('should show COMPLETE step with client secret after backend app creation', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'backend-app-5',
          name: 'My Backend App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'backend-client-id',
                clientSecret: 'backend_secret_xyz',
                redirectUris: [] as string[],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create → COMPLETE
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      });

      expect(screen.getByTestId('application-client-secret-value')).toHaveTextContent('backend_secret_xyz');
    });

    it('should show only NAME in breadcrumb for backend platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      // On the NAME step the breadcrumb shows only the current step for the backend flow.
      expect(screen.getByText('Create an Application')).toBeInTheDocument();
      // Design/Options/Experience should NOT appear in breadcrumb
      expect(screen.queryByText('Design')).not.toBeInTheDocument();
      expect(screen.queryByText('Sign In Options')).not.toBeInTheDocument();
    });

    it('should not show a flow-not-found error when creating a backend app without a selected auth flow', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'backend-app-6', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create (no auth flow selected)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      expect(screen.queryByText(/no.*flow/i)).not.toBeInTheDocument();
    });
  });

  describe('Hosting URL / Application URL', () => {
    it('should include url in create request when hosting URL is provided', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });

      await user.type(screen.getByTestId('hosting-url-input'), 'https://myapp.example.com');

      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const createAppCall = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
      expect(createAppCall.url).toBe('https://myapp.example.com');
    });

    it('should not include url in create request when hosting URL is not provided', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });

      // Do not type a hosting URL — proceed directly
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const createAppCall = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
      expect(createAppCall.url).toBeUndefined();
    });
  });

  describe('Flow Generation', () => {
    it('should generate flow and create application when integrations are selected but no flow matches', async () => {
      // Mock createFlow to return success
      mockCreateFlow.mockImplementation((_data, {onSuccess}: {onSuccess: (flow: unknown) => void}) => {
        onSuccess({
          id: 'generated-flow-id',
          name: 'Generated Flow',
          handle: 'generated-flow',
        });
      });

      // Mock createApplication to success
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-with-generated-flow', name: 'My App'} as Application);
      });

      // Override MockConfigureSignInOptions to simulate selection without setting a flow
      const ConfigureSignInOptionsModule = await import(
        '../../components/create-application/configure-signin-options/ConfigureSignInOptions'
      );
      const useApplicationCreateContextModule = await import('../../hooks/useApplicationCreateContext');

      vi.mocked(ConfigureSignInOptionsModule.default).mockImplementation(
        ({onReadyChange}: {onReadyChange?: (ready: boolean) => void}) => {
          const {setSelectedAuthFlow, setIntegrations} = useApplicationCreateContextModule.default();

          const handleSetup = () => {
            // Explicitly set flow to null to trigger generation logic
            setSelectedAuthFlow(null);
            // Explicitly set integrations
            setIntegrations({basic_auth: true});
            onReadyChange?.(true);
          };

          return (
            <div data-testid="application-configure-sign-in">
              <button type="button" data-testid="setup-flow-generation" onClick={handleSetup}>
                Setup Flow Generation
              </button>
            </div>
          );
        },
      );

      renderWithProviders();

      // Navigate to options step
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // At Options step
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });

      // Trigger setup
      await user.click(screen.getByTestId('setup-flow-generation'));

      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Verify generateFlowGraph called
      await waitFor(() => {
        expect(mockGenerateFlowGraph).toHaveBeenCalled();
        expect(mockCreateFlow).toHaveBeenCalled();
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // Verify createApplication called with generated flow ID
      const createAppCall = mockCreateApplication.mock.calls[0][0] as Application;
      expect(createAppCall.authFlowId).toBe('generated-flow-id');
    });

    it('should show error when flow generation fails', async () => {
      // Mock createFlow to fail
      mockCreateFlow.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(new Error('Flow generation failed'));
      });

      // Override MockConfigureSignInOptions to simulate selection without setting a flow
      const ConfigureSignInOptionsModule = await import(
        '../../components/create-application/configure-signin-options/ConfigureSignInOptions'
      );
      const useApplicationCreateContextModule = await import('../../hooks/useApplicationCreateContext');

      vi.mocked(ConfigureSignInOptionsModule.default).mockImplementation(
        ({onReadyChange}: {onReadyChange?: (ready: boolean) => void}) => {
          const {setSelectedAuthFlow, setIntegrations} = useApplicationCreateContextModule.default();

          const handleSetup = () => {
            setSelectedAuthFlow(null);
            setIntegrations({basic_auth: true});
            onReadyChange?.(true);
          };

          return (
            <div data-testid="application-configure-sign-in">
              <button type="button" data-testid="setup-flow-generation-error" onClick={handleSetup}>
                Setup Flow Generation Error
              </button>
            </div>
          );
        },
      );

      renderWithProviders();

      // Navigate to trigger point
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Options step
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });

      // Trigger setup
      await user.click(screen.getByTestId('setup-flow-generation-error'));

      // OPTIONS → EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // EXPERIENCE → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(screen.getByText('Flow generation failed')).toBeInTheDocument();
      });
    });
  });

  describe('MCP Client - Name step', () => {
    const selectMcpClientTemplate = async () => {
      await user.click(screen.getByTestId('select-mcp-client-template'));
    };

    it('should not render client type cards on the NAME step for the mcp-client template', async () => {
      renderWithProviders();

      await selectMcpClientTemplate();

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    });

    it('should show the generic "Create an Application" breadcrumb label for the mcp-client template', async () => {
      renderWithProviders();

      await selectMcpClientTemplate();

      expect(screen.getByText('Create an Application')).toBeInTheDocument();
    });

    it('should not render client type cards on the NAME step for non-mcp templates', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      expect(screen.queryByRole('radio')).not.toBeInTheDocument();
    });

    it('should show the generic breadcrumb label for non-mcp templates', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      expect(screen.getByText('Create an Application')).toBeInTheDocument();
    });
  });

  describe('MCP Client - Client type step', () => {
    const selectMcpClientTemplateAndName = async (name = 'My MCP App') => {
      await user.click(screen.getByTestId('select-mcp-client-template'));
      await user.type(screen.getByTestId('app-name-input'), name);
      // NAME -> CLIENT_TYPE
      await user.click(screen.getByRole('button', {name: /continue/i}));
    };

    it('should render the client type cards on the Client type step', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      expect(screen.getAllByRole('radio')).toHaveLength(2);
    });

    it('should default-select the user-delegated card', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
      expect(userDelegatedCard).toHaveAttribute('aria-checked', 'true');
      expect(m2mCard).toHaveAttribute('aria-checked', 'false');
    });

    it('should select the machine-to-machine card when clicked', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      expect(m2mCard).toHaveAttribute('aria-checked', 'true');
      expect(userDelegatedCard).toHaveAttribute('aria-checked', 'false');
    });

    it('should show the "Client type" breadcrumb label for the mcp-client template', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      const breadcrumbNav = screen.getByRole('navigation');
      expect(within(breadcrumbNav).getByText('Client type')).toBeInTheDocument();
    });

    it('should show the "what you get" preview panel and swap its content with the selection', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      expect(screen.getByText('Public client')).toBeInTheDocument();

      const [, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      expect(screen.queryByText('Public client')).not.toBeInTheDocument();
      expect(screen.getByText('Confidential client')).toBeInTheDocument();
    });

    it('should show the redirect URI editor inline for the user-delegated client type', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      expect(screen.getByTestId('application-configure-mcp-connection')).toBeInTheDocument();
    });

    it('should hide the redirect URI editor for the machine-to-machine client type', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();
      const [, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      expect(screen.queryByTestId('application-configure-mcp-connection')).not.toBeInTheDocument();
    });

    it('should disable Continue on the Client type step until a valid redirect URI is entered for the user-delegated client type', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();

      const uriInput = screen.getByPlaceholderText('http://localhost:8080/callback');
      await user.type(uriInput, 'https://agent.example.com/oauth/cb');

      expect(screen.getByTestId('application-wizard-next-button')).toBeEnabled();
    });

    it('should keep Continue disabled when the redirect URI is invalid', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      const uriInput = screen.getByPlaceholderText('http://localhost:8080/callback');
      await user.type(uriInput, 'http://example.com/cb');

      expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();
    });

    it('should enable Continue immediately when the machine-to-machine client type is selected', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();
      expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();

      const [, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      expect(screen.getByTestId('application-wizard-next-button')).toBeEnabled();
    });

    it('should re-disable Continue when switching back to the user-delegated client type without a redirect URI', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();
      const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);
      expect(screen.getByTestId('application-wizard-next-button')).toBeEnabled();

      await user.click(userDelegatedCard);

      expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();
    });

    it('should copy the MCP Inspector callback URI without filling any redirect URI input', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      await user.click(screen.getByRole('button', {name: 'Copy MCP Inspector callback URI'}));

      expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();
      expect(screen.queryByDisplayValue('http://localhost:6274/oauth/callback')).not.toBeInTheDocument();
    });

    it('should create the application directly from the Client type step for the machine-to-machine client type', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'mcp-app-1', name: 'My MCP App'} as Application);
      });

      renderWithProviders();

      await selectMcpClientTemplateAndName();
      const [, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      // CLIENT_TYPE -> create (no separate Connection step)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      expect(screen.queryByTestId('application-configure-mcp-connection')).not.toBeInTheDocument();
    });
  });

  describe('MCP Client - Submission & Connect completion', () => {
    const selectMcpClientTemplateAndName = async (name = 'My MCP App') => {
      await user.click(screen.getByTestId('select-mcp-client-template'));
      await user.type(screen.getByTestId('app-name-input'), name);
      // NAME -> CLIENT_TYPE
      await user.click(screen.getByRole('button', {name: /continue/i}));
    };

    const createUserDelegatedApp = async (redirectUri = 'https://agent.example.com/oauth/cb') => {
      await selectMcpClientTemplateAndName();

      const uriInput = screen.getByPlaceholderText('http://localhost:8080/callback');
      await user.type(uriInput, redirectUri);

      // CLIENT_TYPE -> create
      await user.click(screen.getByTestId('application-wizard-next-button'));
    };

    const createM2mApp = async () => {
      await selectMcpClientTemplateAndName();
      const [, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      // CLIENT_TYPE -> create (no separate Connection step for M2M)
      await user.click(screen.getByTestId('application-wizard-next-button'));
    };

    it('should submit the user-delegated oauth2 config spread from the seeded template config with the collected redirect URI', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'mcp-app-1', name: 'My MCP App'} as Application);
      });

      renderWithProviders();
      await createUserDelegatedApp('https://agent.example.com/oauth/cb');

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const requestBody = mockCreateApplication.mock.calls[0][0] as Application & {
        userAttributes?: string[];
        isRegistrationFlowEnabled?: boolean;
      };
      expect(requestBody.template).toBe('mcp-client');

      const oauth2Config = requestBody.inboundAuthConfig?.[0];
      expect(oauth2Config?.type).toBe('oauth2');
      expect(oauth2Config?.config).toMatchObject({
        grantTypes: ['authorization_code', 'refresh_token'],
        responseTypes: ['code'],
        redirectUris: ['https://agent.example.com/oauth/cb'],
        pkceRequired: true,
        tokenEndpointAuthMethod: 'none',
        publicClient: true,
      });

      expect(requestBody.userAttributes).toEqual(['given_name', 'family_name', 'email', 'groups']);
      expect(requestBody.isRegistrationFlowEnabled).toBe(true);
    });

    it('should not submit an empty redirect URI row left blank in the editor', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'mcp-app-1', name: 'My MCP App'} as Application);
      });

      renderWithProviders();
      await selectMcpClientTemplateAndName();

      const uriInput = screen.getByPlaceholderText('http://localhost:8080/callback');
      await user.type(uriInput, 'https://agent.example.com/oauth/cb');

      // Add a second row and leave it empty.
      await user.click(screen.getByRole('button', {name: /add redirect uri/i}));

      // CLIENT_TYPE -> create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const requestBody = mockCreateApplication.mock.calls[0][0] as Application;
      const oauth2Config = requestBody.inboundAuthConfig?.[0];
      expect(oauth2Config?.config?.redirectUris).toEqual(['https://agent.example.com/oauth/cb']);
    });

    it('should submit the machine-to-machine oauth2 config with client_credentials overrides and no redirect URIs', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'mcp-app-2', name: 'My MCP App'} as Application);
      });

      renderWithProviders();
      await createM2mApp();

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const requestBody = mockCreateApplication.mock.calls[0][0] as Application & {
        userAttributes?: string[];
        isRegistrationFlowEnabled?: boolean;
      };
      expect(requestBody.template).toBe('mcp-client');

      const oauth2Config = requestBody.inboundAuthConfig?.[0];
      expect(oauth2Config?.type).toBe('oauth2');
      expect(oauth2Config?.config).toMatchObject({
        grantTypes: ['client_credentials'],
        responseTypes: [],
        redirectUris: [],
        pkceRequired: false,
        publicClient: false,
        tokenEndpointAuthMethod: 'client_secret_basic',
      });

      expect(requestBody.userAttributes).toBeUndefined();
      expect(requestBody.isRegistrationFlowEnabled).toBeUndefined();
    });

    it('should show the MCP connect completion screen for the user-delegated client even without a client secret', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'mcp-app-3',
          name: 'My MCP App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'mcp-client-id',
                redirectUris: ['https://agent.example.com/oauth/cb'],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();
      await createUserDelegatedApp('https://agent.example.com/oauth/cb');

      await waitFor(() => {
        expect(screen.getByTestId('application-mcp-connect-complete')).toBeInTheDocument();
      });

      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
      expect(screen.getByTestId('mcp-connect-complete-client-id')).toHaveTextContent('mcp-client-id');
      expect(screen.getByTestId('mcp-connect-complete-redirect-uris')).toHaveTextContent(
        'https://agent.example.com/oauth/cb',
      );
      expect(screen.getByTestId('mcp-connect-complete-client-type')).toHaveTextContent('userDelegated');
    });

    it('should show the MCP connect completion screen for the machine-to-machine client with the client secret', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({
          id: 'mcp-app-4',
          name: 'My MCP App',
          inboundAuthConfig: [
            {
              type: 'oauth2',
              config: {
                clientId: 'mcp-client-id-m2m',
                clientSecret: 'mcp-client-secret',
                grantTypes: ['client_credentials'],
                responseTypes: [],
                redirectUris: [],
              },
            },
          ],
        } as Application);
      });

      renderWithProviders();
      await createM2mApp();

      await waitFor(() => {
        expect(screen.getByTestId('application-mcp-connect-complete')).toBeInTheDocument();
      });

      expect(screen.getByTestId('mcp-connect-complete-app-name')).toHaveTextContent('My MCP App');
      expect(screen.getByTestId('mcp-connect-complete-client-id')).toHaveTextContent('mcp-client-id-m2m');
      expect(screen.getByTestId('mcp-connect-complete-client-secret')).toHaveTextContent('mcp-client-secret');
      expect(screen.getByTestId('mcp-connect-complete-client-type')).toHaveTextContent('m2m');
    });

    it('should navigate to the created application when continue is clicked on the MCP connect completion screen', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'mcp-app-5', name: 'My MCP App'} as Application);
      });

      renderWithProviders();
      await createM2mApp();

      await waitFor(() => {
        expect(screen.getByTestId('application-mcp-connect-complete')).toBeInTheDocument();
      });

      await user.click(screen.getByTestId('mcp-connect-complete-continue'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/mcp-app-5');
      });
    });
  });

  describe('Progress bar (visibleSteps-based)', () => {
    const getProgressValue = (): number => Number(screen.getByRole('progressbar').getAttribute('aria-valuenow'));

    it('increases monotonically as the user advances through a generic template flow (regression)', async () => {
      renderWithProviders();

      const nameProgress = getProgressValue();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME -> DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const designProgress = getProgressValue();
      expect(designProgress).toBeGreaterThan(nameProgress);

      // DESIGN -> OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const optionsProgress = getProgressValue();
      expect(optionsProgress).toBeGreaterThan(designProgress);

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      // OPTIONS -> EXPERIENCE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const experienceProgress = getProgressValue();
      expect(experienceProgress).toBeGreaterThan(optionsProgress);
      expect(experienceProgress).toBeLessThanOrEqual(100);
    });

    it('advances the progress from NAME to DESIGN for a non-mcp template when CONFIGURE is skipped (regression)', async () => {
      const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
      vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('NONE');

      renderWithProviders();

      const nameProgress = getProgressValue();
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME -> DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const designProgress = getProgressValue();

      expect(designProgress).toBeGreaterThan(nameProgress);
    });

    it('keeps the same progress on the Client type step when switching between client types', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-mcp-client-template'));
      await user.type(screen.getByTestId('app-name-input'), 'My MCP App');
      // NAME -> CLIENT_TYPE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const clientTypeProgressUserDelegated = getProgressValue();
      const [, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      // The redirect URI editor is now embedded inline within the Client type step rather than
      // a separate step, so switching client types no longer changes visibleSteps — progress
      // stays put.
      expect(getProgressValue()).toBe(clientTypeProgressUserDelegated);
    });
  });
});
