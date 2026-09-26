// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {ApplicationCreateFlowStep, ApplicationCreateFlowSignInApproach} from '@thunderid/configure-applications';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../contexts/ApplicationCreate/ApplicationCreateContext';
import useApplicationCreateContext from '../useApplicationCreateContext';

describe('useApplicationCreateContext', () => {
  const mockContextValue: ApplicationCreateContextType = {
    relyingPartyId: '',
    setRelyingPartyId: vi.fn(),
    relyingPartyName: '',
    setRelyingPartyName: vi.fn(),
    currentStep: ApplicationCreateFlowStep.DETAILS,
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
    signInApproach: ApplicationCreateFlowSignInApproach.REDIRECT_BASED,
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
  };

  const createWrapper = (contextValue: ApplicationCreateContextType | undefined) => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ApplicationCreateContext.Provider value={contextValue}>{children}</ApplicationCreateContext.Provider>;
    }
    return Wrapper;
  };

  it('should return context values when used within provider', () => {
    const {result} = renderHook(() => useApplicationCreateContext(), {
      wrapper: createWrapper(mockContextValue),
    });

    expect(result.current.currentStep).toBe(ApplicationCreateFlowStep.DETAILS);
    expect(result.current.appName).toBe('Test App');
    expect(result.current.selectedTheme).toBeNull();
    expect(result.current.signInApproach).toBe(ApplicationCreateFlowSignInApproach.REDIRECT_BASED);
  });

  it('should throw an error when used outside of ApplicationCreateProvider', () => {
    expect(() => {
      renderHook(() => useApplicationCreateContext(), {
        wrapper: createWrapper(undefined),
      });
    }).toThrow('useApplicationCreateContext must be used within an ApplicationCreateProvider');
  });

  it('should return setter functions', () => {
    const {result} = renderHook(() => useApplicationCreateContext(), {
      wrapper: createWrapper(mockContextValue),
    });

    expect(typeof result.current.setCurrentStep).toBe('function');
    expect(typeof result.current.setAppName).toBe('function');
    expect(typeof result.current.setThemeId).toBe('function');
    expect(typeof result.current.setSelectedTheme).toBe('function');
    expect(typeof result.current.setAppLogo).toBe('function');
    expect(typeof result.current.setIntegrations).toBe('function');
    expect(typeof result.current.toggleIntegration).toBe('function');
    expect(typeof result.current.reset).toBe('function');
  });

  it('should return app configuration values', () => {
    const contextWithValues: ApplicationCreateContextType = {
      ...mockContextValue,
      appName: 'My Application',
      themeId: null,
      selectedTheme: null,
      appLogo: 'https://example.com/logo.png',
      hostingUrl: 'https://myapp.com',
      callbackUrlFromConfig: 'https://myapp.com/callback',
    };

    const {result} = renderHook(() => useApplicationCreateContext(), {
      wrapper: createWrapper(contextWithValues),
    });

    expect(result.current.appName).toBe('My Application');
    expect(result.current.themeId).toBeNull();
    expect(result.current.selectedTheme).toBeNull();
    expect(result.current.appLogo).toBe('https://example.com/logo.png');
    expect(result.current.hostingUrl).toBe('https://myapp.com');
    expect(result.current.callbackUrlFromConfig).toBe('https://myapp.com/callback');
  });

  it('should return integrations state', () => {
    const contextWithIntegrations: ApplicationCreateContextType = {
      ...mockContextValue,
      integrations: {
        google: true,
        github: false,
        microsoft: true,
      },
    };

    const {result} = renderHook(() => useApplicationCreateContext(), {
      wrapper: createWrapper(contextWithIntegrations),
    });

    expect(result.current.integrations.google).toBe(true);
    expect(result.current.integrations.github).toBe(false);
    expect(result.current.integrations.microsoft).toBe(true);
  });

  it('should return onboarding state', () => {
    const contextWithOnboarding: ApplicationCreateContextType = {
      ...mockContextValue,
      hasCompletedOnboarding: true,
    };

    const {result} = renderHook(() => useApplicationCreateContext(), {
      wrapper: createWrapper(contextWithOnboarding),
    });

    expect(result.current.hasCompletedOnboarding).toBe(true);
  });

  it('should return error state', () => {
    const contextWithError: ApplicationCreateContextType = {
      ...mockContextValue,
      error: 'Something went wrong',
    };

    const {result} = renderHook(() => useApplicationCreateContext(), {
      wrapper: createWrapper(contextWithError),
    });

    expect(result.current.error).toBe('Something went wrong');
  });

  it('should return different flow steps', () => {
    const steps = [
      ApplicationCreateFlowStep.DETAILS,
      ApplicationCreateFlowStep.DESIGN,
      ApplicationCreateFlowStep.SECURITY,
      ApplicationCreateFlowStep.ORGANIZATION_UNIT,
      ApplicationCreateFlowStep.CONFIGURE,
    ];

    steps.forEach((step) => {
      const contextWithStep: ApplicationCreateContextType = {
        ...mockContextValue,
        currentStep: step,
      };

      const {result} = renderHook(() => useApplicationCreateContext(), {
        wrapper: createWrapper(contextWithStep),
      });

      expect(result.current.currentStep).toBe(step);
    });
  });

  it('should return sign-in approach values', () => {
    const approaches = [
      ApplicationCreateFlowSignInApproach.REDIRECT_BASED,
      ApplicationCreateFlowSignInApproach.EMBEDDED,
    ];

    approaches.forEach((approach) => {
      const contextWithApproach: ApplicationCreateContextType = {
        ...mockContextValue,
        signInApproach: approach,
      };

      const {result} = renderHook(() => useApplicationCreateContext(), {
        wrapper: createWrapper(contextWithApproach),
      });

      expect(result.current.signInApproach).toBe(approach);
    });
  });
});
