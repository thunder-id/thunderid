// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import type {Application} from '@thunderid/configure-applications';
import type {Theme} from '@thunderid/design';
import {fireEvent, render, screen, waitFor, within} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {useEffect} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import ApplicationCreateProvider from '../../contexts/ApplicationCreate/ApplicationCreateProvider';
import useApplicationCreateContext from '../../hooks/useApplicationCreateContext';
import {OrganizationUnitDefaultItem} from '../../models/application-create-flow';
import ApplicationCreatePage from '../ApplicationCreatePage';

// Mock functions
const mockCreateApplication = vi.fn();
const mockNavigate = vi.fn();
let mockPathname = '/';
const mockUseGetApplications = vi.hoisted(() => vi.fn());
const mockUserTypes = vi.hoisted(() => ({
  types: [
    {id: 'customer', name: 'customer', displayName: 'Customer'},
    {id: 'employee', name: 'employee', displayName: 'Employee'},
  ],
}));

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

vi.mock('@thunderid/configure-applications', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-applications')>()),
  useGetApplications: mockUseGetApplications,
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
  DefaultTheme: {},
}));

// Mock application API. isPending is backed by real state, as in the actual hook, so the wizard can
// keep the submit button disabled until the create settles.
vi.mock('../../api/useCreateApplication', async () => {
  const {useState} = await vi.importActual<typeof import('react')>('react');

  const useMockCreateApplication = () => {
    const [isPending, setIsPending] = useState(false);
    const [data, setData] = useState<Application | undefined>(undefined);

    return {
      isPending,
      data,
      mutate: (
        data: unknown,
        options?: {onError?: (err: Error) => void; onSuccess?: (app: Application) => void},
      ): void => {
        setIsPending(true);
        mockCreateApplication(data, {
          onError: (err: Error) => {
            setIsPending(false);
            options?.onError?.(err);
          },
          onSuccess: (app: Application) => {
            setIsPending(false);
            setData(app);
            options?.onSuccess?.(app);
          },
        });
      },
    };
  };

  return {default: useMockCreateApplication};
});

