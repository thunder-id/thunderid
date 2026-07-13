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

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {AuthenticatorTypes, IdentityProviderTypes, type IdentityProvider} from '@thunderid/configure-connections';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureSignInOptions, {type ConfigureSignInOptionsProps} from '../ConfigureSignInOptions';
import type {BasicFlowDefinition} from '@/features/flows/models/responses';
import findMatchingFlowForIntegrations from '@/features/flows/utils/findMatchingFlowForIntegrations';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'applications:onboarding.configure.SignInOptions.title': 'Configure Sign-In Options',
        'applications:onboarding.configure.SignInOptions.subtitle': 'Choose how users will sign in',
        'applications:onboarding.configure.SignInOptions.error': 'Failed to load sign-in options',
        'applications:onboarding.configure.SignInOptions.noSelectionWarning':
          'Please select at least one sign-in option',
        'applications:onboarding.configure.SignInOptions.hint': 'You can customize this later',
        'applications:onboarding.configure.SignInOptions.usernamePassword': 'Username & Password',
        'applications:onboarding.configure.SignInOptions.google': 'Google',
        'applications:onboarding.configure.SignInOptions.github': 'GitHub',
        'applications:onboarding.configure.SignInOptions.notConfigured': 'Not configured',
      };
      return translations[key] || key;
    },
  }),
}));

// Mock useIdentityProviders
interface MockIdentityProviderResponse {
  data: IdentityProvider[] | null;
  isLoading: boolean;
  error: Error | null;
}
const mockUseIdentityProviders = vi.fn<() => MockIdentityProviderResponse>();
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useIdentityProviders: () => mockUseIdentityProviders(),
}));

// Mock useGetFlows
interface MockFlowsResponse {
  data: {flows: BasicFlowDefinition[]} | null;
  isLoading: boolean;
  error: Error | null;
}
const mockUseGetFlows = vi.fn<() => MockFlowsResponse>();
vi.mock('@/features/flows/api/useGetFlows', () => ({
  default: () => mockUseGetFlows(),
}));

// Mock useApplicationCreateContext - need to mock the correct path used by the component
const mockSetSelectedAuthFlow = vi.fn();
const mockSetIntegrations = vi.fn();
const mockSelectedAuthFlow: BasicFlowDefinition | null = null;
vi.mock('@/features/applications/hooks/useApplicationCreateContext', () => ({
  default: () => ({
    selectedAuthFlow: mockSelectedAuthFlow,
    setSelectedAuthFlow: mockSetSelectedAuthFlow,
    setIntegrations: mockSetIntegrations,
  }),
}));

// Mock findMatchingFlowForIntegrations
vi.mock('@/features/flows/utils/findMatchingFlowForIntegrations', () => ({
  default: vi.fn(() => null),
}));

// Mock child components
vi.mock('../FlowsListView', () => ({
  default: ({onFlowSelect, onClearSelection}: {onFlowSelect: (id: string) => void; onClearSelection: () => void}) => (
    <div data-testid="flows-list-view">
      <button type="button" data-testid="select-flow-btn" onClick={() => onFlowSelect('flow-1')}>
        Select Flow
      </button>
      <button type="button" data-testid="clear-flow-btn" onClick={() => onClearSelection()}>
        Clear
      </button>
    </div>
  ),
}));

vi.mock('../IndividualMethodsToggleView', () => ({
  default: ({onIntegrationToggle}: {onIntegrationToggle: (id: string) => void}) => (
    <div data-testid="individual-methods-view">
      <button
        type="button"
        data-testid="toggle-basic-auth"
        onClick={() => onIntegrationToggle(AuthenticatorTypes.CREDENTIALS_AUTH)}
      >
        Toggle Basic Auth
      </button>
      <button type="button" data-testid="toggle-google" onClick={() => onIntegrationToggle('google-idp')}>
        Toggle Google
      </button>
    </div>
  ),
}));

