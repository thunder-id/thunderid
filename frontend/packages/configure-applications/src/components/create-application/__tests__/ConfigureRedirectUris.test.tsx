// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {AllowedOriginTypes, createRow} from '@thunderid/configure-settings';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../../contexts/ApplicationCreate/ApplicationCreateContext';
import type {ApplicationTemplate} from '../../../models/application-templates';
import ConfigureRedirectUris from '../ConfigureRedirectUris';

let translationLookup = (key: string): string => key;

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => translationLookup(key),
  }),
}));

const renderWithContext = (contextOverrides: Partial<ApplicationCreateContextType> = {}) => {
  const baseContext: ApplicationCreateContextType = {
    currentStep: null as unknown as ApplicationCreateContextType['currentStep'],
    setCurrentStep: vi.fn(),
    appName: 'Test App',
    setAppName: vi.fn(),
    ouId: '',
    setOuId: vi.fn(),
    themeId: null,
    setThemeId: vi.fn(),
    selectedTheme: null,
    setSelectedTheme: vi.fn(),
    layoutId: null,
    setLayoutId: vi.fn(),
    selectedLayout: null,
    setSelectedLayout: vi.fn(),
    appLogo: null,
    setAppLogo: vi.fn(),
    selectedColor: '',
    setSelectedColor: vi.fn(),
    integrations: {},
    setIntegrations: vi.fn(),
    toggleIntegration: vi.fn(),
    isEmailOtpMfaEnabled: false,
    setIsEmailOtpMfaEnabled: vi.fn(),
    isSmsOtpMfaEnabled: false,
    setIsSmsOtpMfaEnabled: vi.fn(),
    smsOtpSenderId: '',
    setSmsOtpSenderId: vi.fn(),
    selectedAuthFlow: null,
    setSelectedAuthFlow: vi.fn(),
    signInApproach: null as unknown as ApplicationCreateContextType['signInApproach'],
    setSignInApproach: vi.fn(),
    registrationFlowId: null,
    setRegistrationFlowId: vi.fn(),
    isRegistrationFlowEnabled: false,
    setIsRegistrationFlowEnabled: vi.fn(),
    recoveryFlowId: null,
    setRecoveryFlowId: vi.fn(),
    isRecoveryFlowEnabled: false,
    setIsRecoveryFlowEnabled: vi.fn(),
    signOutFlowId: null,
    setSignOutFlowId: vi.fn(),
    isSignOutFlowEnabled: false,
    setIsSignOutFlowEnabled: vi.fn(),
    redirectUris: [],
    setRedirectUris: vi.fn(),
    postLogoutRedirectUris: [],
    setPostLogoutRedirectUris: vi.fn(),
    corsOrigins: [],
    setCorsOrigins: vi.fn(),
    ouDefaults: {signIn: false, signUp: false, recovery: false, signOut: false, theme: false, layout: false},
    setOuDefaults: vi.fn(),
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
    ...contextOverrides,
  };

  return render(
    <ApplicationCreateContext.Provider value={baseContext}>
      <ConfigureRedirectUris />
    </ApplicationCreateContext.Provider>,
  );
};

const reactTemplate: ApplicationTemplate = {
  id: 'react',
  capabilities: {cors: true},
  devServer: {id: 'vite', label: 'Vite', url: 'http://localhost:5173'},
};

const nextJsTemplate: ApplicationTemplate = {
  id: 'nextjs',
  devServer: {id: 'nextjs', label: 'Next.js', url: 'http://localhost:3000'},
};

const expressTemplate: ApplicationTemplate = {
  id: 'express',
};

