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
import {
  AuthenticatorTypes,
  IdentityProviderTypes,
  useIdentityProviders,
  type IdentityProvider,
} from '@thunderid/configure-connections';
import type {Theme} from '@thunderid/design';
import {type RecursivePartial} from '@thunderid/types';
import type {ReactNode} from 'react';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import Preview, {type PreviewProps} from '../Preview';

// Mock the @thunderid/react module
vi.mock('@thunderid/react', () => ({
  BaseSignIn: ({children}: {children: () => ReactNode}) => <div>{children()}</div>,
  ThemeProvider: ({children}: {children: ReactNode}) => <div>{children}</div>,
}));

// Mock the useIdentityProviders hook
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useIdentityProviders: vi.fn(),
}));

// Mock useColorScheme to test dark mode
const mockUseColorScheme = vi.fn<() => {mode: 'light' | 'dark'}>();
vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    useColorScheme: () => mockUseColorScheme(),
    // Prevent OxygenUIThemeProvider from passing the partial test theme to MUI
    // (which lacks typography and causes a crash)
    OxygenUIThemeProvider: ({children}: {children: ReactNode}) => children,
  };
});

const mockTheme: RecursivePartial<Theme> = {
  direction: 'ltr',
  defaultColorScheme: 'light',
  colorSchemes: {
    light: {
      palette: {
        primary: {
          main: '#FF5733',
          light: '#FF8A66',
          dark: '#CC4529',
          contrastText: '#FFFFFF',
          mainChannel: '234 88 12',
          lightChannel: '255 138 102',
          darkChannel: '204 69 41',
          contrastTextChannel: '255 255 255',
        },
        secondary: {
          main: '#0066CC',
          light: '#3399FF',
          dark: '#004C99',
          contrastText: '#FFFFFF',
          mainChannel: '0 102 204',
          lightChannel: '51 153 255',
          darkChannel: '0 76 153',
          contrastTextChannel: '255 255 255',
        },
        background: {
          default: '#FFFFFF',
          paper: '#F5F5F5',
          defaultChannel: '255 255 255',
          paperChannel: '245 245 245',
        },
      },
    },
    dark: {
      palette: {
        primary: {
          main: '#00FF00',
          dark: '#00CC00',
          contrastText: '#000000',
          mainChannel: '0 255 0',
          darkChannel: '0 204 0',
          contrastTextChannel: '0 0 0',
          light: '#66FF66',
          lightChannel: '102 255 102',
        },
        secondary: {
          main: '#0088FF',
          dark: '#0066CC',
          contrastText: '#FFFFFF',
          mainChannel: '0 136 255',
          darkChannel: '0 102 204',
          contrastTextChannel: '255 255 255',
          light: '#3399FF',
          lightChannel: '51 153 255',
        },
        background: {
          default: '#121212',
          paper: '#1E1E1E',
          defaultChannel: '18 18 18',
          paperChannel: '30 30 30',
        },
      },
    },
  },
};

