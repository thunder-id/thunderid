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
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ApplicationCreateContext, {
  type ApplicationCreateContextType,
} from '../../../contexts/ApplicationCreate/ApplicationCreateContext';
import {PlatformApplicationTemplate, TechnologyApplicationTemplate} from '../../../models/application-templates';
import ConfigureStack from '../ConfigureStack';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const renderWithContext = (
  props: Parameters<typeof ConfigureStack>[0],
  contextOverrides: Partial<ApplicationCreateContextType> = {},
) => {
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
      <ConfigureStack {...props} />
    </ApplicationCreateContext.Provider>,
  );
};

describe('ConfigureStack', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('rendering', () => {
    it('renders the gallery heading', () => {
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      expect(screen.getByText('applications:onboarding.configure.stack.title')).toBeInTheDocument();
    });

    it('renders all technology templates', () => {
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      expect(screen.getByText('applications:onboarding.configure.stack.technology.express.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.technology.react.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.technology.nextjs.title')).toBeInTheDocument();
    });

    it('renders all platform templates', () => {
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      expect(screen.getByText('applications:onboarding.configure.stack.platform.browser.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.platform.full_stack.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.platform.mobile.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.platform.backend.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.platform.custom.title')).toBeInTheDocument();
    });

    it('renders category filter chips', () => {
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      expect(screen.getByText('applications:onboarding.configure.stack.category.all')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.category.web')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.category.backend')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.stack.category.mobile')).toBeInTheDocument();
    });

    it('does not show "Coming Soon" badge when no templates are disabled', () => {
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      expect(screen.queryByText('Coming Soon')).not.toBeInTheDocument();
    });

    it('renders without onReadyChange callback', () => {
      expect(() => renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn()})).not.toThrow();
    });
  });

  describe('category filtering', () => {
    it('shows only web templates after clicking Web filter', async () => {
      const user = userEvent.setup();
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      await user.click(screen.getByText('applications:onboarding.configure.stack.category.web'));

      expect(screen.getByText('applications:onboarding.configure.stack.technology.react.title')).toBeInTheDocument();
      expect(
        screen.queryByText('applications:onboarding.configure.stack.technology.express.title'),
      ).not.toBeInTheDocument();
    });

    it('shows only backend templates after clicking Backend filter', async () => {
      const user = userEvent.setup();
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      await user.click(screen.getByText('applications:onboarding.configure.stack.category.backend'));

      expect(screen.getByText('applications:onboarding.configure.stack.technology.express.title')).toBeInTheDocument();
      expect(
        screen.queryByText('applications:onboarding.configure.stack.technology.react.title'),
      ).not.toBeInTheDocument();
    });

    it('shows Next.js under both Web and Backend filters', async () => {
      const user = userEvent.setup();
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      await user.click(screen.getByText('applications:onboarding.configure.stack.category.web'));
      expect(screen.getByText('applications:onboarding.configure.stack.technology.nextjs.title')).toBeInTheDocument();

      await user.click(screen.getByText('applications:onboarding.configure.stack.category.backend'));
      expect(screen.getByText('applications:onboarding.configure.stack.technology.nextjs.title')).toBeInTheDocument();
    });

    it('restores all templates after clicking All filter', async () => {
      const user = userEvent.setup();
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      await user.click(screen.getByText('applications:onboarding.configure.stack.category.web'));
      await user.click(screen.getByText('applications:onboarding.configure.stack.category.all'));

      expect(screen.getByText('applications:onboarding.configure.stack.technology.express.title')).toBeInTheDocument();
    });
  });

  describe('template selection', () => {
    it('calls setSelectedTechnology when a technology card is clicked', async () => {
      const user = userEvent.setup();
      const setSelectedTechnology = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {setSelectedTechnology},
      );

      await user.click(screen.getByText('applications:onboarding.configure.stack.technology.react.title'));

      expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
    });

    it('calls setSelectedPlatform when a platform card is clicked', async () => {
      const user = userEvent.setup();
      const setSelectedPlatform = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {setSelectedPlatform},
      );

      await user.click(screen.getByText('applications:onboarding.configure.stack.platform.browser.title'));

      expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BROWSER);
    });

    it('clears platform when technology card is clicked', async () => {
      const user = userEvent.setup();
      const setSelectedTechnology = vi.fn();
      const setSelectedPlatform = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {setSelectedTechnology, setSelectedPlatform, selectedPlatform: PlatformApplicationTemplate.BROWSER},
      );

      await user.click(screen.getByText('applications:onboarding.configure.stack.technology.react.title'));

      expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
      expect(setSelectedPlatform).toHaveBeenCalledWith(null);
    });

    it('clears technology and sets platform when platform card is clicked', async () => {
      const user = userEvent.setup();
      const setSelectedTechnology = vi.fn();
      const setSelectedPlatform = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {setSelectedTechnology, setSelectedPlatform, selectedTechnology: TechnologyApplicationTemplate.REACT},
      );

      await user.click(screen.getByText('applications:onboarding.configure.stack.platform.browser.title'));

      expect(setSelectedTechnology).toHaveBeenCalledWith(null);
      expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BROWSER);
    });

    it('selects the Custom platform template', async () => {
      const user = userEvent.setup();
      const setSelectedTechnology = vi.fn();
      const setSelectedPlatform = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {setSelectedTechnology, setSelectedPlatform},
      );

      await user.click(screen.getByText('applications:onboarding.configure.stack.platform.custom.title'));

      expect(setSelectedTechnology).toHaveBeenCalledWith(null);
      expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.CUSTOM);
    });
  });

  describe('OAuth config syncing', () => {
    it('syncs OAuth configuration on mount', () => {
      const setSelectedTemplateConfig = vi.fn();
      const mockOnOAuthConfigChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: mockOnOAuthConfigChange, onReadyChange: vi.fn()},
        {setSelectedTemplateConfig},
      );

      expect(setSelectedTemplateConfig).toHaveBeenCalled();
      expect(mockOnOAuthConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({scopes: ['openid', 'profile', 'email']}),
      );
    });

    it('syncs correct OAuth config when React is selected', () => {
      const mockOnOAuthConfigChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: mockOnOAuthConfigChange, onReadyChange: vi.fn()},
        {selectedTechnology: TechnologyApplicationTemplate.REACT},
      );

      expect(mockOnOAuthConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          publicClient: expect.any(Boolean) as boolean,
          pkceRequired: expect.any(Boolean) as boolean,
          grantTypes: expect.any(Array) as string[],
          responseTypes: expect.any(Array) as string[],
          redirectUris: expect.any(Array) as string[],
          scopes: ['openid', 'profile', 'email'],
        }),
      );
    });

    it('uses platform template config when technology is OTHER', () => {
      const setSelectedTemplateConfig = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {
          setSelectedTemplateConfig,
          selectedTechnology: TechnologyApplicationTemplate.OTHER,
          selectedPlatform: PlatformApplicationTemplate.MOBILE,
        },
      );

      expect(setSelectedTemplateConfig).toHaveBeenCalledWith(
        expect.objectContaining({
          defaults: expect.objectContaining({name: 'Mobile Application'}) as unknown,
        }),
      );
    });

    it('uses default React config when nothing is selected', () => {
      const setSelectedTemplateConfig = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {setSelectedTemplateConfig, selectedTechnology: null, selectedPlatform: null},
      );

      expect(setSelectedTemplateConfig).toHaveBeenCalledWith(
        expect.objectContaining({
          defaults: expect.objectContaining({name: 'React Application'}) as unknown,
        }),
      );
    });

    it('uses platform template when platform is selected and no technology', () => {
      const setSelectedTemplateConfig = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {
          setSelectedTemplateConfig,
          selectedTechnology: null,
          selectedPlatform: PlatformApplicationTemplate.FULL_STACK,
        },
      );

      expect(setSelectedTemplateConfig).toHaveBeenCalledWith(
        expect.objectContaining({
          defaults: expect.objectContaining({name: 'Full-Stack Application'}) as unknown,
        }),
      );
    });

    it('handles template with empty redirectUris', () => {
      const mockOnOAuthConfigChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: mockOnOAuthConfigChange, onReadyChange: vi.fn()},
        {selectedPlatform: PlatformApplicationTemplate.BACKEND},
      );

      expect(mockOnOAuthConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({redirectUris: expect.any(Array) as string[]}),
      );
    });
  });

  describe('readiness', () => {
    it('calls onReadyChange true when a technology is selected', () => {
      const onReadyChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange},
        {selectedTechnology: TechnologyApplicationTemplate.REACT},
      );

      expect(onReadyChange).toHaveBeenCalledWith(true);
    });

    it('calls onReadyChange true when a platform is selected', () => {
      const onReadyChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange},
        {selectedPlatform: PlatformApplicationTemplate.BROWSER},
      );

      expect(onReadyChange).toHaveBeenCalledWith(true);
    });

    it('calls onReadyChange false when technology is OTHER and no platform selected', () => {
      const onReadyChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange},
        {selectedTechnology: TechnologyApplicationTemplate.OTHER, selectedPlatform: null},
      );

      expect(onReadyChange).toHaveBeenCalledWith(false);
    });

    it('calls onReadyChange true when OTHER technology with a platform selected', () => {
      const onReadyChange = vi.fn();

      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange},
        {
          selectedTechnology: TechnologyApplicationTemplate.OTHER,
          selectedPlatform: PlatformApplicationTemplate.BROWSER,
        },
      );

      expect(onReadyChange).toHaveBeenCalledWith(true);
    });
  });
});