describe('ConfigureRedirectUris', () => {
  let user: ReturnType<typeof userEvent.setup>;

  beforeEach(() => {
    vi.clearAllMocks();
    translationLookup = (key: string): string => key;
    user = userEvent.setup();
  });

  it('does not render a dev server banner or CORS section for a template without either', () => {
    renderWithContext({selectedTemplateConfig: expressTemplate});

    expect(screen.queryByTestId('application-dev-server-banner')).not.toBeInTheDocument();
    expect(screen.queryByText('applications:onboarding.configure.details.corsOrigins.title')).not.toBeInTheDocument();
  });

  it('renders the dev server banner with the "redirect URIs & CORS origins" copy for a CORS-applicable template', () => {
    renderWithContext({selectedTemplateConfig: reactTemplate});

    expect(screen.getByTestId('application-dev-server-banner')).toBeInTheDocument();
    expect(
      screen.getByText('applications:onboarding.configure.details.devServer.addToRedirectAndCors'),
    ).toBeInTheDocument();
  });

  it('renders the dev server banner with the "redirect URIs" only copy for a non-CORS template', () => {
    renderWithContext({selectedTemplateConfig: nextJsTemplate});

    expect(screen.getByTestId('application-dev-server-banner')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.details.devServer.addToRedirect')).toBeInTheDocument();
    expect(
      screen.queryByText('applications:onboarding.configure.details.devServer.addToRedirectAndCors'),
    ).not.toBeInTheDocument();
  });

  it('renders the CORS Allowed Origins section only when the template capability is enabled', () => {
    renderWithContext({selectedTemplateConfig: reactTemplate});

    expect(screen.getByText('applications:onboarding.configure.details.corsOrigins.title')).toBeInTheDocument();
  });

  it("adds the dev server's URL to both redirect URIs and CORS origins on quick-add for a CORS-applicable template", async () => {
    const setRedirectUris = vi.fn();
    const setCorsOrigins = vi.fn();

    renderWithContext({
      selectedTemplateConfig: reactTemplate,
      redirectUris: [],
      setRedirectUris,
      corsOrigins: [],
      setCorsOrigins,
    });

    await user.click(screen.getByText('applications:onboarding.configure.details.devServer.addToRedirectAndCors'));

    expect(setRedirectUris).toHaveBeenCalledWith(['http://localhost:5173']);
    // Quick-add always produces an exact origin, never a pattern.
    expect(setCorsOrigins).toHaveBeenCalledWith([
      expect.objectContaining({type: AllowedOriginTypes.ORIGIN, value: 'http://localhost:5173'}),
    ]);
  });

  it('only adds the dev server URL to redirect URIs, not CORS origins, for a non-CORS template', async () => {
    const setRedirectUris = vi.fn();
    const setCorsOrigins = vi.fn();

    renderWithContext({
      selectedTemplateConfig: nextJsTemplate,
      redirectUris: [],
      setRedirectUris,
      corsOrigins: [],
      setCorsOrigins,
    });

    await user.click(screen.getByText('applications:onboarding.configure.details.devServer.addToRedirect'));

    expect(setRedirectUris).toHaveBeenCalledWith(['http://localhost:3000']);
    expect(setCorsOrigins).not.toHaveBeenCalled();
  });

  it('does not duplicate the dev server URL when it is already present', async () => {
    const setRedirectUris = vi.fn();
    const setCorsOrigins = vi.fn();

    renderWithContext({
      selectedTemplateConfig: reactTemplate,
      redirectUris: ['http://localhost:5173'],
      setRedirectUris,
      corsOrigins: [createRow(AllowedOriginTypes.ORIGIN, 'http://localhost:5173')],
      setCorsOrigins,
    });

    await user.click(screen.getByText('applications:onboarding.configure.details.devServer.addToRedirectAndCors'));

    expect(setRedirectUris).not.toHaveBeenCalled();
    expect(setCorsOrigins).not.toHaveBeenCalled();
  });

  it('defaults to reusing the redirect URIs for post-logout redirect and hides the separate editor', () => {
    const setPostLogoutRedirectUris = vi.fn();

    renderWithContext({
      selectedTemplateConfig: expressTemplate,
      redirectUris: ['https://example.com/callback'],
      setPostLogoutRedirectUris,
    });

    expect(screen.getByRole('checkbox')).toBeChecked();
    expect(screen.queryByText('applications:edit.general.postLogoutRedirectUris.title')).not.toBeInTheDocument();
    expect(setPostLogoutRedirectUris).toHaveBeenCalledWith(['https://example.com/callback']);
  });

  it('reveals the Post-Logout Redirect URIs editor once unchecked, and stops mirroring redirect URIs', async () => {
    const setPostLogoutRedirectUris = vi.fn();

    renderWithContext({
      selectedTemplateConfig: expressTemplate,
      redirectUris: ['https://example.com/callback'],
      setPostLogoutRedirectUris,
    });

    await user.click(screen.getByRole('checkbox'));

    expect(screen.getByText('applications:edit.general.postLogoutRedirectUris.title')).toBeInTheDocument();

    setPostLogoutRedirectUris.mockClear();
    await user.click(screen.getByRole('button', {name: 'applications:edit.general.postLogoutRedirectUris.addUri'}));

    // Once unchecked, the editor operates on its own list rather than being overwritten to mirror
    // the redirect URIs (the sync effect only runs while useSameAsRedirect is true). The list
    // was empty, so it was showing one placeholder row; "Add URI" appends relative to what's on
    // screen, landing on two empty rows rather than one (which would look like a no-op click).
    expect(setPostLogoutRedirectUris).toHaveBeenCalledWith(['', '']);
  });

  it('adds a second visible row on the first "Add URI" click when the list starts empty', async () => {
    const setRedirectUris = vi.fn();

    renderWithContext({
      selectedTemplateConfig: expressTemplate,
      redirectUris: [],
      setRedirectUris,
    });

    // With no redirect URIs yet, a single placeholder row is shown even though the real list is
    // empty. Clicking "Add URI" must land on two rows, not one — landing on one would render
    // identically to the placeholder already on screen and look like the click did nothing.
    await user.click(screen.getByRole('button', {name: 'applications:edit.general.redirectUris.addUri'}));

    expect(setRedirectUris).toHaveBeenCalledWith(['', '']);
  });

  it('flags an invalid CORS origin (a path is not a bare origin) instead of accepting it as a pattern', async () => {
    const setCorsOrigins = vi.fn();

    renderWithContext({
      selectedTemplateConfig: reactTemplate,
      corsOrigins: [createRow(AllowedOriginTypes.ORIGIN, 'https://example.com/some/path')],
      setCorsOrigins,
    });

    const corsInput = screen.getByPlaceholderText('applications:onboarding.configure.details.corsOrigins.placeholder');
    await user.click(corsInput);
    await user.tab();

    expect(await screen.findByText('settings:cors.validation.invalidOrigin')).toBeInTheDocument();
  });
});