describe('ConfigureSignInOptions', () => {
  const mockOnIntegrationToggle = vi.fn();
  const mockOnReadyChange = vi.fn();

  const mockIdentityProviders: IdentityProvider[] = [
    {
      id: 'google-idp',
      name: 'Google',
      type: IdentityProviderTypes.GOOGLE,
      description: 'Sign in with Google',
    },
    {
      id: 'github-idp',
      name: 'GitHub',
      type: IdentityProviderTypes.GITHUB,
      description: 'Sign in with GitHub',
    },
  ];

  const mockFlows: BasicFlowDefinition[] = [
    {
      id: 'flow-1',
      name: 'Basic Flow',
    } as BasicFlowDefinition,
  ];

  const defaultProps: ConfigureSignInOptionsProps = {
    integrations: {
      [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
    },
    onIntegrationToggle: mockOnIntegrationToggle,
    onReadyChange: mockOnReadyChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseIdentityProviders.mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    });
    mockUseGetFlows.mockReturnValue({
      data: {flows: mockFlows},
      isLoading: false,
      error: null,
    });
  });

  const renderComponent = (props: Partial<ConfigureSignInOptionsProps> = {}) =>
    render(<ConfigureSignInOptions {...defaultProps} {...props} />);

  describe('rendering', () => {
    it('should render title and subtitle', () => {
      renderComponent();

      expect(screen.getByText('Configure Sign-In Options')).toBeInTheDocument();
      expect(screen.getByText('Choose how users will sign in')).toBeInTheDocument();
    });

    it('should render IndividualMethodsToggleView', () => {
      renderComponent();

      expect(screen.getByTestId('individual-methods-view')).toBeInTheDocument();
    });

    it('should render FlowsListView', () => {
      renderComponent();

      expect(screen.getByTestId('flows-list-view')).toBeInTheDocument();
    });

    it('should render hint text', () => {
      renderComponent();

      expect(screen.getByText('You can customize this later')).toBeInTheDocument();
    });
  });

  describe('loading state', () => {
    it('should show loading spinner when identity providers are loading', () => {
      mockUseIdentityProviders.mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
      });

      renderComponent();

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('should show loading spinner when flows are loading', () => {
      mockUseGetFlows.mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
      });

      renderComponent();

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });
  });

  describe('error state', () => {
    it('should show error alert when identity providers fetch fails', () => {
      mockUseIdentityProviders.mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error('Failed to fetch providers'),
      });

      renderComponent();

      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText('Failed to load sign-in options')).toBeInTheDocument();
    });

    it('should show error alert when flows fetch fails', () => {
      mockUseGetFlows.mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error('Failed to fetch flows'),
      });

      renderComponent();

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  describe('validation warning', () => {
    it('should show warning when no options are selected', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
        },
      });

      expect(screen.getByText('Please select at least one sign-in option')).toBeInTheDocument();
    });

    it('should not show warning when at least one option is selected', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      expect(screen.queryByText('Please select at least one sign-in option')).not.toBeInTheDocument();
    });
  });

  describe('integration toggle', () => {
    it('should call onIntegrationToggle when toggling an integration', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByTestId('toggle-basic-auth'));

      expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.CREDENTIALS_AUTH);
    });

    it('should select matching flow when integration toggle matches a flow', async () => {
      const user = userEvent.setup();
      const mockedFindMatchingFlow = vi.mocked(findMatchingFlowForIntegrations);
      mockedFindMatchingFlow.mockReturnValue(mockFlows[0]);

      renderComponent();

      await user.click(screen.getByTestId('toggle-basic-auth'));

      expect(mockSetSelectedAuthFlow).toHaveBeenCalledWith(mockFlows[0]);
    });
  });

  describe('flow selection', () => {
    it('should call setSelectedAuthFlow when selecting a flow', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByTestId('select-flow-btn'));

      expect(mockSetSelectedAuthFlow).toHaveBeenCalled();
    });

    it('should call setSelectedAuthFlow with null when clearing selection', async () => {
      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByTestId('clear-flow-btn'));

      expect(mockSetSelectedAuthFlow).toHaveBeenCalledWith(null);
    });
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with false when no options are selected', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
        },
      });

      expect(mockOnReadyChange).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with true when at least one option is selected', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should not throw when onReadyChange is not provided', () => {
      expect(() => {
        render(
          <ConfigureSignInOptions
            integrations={{[AuthenticatorTypes.CREDENTIALS_AUTH]: false}}
            onIntegrationToggle={mockOnIntegrationToggle}
          />,
        );
      }).not.toThrow();
    });
  });

  describe('empty data handling', () => {
    it('should handle empty identity providers list', () => {
      mockUseIdentityProviders.mockReturnValue({
        data: [],
        isLoading: false,
        error: null,
      });

      renderComponent();

      expect(screen.getByTestId('individual-methods-view')).toBeInTheDocument();
    });

    it('should handle empty flows list', () => {
      mockUseGetFlows.mockReturnValue({
        data: {flows: []},
        isLoading: false,
        error: null,
      });

      renderComponent();

      expect(screen.getByTestId('flows-list-view')).toBeInTheDocument();
    });

    it('should handle null data from useIdentityProviders', () => {
      mockUseIdentityProviders.mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
      });

      renderComponent();

      expect(screen.getByTestId('individual-methods-view')).toBeInTheDocument();
    });

    it('should handle null flows from useGetFlows', () => {
      mockUseGetFlows.mockReturnValue({
        data: null,
        isLoading: false,
        error: null,
      });

      renderComponent();

      expect(screen.getByTestId('flows-list-view')).toBeInTheDocument();
    });
  });
});
