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

import {render, screen} from '@thunderid/test-utils';
import React from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationCreateProvider from '../ApplicationCreateProvider';
import useApplicationCreate from '../useApplicationCreate';

// Mock useGetApplications
const mockUseGetApplications = vi.fn().mockReturnValue({
  data: {
    applications: [],
  },
});

vi.mock('../../api/useGetApplications', () => ({
  __esModule: true,
  default: mockUseGetApplications,
}));

// Mock generateAppPrimaryColorSuggestions
vi.mock('../../utils/generateAppPrimaryColorSuggestions', () => ({
  __esModule: true,
  default: () => ['#3B82F6'],
}));

// Mock useConfig to avoid ConfigProvider requirement
vi.mock('@thunderid/contexts', async (importOriginal) => {
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-type-assertion
  const actual = (await importOriginal()) as Record<string, unknown>;

  return {
    ...actual,
    useConfig: () => ({
      endpoints: {
        server: 'http://localhost:3001',
      },
    }),
  };
});

// Test component to consume the hook
function TestConsumer() {
  const context = useApplicationCreate();

  return <div data-testid="context-available">{typeof context}</div>;
}

// Test component without provider
function TestConsumerWithoutProvider() {
  const context = useApplicationCreate();

  return <div data-testid="context">{JSON.stringify(context)}</div>;
}

// Simple test wrapper that provides all necessary providers
function TestWrapper({children}: {children: React.ReactNode}) {
  return children;
}

