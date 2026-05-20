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
import {render, screen, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ApplicationCreateProvider from '../../contexts/ApplicationCreate/ApplicationCreateProvider';
import type {Application} from '../../models/application';
import ApplicationCreatePage from '../ApplicationCreatePage';

// Mock functions
const mockCreateApplication = vi.fn();
const mockNavigate = vi.fn();

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
vi.mock('../../../user-types/api/useGetUserTypes', () => ({
  default: () => ({
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
vi.mock('../../../integrations/api/useIdentityProviders', () => ({
  default: () => ({
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
            <button type="button" data-testid="toggle-integration" onClick={() => onIntegrationToggle('basic_auth')}>
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
  }: {
    onReadyChange: (ready: boolean) => void;
    onApproachChange: (approach: string) => void;
    selectedApproach: string;
    userTypes: {name: string}[];
    selectedUserTypes: string[];
    onUserTypesChange: (types: string[]) => void;
  }) => {
    setTimeout(() => onReadyChange(true), 0);
    return (
      <div data-testid="application-configure-experience">
        <span data-testid="current-approach">{selectedApproach}</span>
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

vi.mock('../../components/create-application/ConfigureStack', async () => {
  const useApplicationCreateContextModule = await import('../../hooks/useApplicationCreateContext');

  return {
    default: ({onReadyChange}: {onReadyChange: (ready: boolean) => void}) => {
      const {setSelectedPlatform, setSelectedTemplateConfig} = useApplicationCreateContextModule.default();

      setTimeout(() => onReadyChange(true), 0);

      const handleSelectBackend = () => {
        setSelectedPlatform('BACKEND');
        setSelectedTemplateConfig({
          id: 'backend',
          creationFlow: {
            steps: ['STACK', 'NAME', 'ORGANIZATION_UNIT', 'COMPLETE'],
          },
        });
      };

      return (
        <div data-testid="application-configure-stack">
          Configure Stack
          <button type="button" data-testid="select-backend-platform" onClick={handleSelectBackend}>
            Select Backend
          </button>
        </div>
      );
    },
  };
});

vi.mock('../../components/create-application/ConfigureDetails', () => ({
  default: ({
    onReadyChange,
    onCallbackUrlChange,
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

describe('ApplicationCreatePage', () => {
  let user: ReturnType<typeof userEvent.setup>;

  const renderWithProviders = () =>
    render(
      <ApplicationCreateProvider>
        <ApplicationCreatePage />
      </ApplicationCreateProvider>,
    );

  beforeEach(async () => {
    user = userEvent.setup();

    window.history.replaceState({}, '', '/');

    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);

    const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
    vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('URL');
  });

  describe('Initial Rendering', () => {
    it('should render the stack step by default', () => {
      renderWithProviders();

      expect(screen.getByTestId('application-configure-stack')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-name')).not.toBeInTheDocument();
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

      expect(screen.getByText('Technology Stack')).toBeInTheDocument();
    });
  });

  describe('Step Navigation', () => {
    it('should disable Continue button when name is empty', async () => {
      renderWithProviders();

      // Navigate from STACK to NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      const continueButton = screen.getByRole('button', {name: /continue/i});
      expect(continueButton).toBeDisabled();
    });

    it('should enable Continue button when name is entered', async () => {
      renderWithProviders();

      // Navigate from STACK to NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      const continueButton = screen.getByRole('button', {name: /continue/i});
      expect(continueButton).toBeEnabled();
    });

    it('should navigate to design step from name step', async () => {
      renderWithProviders();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-name')).not.toBeInTheDocument();
    });

    it('should show preview from design step onwards', async () => {
      renderWithProviders();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('preview')).toBeInTheDocument();
    });

    it('should navigate through all steps', async () => {
      renderWithProviders();

      // Step 1: Stack
      expect(screen.getByTestId('application-configure-stack')).toBeInTheDocument();
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 2: Name
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 3: Design
      expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 4: Sign In Options
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 5: Experience
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-experience')).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 6: Configure Details
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
    });

    it('should show Back button from name step onwards', async () => {
      renderWithProviders();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByRole('button', {name: /back/i})).toBeInTheDocument();
    });

    it('should navigate back to previous step', async () => {
      renderWithProviders();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // NAME → STACK (back)
      await user.click(screen.getByRole('button', {name: /back/i}));

      expect(screen.getByTestId('application-configure-stack')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-name')).not.toBeInTheDocument();
    });
  });

  describe('Breadcrumb Navigation', () => {
    it('should update breadcrumb as user progresses', async () => {
      renderWithProviders();

      expect(screen.getByText('Technology Stack')).toBeInTheDocument();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // NAME → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // DESIGN → OPTIONS
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const firstBreadcrumb = screen.getByText('Technology Stack');
      await user.click(firstBreadcrumb);

      expect(screen.getByTestId('application-configure-stack')).toBeInTheDocument();
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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'Test App');

      expect(nameInput).toHaveValue('Test App');
    });

    it('should preserve app name when navigating between steps', async () => {
      renderWithProviders();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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
      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Should NOT show configure details step
      await waitFor(() => {
        expect(screen.queryByTestId('application-configure-details')).not.toBeInTheDocument();
        expect(mockCreateApplication).toHaveBeenCalled();
      });
    });
  });

  describe('Error Handling', () => {
    it('should show error when application creation fails', async () => {
      mockCreateApplication.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(new Error('Failed to create application'));
      });

      renderWithProviders();

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // Select BACKEND platform in STACK step
      await user.click(screen.getByTestId('select-backend-platform'));

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Enter app name
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → [create immediately]
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create → COMPLETE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      });

      expect(screen.getByTestId('application-client-secret-value')).toHaveTextContent('backend_secret_xyz');
    });

    it('should show only STACK and NAME in breadcrumb for backend platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // On NAME step breadcrumb should show STACK (clickable) and NAME (current)
      expect(screen.getByText('Technology Stack')).toBeInTheDocument();
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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create (no auth flow selected)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      expect(screen.queryByText(/no.*flow/i)).not.toBeInTheDocument();
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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

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

      // STACK → NAME
      await user.click(screen.getByRole('button', {name: /continue/i}));
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
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByText('Flow generation failed')).toBeInTheDocument();
      });
    });
  });
});
