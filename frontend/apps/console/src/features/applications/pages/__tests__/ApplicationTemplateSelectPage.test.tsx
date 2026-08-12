// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {fireEvent, render, screen} from '@thunderid/test-utils';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../contexts/ApplicationCreate/ApplicationCreateContext';
import {ApplicationCreateFlowStep} from '../../models/application-create-flow';
import {PlatformApplicationTemplate, TechnologyApplicationTemplate} from '../../models/application-templates';
import ApplicationTemplateSelectPage from '../ApplicationTemplateSelectPage';

const mockNavigate = vi.fn(() => Promise.resolve());
const mockLoggerError = vi.fn();
let mockPathname = '/applications/types';
let mockSearchParams = new URLSearchParams();

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({info: vi.fn(), warn: vi.fn(), error: mockLoggerError, debug: vi.fn(), withComponent: vi.fn()}),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({pathname: mockPathname}),
    useSearchParams: () => [mockSearchParams, vi.fn()],
    Link: ({to, children = null}: {to: string; children?: ReactNode}) => <a href={to}>{children}</a>,
  };
});

const buildContext = (overrides: Partial<ApplicationCreateContextType> = {}): ApplicationCreateContextType =>
  ({
    currentStep: ApplicationCreateFlowStep.DETAILS,
    setCurrentStep: vi.fn(),
    appName: '',
    setAppName: vi.fn(),
    ouId: '',
    setOuId: vi.fn(),
    themeId: null,
    setThemeId: vi.fn(),
    selectedTheme: null,
    setSelectedTheme: vi.fn(),
    appLogo: null,
    setAppLogo: vi.fn(),
    selectedColor: '',
    setSelectedColor: vi.fn(),
    integrations: {},
    setIntegrations: vi.fn(),
    toggleIntegration: vi.fn(),
    selectedAuthFlow: null,
    setSelectedAuthFlow: vi.fn(),
    signInApproach: null as unknown as ApplicationCreateContextType['signInApproach'],
    setSignInApproach: vi.fn(),
    selectedTechnology: null,
    setSelectedTechnology: vi.fn(),
    selectedPlatform: null,
    setSelectedPlatform: vi.fn(),
    selectedTemplateConfig: null,
    setSelectedTemplateConfig: vi.fn(),
    mcpClientType: 'userDelegated',
    setMcpClientType: vi.fn(),
    mcpRedirectUris: [],
    setMcpRedirectUris: vi.fn(),
    hostingUrl: '',
    setHostingUrl: vi.fn(),
    callbackUrlFromConfig: '',
    setCallbackUrlFromConfig: vi.fn(),
    hasCompletedOnboarding: false,
    setHasCompletedOnboarding: vi.fn(),
    error: null,
    setError: vi.fn(),
    reset: vi.fn(),
    relyingPartyId: '',
    setRelyingPartyId: vi.fn(),
    relyingPartyName: '',
    setRelyingPartyName: vi.fn(),
    ...overrides,
  }) as ApplicationCreateContextType;

const renderPage = (overrides: Partial<ApplicationCreateContextType> = {}) =>
  render(
    <ApplicationCreateContext.Provider value={buildContext(overrides)}>
      <ApplicationTemplateSelectPage />
    </ApplicationCreateContext.Provider>,
  );

describe('ApplicationTemplateSelectPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPathname = '/applications/types';
    mockSearchParams = new URLSearchParams();
  });

  it('renders the heading and all templates', () => {
    renderPage();

    expect(screen.getByRole('heading', {name: 'Choose a type'})).toBeInTheDocument();
    expect(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`)).toBeInTheDocument();
    expect(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.MCP_CLIENT}`)).toBeInTheDocument();
    expect(screen.getByTestId(`template-card-${PlatformApplicationTemplate.BACKEND}`)).toBeInTheDocument();
  });

  it('filters templates by category', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByText('AI'));

    expect(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.MCP_CLIENT}`)).toBeInTheDocument();
    expect(screen.queryByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`)).not.toBeInTheDocument();
  });

  it('seeds a technology template and launches the wizard when a card is clicked', async () => {
    const user = userEvent.setup();
    const setSelectedTechnology = vi.fn();
    const setSelectedPlatform = vi.fn();
    const setSelectedTemplateConfig = vi.fn();
    const setCurrentStep = vi.fn();

    renderPage({setSelectedTechnology, setSelectedPlatform, setSelectedTemplateConfig, setCurrentStep});

    await user.click(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`));

    expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
    expect(setSelectedPlatform).toHaveBeenCalledWith(null);
    expect(setSelectedTemplateConfig).toHaveBeenCalled();
    // React uses the default flow whose first step is ORGANIZATION_UNIT.
    expect(setCurrentStep).toHaveBeenCalledWith(ApplicationCreateFlowStep.ORGANIZATION_UNIT);
    expect(mockNavigate).toHaveBeenCalledWith(`/applications/create?type=${TechnologyApplicationTemplate.REACT}`, {
      replace: false,
    });
  });

  it('seeds a platform template and clears technology when a platform card is clicked', async () => {
    const user = userEvent.setup();
    const setSelectedTechnology = vi.fn();
    const setSelectedPlatform = vi.fn();

    renderPage({setSelectedTechnology, setSelectedPlatform});

    await user.click(screen.getByTestId(`template-card-${PlatformApplicationTemplate.BACKEND}`));

    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BACKEND);
    expect(setSelectedTechnology).toHaveBeenCalledWith(null);
    expect(mockNavigate).toHaveBeenCalledWith(`/applications/create?type=${PlatformApplicationTemplate.BACKEND}`, {
      replace: false,
    });
  });

  it('advances to ORGANIZATION_UNIT first for the MCP client template', async () => {
    const user = userEvent.setup();
    const setCurrentStep = vi.fn();

    renderPage({setCurrentStep});

    await user.click(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.MCP_CLIENT}`));

    // mcp-client flow: ORGANIZATION_UNIT → DETAILS → CLIENT_TYPE → COMPLETE; first step is ORGANIZATION_UNIT.
    expect(setCurrentStep).toHaveBeenCalledWith(ApplicationCreateFlowStep.ORGANIZATION_UNIT);
    expect(mockNavigate).toHaveBeenCalledWith(`/applications/create?type=${TechnologyApplicationTemplate.MCP_CLIENT}`, {
      replace: false,
    });
  });

  it('navigates back to the Get Started page when in the welcome flow', () => {
    mockPathname = '/welcome/get-started/applications/types';
    renderPage();

    expect(screen.getByRole('link')).toHaveAttribute('href', '/welcome/get-started');
  });

  it('launches the wizard under the welcome flow prefix when in the welcome flow', async () => {
    const user = userEvent.setup();
    mockPathname = '/welcome/get-started/applications/types';

    renderPage();

    await user.click(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`));

    expect(mockNavigate).toHaveBeenCalledWith(
      `/welcome/get-started/applications/create?type=${TechnologyApplicationTemplate.REACT}`,
      {replace: false},
    );
  });

  it('navigates back to the applications list outside the welcome flow', () => {
    renderPage();

    expect(screen.getByRole('link')).toHaveAttribute('href', '/applications');
  });

  it('selects a template when Enter is pressed on a card', () => {
    const setSelectedTechnology = vi.fn();

    renderPage({setSelectedTechnology});

    fireEvent.keyDown(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`), {key: 'Enter'});

    expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
    expect(mockNavigate).toHaveBeenCalledWith(`/applications/create?type=${TechnologyApplicationTemplate.REACT}`, {
      replace: false,
    });
  });

  it('selects a template when Space is pressed on a card', () => {
    const setSelectedTechnology = vi.fn();

    renderPage({setSelectedTechnology});

    fireEvent.keyDown(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`), {key: ' '});

    expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
    expect(mockNavigate).toHaveBeenCalledWith(`/applications/create?type=${TechnologyApplicationTemplate.REACT}`, {
      replace: false,
    });
  });

  it('ignores keys other than Enter and Space', () => {
    const setSelectedTechnology = vi.fn();

    renderPage({setSelectedTechnology});

    fireEvent.keyDown(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`), {key: 'Tab'});

    expect(setSelectedTechnology).not.toHaveBeenCalled();
  });

  it('auto-selects the template and launches the wizard when a type query param is present', async () => {
    const setSelectedTechnology = vi.fn();
    mockSearchParams = new URLSearchParams({type: TechnologyApplicationTemplate.REACT});

    renderPage({setSelectedTechnology});

    await vi.waitFor(() => {
      expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
    });
    // Replaces the pass-through gallery entry so going back from the wizard reaches the entry
    // point rather than bouncing forward again.
    expect(mockNavigate).toHaveBeenCalledWith(`/applications/create?type=${TechnologyApplicationTemplate.REACT}`, {
      replace: true,
    });
  });

  it('does not render the template gallery while passing through on a deep link', () => {
    mockSearchParams = new URLSearchParams({type: TechnologyApplicationTemplate.REACT});

    renderPage();

    expect(screen.queryByRole('heading', {name: 'Choose a type'})).not.toBeInTheDocument();
    expect(screen.queryByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`)).not.toBeInTheDocument();
  });

  it('ignores an unknown type query param', () => {
    const setSelectedTechnology = vi.fn();
    mockSearchParams = new URLSearchParams({type: 'not-a-real-type'});

    renderPage({setSelectedTechnology});

    expect(setSelectedTechnology).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('logs an error when navigating to the wizard fails', async () => {
    const user = userEvent.setup();
    mockNavigate.mockRejectedValueOnce(new Error('navigation failed'));

    renderPage();

    await user.click(screen.getByTestId(`template-card-${TechnologyApplicationTemplate.REACT}`));

    await vi.waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith('Failed to navigate to application creation wizard', {
        error: new Error('navigation failed'),
        template: TechnologyApplicationTemplate.REACT,
      });
    });
  });
});