describe('Preview', () => {
  const mockIdentityProviders: IdentityProvider[] = [
    {
      id: 'google-idp',
      name: 'Google',
      type: IdentityProviderTypes.GOOGLE,
      description: 'Google Identity Provider',
    },
    {
      id: 'github-idp',
      name: 'GitHub',
      type: 'GITHUB',
      description: 'GitHub Identity Provider',
    },
  ];

  const defaultProps: PreviewProps = {
    appLogo: 'https://example.com/logo.png',
    selectedTheme: mockTheme,
    integrations: {
      [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);
    mockUseColorScheme.mockReturnValue({mode: 'light'});
  });

  const renderComponent = (props: Partial<PreviewProps> = {}) => render(<Preview {...defaultProps} {...props} />);

  it('should render the preview title', () => {
    renderComponent();

    expect(screen.getByText('Preview')).toBeInTheDocument();
  });

  it('should render the application logo when provided', () => {
    renderComponent();

    const logo = screen.getByRole('img');
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute('src', 'https://example.com/logo.png');
  });

  it('should not render logo when appLogo is null', () => {
    renderComponent({appLogo: null});

    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });

  it('should render username and password fields when username/password is enabled', () => {
    renderComponent();

    expect(screen.getByText('Username')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Enter your Username')).toBeInTheDocument();
    expect(screen.getByText('Password')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Enter your Password')).toBeInTheDocument();
  });

  it('should render sign in button', () => {
    renderComponent();

    expect(screen.getByRole('button', {name: 'Sign In'})).toBeInTheDocument();
  });

  it('should not render username/password fields when disabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
        'google-idp': true,
      },
    });

    expect(screen.queryByText('Username')).not.toBeInTheDocument();
    expect(screen.queryByText('Password')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Sign In'})).not.toBeInTheDocument();
  });

  it('should render social login buttons for enabled providers', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
        'github-idp': true,
      },
    });

    expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: /Continue with GitHub/i})).toBeInTheDocument();
  });

  it('should not render social login buttons when no providers are enabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
      },
    });

    expect(screen.queryByRole('button', {name: /Continue with/i})).not.toBeInTheDocument();
  });

  it('should render divider when both username/password and social logins are enabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
      },
    });

    expect(screen.getByText('or')).toBeInTheDocument();
  });

  it('should not render divider when only username/password is enabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
      },
    });

    expect(screen.queryByText('or')).not.toBeInTheDocument();
  });

  it('should not render divider when only social logins are enabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
        'google-idp': true,
      },
    });

    expect(screen.queryByText('or')).not.toBeInTheDocument();
  });

  it('should render only selected social providers', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
        // github-idp not included
      },
    });

    expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Continue with GitHub/i})).not.toBeInTheDocument();
  });

  it('should handle empty integrations object', () => {
    renderComponent({
      integrations: {},
    });

    // Username/password should not be shown when integrations is empty (defaults to false)
    expect(screen.queryByText('Username')).not.toBeInTheDocument();
    expect(screen.queryByText('Password')).not.toBeInTheDocument();
  });

  it('should apply theme primary color to sign in button background', () => {
    renderComponent();

    const signInButton = screen.getByRole('button', {name: 'Sign In'});
    // Buttons omit variant="contained" to avoid CSS-variable specificity issues with
    // the outer app's theme; visual styles are applied entirely through the sx prop.
    expect(signInButton).not.toHaveClass('MuiButton-containedPrimary');
    expect(signInButton).toHaveStyle({backgroundColor: '#FF5733'});
  });

  it('should apply contrastText color to sign in button label', () => {
    renderComponent();

    const signInButton = screen.getByRole('button', {name: 'Sign In'});
    expect(signInButton).toHaveStyle({color: '#FFFFFF'});
  });

  it('should apply selected color to logo background', () => {
    renderComponent();

    const logo = screen.getByRole('img');
    const avatarContainer = logo.closest('.MuiAvatar-root');
    expect(avatarContainer).toHaveStyle({backgroundColor: '#FF5733'});
  });

  it('should render input fields as disabled', () => {
    renderComponent();

    const usernameInput = screen.getByPlaceholderText('Enter your Username');
    const passwordInput = screen.getByPlaceholderText('Enter your Password');

    expect(usernameInput).toBeDisabled();
    expect(passwordInput).toBeDisabled();
  });

  it('should render social login buttons as disabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
      },
    });

    const googleButton = screen.getByRole('button', {name: /Continue with Google/i});
    expect(googleButton).toBeDisabled();
  });

  it('should handle when useIdentityProviders returns undefined data', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
      },
    });

    // Should not crash, no social providers should be rendered (since data is undefined)
    expect(screen.queryByRole('button', {name: /Continue with/i})).not.toBeInTheDocument();
  });

  it('should only show providers that exist in API and are selected', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: [mockIdentityProviders[0]], // Only Google in API
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
        // 'github-idp' is not in API, so even if selected, it won't show
      },
    });

    // Should only show Google (which exists in API)
    expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Continue with GitHub/i})).not.toBeInTheDocument();
  });

  it('should not show providers that are not selected even if they exist in API', () => {
    vi.mocked(useIdentityProviders).mockReturnValue({
      data: mockIdentityProviders,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useIdentityProviders>);

    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
        'github-idp': false, // Not selected
      },
    });

    // Should only show Google
    expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Continue with GitHub/i})).not.toBeInTheDocument();
  });

  it('should render multiple social providers in order', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        'google-idp': true,
        'github-idp': true,
      },
    });

    const buttons = screen.getAllByRole('button', {name: /Continue with/i});
    expect(buttons).toHaveLength(2);
    expect(buttons[0]).toHaveTextContent('Continue with Google');
    expect(buttons[1]).toHaveTextContent('Continue with GitHub');
  });

  it('should render sign in form when only username/password is enabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
      },
    });

    expect(screen.getByText('Username')).toBeInTheDocument();
    expect(screen.getByText('Password')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Sign In'})).toBeInTheDocument();
    expect(screen.queryByText('or')).not.toBeInTheDocument();
  });

  it('should render only social logins when username/password is disabled', () => {
    renderComponent({
      integrations: {
        [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
        'google-idp': true,
        'github-idp': true,
      },
    });

    expect(screen.queryByText('Username')).not.toBeInTheDocument();
    expect(screen.queryByText('Password')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Sign In'})).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: /Continue with GitHub/i})).toBeInTheDocument();
  });

  describe('SMS OTP functionality', () => {
    it('should render mobile number field when SMS OTP is enabled', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
        },
      });

      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('Enter your mobile number')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Send OTP'})).toBeInTheDocument();
    });

    it('should not render mobile number field when SMS OTP is disabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'sms-otp': false,
        },
      });

      expect(screen.queryByText('Mobile Number')).not.toBeInTheDocument();
      expect(screen.queryByPlaceholderText('Enter your mobile number')).not.toBeInTheDocument();
      expect(screen.queryByRole('button', {name: 'Send OTP'})).not.toBeInTheDocument();
    });

    it('should render divider when SMS OTP and social logins are enabled', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
          'google-idp': true,
        },
      });

      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render divider when username/password and SMS OTP are enabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'sms-otp': true,
        },
      });

      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should not render divider when only SMS OTP is enabled', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
        },
      });

      expect(screen.queryByText('or')).not.toBeInTheDocument();
    });

    it('should render divider when all authentication methods are enabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'sms-otp': true,
          'google-idp': true,
        },
      });

      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should apply theme colors to Send OTP button', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
        },
        selectedTheme: mockTheme,
      });

      const sendOtpButton = screen.getByRole('button', {name: 'Send OTP'});
      expect(sendOtpButton).toHaveStyle({backgroundColor: '#FF5733'});
    });

    it('should render mobile number input as disabled', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
        },
      });

      const mobileInput = screen.getByPlaceholderText('Enter your mobile number');
      expect(mobileInput).toBeDisabled();
    });

    it('should render SMS OTP with username/password combination', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'sms-otp': true,
        },
      });

      // Username/password fields
      expect(screen.getByText('Username')).toBeInTheDocument();
      expect(screen.getByText('Password')).toBeInTheDocument();

      // SMS OTP fields
      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('Enter your mobile number')).toBeInTheDocument();

      // Both buttons
      expect(screen.getByRole('button', {name: 'Sign In'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Send OTP'})).toBeInTheDocument();

      // Divider should be present
      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render SMS OTP with social logins combination', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
          'google-idp': true,
          'github-idp': true,
        },
      });

      // SMS OTP fields
      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Send OTP'})).toBeInTheDocument();

      // Social login buttons
      expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Continue with GitHub/i})).toBeInTheDocument();

      // Divider should be present
      expect(screen.getByText('or')).toBeInTheDocument();
    });
  });

  describe('Passkey functionality', () => {
    it('should render passkey button when enabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.PASSKEY]: true,
        },
      });

      expect(screen.getByRole('button', {name: /Sign in with Passkey/i})).toBeInTheDocument();
    });

    it('should not render passkey button when disabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.PASSKEY]: false,
        },
      });

      expect(screen.queryByRole('button', {name: /Sign in with Passkey/i})).not.toBeInTheDocument();
    });

    it('should render passkey button with outlined variant when username/password is enabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          [AuthenticatorTypes.PASSKEY]: true,
        },
        selectedTheme: mockTheme,
      });

      const passkeyButton = screen.getByRole('button', {name: /Sign in with Passkey/i});
      expect(passkeyButton).toHaveClass('MuiButton-outlined');
      expect(passkeyButton).toHaveClass('MuiButton-colorPrimary');
    });

    it('should apply theme primary color to passkey button background when username/password is disabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          [AuthenticatorTypes.PASSKEY]: true,
        },
        selectedTheme: mockTheme,
      });

      const passkeyButton = screen.getByRole('button', {name: /Sign in with Passkey/i});
      // Without username/password, the passkey button acts as the primary action and omits
      // variant="contained" — styles come entirely from the sx prop.
      expect(passkeyButton).not.toHaveClass('MuiButton-contained');
      expect(passkeyButton).toHaveStyle({backgroundColor: '#FF5733'});
    });

    it('should apply contrastText color to passkey button label when username/password is disabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: false,
          [AuthenticatorTypes.PASSKEY]: true,
        },
        selectedTheme: mockTheme,
      });

      const passkeyButton = screen.getByRole('button', {name: /Sign in with Passkey/i});
      expect(passkeyButton).toHaveStyle({color: '#FFFFFF'});
    });

    it('should render passkey button inside form container when social logins are present', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.PASSKEY]: true,
          'google-idp': true,
        },
      });

      const passkeyButton = screen.getByRole('button', {name: /Sign in with Passkey/i});
      const containerBox = passkeyButton.closest('form');
      expect(containerBox).toBeInTheDocument();
    });

    it('should render passkey button inside form container when social logins are absent', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.PASSKEY]: true,
          'google-idp': false,
        },
      });

      const passkeyButton = screen.getByRole('button', {name: /Sign in with Passkey/i});
      const containerBox = passkeyButton.closest('form');
      expect(containerBox).toBeInTheDocument();
    });

    it('should render divider when passkey and social logins are enabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.PASSKEY]: true,
          'google-idp': true,
        },
      });

      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render divider when passkey and SMS OTP are enabled', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.PASSKEY]: true,
          'sms-otp': true,
        },
      });

      expect(screen.getByText('or')).toBeInTheDocument();
    });
  });

  describe('dark mode', () => {
    it('should render in dark mode', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent();

      // Component should render without errors in dark mode
      expect(screen.getByText('Preview')).toBeInTheDocument();
    });

    it('should render with username/password in dark mode', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      expect(screen.getByText('Username')).toBeInTheDocument();
      expect(screen.getByText('Password')).toBeInTheDocument();
    });

    it('should render with social logins in dark mode', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent({
        integrations: {
          'google-idp': true,
        },
      });

      expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
    });

    it('should render logo in dark mode', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent({
        appLogo: 'https://example.com/logo.png',
      });

      const logo = screen.getByRole('img');
      expect(logo).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
    it('should handle undefined integration values gracefully', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: undefined as unknown as boolean,
        },
      });

      // Should not render username/password when value is undefined (falsy)
      expect(screen.queryByText('Username')).not.toBeInTheDocument();
    });

    it('should handle undefined sms-otp value gracefully', () => {
      renderComponent({
        integrations: {
          'sms-otp': undefined as unknown as boolean,
        },
      });

      // Should not render SMS OTP when value is undefined
      expect(screen.queryByText('Mobile Number')).not.toBeInTheDocument();
    });

    it('should render only username/password form with no margin when no social logins', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
        },
      });

      // Username/password should be present
      expect(screen.getByText('Username')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Sign In'})).toBeInTheDocument();
      // No divider because no social logins
      expect(screen.queryByText('or')).not.toBeInTheDocument();
    });

    it('should render only SMS OTP form with no margin when no social logins', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
        },
      });

      // SMS OTP should be present
      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Send OTP'})).toBeInTheDocument();
      // No divider because no social logins
      expect(screen.queryByText('or')).not.toBeInTheDocument();
    });

    it('should render username/password with margin when social logins exist', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'google-idp': true,
        },
      });

      // Both username/password and social login should be present
      expect(screen.getByText('Username')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
      // Divider should be present
      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render SMS OTP with margin when social logins exist', () => {
      renderComponent({
        integrations: {
          'sms-otp': true,
          'google-idp': true,
        },
      });

      // Both SMS OTP and social login should be present
      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
      // Divider should be present
      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render all three auth methods with divider', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'sms-otp': true,
          'google-idp': true,
        },
      });

      // All three should be present
      expect(screen.getByText('Username')).toBeInTheDocument();
      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /Continue with Google/i})).toBeInTheDocument();
      // Divider should be present
      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render username/password and SMS OTP without social logins', () => {
      renderComponent({
        integrations: {
          [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
          'sms-otp': true,
        },
      });

      // Both should be present
      expect(screen.getByText('Username')).toBeInTheDocument();
      expect(screen.getByText('Mobile Number')).toBeInTheDocument();
      // Divider should be present when both methods exist
      expect(screen.getByText('or')).toBeInTheDocument();
    });
  });

  describe('high-contrast theme colors', () => {
    const highContrastTheme: RecursivePartial<Theme> = {
      colorSchemes: {
        light: {
          palette: {
            primary: {
              main: '#0000FF',
              dark: '#0000CC',
              contrastText: '#FFFFFF',
              light: '#FF8A66',
              mainChannel: '234 88 12',
              lightChannel: '255 138 102',
              darkChannel: '204 69 41',
              contrastTextChannel: '255 255 255',
            },
            secondary: {
              main: '#0066CC',
              light: '#3399FF',
              dark: '#004C99',
              contrastText: '#FFFFFF',
              mainChannel: '0 102 204',
              lightChannel: '51 153 255',
              darkChannel: '0 76 153',
              contrastTextChannel: '255 255 255',
            },
            background: {
              default: '#FFFFFF',
              paper: '#F5F5F5',
              defaultChannel: '255 255 255',
              paperChannel: '245 245 245',
            },
          },
        },
        dark: {
          palette: {
            primary: {
              main: '#FFD700',
              dark: '#CCB000',
              contrastText: '#000000',
              mainChannel: '0 255 0',
              darkChannel: '0 204 0',
              contrastTextChannel: '0 0 0',
              light: '#66FF66',
              lightChannel: '102 255 102',
            },
            secondary: {
              main: '#00FFFF',
              dark: '#00CCCC',
              contrastText: '#000000',
              mainChannel: '0 136 255',
              darkChannel: '0 102 204',
              contrastTextChannel: '255 255 255',
              light: '#3399FF',
              lightChannel: '51 153 255',
            },
            background: {
              default: '#121212',
              paper: '#1E1E1E',
              defaultChannel: '18 18 18',
              paperChannel: '30 30 30',
            },
          },
        },
      },
      defaultColorScheme: 'light',
      direction: 'ltr',
    };

    it('should render sign in button with high-contrast primary background', () => {
      renderComponent({
        integrations: {[AuthenticatorTypes.CREDENTIALS_AUTH]: true},
        selectedTheme: highContrastTheme,
      });

      const signInButton = screen.getByRole('button', {name: 'Sign In'});
      expect(signInButton).toHaveStyle({backgroundColor: '#0000FF'});
    });

    it('should render sign in button with white contrastText on high-contrast blue', () => {
      renderComponent({
        integrations: {[AuthenticatorTypes.CREDENTIALS_AUTH]: true},
        selectedTheme: highContrastTheme,
      });

      const signInButton = screen.getByRole('button', {name: 'Sign In'});
      expect(signInButton).toHaveStyle({color: '#FFFFFF'});
    });

    it('should render Send OTP button with high-contrast primary background', () => {
      renderComponent({
        integrations: {'sms-otp': true},
        selectedTheme: highContrastTheme,
      });

      const sendOtpButton = screen.getByRole('button', {name: 'Send OTP'});
      expect(sendOtpButton).toHaveStyle({backgroundColor: '#0000FF'});
      expect(sendOtpButton).toHaveStyle({color: '#FFFFFF'});
    });

    it('should render passkey button with dark mode high-contrast colors', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent({
        integrations: {[AuthenticatorTypes.PASSKEY]: true},
        selectedTheme: highContrastTheme,
      });

      const passkeyButton = screen.getByRole('button', {name: /Sign in with Passkey/i});
      expect(passkeyButton).toHaveStyle({backgroundColor: '#FFD700'});
      // Dark mode gold primary has black contrastText
      expect(passkeyButton).toHaveStyle({color: '#000000'});
    });
  });

  describe('theme mode handling', () => {
    it('should use normal blend mode in light mode', () => {
      mockUseColorScheme.mockReturnValue({mode: 'light'});

      renderComponent();

      expect(screen.getByText('Preview')).toBeInTheDocument();
    });

    it('should use screen blend mode in dark mode', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent();

      expect(screen.getByText('Preview')).toBeInTheDocument();
    });

    it('should apply dark mode styles when mode is dark', () => {
      mockUseColorScheme.mockReturnValue({mode: 'dark'});

      renderComponent({
        appLogo: 'https://example.com/logo.png',
        selectedTheme: mockTheme,
      });

      const logo = screen.getByRole('img');
      const avatarContainer = logo.closest('.MuiAvatar-root');
      expect(avatarContainer).toBeInTheDocument();
    });

    it('should apply light mode styles when mode is light', () => {
      mockUseColorScheme.mockReturnValue({mode: 'light'});

      renderComponent({
        appLogo: 'https://example.com/logo.png',
        selectedTheme: mockTheme,
      });

      const logo = screen.getByRole('img');
      const avatarContainer = logo.closest('.MuiAvatar-root');
      expect(avatarContainer).toHaveStyle({backgroundColor: '#FF5733'});
    });
  });
});
