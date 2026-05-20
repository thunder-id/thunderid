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
import {ApplicationCreateFlowSignInApproach} from '../../../models/application-create-flow';
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

  it('renders technology and platform sections', () => {
    renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

    expect(screen.getByText('applications:onboarding.configure.stack.technology.title')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.platform.title')).toBeInTheDocument();
  });

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

    renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()}, {setSelectedPlatform});

    await user.click(screen.getByText('applications:onboarding.configure.stack.platform.browser.title'));

    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BROWSER);
  });

  it('syncs the OAuth configuration on mount', () => {
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

  it('shows only technology section when stackTypes.technology is true', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
      stackTypes: {technology: true, platform: false},
    });

    expect(screen.getByText('applications:onboarding.configure.stack.technology.title')).toBeInTheDocument();
    expect(screen.queryByText('applications:onboarding.configure.stack.platform.title')).not.toBeInTheDocument();
  });

  it('shows only platform section when stackTypes.platform is true', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
      stackTypes: {technology: false, platform: true},
    });

    expect(screen.queryByText('applications:onboarding.configure.stack.technology.title')).not.toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.platform.title')).toBeInTheDocument();
  });

  it('updates template config when technology selection changes', async () => {
    const user = userEvent.setup();
    const setSelectedTemplateConfig = vi.fn();
    const mockOnOAuthConfigChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: mockOnOAuthConfigChange,
        onReadyChange: vi.fn(),
      },
      {setSelectedTemplateConfig},
    );

    await user.click(screen.getByText('applications:onboarding.configure.stack.technology.react.title'));

    expect(setSelectedTemplateConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        defaults: expect.objectContaining({name: 'React Application'}) as unknown,
      }),
    );
  });

  it('updates template config when platform selection changes', async () => {
    const user = userEvent.setup();
    const setSelectedTemplateConfig = vi.fn();
    const setSelectedTechnology = vi.fn();
    const setSelectedPlatform = vi.fn();
    const mockOnOAuthConfigChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: mockOnOAuthConfigChange,
        onReadyChange: vi.fn(),
      },
      {setSelectedTemplateConfig, setSelectedTechnology, setSelectedPlatform},
    );

    await user.click(screen.getByText('applications:onboarding.configure.stack.platform.mobile.title'));

    expect(setSelectedTechnology).toHaveBeenCalledWith(null);
    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.MOBILE);
  });

  it('highlights selected technology card', () => {
    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {selectedTechnology: TechnologyApplicationTemplate.REACT},
    );

    const reactCard = screen.getByText('applications:onboarding.configure.stack.technology.react.title');
    expect(reactCard).toBeInTheDocument();
  });

  it('highlights selected platform card', () => {
    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {selectedPlatform: PlatformApplicationTemplate.BROWSER},
    );

    const browserCard = screen.getByText('applications:onboarding.configure.stack.platform.browser.title');
    expect(browserCard).toBeInTheDocument();
  });

  it('calls onReadyChange based on selection state', () => {
    const onReadyChange = vi.fn();

    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange,
    });

    // Should be ready when no selection is required (both sections shown)
    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('calls onReadyChange false when technology is OTHER but platform not selected', () => {
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange,
      },
      {selectedTechnology: TechnologyApplicationTemplate.OTHER, selectedPlatform: null},
    );

    expect(onReadyChange).toHaveBeenCalledWith(false);
  });

  it('calls onReadyChange true when required technology is selected', () => {
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange,
        stackTypes: {technology: true, platform: false},
      },
      {selectedTechnology: TechnologyApplicationTemplate.REACT},
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('calls onReadyChange false when platform is required but not selected', () => {
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange,
        stackTypes: {technology: false, platform: true},
      },
      {selectedPlatform: null},
    );

    expect(onReadyChange).toHaveBeenCalledWith(false);
  });

  it('renders all technology options', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
    });

    expect(screen.getByText('applications:onboarding.configure.stack.technology.express.title')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.technology.react.title')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.technology.nextjs.title')).toBeInTheDocument();
  });

  it('renders all platform options', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
    });

    expect(screen.getByText('applications:onboarding.configure.stack.platform.browser.title')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.platform.full_stack.title')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.platform.mobile.title')).toBeInTheDocument();
    expect(screen.getByText('applications:onboarding.configure.stack.platform.backend.title')).toBeInTheDocument();
  });

  it('shows divider when both technology and platform sections are visible', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
    });

    expect(screen.getByText('applications:onboarding.configure.stack.dividerLabel')).toBeInTheDocument();
  });

  it('does not show divider when only one section is visible', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
      stackTypes: {technology: true, platform: false},
    });

    expect(screen.queryByText('applications:onboarding.configure.stack.dividerLabel')).not.toBeInTheDocument();
  });

  it('shows "Coming Soon" badge for disabled technology options', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
      onReadyChange: vi.fn(),
    });

    expect(screen.getByText('Coming Soon')).toBeInTheDocument();
  });

  it('does not call setSelectedTechnology when clicking disabled technology card', async () => {
    const user = userEvent.setup({pointerEventsCheck: 0});
    const setSelectedTechnology = vi.fn();

    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {setSelectedTechnology},
    );

    // Next.js is disabled, clicking should not trigger the handler
    const nextjsCard = screen.getByText('applications:onboarding.configure.stack.technology.nextjs.title');
    await user.click(nextjsCard);

    // setSelectedTechnology should not have been called with NEXTJS
    expect(setSelectedTechnology).not.toHaveBeenCalledWith(TechnologyApplicationTemplate.NEXTJS);
  });

  it('hides platform section and divider when signInApproach is EMBEDDED', () => {
    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange: vi.fn(),
        stackTypes: {technology: true, platform: true},
      },
      {signInApproach: ApplicationCreateFlowSignInApproach.EMBEDDED},
    );

    expect(screen.getByText('applications:onboarding.configure.stack.technology.title')).toBeInTheDocument();
    expect(screen.queryByText('applications:onboarding.configure.stack.platform.title')).not.toBeInTheDocument();
    expect(screen.queryByText('applications:onboarding.configure.stack.dividerLabel')).not.toBeInTheDocument();
  });

  it('auto-selects first platform when technology section is hidden and no platform selected', () => {
    const setSelectedPlatform = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange: vi.fn(),
        stackTypes: {technology: false, platform: true},
      },
      {setSelectedPlatform, selectedPlatform: null},
    );

    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BROWSER);
  });

  it('does not auto-select platform when technology section is visible', () => {
    const setSelectedPlatform = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange: vi.fn(),
        stackTypes: {technology: true, platform: true},
      },
      {setSelectedPlatform, selectedPlatform: null},
    );

    expect(setSelectedPlatform).not.toHaveBeenCalled();
  });

  it('selects server platform when clicked', async () => {
    const user = userEvent.setup();
    const setSelectedPlatform = vi.fn();
    const setSelectedTechnology = vi.fn();

    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {setSelectedPlatform, setSelectedTechnology},
    );

    await user.click(screen.getByText('applications:onboarding.configure.stack.platform.full_stack.title'));

    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.FULL_STACK);
    expect(setSelectedTechnology).toHaveBeenCalledWith(null);
  });

  it('selects backend platform when clicked', async () => {
    const user = userEvent.setup();
    const setSelectedPlatform = vi.fn();
    const setSelectedTechnology = vi.fn();

    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {setSelectedPlatform, setSelectedTechnology},
    );

    await user.click(screen.getByText('applications:onboarding.configure.stack.platform.backend.title'));

    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BACKEND);
    expect(setSelectedTechnology).toHaveBeenCalledWith(null);
  });

  it('uses platform template when technology is OTHER', () => {
    const setSelectedTemplateConfig = vi.fn();
    const mockOnOAuthConfigChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: mockOnOAuthConfigChange,
        onReadyChange: vi.fn(),
      },
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

  it('uses inferred technology from existing oauthConfig', () => {
    const setSelectedTemplateConfig = vi.fn();
    const mockOnOAuthConfigChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: {
          publicClient: true,
          pkceRequired: true,
          grantTypes: ['authorization_code'],
          responseTypes: ['code'],
          redirectUris: ['http://localhost:3000/callback'],
          tokenEndpointAuthMethod: 'none',
          scopes: ['openid', 'profile'],
        },
        onOAuthConfigChange: mockOnOAuthConfigChange,
        onReadyChange: vi.fn(),
      },
      {setSelectedTemplateConfig, selectedTechnology: null, selectedPlatform: null},
    );

    expect(setSelectedTemplateConfig).toHaveBeenCalled();
  });

  it('resolves technology to OTHER when platform is selected but no technology', () => {
    const setSelectedTemplateConfig = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange: vi.fn(),
      },
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

  it('calls onReadyChange true when OTHER technology with platform selected', () => {
    const onReadyChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange,
      },
      {
        selectedTechnology: TechnologyApplicationTemplate.OTHER,
        selectedPlatform: PlatformApplicationTemplate.BROWSER,
      },
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('clears technology and sets platform when platform card is clicked', async () => {
    const user = userEvent.setup();
    const setSelectedTechnology = vi.fn();
    const setSelectedPlatform = vi.fn();

    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {
        setSelectedTechnology,
        setSelectedPlatform,
        selectedTechnology: TechnologyApplicationTemplate.REACT,
      },
    );

    await user.click(screen.getByText('applications:onboarding.configure.stack.platform.browser.title'));

    expect(setSelectedTechnology).toHaveBeenCalledWith(null);
    expect(setSelectedPlatform).toHaveBeenCalledWith(PlatformApplicationTemplate.BROWSER);
  });

  it('clears platform when technology card is clicked', async () => {
    const user = userEvent.setup();
    const setSelectedTechnology = vi.fn();
    const setSelectedPlatform = vi.fn();

    renderWithContext(
      {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
      {
        setSelectedTechnology,
        setSelectedPlatform,
        selectedPlatform: PlatformApplicationTemplate.BROWSER,
      },
    );

    await user.click(screen.getByText('applications:onboarding.configure.stack.technology.react.title'));

    expect(setSelectedTechnology).toHaveBeenCalledWith(TechnologyApplicationTemplate.REACT);
    expect(setSelectedPlatform).toHaveBeenCalledWith(null);
  });

  it('renders without onReadyChange callback', () => {
    renderWithContext({
      oauthConfig: null,
      onOAuthConfigChange: vi.fn(),
    });

    expect(screen.getByText('applications:onboarding.configure.stack.technology.title')).toBeInTheDocument();
  });

  it('syncs OAuth config with correct structure including all fields', () => {
    const mockOnOAuthConfigChange = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: mockOnOAuthConfigChange,
        onReadyChange: vi.fn(),
      },
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

  it('does not auto-select platform when already selected', () => {
    const setSelectedPlatform = vi.fn();

    renderWithContext(
      {
        oauthConfig: null,
        onOAuthConfigChange: vi.fn(),
        onReadyChange: vi.fn(),
        stackTypes: {technology: false, platform: true},
      },
      {setSelectedPlatform, selectedPlatform: PlatformApplicationTemplate.MOBILE},
    );

    expect(setSelectedPlatform).not.toHaveBeenCalled();
  });

  describe('hover states and conditional styling', () => {
    it('should render technology card with correct structure when not selected', () => {
      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {selectedTechnology: TechnologyApplicationTemplate.OTHER},
      );

      const reactTitle = screen.getByText('applications:onboarding.configure.stack.technology.react.title');
      expect(reactTitle).toBeInTheDocument();
    });

    it('should render technology card with correct structure when selected', () => {
      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {selectedTechnology: TechnologyApplicationTemplate.REACT},
      );

      const reactTitle = screen.getByText('applications:onboarding.configure.stack.technology.react.title');
      expect(reactTitle).toBeInTheDocument();
    });

    it('should render platform card with correct structure when not selected', () => {
      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {selectedPlatform: PlatformApplicationTemplate.FULL_STACK},
      );

      const browserTitle = screen.getByText('applications:onboarding.configure.stack.platform.browser.title');
      expect(browserTitle).toBeInTheDocument();
    });

    it('should render platform card with correct structure when selected', () => {
      renderWithContext(
        {oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()},
        {selectedPlatform: PlatformApplicationTemplate.BROWSER},
      );

      const browserTitle = screen.getByText('applications:onboarding.configure.stack.platform.browser.title');
      expect(browserTitle).toBeInTheDocument();
    });

    it('should render disabled technology card with correct structure', () => {
      renderWithContext({oauthConfig: null, onOAuthConfigChange: vi.fn(), onReadyChange: vi.fn()});

      const nextjsTitle = screen.getByText('applications:onboarding.configure.stack.technology.nextjs.title');
      expect(nextjsTitle).toBeInTheDocument();
      expect(screen.getByText('Coming Soon')).toBeInTheDocument();
    });
  });

  describe('technology resolution logic', () => {
    it('should use default technology when stackTypes.technology is true and nothing selected', () => {
      const setSelectedTemplateConfig = vi.fn();

      renderWithContext(
        {
          oauthConfig: null,
          onOAuthConfigChange: vi.fn(),
          onReadyChange: vi.fn(),
          stackTypes: {technology: true, platform: true},
        },
        {setSelectedTemplateConfig, selectedTechnology: null, selectedPlatform: null},
      );

      // Should use default technology (React)
      expect(setSelectedTemplateConfig).toHaveBeenCalledWith(
        expect.objectContaining({
          defaults: expect.objectContaining({name: 'React Application'}) as unknown,
        }),
      );
    });

    it('should use OTHER technology when stackTypes.technology is false', () => {
      const setSelectedTemplateConfig = vi.fn();

      renderWithContext(
        {
          oauthConfig: null,
          onOAuthConfigChange: vi.fn(),
          onReadyChange: vi.fn(),
          stackTypes: {technology: false, platform: true},
        },
        {setSelectedTemplateConfig, selectedTechnology: null, selectedPlatform: PlatformApplicationTemplate.BROWSER},
      );

      // Should use platform template since technology is hidden
      expect(setSelectedTemplateConfig).toHaveBeenCalledWith(
        expect.objectContaining({
          defaults: expect.objectContaining({name: 'Browser Application'}) as unknown,
        }),
      );
    });
  });

  describe('OAuth config template handling', () => {
    it('should handle template with empty redirectUris', () => {
      const mockOnOAuthConfigChange = vi.fn();

      renderWithContext(
        {
          oauthConfig: null,
          onOAuthConfigChange: mockOnOAuthConfigChange,
          onReadyChange: vi.fn(),
        },
        {selectedPlatform: PlatformApplicationTemplate.BACKEND},
      );

      expect(mockOnOAuthConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          redirectUris: expect.any(Array) as string[],
        }),
      );
    });

    it('should handle template with responseTypes', () => {
      const mockOnOAuthConfigChange = vi.fn();

      renderWithContext(
        {
          oauthConfig: null,
          onOAuthConfigChange: mockOnOAuthConfigChange,
          onReadyChange: vi.fn(),
        },
        {selectedTechnology: TechnologyApplicationTemplate.REACT},
      );

      expect(mockOnOAuthConfigChange).toHaveBeenCalledWith(
        expect.objectContaining({
          responseTypes: expect.any(Array) as string[],
        }),
      );
    });
  });
});