// Mock user types API
vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypes: () => ({
    data: mockUserTypes,
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
const {mockCreateFlow, mockGenerateFlowGraph, mockDeleteFlow} = vi.hoisted(() => ({
  mockCreateFlow: vi.fn(),
  mockGenerateFlowGraph: vi.fn(),
  mockDeleteFlow: vi.fn(),
}));

vi.mock('../../../flows/api/useCreateFlow', () => ({
  default: () => ({
    mutate: mockCreateFlow,
    isPending: false,
  }),
}));

vi.mock('../../../flows/api/useDeleteFlow', () => ({
  default: () => ({
    mutate: mockDeleteFlow,
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

vi.mock('../../../flows/api/useGetFlowById', () => ({
  default: () => ({data: undefined, isLoading: false, error: null}),
}));

// Mock configuration type utility
vi.mock('../../utils/getConfigurationTypeFromTemplate', () => ({
  default: vi.fn(() => 'URL'),
}));

const {mockUseHasMultipleOUs, mockUseGetOrganizationUnit} = vi.hoisted(() => ({
  mockUseHasMultipleOUs: vi.fn(),
  mockUseGetOrganizationUnit: vi.fn(),
}));

vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: mockUseHasMultipleOUs,
  useGetOrganizationUnit: mockUseGetOrganizationUnit,
  OrganizationUnitPickerScreen: ({
    value,
    onChange,
    onBack,
    onContinue,
  }: {
    value: string;
    onChange: (ouId: string) => void;
    onBack: () => void;
    onContinue: () => void;
  }) => (
    <div data-testid="ou-picker-screen">
      <button type="button" data-testid="ou-picker-select-ou1" onClick={() => onChange('ou-1')}>
        Select OU 1
      </button>
      <button type="button" data-testid="ou-picker-back" onClick={onBack}>
        Back
      </button>
      <button type="button" data-testid="ou-picker-continue" disabled={!value} onClick={onContinue}>
        Continue
      </button>
    </div>
  ),
}));

// Mock child components
//
// The DETAILS step now renders ConfigureApplicationDetails (name field + OU banner/"Change" link
// + OU-defaults accordion + user access section), replacing the old bare ConfigureName step. The
// OU-defaults accordion and user access section are exercised by their own component tests, so
// this mock keeps only the name field and the "Change" link — the bits this page-level suite
// orchestrates around (step readiness, reopening the OU picker). Root testid kept as
// "application-configure-name" (rather than the real component's "application-configure-details")
// to avoid colliding with the CONFIGURE step's mock below, which never renders at the same time
// but shares the same real-component testid convention.
vi.mock('../../components/create-application/ConfigureApplicationDetails', async () => {
  const {useEffect} = await import('react');
  const MockConfigureApplicationDetails = ({
    hasMultipleOUs,
    onChangeOu,
    appName,
    onAppNameChange,
    appLogo,
    onLogoSelect,
    onReadyChange = undefined,
    existingAppNames = [],
  }: {
    hasMultipleOUs: boolean;
    onChangeOu: () => void;
    appName: string;
    onAppNameChange: (name: string) => void;
    appLogo: string | null;
    onLogoSelect: (logo: string) => void;
    onReadyChange?: (ready: boolean) => void;
    existingAppNames?: string[];
  }) => {
    // Mirror the real component: readiness is broadcast from an effect (including on mount) and a
    // name already in the list is treated as a duplicate that blocks readiness.
    const isDuplicate = appName.length > 0 && existingAppNames.includes(appName);
    useEffect(() => {
      onReadyChange?.(appName.trim().length > 0 && !isDuplicate);
    }, [appName, isDuplicate, onReadyChange]);
    return (
      <div data-testid="application-configure-name" data-existing-names={existingAppNames.join(',')}>
        {hasMultipleOUs && (
          <button type="button" onClick={onChangeOu}>
            Change
          </button>
        )}
        {appLogo ? <span data-testid="preview-logo">{appLogo}</span> : null}
        <button type="button" data-testid="logo-select-btn" onClick={() => onLogoSelect('test-logo.png')}>
          Select Logo
        </button>
        <input
          data-testid="app-name-input"
          value={appName}
          onChange={(e) => onAppNameChange(e.target.value)}
          placeholder="Enter app name"
        />
        {isDuplicate ? <span data-testid="app-name-duplicate-error">duplicate</span> : null}
      </div>
    );
  };
  return {default: MockConfigureApplicationDetails};
});

// The Design step now also hosts the sign-in approach picker (hosted pages vs. embedded) that used
// to live on a separate Experience step, so this mock folds what used to be the ConfigureExperience
// mock into it. Real theme/layout data-fetching hooks are skipped since this whole component is
// mocked.
function DefaultConfigureDesignImpl({
  onThemeSelect = undefined,
  onReadyChange = undefined,
  onApproachChange,
  selectedApproach,
  allowEmbeddedApproach,
}: {
  onThemeSelect?: (themeId: string, themeConfig: Theme) => void;
  onReadyChange?: (ready: boolean) => void;
  onApproachChange: (approach: string) => void;
  selectedApproach: string;
  allowEmbeddedApproach: boolean;
}): JSX.Element {
  // Runs once on mount only: onReadyChange(true) becomes a no-op-content-but-new-reference
  // state update in the parent, so calling it on every render would reschedule itself forever.
  useEffect(() => {
    const timer = setTimeout(() => onReadyChange?.(true), 0);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    <div data-testid="application-configure-design">
      <span data-testid="current-approach">{selectedApproach}</span>
      <span data-testid="allow-embedded-approach">{String(allowEmbeddedApproach)}</span>
      <button type="button" data-testid="select-embedded-approach" onClick={() => onApproachChange('EMBEDDED')}>
        Select Embedded
      </button>
      <button type="button" data-testid="select-inbuilt-approach" onClick={() => onApproachChange('REDIRECT_BASED')}>
        Select Inbuilt
      </button>
      <button type="button" data-testid="select-theme-btn" onClick={() => onThemeSelect?.('theme-1', {} as Theme)}>
        Select Theme
      </button>
    </div>
  );
}

vi.mock('../../components/create-application/ConfigureDesign', () => ({
  default: vi.fn(DefaultConfigureDesignImpl),
}));

function DefaultConfigureSignInOptionsImpl({
  onIntegrationToggle,
  onReadyChange = undefined,
}: {
  onIntegrationToggle: (id: string) => void;
  onReadyChange?: (ready: boolean) => void;
}) {
  const {setSelectedAuthFlow, setIntegrations} = useApplicationCreateContext();

  // Simulates picking a pre-configured flow, so most tests reach a "ready" DESIGN step without
  // exercising the real flow-generation path. Mirrors the real handleFlowSelect behavior by also
  // clearing integrations — a selected flow and enabled integrations never coexist in production
  // (see ensureFlowAndCreateApplication's hasEnabledIntegrations gate), so tests shouldn't simulate
  // that combination either. Runs once on mount only: the parent's onReadyChange handler replaces
  // its state object on every call, so re-running this on every render would reschedule itself
  // forever and never let the test runner proceed.
  useEffect(() => {
    const timer = setTimeout(() => {
      setIntegrations({});
      setSelectedAuthFlow({
        id: 'test-flow-id',
        name: 'Test Flow',
        flowType: 'AUTHENTICATION',
        handle: 'test-flow',
        activeVersion: 1,
        createdAt: '2024-01-01T00:00:00Z',
        updatedAt: '2024-01-01T00:00:00Z',
      });
      onReadyChange?.(true);
    }, 0);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div data-testid="application-configure-sign-in">
      <button type="button" data-testid="toggle-integration" onClick={() => onIntegrationToggle('credentials_auth')}>
        Toggle Integration
      </button>
    </div>
  );
}

// ConfigureSecuritySettings itself is left real: it's a thin wrapper (heading + an OU-defaults
// skip check) around ConfigureSignInOptions, so only the latter needs mocking here.
vi.mock('../../components/create-application/configure-signin-options/ConfigureSignInOptions', () => ({
  default: vi.fn(DefaultConfigureSignInOptionsImpl),
}));

function DefaultConfigureDetailsImpl({
  onReadyChange,
  onCallbackUrlChange,
  onHostingUrlChange,
}: {
  onReadyChange: (ready: boolean) => void;
  onCallbackUrlChange: (url: string) => void;
  onHostingUrlChange: (url: string) => void;
}): JSX.Element {
  // Runs once on mount only: onReadyChange(true) becomes a no-op-content-but-new-reference
  // state update in the parent, so calling it on every render would reschedule itself forever.
  useEffect(() => {
    const timer = setTimeout(() => onReadyChange(true), 0);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
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
}

vi.mock('../../components/create-application/ConfigureDetails', () => ({
  default: vi.fn(DefaultConfigureDetailsImpl),
}));

vi.mock('@thunderid/configure-design', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-design')>()),
  GatePreview: ({
    showToolbar = undefined,
    viewport = undefined,
  }: {
    showToolbar?: boolean;
    viewport?: {width: string; height: string};
  }) => (
    <div data-testid="preview" data-show-toolbar={String(showToolbar)} data-viewport-width={viewport?.width ?? ''} />
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
  const {setSelectedTechnology, setSelectedPlatform, setSelectedTemplateConfig, setCurrentStep, setOuDefaults} =
    useApplicationCreateContext();

  const seed = (technology: unknown, platform: unknown, template: unknown): void => {
    setSelectedTechnology(technology as never);
    setSelectedPlatform(platform as never);
    setSelectedTemplateConfig(template as never);
    setCurrentStep('ORGANIZATION_UNIT');
  };

  return (
    <div>
      {/* Toggling the Details step's "use organization unit defaults" checkboxes normally does this;
          seeding it directly here keeps these tests independent of that UI (useGetOrganizationUnit is
          mocked to return no organization unit, which would otherwise hide those checkboxes entirely). */}
      <button
        type="button"
        data-testid="seed-ou-default-sign-in"
        onClick={() =>
          setOuDefaults({
            [OrganizationUnitDefaultItem.SIGN_IN]: true,
            [OrganizationUnitDefaultItem.SIGN_UP]: false,
            [OrganizationUnitDefaultItem.RECOVERY]: false,
            [OrganizationUnitDefaultItem.SIGN_OUT]: false,
            [OrganizationUnitDefaultItem.THEME]: false,
            [OrganizationUnitDefaultItem.LAYOUT]: false,
          })
        }
      >
        Seed Sign-In OU Default
      </button>
      <button
        type="button"
        data-testid="seed-ou-default-design"
        onClick={() =>
          setOuDefaults({
            [OrganizationUnitDefaultItem.SIGN_IN]: false,
            [OrganizationUnitDefaultItem.SIGN_UP]: false,
            [OrganizationUnitDefaultItem.RECOVERY]: false,
            [OrganizationUnitDefaultItem.SIGN_OUT]: false,
            [OrganizationUnitDefaultItem.THEME]: true,
            [OrganizationUnitDefaultItem.LAYOUT]: true,
          })
        }
      >
        Seed Design OU Default
      </button>
      <button
        type="button"
        data-testid="seed-ou-default-sign-in-and-design"
        onClick={() =>
          setOuDefaults({
            [OrganizationUnitDefaultItem.SIGN_IN]: true,
            [OrganizationUnitDefaultItem.SIGN_UP]: false,
            [OrganizationUnitDefaultItem.RECOVERY]: false,
            [OrganizationUnitDefaultItem.SIGN_OUT]: false,
            [OrganizationUnitDefaultItem.THEME]: true,
            [OrganizationUnitDefaultItem.LAYOUT]: true,
          })
        }
      >
        Seed Sign-In and Design OU Default
      </button>
      <button
        type="button"
        aria-label="seed server template"
        data-testid="select-backend-platform"
        onClick={() =>
          seed(null, 'BACKEND', {
            id: 'backend',
            creationFlow: {
              steps: ['ORGANIZATION_UNIT', 'DETAILS', 'COMPLETE'],
              previewSteps: [],
              allowsUserLogins: false,
            },
          })
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
            type: 'mobile',
            creationFlow: {
              steps: ['ORGANIZATION_UNIT', 'DETAILS', 'SECURITY', 'DESIGN', 'CONFIGURE', 'COMPLETE'],
              previewSteps: ['DETAILS', 'SECURITY', 'DESIGN'],
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
            type: 'browser',
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
        aria-label="seed android technology template"
        data-testid="select-android-technology"
        onClick={() =>
          seed('ANDROID', null, {
            id: 'android',
            type: 'mobile',
            previewDevice: 'mobile',
            creationFlow: {
              steps: ['ORGANIZATION_UNIT', 'DETAILS', 'SECURITY', 'DESIGN', 'CONFIGURE', 'COMPLETE'],
              previewSteps: ['DETAILS', 'SECURITY', 'DESIGN'],
            },
            defaults: {
              signInApproach: 'EMBEDDED',
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
        Select Android
      </button>
      <button
        type="button"
        aria-label="seed full-stack template"
        data-testid="select-fullstack-platform"
        onClick={() =>
          seed(null, 'FULL_STACK', {
            id: 'full-stack',
            type: 'fullstack',
            defaults: {
              inboundAuthConfig: [
                {
                  type: 'oauth2',
                  config: {
                    grantTypes: ['authorization_code', 'refresh_token'],
                    responseTypes: ['code'],
                    publicClient: false,
                    tokenEndpointAuthMethod: 'client_secret_basic',
                  },
                },
              ],
            },
          })
        }
      >
        Select Full-stack
      </button>
      <button
        type="button"
        aria-label="seed mcp template"
        data-testid="select-mcp-client-template"
        onClick={() =>
          seed(null, null, {
            id: 'mcp-client',
            creationFlow: {steps: ['ORGANIZATION_UNIT', 'DETAILS', 'CLIENT_TYPE', 'COMPLETE'], previewSteps: []},
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

  // Walks DETAILS → SECURITY → DESIGN, the point from which the Design step's own approach/theme
  // controls (see the mock above) and the CONFIGURE step become reachable with a single further
  // Continue click.
  const goToDesignStep = async () => {
    await user.type(screen.getByTestId('app-name-input'), 'My App');
    // DETAILS → SECURITY
    await user.click(screen.getByRole('button', {name: /continue/i}));

    await waitFor(() => {
      expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
    });
    // SECURITY → DESIGN
    await user.click(screen.getByRole('button', {name: /continue/i}));

    await waitFor(() => {
      expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
    });
  };

  beforeEach(async () => {
    user = userEvent.setup();

    window.history.replaceState({}, '', '/');
    mockPathname = '/';

    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);

    // Flow cleanup succeeds unless a test says otherwise.
    mockDeleteFlow.mockImplementation((_flowId: string, options?: {onSuccess?: () => void}) => {
      options?.onSuccess?.();
    });

    const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
    vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('URL');

    mockUseHasMultipleOUs.mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [],
    });

    mockUseGetOrganizationUnit.mockReturnValue({
      data: undefined,
      isLoading: false,
    });

    mockUseGetApplications.mockReturnValue({data: {applications: []}});
    mockUserTypes.types = [
      {id: 'customer', name: 'customer', displayName: 'Customer'},
      {id: 'employee', name: 'employee', displayName: 'Employee'},
    ];
  });

  describe('Initial Rendering', () => {
    it('should render the name step by default', () => {
      renderWithProviders();

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
    });

    it('should show the preview panel on the first (details) step', () => {
      renderWithProviders();

      expect(screen.getByTestId('preview')).toBeInTheDocument();
    });

    it('should render close button', () => {
      const {container} = renderWithProviders();

      const buttons = container.querySelectorAll('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should show breadcrumb with current step', () => {
      renderWithProviders();

      expect(screen.getByText('Details')).toBeInTheDocument();
    });
  });

  describe('Organization unit picker (pre-wizard step)', () => {
    it('shows the organization unit picker instead of the wizard when multiple OUs exist and none is picked yet', () => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [{id: 'ou-1', name: 'Default'}],
      });

      renderWithProviders();

      expect(screen.getByTestId('ou-picker-screen')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-name')).not.toBeInTheDocument();
    });

    it('does not show the picker when only one organization unit exists', () => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: false,
        isLoading: false,
        ouList: [{id: 'ou-1', name: 'Default'}],
      });

      renderWithProviders();

      expect(screen.queryByTestId('ou-picker-screen')).not.toBeInTheDocument();
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
    });

    it('proceeds to the wizard once an organization unit is picked and continue is clicked', async () => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [{id: 'ou-1', name: 'Default'}],
      });

      renderWithProviders();

      await user.click(screen.getByTestId('ou-picker-select-ou1'));
      await user.click(screen.getByTestId('ou-picker-continue'));

      expect(screen.queryByTestId('ou-picker-screen')).not.toBeInTheDocument();
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
    });

    it('navigates back to the template gallery when Back is clicked before anything is picked', async () => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [{id: 'ou-1', name: 'Default'}],
      });

      renderWithProviders();

      await user.click(screen.getByTestId('ou-picker-back'));

      expect(mockNavigate).toHaveBeenCalledWith('/applications/types');
    });

    it('reopens the picker from the Details step Change link, and Back cancels the change', async () => {
      mockUseHasMultipleOUs.mockReturnValue({
        hasMultipleOUs: true,
        isLoading: false,
        ouList: [{id: 'ou-1', name: 'Default'}],
      });

      renderWithProviders();

      await user.click(screen.getByTestId('ou-picker-select-ou1'));
      await user.click(screen.getByTestId('ou-picker-continue'));
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();

      await user.click(screen.getByRole('button', {name: 'Change'}));
      expect(screen.getByTestId('ou-picker-screen')).toBeInTheDocument();

      const navigateCallCountBeforeBack = mockNavigate.mock.calls.length;
      await user.click(screen.getByTestId('ou-picker-back'));

      // Back from a reopened ("Change") picker cancels the change instead of leaving the wizard.
      expect(mockNavigate).toHaveBeenCalledTimes(navigateCallCountBeforeBack);
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
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

    it('should navigate to the security step from the details step', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-name')).not.toBeInTheDocument();
    });

    it('should show preview from the security step onwards', async () => {
      renderWithProviders();

      const nameInput = screen.getByTestId('app-name-input');
      await user.type(nameInput, 'My App');

      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByTestId('preview')).toBeInTheDocument();
    });

    it('should navigate through all steps', async () => {
      renderWithProviders();

      // Step 1: Details
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 2: Security Settings (Sign In Options)
      expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 3: Design (theme + sign-in approach)
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      });
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Step 4: Configure Details
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
    });

    it('should show Back button from the security step onwards', async () => {
      renderWithProviders();

      // DETAILS is the first step, so there is no Back button yet.
      expect(screen.queryByRole('button', {name: /back/i})).not.toBeInTheDocument();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getByRole('button', {name: /back/i})).toBeInTheDocument();
    });

    it('should navigate back to previous step', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));
      expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();

      // SECURITY → DETAILS (back)
      await user.click(screen.getByRole('button', {name: /back/i}));

      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-sign-in')).not.toBeInTheDocument();
    });
  });

  describe('Breadcrumb Navigation', () => {
    it('should update breadcrumb as user progresses', async () => {
      renderWithProviders();

      expect(screen.getByText('Details')).toBeInTheDocument();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      expect(screen.getAllByText('Security').length).toBeGreaterThan(0);

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // The Design step's breadcrumb label is "Experience" (it covers both theme and sign-in
      // approach, not just visual design).
      expect(screen.getByText('Experience')).toBeInTheDocument();
    });

    it('should allow clicking on previous breadcrumb steps', async () => {
      renderWithProviders();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      const firstBreadcrumb = screen.getByText('Details');
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

      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));
      // SECURITY → DETAILS (back)
      await user.click(screen.getByRole('button', {name: /back/i}));

      expect(screen.getByTestId('app-name-input')).toHaveValue('My App');
    });

    it('should update logo in state', async () => {
      renderWithProviders();

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
      await goToDesignStep();
      // DESIGN → CONFIGURE
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
      // A template without an explicit type resolves to the custom fallback.
      expect(createAppCall.type).toBe('custom');
    });

    it('should navigate to application details page after creation', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      // Navigate through all steps
      await goToDesignStep();
      // DESIGN → CONFIGURE
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

      await goToDesignStep();

      // Select embedded approach on the Design step, then it creates the application directly
      // (embedded skips CONFIGURE).
      const selectEmbeddedBtn = screen.getByTestId('select-embedded-approach');
      await user.click(selectEmbeddedBtn);
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

      await goToDesignStep();

      await user.click(screen.getByTestId('select-embedded-approach'));
      // DESIGN → Create (embedded skips configure)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should NOT show configure details step
      await waitFor(() => {
        expect(screen.queryByTestId('application-configure-details')).not.toBeInTheDocument();
        expect(mockCreateApplication).toHaveBeenCalled();
      });
    });
  });

  describe('Embedded Approach Availability', () => {
    it('should offer the embedded approach for the wallet platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-wallet-platform'));
      await goToDesignStep();

      expect(screen.getByTestId('allow-embedded-approach')).toHaveTextContent('true');
    });

    it('should offer the embedded approach for the Android technology template', async () => {
      // Android (like iOS/Flutter) is a public client for the same reason browser SPAs are, but
      // it's a native app-native flow, not a redirect-only one — the embedded approach must still
      // be offered, matching the generic Mobile platform template's behavior.
      renderWithProviders();

      await user.click(screen.getByTestId('select-android-technology'));
      await goToDesignStep();

      expect(screen.getByTestId('allow-embedded-approach')).toHaveTextContent('true');
    });

    it('should show the CONFIGURE step after DESIGN for the wallet platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-wallet-platform'));
      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // The CONFIGURE step has no hosted sign-in screen to preview, so the preview stays hidden.
      expect(screen.queryByTestId('preview')).not.toBeInTheDocument();
    });

    it('should disallow the embedded approach for browser-based SPAs (redirect-only public clients)', async () => {
      // Browser SPAs are redirect-only, so there's no native sign-in approach to choose — only
      // the theme section of the Design step applies to them.
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      });
      expect(screen.getByTestId('allow-embedded-approach')).toHaveTextContent('false');
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
    });

    it('should still reach the CONFIGURE step for a redirect-capable template with a prefilled placeholder redirect URI', async () => {
      // Templates ship a placeholder redirectUris value so getConfigurationTypeFromTemplate
      // (mocked here to match its real behavior for such a template) returns NONE; the CONFIGURE
      // step must still show because the template is redirect-capable, so the redirect URI needs
      // to be confirmed/edited rather than silently left at the placeholder.
      const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
      vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('NONE');

      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
    });
  });

  describe('Preview Device', () => {
    it('always hides the preview toolbar', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));

      await waitFor(() => {
        expect(screen.getByTestId('preview')).toHaveAttribute('data-show-toolbar', 'false');
      });
    });

    it('renders the mobile-sized viewport for the Android technology template', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-android-technology'));

      await waitFor(() => {
        const preview = screen.getByTestId('preview');
        expect(preview).toHaveAttribute('data-show-toolbar', 'false');
        expect(preview).toHaveAttribute('data-viewport-width', '40%');
      });
    });

    it('renders the default desktop-sized viewport for non-mobile templates', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));

      await waitFor(() => {
        expect(screen.getByTestId('preview')).toHaveAttribute('data-viewport-width', '');
      });
    });
  });

  describe('Organization Unit Defaults (empty-step skipping)', () => {
    it('should skip the SECURITY step when Sign In is snapshotted from the organization unit default', async () => {
      // With Sign In inherited, ConfigureSecuritySettings would have nothing left to render (Sign
      // Up/Recovery/Sign Out live elsewhere), so the step itself should be skipped rather than
      // shown empty.
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await user.click(screen.getByTestId('seed-ou-default-sign-in'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → DESIGN (SECURITY is skipped)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      });
      expect(screen.queryByTestId('application-configure-sign-in')).not.toBeInTheDocument();
    });

    it('should skip the DESIGN step when Theme and Layout are snapshotted from the organization unit default', async () => {
      // With both Theme and Layout inherited, ConfigureDesign would have nothing left to render,
      // so the step itself should be skipped rather than shown empty.
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {id: 'ou-1', themeId: 'theme-1', layoutId: 'layout-1'},
        isLoading: false,
      });
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await user.click(screen.getByTestId('seed-ou-default-design'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → CONFIGURE (DESIGN is skipped)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
    });

    it('should skip the DESIGN step when only Theme is available on the organization unit and it is snapshotted', async () => {
      // The organization unit has no layoutId, so the "Design" default was only ever available
      // for Theme. Opting into that (the only available default) should still skip the step
      // rather than forcing a manual Layout pick.
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {id: 'ou-1', themeId: 'theme-1', layoutId: undefined},
        isLoading: false,
      });
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await user.click(screen.getByTestId('seed-ou-default-design'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → CONFIGURE (DESIGN is skipped)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
    });

    it('should not skip the DESIGN step when Theme is available but not snapshotted, even if Layout is unavailable', async () => {
      // Sanity check for the fix above: an unavailable item must not make the step disappear on
      // its own — the available one still has to actually be opted into.
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {id: 'ou-1', themeId: 'theme-1', layoutId: undefined},
        isLoading: false,
      });
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY (no OU defaults seeded)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → DESIGN (not skipped, since Theme was never opted into)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      });
    });

    it('should skip both SECURITY and DESIGN when all their organization unit defaults are used', async () => {
      mockUseGetOrganizationUnit.mockReturnValue({
        data: {id: 'ou-1', themeId: 'theme-1', layoutId: 'layout-1'},
        isLoading: false,
      });
      renderWithProviders();

      await user.click(screen.getByTestId('select-browser-platform'));
      await user.click(screen.getByTestId('seed-ou-default-sign-in-and-design'));
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → CONFIGURE (both SECURITY and DESIGN are skipped)
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      expect(screen.queryByTestId('application-configure-sign-in')).not.toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
    });

    it('should send the canonical type in the create payload for the wallet (mobile) platform', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-mobile', name: 'My App'} as Application);
      });

      renderWithProviders();
      await user.click(screen.getByTestId('select-wallet-platform'));
      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });
      expect((mockCreateApplication.mock.calls[0][0] as Application).type).toBe('mobile');
    });

    it('should send the canonical type in the create payload for the full-stack platform', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-fullstack', name: 'My App'} as Application);
      });

      renderWithProviders();
      await user.click(screen.getByTestId('select-fullstack-platform'));
      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });
      expect((mockCreateApplication.mock.calls[0][0] as Application).type).toBe('fullstack');
    });
  });

  describe('Error Handling', () => {
    it('should exclude a newly created application from duplicate-name validation after the list refreshes', async () => {
      let applicationsData = {applications: [] as Application[]};
      mockUseGetApplications.mockImplementation(() => ({data: applicationsData}));
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        applicationsData = {applications: [{id: 'app-123', name: 'My App'} as Application]};
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // The mocked navigation keeps the wizard mounted. Return to Details after the application
      // list has refreshed with the created name and verify the submitted name is not treated as a
      // duplicate while the wizard is still open.
      await user.click(screen.getByRole('button', {name: 'Details'}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      });
      expect(screen.getByTestId('application-configure-name')).toHaveAttribute('data-existing-names', '');
      expect(screen.queryByTestId('app-name-duplicate-error')).not.toBeInTheDocument();
    });

    it('should restore duplicate-name validation when creation fails after a successful submission', async () => {
      const duplicateError = Object.assign(new Error('Bad Request'), {
        response: {status: 400, data: {code: 'APP-1020', message: 'Application already exists'}},
      });
      let applicationsData = {applications: [] as Application[]};
      mockUseGetApplications.mockImplementation(() => ({data: applicationsData}));
      let createAttempt = 0;
      mockCreateApplication.mockImplementation(
        (_data, options: {onError: (error: Error) => void; onSuccess: (app: Application) => void}) => {
          createAttempt += 1;
          if (createAttempt === 1) {
            applicationsData = {applications: [{id: 'app-123', name: 'My App'} as Application]};
            options.onSuccess({id: 'app-123', name: 'My App'} as Application);
            return;
          }
          options.onError(duplicateError);
        },
      );

      renderWithProviders();

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create successfully once.
      await user.click(screen.getByTestId('application-wizard-next-button'));
      await waitFor(() => expect(mockCreateApplication).toHaveBeenCalledTimes(1));

      // Submit the same still-mounted wizard again. The second failure clears isCreationSubmitted,
      // so the refreshed application name is visible to duplicate validation again.
      await user.click(screen.getByTestId('application-wizard-next-button'));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
        expect(screen.getByTestId('app-name-duplicate-error')).toBeInTheDocument();
      });
      expect(screen.getByTestId('application-configure-name')).toHaveAttribute(
        'data-existing-names',
        expect.stringContaining('My App'),
      );
    });

    it('should show error when application creation fails', async () => {
      mockCreateApplication.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(new Error('Failed to create application'));
      });

      renderWithProviders();

      await goToDesignStep();
      // DESIGN → CONFIGURE
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

      await goToDesignStep();
      // DESIGN → CONFIGURE
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

      const closeButton = within(screen.getByRole('alert')).getByLabelText(/close/i);
      await user.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByText(/failed to create application/i)).not.toBeInTheDocument();
      });
    });

    it('should show the duplicate name message and return to the details step on APP-1020', async () => {
      const duplicateError = Object.assign(new Error('Bad Request'), {
        response: {status: 400, data: {code: 'APP-1020', message: 'Application already exists'}},
      });
      mockCreateApplication.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(duplicateError);
      });

      renderWithProviders();

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create (rejected with APP-1020)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(
        () => {
          expect(screen.getByText(/an application with this name already exists/i)).toBeInTheDocument();
        },
        {timeout: 10000},
      );
      expect(screen.queryByText(/bad request/i)).not.toBeInTheDocument();
      // The wizard navigates back to the details step, which hosts the name field.
      expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
    });

    it('should block resubmitting the same name after APP-1020 until it is edited', async () => {
      const duplicateError = Object.assign(new Error('Bad Request'), {
        response: {status: 400, data: {code: 'APP-1020', message: 'Application already exists'}},
      });
      mockCreateApplication.mockImplementation((_data, {onError}: {onError: (error: Error) => void}) => {
        onError(duplicateError);
      });

      renderWithProviders();

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create (rejected with APP-1020)
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Back on the details step, the rejected name is now treated as existing: flagged as a
      // duplicate and Continue disabled, so the same name cannot be resubmitted.
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-name')).toBeInTheDocument();
      });
      expect(screen.getByTestId('application-configure-name')).toHaveAttribute(
        'data-existing-names',
        expect.stringContaining('My App'),
      );
      expect(screen.getByTestId('app-name-duplicate-error')).toBeInTheDocument();
      expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();
      expect(screen.getByText(/an application with this name already exists/i)).toBeInTheDocument();

      // Editing to a new name clears the duplicate flag and re-enables Continue. The stale create
      // error is cleared too, by the provider's form fingerprint.
      await user.type(screen.getByTestId('app-name-input'), ' v2');

      await waitFor(() => {
        expect(screen.queryByTestId('app-name-duplicate-error')).not.toBeInTheDocument();
      });
      expect(screen.queryByText(/an application with this name already exists/i)).not.toBeInTheDocument();
      expect(screen.getByTestId('application-wizard-next-button')).toBeEnabled();
    });
  });

  describe('Theme Selection', () => {
    it('should allow selecting a theme', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await goToDesignStep();

      // Select a theme
      const selectThemeBtn = screen.getByTestId('select-theme-btn');
      await user.click(selectThemeBtn);

      // DESIGN → CONFIGURE
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
      // DETAILS → SECURITY
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

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });

      const callbackInput = screen.getByTestId('callback-url-input');
      await user.type(callbackInput, 'https://example.com/callback');

      expect(callbackInput).toHaveValue('https://example.com/callback');
    });
  });

  describe('Client Secret Handoff (navigate to detail page)', () => {
    it('should navigate to application details with justCreatedSecret state when clientSecret is created', async () => {
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

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should navigate straight to the detail page, carrying the secret in navigation state
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123', {
          state: {
            // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
            justCreatedSecret: expect.objectContaining({
              appName: 'My App',
              clientId: 'test-client-id',
              clientSecret: 'test_secret_12345',
            }),
          },
        });
      });

      // Should not render the secret inline as a wizard step
      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
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

      await goToDesignStep();
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      // Should navigate directly to application details page, with no navigation state
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123');
      });

      // Should not show COMPLETE step
      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
    });
  });

  describe('Backend Platform (BACKEND / M2M) Flow', () => {
    it('should skip DESIGN, SECURITY and CONFIGURE and create app directly from NAME step', async () => {
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

      // Should not have visited DESIGN, SECURITY or CONFIGURE
      expect(screen.queryByTestId('application-configure-design')).not.toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-sign-in')).not.toBeInTheDocument();
      expect(screen.queryByTestId('application-configure-details')).not.toBeInTheDocument();
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

    it('should create backend app without allowedUserTypes', async () => {
      mockUserTypes.types = [{id: 'customer', name: 'customer', displayName: 'Customer'}];
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'backend-app-no-user-types', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      const createAppCall = mockCreateApplication.mock.calls[0][0] as Record<string, unknown>;
      expect(createAppCall.allowedUserTypes).toBeUndefined();
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

    it('should navigate to details with justCreatedSecret state after backend app creation', async () => {
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

      // NAME → create → navigate to detail page with the secret
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/backend-app-5', {
          state: {
            // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
            justCreatedSecret: expect.objectContaining({
              clientSecret: 'backend_secret_xyz',
            }),
          },
        });
      });
    });

    it('should show only DETAILS in breadcrumb for backend platform', async () => {
      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));

      // On the DETAILS step the breadcrumb shows only the current step for the backend flow.
      expect(screen.getByText('Details')).toBeInTheDocument();
      // Design/Security/Configure should NOT appear in breadcrumb
      expect(screen.queryByText('Experience')).not.toBeInTheDocument();
      expect(screen.queryByText('Security')).not.toBeInTheDocument();
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

    it('should not flag the just-created app as a duplicate of itself after the create-triggered list refetch', async () => {
      // Models the create success's list invalidation resolving while the wizard is still mounted
      // (navigate() hasn't unmounted it yet): the refetched applications list now legitimately
      // contains the app that was just created, under the exact name just submitted.
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        mockUseGetApplications.mockReturnValue({
          data: {applications: [{id: 'backend-app-7', name: 'My Backend App', clientId: 'backend-client-7'}]},
        });
        onSuccess({id: 'backend-app-7', name: 'My Backend App'} as Application);
      });

      renderWithProviders();

      await user.click(screen.getByTestId('select-backend-platform'));
      await user.type(screen.getByTestId('app-name-input'), 'My Backend App');

      // NAME → create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(mockCreateApplication).toHaveBeenCalled();
      });

      // The wizard doesn't unmount in this test (navigate() is a no-op mock), so force a re-render
      // unrelated to the name field to pick up the refetched list, the same way any incidental
      // re-render during the real navigate() gap would.
      await user.click(screen.getByTestId('logo-select-btn'));

      expect(screen.queryByTestId('app-name-duplicate-error')).not.toBeInTheDocument();
    });
  });

  describe('Hosting URL / Application URL', () => {
    it('should include url in create request when hosting URL is provided', async () => {
      mockCreateApplication.mockImplementation((_data, {onSuccess}: {onSuccess: (app: Application) => void}) => {
        onSuccess({id: 'app-123', name: 'My App'} as Application);
      });

      renderWithProviders();

      await goToDesignStep();
      // DESIGN → CONFIGURE
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

      await goToDesignStep();
      // DESIGN → CONFIGURE
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
    afterEach(async () => {
      // These tests override the ConfigureSignInOptions mock implementation directly; restore the
      // default auto-resolving implementation so later tests aren't left with the override.
      const ConfigureSignInOptionsModule = await import(
        '../../components/create-application/configure-signin-options/ConfigureSignInOptions'
      );
      vi.mocked(ConfigureSignInOptionsModule.default).mockImplementation(DefaultConfigureSignInOptionsImpl);
    });

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

      // Navigate to the security (sign in options) step
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // At Security step
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });

      // Trigger setup
      await user.click(screen.getByTestId('setup-flow-generation'));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      });
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
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
      // DETAILS → SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));

      // Security step
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });

      // Trigger setup
      await user.click(screen.getByTestId('setup-flow-generation-error'));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY → DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
      });
      // DESIGN → CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      await waitFor(() => {
        expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
      });
      // CONFIGURE → Create
      await user.click(screen.getByTestId('application-wizard-next-button'));

      await waitFor(() => {
        expect(screen.getByText('Failed to generate the authentication flow. Please try again.')).toBeInTheDocument();
      });
    });

    describe('Rollback and reuse of the generated flow', () => {
      const CREATE_ERROR = /failed to create application/i;

      // Puts the wizard in the flow-generation state: no pre-selected flow, one integration on.
      const setupFlowGeneration = async (): Promise<void> => {
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
                <button type="button" data-testid="setup-flow-rollback" onClick={handleSetup}>
                  Setup Flow Generation
                </button>
                <button
                  type="button"
                  data-testid="clear-integrations"
                  onClick={() => {
                    // Leaves selectedAuthFlow alone, so the generated flow stays selected with no
                    // integrations on: the flow-only short-circuit.
                    setIntegrations({});
                    onReadyChange?.(true);
                  }}
                >
                  Clear Integrations
                </button>
              </div>
            );
          },
        );
      };

      // SECURITY → DESIGN → CONFIGURE → Create.
      const submitWizard = async (): Promise<void> => {
        await waitFor(() => {
          expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
        });
        await user.click(screen.getByRole('button', {name: /continue/i}));
        await waitFor(() => {
          expect(screen.getByTestId('application-configure-design')).toBeInTheDocument();
        });
        await user.click(screen.getByRole('button', {name: /continue/i}));
        await waitFor(() => {
          expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
        });
        // The step reports readiness a tick after it mounts, and the button is disabled until then.
        await waitFor(() => {
          expect(screen.getByTestId('application-wizard-next-button')).toBeEnabled();
        });
        await user.click(screen.getByTestId('application-wizard-next-button'));
      };

      const generateFlowAndSubmit = async (appName = 'My App'): Promise<void> => {
        await setupFlowGeneration();
        renderWithProviders();

        await user.type(screen.getByTestId('app-name-input'), appName);
        // DETAILS → SECURITY
        await user.click(screen.getByRole('button', {name: /continue/i}));
        await waitFor(() => {
          expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
        });
        await user.click(screen.getByTestId('setup-flow-rollback'));

        await submitWizard();
      };

      const mockFlowCreateSuccess = (): void => {
        let created = 0;
        mockCreateFlow.mockImplementation((_data: unknown, {onSuccess}: {onSuccess: (flow: unknown) => void}) => {
          created += 1;
          onSuccess({
            id: created === 1 ? 'generated-flow-id' : `generated-flow-id-${created}`,
            name: 'My App Sign-in Flow',
            handle: created === 1 ? 'my-app-sign-in-a1b2c3' : `my-app-sign-in-a1b2c3-${created}`,
          });
        });
      };

      // The failure is delivered on a later tick, as a real request would: the flow create that
      // precedes it commits its own render (which clears any stale error via the provider's form
      // fingerprint) before the create error is set, so the error stays on screen.
      const mockApplicationCreateFailure = (code?: string): void => {
        mockCreateApplication.mockImplementation((_data: unknown, {onError}: {onError: (err: Error) => void}) => {
          const err = new Error('Failed to create application');
          if (code) {
            (err as {response?: {data?: {code?: string}}}).response = {data: {code}};
          }
          setTimeout(() => onError(err), 0);
        });
      };

      it('should not start a second application create while the first one is in flight', async () => {
        mockFlowCreateSuccess();
        // Never settles, so the create stays in flight the way a slow request does.
        mockCreateApplication.mockImplementation(() => undefined);

        await generateFlowAndSubmit();

        await waitFor(() => {
          expect(mockCreateApplication).toHaveBeenCalledTimes(1);
        });

        // A second submit would reuse the memoized flow and send the same name again. Whichever
        // request then lost on the duplicate name would roll back the flow the winner is bound to,
        // leaving a created application pointing at a deleted flow.
        expect(screen.getByTestId('application-wizard-next-button')).toBeDisabled();
        fireEvent.click(screen.getByTestId('application-wizard-next-button'));

        expect(mockCreateApplication).toHaveBeenCalledTimes(1);
        expect(mockCreateFlow).toHaveBeenCalledTimes(1);
        expect(mockDeleteFlow).not.toHaveBeenCalled();
      });

      it('should delete the generated flow and keep the application error when the application create fails', async () => {
        mockFlowCreateSuccess();
        mockApplicationCreateFailure();

        await generateFlowAndSubmit();

        await waitFor(() => {
          expect(mockDeleteFlow).toHaveBeenCalledWith('generated-flow-id', expect.anything());
        });
        expect(screen.getByText(CREATE_ERROR)).toBeInTheDocument();
      });

      it('should reuse the generated flow on retry when the rollback failed', async () => {
        mockFlowCreateSuccess();
        mockApplicationCreateFailure();
        mockDeleteFlow.mockImplementation((_flowId: string, options?: {onError?: (err: Error) => void}) => {
          options?.onError?.(new Error('Failed to delete flow'));
        });

        await generateFlowAndSubmit();

        await waitFor(() => {
          expect(mockCreateApplication).toHaveBeenCalledTimes(1);
        });

        // Retry from the same step with the same inputs.
        await user.click(screen.getByTestId('application-wizard-next-button'));

        await waitFor(() => {
          expect(mockCreateApplication).toHaveBeenCalledTimes(2);
        });
        expect(mockCreateFlow).toHaveBeenCalledTimes(1);
        expect((mockCreateApplication.mock.calls[1][0] as Application).authFlowId).toBe('generated-flow-id');
      });

      it('should not reuse the flow while its rollback is still in flight', async () => {
        mockFlowCreateSuccess();
        mockApplicationCreateFailure();
        // The delete never settles, so a retry must not bind the application to a flow that is on
        // its way out.
        mockDeleteFlow.mockImplementation(() => undefined);

        await generateFlowAndSubmit();

        await waitFor(() => {
          expect(mockDeleteFlow).toHaveBeenCalledWith('generated-flow-id', expect.anything());
        });

        await user.click(screen.getByTestId('application-wizard-next-button'));

        await waitFor(() => {
          expect(mockCreateFlow).toHaveBeenCalledTimes(2);
        });
        expect((mockCreateApplication.mock.calls[1][0] as Application).authFlowId).toBe('generated-flow-id-2');
      });

      it('should generate a new flow on retry when the rollback succeeded', async () => {
        mockFlowCreateSuccess();
        mockApplicationCreateFailure();

        await generateFlowAndSubmit();

        await waitFor(() => {
          expect(mockDeleteFlow).toHaveBeenCalledWith('generated-flow-id', expect.anything());
        });

        await user.click(screen.getByTestId('application-wizard-next-button'));

        await waitFor(() => {
          expect(mockCreateFlow).toHaveBeenCalledTimes(2);
        });
        expect((mockCreateApplication.mock.calls[1][0] as Application).authFlowId).toBe('generated-flow-id-2');
      });

      it('should not submit a rolled back generated flow when the integrations are turned off', async () => {
        mockFlowCreateSuccess();
        mockApplicationCreateFailure('APP-1020');

        await generateFlowAndSubmit();

        await waitFor(() => {
          expect(mockDeleteFlow).toHaveBeenCalledWith('generated-flow-id', expect.anything());
        });

        // The duplicate-name failure sends the wizard back to the details step.
        await waitFor(() => {
          expect(screen.getByTestId('app-name-input')).toBeInTheDocument();
        });
        // The name error blocks Continue until the field is edited.
        await user.type(screen.getByTestId('app-name-input'), ' Renamed');
        await user.click(screen.getByRole('button', {name: /continue/i}));
        await waitFor(() => {
          expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
        });
        // selectedAuthFlow still points at the deleted flow; with no integrations left, the
        // flow-only short-circuit would otherwise submit that dead id.
        await user.click(screen.getByTestId('clear-integrations'));

        await submitWizard();

        // The dead id is filtered out, so the create is blocked by the missing-flow guard instead of
        // being submitted with a flow that no longer exists.
        await waitFor(() => {
          expect(screen.getByText('onboarding.configure.SignInOptions.noFlowFound')).toBeInTheDocument();
        });
        expect(mockCreateApplication).toHaveBeenCalledTimes(1);
        expect(mockCreateFlow).toHaveBeenCalledTimes(1);
      });

      it('should not delete a flow the wizard did not generate', async () => {
        mockApplicationCreateFailure();

        renderWithProviders();

        // The default sign-in step mock picks a pre-configured flow and clears integrations.
        await goToDesignStep();
        await user.click(screen.getByRole('button', {name: /continue/i}));
        await waitFor(() => {
          expect(screen.getByTestId('application-configure-details')).toBeInTheDocument();
        });
        await user.click(screen.getByTestId('application-wizard-next-button'));

        await waitFor(() => {
          expect(mockCreateApplication).toHaveBeenCalled();
        });
        expect(mockDeleteFlow).not.toHaveBeenCalled();
      });

      it('should delete the stale flow and generate a new one when an input changed between attempts', async () => {
        mockFlowCreateSuccess();
        mockApplicationCreateFailure('APP-1020');
        // The rollback fails, so the flow survives and the memo is kept. Editing an input then makes
        // that flow stale, and it must not be left behind when the next one is generated.
        mockDeleteFlow.mockImplementation((_flowId: string, options?: {onError?: (err: Error) => void}) => {
          options?.onError?.(new Error('Failed to delete flow'));
        });

        await generateFlowAndSubmit();

        // The duplicate-name failure sends the wizard back to the details step.
        await waitFor(() => {
          expect(screen.getByTestId('app-name-input')).toBeInTheDocument();
        });

        await user.type(screen.getByTestId('app-name-input'), ' Renamed');
        // DETAILS → SECURITY
        await user.click(screen.getByRole('button', {name: /continue/i}));
        await waitFor(() => {
          expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
        });
        await user.click(screen.getByTestId('setup-flow-rollback'));

        await submitWizard();

        await waitFor(() => {
          expect(mockCreateFlow).toHaveBeenCalledTimes(2);
        });
        expect(mockDeleteFlow).toHaveBeenCalledWith('generated-flow-id', expect.anything());
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

    it('should show the generic "Details" breadcrumb label for the mcp-client template', async () => {
      renderWithProviders();

      await selectMcpClientTemplate();

      expect(screen.getByText('Details')).toBeInTheDocument();
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

      expect(screen.getByText('Details')).toBeInTheDocument();
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
      expect(userDelegatedCard).toBeChecked();
      expect(m2mCard).not.toBeChecked();
    });

    it('should select the machine-to-machine card when clicked', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      const [userDelegatedCard, m2mCard] = screen.getAllByRole('radio');
      await user.click(m2mCard);

      expect(m2mCard).toBeChecked();
      expect(userDelegatedCard).not.toBeChecked();
    });

    it('should show the "Configuration" breadcrumb label for the mcp-client template', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      const breadcrumbNav = screen.getByRole('navigation');
      expect(within(breadcrumbNav).getByText('Configuration')).toBeInTheDocument();
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

    it('should fill the redirect URI input and enable Continue when the Inspector quick-add is clicked', async () => {
      renderWithProviders();

      await selectMcpClientTemplateAndName();

      await user.click(screen.getByText('Add it to redirect URIs'));

      expect(screen.getByDisplayValue('http://localhost:6274/oauth/callback')).toBeInTheDocument();
      expect(screen.getByTestId('application-wizard-next-button')).toBeEnabled();
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
      // The user-delegated MCP client resolves to the browser type.
      expect(requestBody.type).toBe('browser');

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

      // User attributes are not sent: the server derives them from the seeded scope-to-claims
      // mapping, intersected with the allowed user types' schemas.
      expect(requestBody.userAttributes).toBeUndefined();
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
      mockUserTypes.types = [{id: 'customer', name: 'customer', displayName: 'Customer'}];
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
      // The machine-to-machine MCP client override resolves to the m2m type.
      expect(requestBody.type).toBe('m2m');
      expect(requestBody.allowedUserTypes).toBeUndefined();

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

    it('should navigate directly to the created application for the user-delegated client, without a completion screen', async () => {
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
        expect(mockNavigate).toHaveBeenCalledWith('/applications/mcp-app-3');
      });

      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
    });

    it('should navigate directly to the created application for the machine-to-machine client, without a completion screen', async () => {
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
        expect(mockNavigate).toHaveBeenCalledWith('/applications/mcp-app-4');
      });

      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
    });
  });

  describe('Progress bar (visibleSteps-based)', () => {
    const getProgressValue = (): number => Number(screen.getByRole('progressbar').getAttribute('aria-valuenow'));

    it('increases monotonically as the user advances through a generic template flow (regression)', async () => {
      renderWithProviders();

      const nameProgress = getProgressValue();

      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS -> SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const securityProgress = getProgressValue();
      expect(securityProgress).toBeGreaterThan(nameProgress);

      await waitFor(() => {
        expect(screen.getByTestId('application-configure-sign-in')).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /continue/i})).toBeEnabled();
      });
      // SECURITY -> DESIGN
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const designProgress = getProgressValue();
      expect(designProgress).toBeGreaterThan(securityProgress);

      // DESIGN -> CONFIGURE
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const configureProgress = getProgressValue();
      expect(configureProgress).toBeGreaterThan(designProgress);
      expect(configureProgress).toBeLessThanOrEqual(100);
    });

    it('advances the progress from DETAILS to SECURITY for a non-mcp template when CONFIGURE is skipped (regression)', async () => {
      const getConfigurationTypeFromTemplate = await import('../../utils/getConfigurationTypeFromTemplate');
      vi.mocked(getConfigurationTypeFromTemplate.default).mockReturnValue('NONE');

      renderWithProviders();

      const nameProgress = getProgressValue();
      await user.type(screen.getByTestId('app-name-input'), 'My App');
      // DETAILS -> SECURITY
      await user.click(screen.getByRole('button', {name: /continue/i}));
      const securityProgress = getProgressValue();

      expect(securityProgress).toBeGreaterThan(nameProgress);
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
