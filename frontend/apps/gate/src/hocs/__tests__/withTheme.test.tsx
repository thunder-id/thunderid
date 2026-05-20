/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import type {MouseEventHandler, ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import withTheme from '../withTheme';

// Track the theme passed to OxygenUIThemeProvider
let capturedThemeProviderProps: Record<string, unknown> | undefined;

function MockChild() {
  return <div data-testid="mock-child">Child Component</div>;
}
const WithThemeComponent = withTheme(MockChild);

// Mock useConfig from @thunder/contexts
vi.mock('@thunder/contexts', () => ({
  useConfig: () => ({config: {}}),
}));

// Create mock for useDesign
const mockUseDesign = vi.fn();
vi.mock('@thunderid/design', () => ({
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  useDesign: () => mockUseDesign(),
  StylesheetInjector: () => null,
  GoogleFontLoader: () => null,
  DefaultTheme: {},
}));

const mockUseConfig = vi.hoisted(() => vi.fn());
vi.mock('@thunderid/contexts', () => ({
  useConfig: mockUseConfig,
}));

vi.mock('../../components/Head', () => ({
  default: () => <div data-testid="head" />,
}));

// Track LanguageSwitcher render function props
let mockLanguageSwitcherProps: {
  languages: {code: string; displayName: string; emoji: string}[];
  currentLanguage: string;
  onLanguageChange: (code: string) => void;
  isLoading: boolean;
};

const mockOnLanguageChange = vi.fn();

// Mock LanguageSwitcher from @thunderid/react
vi.mock('@thunderid/react', () => ({
  LanguageSwitcher: ({
    children,
  }: {
    children: (props: {
      languages: {code: string; displayName: string; emoji: string}[];
      currentLanguage: string;
      onLanguageChange: (code: string) => void;
      isLoading: boolean;
    }) => ReactNode;
  }) => {
    const props = mockLanguageSwitcherProps;
    return children({...props, onLanguageChange: mockOnLanguageChange});
  },
}));

// Mock OxygenUI components - capture props passed to theme provider
vi.mock('@wso2/oxygen-ui', () => ({
  AcrylicOrangeTheme: {palette: {primary: {main: '#ff5700'}}},
  HighContrastTheme: {palette: {primary: {main: '#000000'}}},
  createOxygenTheme: (theme: unknown) => theme,
  OxygenUIThemeProvider: ({children, ...rest}: {children: ReactNode; theme?: unknown}) => {
    capturedThemeProviderProps = {...rest};
    return <div data-testid="theme-provider">{children}</div>;
  },
  ColorSchemeToggle: () => <div data-testid="color-scheme-toggle">Toggle</div>,
  CircularProgress: () => <div data-testid="circular-progress">Loading...</div>,
  Box: ({children}: {children: ReactNode}) => <div data-testid="box">{children}</div>,
  Button: ({children, onClick = undefined}: {children: ReactNode; onClick?: MouseEventHandler<HTMLButtonElement>}) => (
    <button type="button" data-testid="language-button" onClick={onClick}>
      {children}
    </button>
  ),
  Menu: ({children, open}: {children: ReactNode; open: boolean}) =>
    open ? <div data-testid="language-menu">{children}</div> : null,
  MenuItem: ({children, onClick = undefined}: {children: ReactNode; onClick?: () => void}) => (
    // eslint-disable-next-line jsx-a11y/interactive-supports-focus
    <div data-testid="menu-item" role="menuitem" onClick={onClick} onKeyDown={undefined}>
      {children}
    </div>
  ),
  Typography: ({children}: {children: ReactNode}) => <span>{children}</span>,
}));

// Mock @wso2/oxygen-ui-icons-react
vi.mock('@wso2/oxygen-ui-icons-react', () => ({
  ChevronDown: () => <span data-testid="chevron-down" />,
}));

describe('withTheme', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedThemeProviderProps = undefined;
    mockUseDesign.mockReturnValue({
      transformedTheme: null,
      isLoading: false,
    });
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          product_name: 'ThunderID',
          favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
        },
      },
    });
    mockLanguageSwitcherProps = {
      languages: [],
      currentLanguage: 'en-US',
      onLanguageChange: mockOnLanguageChange,
      isLoading: false,
    };
  });

  it('renders without crashing', () => {
    const {container} = render(<WithThemeComponent />);
    expect(container).toBeInTheDocument();
  });

  it('renders OxygenUIThemeProvider', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('theme-provider')).toBeInTheDocument();
  });

  it('renders ColorSchemeToggle', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('color-scheme-toggle')).toBeInTheDocument();
  });

  it('renders wrapped component when not loading', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('renders CircularProgress when loading', () => {
    mockUseDesign.mockReturnValue({
      transformedTheme: null,
      isLoading: true,
    });

    render(<WithThemeComponent />);
    expect(screen.getByTestId('circular-progress')).toBeInTheDocument();
    expect(screen.queryByTestId('mock-child')).not.toBeInTheDocument();
  });

  it('passes null theme to OxygenUIThemeProvider when useDesign returns null', () => {
    mockUseDesign.mockReturnValue({
      theme: null,
      isLoading: false,
    });

    render(<WithThemeComponent />);
    const defaultEntry = (capturedThemeProviderProps?.themes as {key: string; theme: unknown}[])?.find(
      (t) => t.key === 'default',
    );
    expect(defaultEntry?.theme).toEqual({});
  });

  it('passes undefined theme to OxygenUIThemeProvider when useDesign returns undefined', () => {
    mockUseDesign.mockReturnValue({
      theme: undefined,
      isLoading: false,
    });

    render(<WithThemeComponent />);
    const defaultEntry = (capturedThemeProviderProps?.themes as {key: string; theme: unknown}[])?.find(
      (t) => t.key === 'default',
    );
    expect(defaultEntry?.theme).toEqual({});
  });

  it('passes transformedTheme to OxygenUIThemeProvider when available', () => {
    const mockTheme = {palette: {primary: {main: '#ff0000'}}};
    mockUseDesign.mockReturnValue({
      theme: mockTheme,
      isLoading: false,
    });

    render(<WithThemeComponent />);
    const defaultEntry = (capturedThemeProviderProps?.themes as {key: string; theme: unknown}[])?.find(
      (t) => t.key === 'default',
    );
    expect(defaultEntry?.theme).toEqual(mockTheme);
  });

  it('does not render language switcher button when only one language is available', () => {
    render(<WithThemeComponent />);
    expect(screen.queryByTestId('language-button')).not.toBeInTheDocument();
  });

  it('renders language switcher button when multiple languages are available', () => {
    mockLanguageSwitcherProps = {
      languages: [
        {code: 'en-US', displayName: 'English', emoji: '🇺🇸'},
        {code: 'fr-FR', displayName: 'French', emoji: '🇫🇷'},
      ],
      currentLanguage: 'en-US',
      onLanguageChange: mockOnLanguageChange,
      isLoading: false,
    };

    render(<WithThemeComponent />);
    expect(screen.getByTestId('language-button')).toBeInTheDocument();
  });

  it('shows the current language display name on the switcher button', () => {
    mockLanguageSwitcherProps = {
      languages: [
        {code: 'en-US', displayName: 'English', emoji: '🇺🇸'},
        {code: 'fr-FR', displayName: 'French', emoji: '🇫🇷'},
      ],
      currentLanguage: 'en-US',
      onLanguageChange: mockOnLanguageChange,
      isLoading: false,
    };

    render(<WithThemeComponent />);
    expect(screen.getByText('English')).toBeInTheDocument();
  });

  it('opens the language menu when the language button is clicked', async () => {
    mockLanguageSwitcherProps = {
      languages: [
        {code: 'en-US', displayName: 'English', emoji: '🇺🇸'},
        {code: 'fr-FR', displayName: 'French', emoji: '🇫🇷'},
      ],
      currentLanguage: 'en-US',
      onLanguageChange: mockOnLanguageChange,
      isLoading: false,
    };
    const user = userEvent.setup();
    render(<WithThemeComponent />);

    await user.click(screen.getByTestId('language-button'));

    expect(screen.getByTestId('language-menu')).toBeInTheDocument();
  });

  it('calls onLanguageChange when a language menu item is clicked', async () => {
    mockLanguageSwitcherProps = {
      languages: [
        {code: 'en-US', displayName: 'English', emoji: '🇺🇸'},
        {code: 'fr-FR', displayName: 'French', emoji: '🇫🇷'},
      ],
      currentLanguage: 'en-US',
      onLanguageChange: mockOnLanguageChange,
      isLoading: false,
    };
    const user = userEvent.setup();
    render(<WithThemeComponent />);

    await user.click(screen.getByTestId('language-button'));
    const menuItems = screen.getAllByTestId('menu-item');
    await user.click(menuItems[1]); // Click French

    expect(mockOnLanguageChange).toHaveBeenCalledWith('fr-FR');
  });

  it('shows loading spinner when isLoading is true and theme is present', () => {
    const mockTheme = {palette: {primary: {main: '#ff0000'}}};
    mockUseDesign.mockReturnValue({
      transformedTheme: mockTheme,
      isLoading: true,
    });

    render(<WithThemeComponent />);
    expect(screen.getByTestId('circular-progress')).toBeInTheDocument();
    expect(screen.queryByTestId('mock-child')).not.toBeInTheDocument();
  });

  it('renders Head', () => {
    render(<WithThemeComponent />);
    expect(screen.getByTestId('head')).toBeInTheDocument();
  });

  it('includes custom object themes from config in the theme list', () => {
    const objectTheme = {palette: {primary: {main: '#aabbcc'}}};
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          design: {
            themes: [{key: 'custom', label: 'Custom Theme', theme: objectTheme}],
          },
        },
      },
    });

    render(<WithThemeComponent />);
    expect(capturedThemeProviderProps?.themes).toEqual(
      expect.arrayContaining([expect.objectContaining({key: 'custom', label: 'Custom Theme'})]),
    );
  });

  it('includes custom string themes from config in the theme list', () => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          design: {
            themes: [{key: 'external', label: 'External Theme', theme: 'https://example.com/theme.json'}],
          },
        },
      },
    });

    render(<WithThemeComponent />);
    expect(capturedThemeProviderProps?.themes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({key: 'external', label: 'External Theme', theme: 'https://example.com/theme.json'}),
      ]),
    );
  });

  it('wraps different components correctly', () => {
    function AnotherChild() {
      return <div data-testid="another-child">Another Component</div>;
    }
    const AnotherWrapped = withTheme(AnotherChild);

    render(<AnotherWrapped />);
    expect(screen.getByTestId('another-child')).toBeInTheDocument();
    expect(screen.getByTestId('theme-provider')).toBeInTheDocument();
  });
});
