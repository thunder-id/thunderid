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

import {renderHook} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../contexts/ApplicationCreate/ApplicationCreateContext';
import {ApplicationCreateFlowStep, ApplicationCreateFlowSignInApproach} from '../../models/application-create-flow';
import useApplicationCreateContext from '../useApplicationCreateContext';

describe('useApplicationCreateContext', () => {
  const mockContextValue: ApplicationCreateContextType = {
    relyingPartyId: '',
    setRelyingPartyId: vi.fn(),
    relyingPartyName: '',
    setRelyingPartyName: vi.fn(),
    currentStep: ApplicationCreateFlowStep.NAME,
    setCurrentStep: vi.fn(),
    appName: 'Test App',
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
    signInApproach: ApplicationCreateFlowSignInApproach.INBUILT,
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

    expect(result.current.currentStep).toBe(ApplicationCreateFlowStep.NAME);
    expect(result.current.appName).toBe('Test App');
    expect(result.current.selectedTheme).toBeNull();
    expect(result.current.signInApproach).toBe(ApplicationCreateFlowSignInApproach.INBUILT);
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
      ApplicationCreateFlowStep.NAME,
      ApplicationCreateFlowStep.DESIGN,
      ApplicationCreateFlowStep.OPTIONS,
      ApplicationCreateFlowStep.EXPERIENCE,
      ApplicationCreateFlowStep.STACK,
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
    const approaches = [ApplicationCreateFlowSignInApproach.INBUILT, ApplicationCreateFlowSignInApproach.EMBEDDED];

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
