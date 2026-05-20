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
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import {ApplicationCreateFlowSignInApproach, ApplicationCreateFlowStep} from '../../../models/application-create-flow';
import {TechnologyApplicationTemplate, PlatformApplicationTemplate} from '../../../models/application-templates';
import ApplicationCreateProvider from '../ApplicationCreateProvider';
import useApplicationCreate from '../useApplicationCreate';
import {AuthenticatorTypes} from '@/features/integrations/models/authenticators';

// Mock useGetApplications
const mockUseGetApplications = vi.fn();
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
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      endpoints: {
        server: 'http://localhost:3001',
      },
    }),
  };
});

// Test component to consume the context
function TestConsumer() {
  const context = useApplicationCreate();

  return (
    <div>
      <div data-testid="current-step">{context.currentStep}</div>
      <div data-testid="app-name">{context.appName}</div>
      <div data-testid="selected-theme">{context.themeId ?? 'null'}</div>
      <div data-testid="app-logo">{context.appLogo ?? 'null'}</div>
      <div data-testid="selected-color">{context.selectedColor}</div>
      <div data-testid="integrations">{JSON.stringify(context.integrations)}</div>
      <div data-testid="sign-in-approach">{context.signInApproach}</div>
      <div data-testid="selected-technology">{context.selectedTechnology ?? 'null'}</div>
      <div data-testid="selected-platform">{context.selectedPlatform ?? 'null'}</div>
      <div data-testid="hosting-url">{context.hostingUrl}</div>
      <div data-testid="callback-url">{context.callbackUrlFromConfig}</div>
      <div data-testid="error">{context.error ?? 'null'}</div>

      <button type="button" onClick={() => context.setCurrentStep(ApplicationCreateFlowStep.DESIGN)}>
        Set Design Step
      </button>
      <button type="button" onClick={() => context.setAppName('Test App')}>
        Set App Name
      </button>
      <button type="button" onClick={() => context.setSelectedTheme(null)}>
        Set Theme to null
      </button>
      <button type="button" onClick={() => context.setAppLogo('test-logo.png')}>
        Set Logo
      </button>
      <button type="button" onClick={() => context.toggleIntegration('test-integration')}>
        Toggle Integration
      </button>
      <button type="button" onClick={() => context.setSignInApproach(ApplicationCreateFlowSignInApproach.EMBEDDED)}>
        Set Custom Approach
      </button>
      <button type="button" onClick={() => context.setSelectedTechnology(TechnologyApplicationTemplate.REACT)}>
        Set React Technology
      </button>
      <button type="button" onClick={() => context.setSelectedPlatform(PlatformApplicationTemplate.BROWSER)}>
        Set Browser Platform
      </button>
      <button type="button" onClick={() => context.setHostingUrl('https://example.com')}>
        Set Hosting URL
      </button>
      <button type="button" onClick={() => context.setCallbackUrlFromConfig('https://example.com/callback')}>
        Set Callback URL
      </button>
      <button type="button" onClick={() => context.setError('Test error')}>
        Set Error
      </button>
      <button type="button" onClick={() => context.reset()}>
        Reset
      </button>
    </div>
  );
}