describe('useApplicationCreate', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default mock: no applications
    mockUseGetApplications.mockReturnValue({
      data: {
        applications: [],
      },
    });
  });
  it('returns context when used within ApplicationCreateProvider', () => {
    render(
      <TestWrapper>
        <ApplicationCreateProvider>
          <TestConsumer />
        </ApplicationCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('context-available')).toHaveTextContent('object');
  });

  it('throws error when used outside ApplicationCreateProvider', () => {
    // Suppress error output in tests
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => null);

    expect(() => {
      render(<TestConsumerWithoutProvider />);
    }).toThrow('useApplicationCreate must be used within ApplicationCreateProvider');

    // Restore console.error
    errorSpy.mockRestore();
  });

  it('provides all required context properties', () => {
    function TestContextProperties() {
      const context = useApplicationCreate();

      const requiredProperties = [
        'currentStep',
        'setCurrentStep',
        'appName',
        'setAppName',
        'selectedTheme',
        'setSelectedTheme',
        'appLogo',
        'setAppLogo',
        'integrations',
        'setIntegrations',
        'toggleIntegration',
        'selectedAuthFlow',
        'setSelectedAuthFlow',
        'signInApproach',
        'setSignInApproach',
        'selectedTechnology',
        'setSelectedTechnology',
        'selectedPlatform',
        'setSelectedPlatform',
        'selectedTemplateConfig',
        'setSelectedTemplateConfig',
        'hostingUrl',
        'setHostingUrl',
        'callbackUrlFromConfig',
        'setCallbackUrlFromConfig',
        'hasCompletedOnboarding',
        'setHasCompletedOnboarding',
        'error',
        'setError',
        'reset',
      ];

      const missingProperties = requiredProperties.filter((prop) => !(prop in context));

      return (
        <div>
          <div data-testid="missing-properties">{JSON.stringify(missingProperties)}</div>
          <div data-testid="has-all-properties">{missingProperties.length === 0 ? 'true' : 'false'}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <ApplicationCreateProvider>
          <TestContextProperties />
        </ApplicationCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('has-all-properties')).toHaveTextContent('true');
    expect(screen.getByTestId('missing-properties')).toHaveTextContent('[]');
  });

  it('returns same context reference across multiple hook calls', () => {
    function TestMultipleHookCalls() {
      const context1 = useApplicationCreate();
      const context2 = useApplicationCreate();

      return (
        <div>
          <div data-testid="same-reference">{(context1 === context2).toString()}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <ApplicationCreateProvider>
          <TestMultipleHookCalls />
        </ApplicationCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('same-reference')).toHaveTextContent('true');
  });

  it('provides functions that are properly typed', () => {
    function TestFunctionTypes() {
      const {
        setCurrentStep,
        setAppName,
        toggleIntegration,
        reset,
        setSelectedTheme,
        setAppLogo,
        setIntegrations,
        setSelectedAuthFlow,
        setSignInApproach,
        setSelectedTechnology,
        setSelectedPlatform,
        setSelectedTemplateConfig,
        setHostingUrl,
        setCallbackUrlFromConfig,
        setHasCompletedOnboarding,
        setError,
      } = useApplicationCreate();

      return (
        <div>
          <div data-testid="setCurrentStep-type">{typeof setCurrentStep}</div>
          <div data-testid="setAppName-type">{typeof setAppName}</div>
          <div data-testid="toggleIntegration-type">{typeof toggleIntegration}</div>
          <div data-testid="reset-type">{typeof reset}</div>
          <div data-testid="setSelectedTheme-type">{typeof setSelectedTheme}</div>
          <div data-testid="setAppLogo-type">{typeof setAppLogo}</div>
          <div data-testid="setIntegrations-type">{typeof setIntegrations}</div>
          <div data-testid="setSelectedAuthFlow-type">{typeof setSelectedAuthFlow}</div>
          <div data-testid="setSignInApproach-type">{typeof setSignInApproach}</div>
          <div data-testid="setSelectedTechnology-type">{typeof setSelectedTechnology}</div>
          <div data-testid="setSelectedPlatform-type">{typeof setSelectedPlatform}</div>
          <div data-testid="setSelectedTemplateConfig-type">{typeof setSelectedTemplateConfig}</div>
          <div data-testid="setHostingUrl-type">{typeof setHostingUrl}</div>
          <div data-testid="setCallbackUrlFromConfig-type">{typeof setCallbackUrlFromConfig}</div>
          <div data-testid="setHasCompletedOnboarding-type">{typeof setHasCompletedOnboarding}</div>
          <div data-testid="setError-type">{typeof setError}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <ApplicationCreateProvider>
          <TestFunctionTypes />
        </ApplicationCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('setCurrentStep-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setAppName-type')).toHaveTextContent('function');
    expect(screen.getByTestId('toggleIntegration-type')).toHaveTextContent('function');
    expect(screen.getByTestId('reset-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setSelectedTheme-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setAppLogo-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setIntegrations-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setSelectedAuthFlow-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setSignInApproach-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setSelectedTechnology-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setSelectedPlatform-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setSelectedTemplateConfig-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setHostingUrl-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setCallbackUrlFromConfig-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setHasCompletedOnboarding-type')).toHaveTextContent('function');
    expect(screen.getByTestId('setError-type')).toHaveTextContent('function');
  });

  it('throws descriptive error message when used outside provider', () => {
    // Suppress error output in tests
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => null);

    let thrownError: Error | null = null;

    try {
      render(<TestConsumerWithoutProvider />);
    } catch (error) {
      thrownError = error as Error;
    }

    expect(thrownError).toBeInstanceOf(Error);
    expect(thrownError?.message).toBe('useApplicationCreate must be used within ApplicationCreateProvider');

    // Restore console.error
    errorSpy.mockRestore();
  });

  it('has exactly 44 properties in the context interface', () => {
    function TestContextProperties() {
      const context = useApplicationCreate();

      return (
        <div>
          <div data-testid="property-count">{Object.keys(context).length}</div>
        </div>
      );
    }

    render(
      <TestWrapper>
        <ApplicationCreateProvider>
          <TestContextProperties />
        </ApplicationCreateProvider>
      </TestWrapper>,
    );

    expect(screen.getByTestId('property-count')).toHaveTextContent('44');
  });
});