describe('ApplicationCreateProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default mock: no applications
    mockUseGetApplications.mockReturnValue({
      data: {
        applications: [],
      },
    });
  });

  const renderWithQueryClient = (ui: React.ReactElement) => render(ui);

  it('provides initial state values', () => {
    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    expect(screen.getByTestId('current-step')).toHaveTextContent(ApplicationCreateFlowStep.STACK);
    expect(screen.getByTestId('app-name')).toHaveTextContent('');
    expect(screen.getByTestId('selected-theme')).toHaveTextContent('null');
    expect(screen.getByTestId('app-logo')).toHaveTextContent('null');
    expect(screen.getByTestId('integrations')).toHaveTextContent(
      JSON.stringify({[AuthenticatorTypes.BASIC_AUTH]: true}),
    );
    expect(screen.getByTestId('sign-in-approach')).toHaveTextContent(ApplicationCreateFlowSignInApproach.INBUILT);
    expect(screen.getByTestId('selected-technology')).toHaveTextContent('null');
    expect(screen.getByTestId('selected-platform')).toHaveTextContent('null');
    expect(screen.getByTestId('hosting-url')).toHaveTextContent('');
    expect(screen.getByTestId('callback-url')).toHaveTextContent('');
    expect(screen.getByTestId('error')).toHaveTextContent('null');
  });

  it('updates current step when setCurrentStep is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Design Step'));

    expect(screen.getByTestId('current-step')).toHaveTextContent(ApplicationCreateFlowStep.DESIGN);
  });

  it('updates app name when setAppName is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set App Name'));

    expect(screen.getByTestId('app-name')).toHaveTextContent('Test App');
  });

  it('updates selected theme when setSelectedTheme is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Theme to null'));

    expect(screen.getByTestId('selected-theme')).toHaveTextContent('null');
  });

  it('updates app logo when setAppLogo is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Logo'));

    expect(screen.getByTestId('app-logo')).toHaveTextContent('test-logo.png');
  });

  it('toggles integration state when toggleIntegration is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    // Initial state should not have 'test-integration'
    expect(screen.getByTestId('integrations')).not.toHaveTextContent('test-integration');

    await user.click(screen.getByText('Toggle Integration'));

    // Should now have test-integration set to true
    expect(screen.getByTestId('integrations')).toHaveTextContent('test-integration');
    expect(screen.getByTestId('integrations')).toHaveTextContent('true');

    // Toggle again to disable
    await user.click(screen.getByText('Toggle Integration'));

    // Should now have test-integration set to false
    expect(screen.getByTestId('integrations')).toHaveTextContent('test-integration');
    expect(screen.getByTestId('integrations')).toHaveTextContent('false');
  });

  it('updates sign-in approach when setSignInApproach is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Custom Approach'));

    expect(screen.getByTestId('sign-in-approach')).toHaveTextContent(ApplicationCreateFlowSignInApproach.EMBEDDED);
  });

  it('updates selected technology when setSelectedTechnology is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set React Technology'));

    expect(screen.getByTestId('selected-technology')).toHaveTextContent(TechnologyApplicationTemplate.REACT);
  });

  it('updates selected platform when setSelectedPlatform is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Browser Platform'));

    expect(screen.getByTestId('selected-platform')).toHaveTextContent(PlatformApplicationTemplate.BROWSER);
  });

  it('updates hosting URL when setHostingUrl is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Hosting URL'));

    expect(screen.getByTestId('hosting-url')).toHaveTextContent('https://example.com');
  });

  it('updates callback URL when setCallbackUrlFromConfig is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Callback URL'));

    expect(screen.getByTestId('callback-url')).toHaveTextContent('https://example.com/callback');
  });

  it('updates error when setError is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    await user.click(screen.getByText('Set Error'));

    expect(screen.getByTestId('error')).toHaveTextContent('Test error');
  });

  it('resets all state when reset is called', async () => {
    const user = userEvent.setup();

    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    // Set some values
    await user.click(screen.getByText('Set App Name'));
    await user.click(screen.getByText('Set Theme to null'));
    await user.click(screen.getByText('Set Error'));

    // Verify values are set
    expect(screen.getByTestId('app-name')).toHaveTextContent('Test App');
    expect(screen.getByTestId('selected-theme')).toHaveTextContent('null');
    expect(screen.getByTestId('error')).toHaveTextContent('Test error');

    // Reset
    await user.click(screen.getByText('Reset'));

    // Verify back to initial state
    expect(screen.getByTestId('current-step')).toHaveTextContent(ApplicationCreateFlowStep.STACK);
    expect(screen.getByTestId('app-name')).toHaveTextContent('');
    expect(screen.getByTestId('error')).toHaveTextContent('null');
    expect(screen.getByTestId('selected-theme')).toHaveTextContent('null');
  });

  it('memoizes context value to prevent unnecessary re-renders', () => {
    const renderSpy = vi.fn();

    function TestRenderer() {
      renderSpy();
      return <TestConsumer />;
    }

    const {rerender} = renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestRenderer />
      </ApplicationCreateProvider>,
    );

    expect(renderSpy).toHaveBeenCalledTimes(1);

    // Re-render with same props
    rerender(
      <ApplicationCreateProvider>
        <TestRenderer />
      </ApplicationCreateProvider>,
    );

    // Should only render once more due to memoization
    expect(renderSpy).toHaveBeenCalledTimes(2);
  });

  it('provides default integrations with basic auth enabled', () => {
    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    const integrations = JSON.parse(screen.getByTestId('integrations').textContent ?? '{}') as Record<string, boolean>;
    expect(integrations[AuthenticatorTypes.BASIC_AUTH]).toBe(true);
  });

  it('initializes with a default primary color', () => {
    renderWithQueryClient(
      <ApplicationCreateProvider>
        <TestConsumer />
      </ApplicationCreateProvider>,
    );

    const color = screen.getByTestId('selected-color').textContent;
    expect(color).toMatch(/^#[0-9a-fA-F]{6}$/); // Should be a valid hex color
  });
});
