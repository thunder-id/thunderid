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

/* eslint-disable sort-keys, @typescript-eslint/typedef, @typescript-eslint/explicit-function-return-type, testing-library/no-container, testing-library/no-node-access */

import {cleanup, render, screen, waitFor} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import {BaseOrganizationSwitcher, Organization} from './BaseOrganizationSwitcher';

// Mock theme data
const mockColors = {
  text: {primary: '#000', secondary: '#666'},
  background: {surface: '#fff', disabled: '#eee', body: {main: '#fff'}},
  border: '#ccc',
  action: {
    hover: '#f0f0f0',
    active: '#e0e0e0',
    selected: '#d0d0d0',
    disabled: '#bbb',
    disabledBackground: '#f5f5f5',
    focus: '#0066cc',
    hoverOpacity: 0.08,
    selectedOpacity: 0.12,
    disabledOpacity: 0.38,
    focusOpacity: 0.12,
    activatedOpacity: 0.12,
  },
  primary: {main: '#0066cc', contrastText: '#fff'},
  secondary: {main: '#666', contrastText: '#fff'},
  error: {main: '#d32f2f', contrastText: '#fff'},
  success: {main: '#2e7d32', contrastText: '#fff'},
  warning: {main: '#ed6c02', contrastText: '#fff'},
  info: {main: '#0288d1', contrastText: '#fff'},
};

const mockTypography = {
  fontFamily: 'Arial, sans-serif',
  fontSizes: {
    xs: '0.75rem',
    sm: '0.875rem',
    md: '1rem',
    lg: '1.125rem',
    xl: '1.25rem',
    '2xl': '1.5rem',
    '3xl': '1.875rem',
  },
  fontWeights: {normal: 400, medium: 500, semibold: 600, bold: 700},
  lineHeights: {tight: 1.25, normal: 1.5, relaxed: 1.75},
};

const mockTypographyVars = {
  fontFamily: 'Arial, sans-serif',
  fontSizes: {
    xs: '0.75rem',
    sm: '0.875rem',
    md: '1rem',
    lg: '1.125rem',
    xl: '1.25rem',
    '2xl': '1.5rem',
    '3xl': '1.875rem',
  },
  fontWeights: {normal: '400', medium: '500', semibold: '600', bold: '700'},
  lineHeights: {tight: '1.25', normal: '1.5', relaxed: '1.75'},
};

const mockColorsVars = {
  ...mockColors,
  action: {
    hover: '#f0f0f0',
    active: '#e0e0e0',
    selected: '#d0d0d0',
    disabled: '#bbb',
    disabledBackground: '#f5f5f5',
    focus: '#0066cc',
    hoverOpacity: '0.08',
    selectedOpacity: '0.12',
    disabledOpacity: '0.38',
    focusOpacity: '0.12',
    activatedOpacity: '0.12',
  },
};

// Mock the dependencies
vi.mock('../../../contexts/Theme/useTheme', () => ({
  default: () => ({
    theme: {
      // ThemeConfig properties (direct access)
      colors: mockColors,
      typography: mockTypography,
      spacing: {unit: 8},
      borderRadius: {small: '2px', medium: '4px', large: '8px'},
      shadows: {
        small: '0 1px 2px rgba(0,0,0,0.1)',
        medium: '0 2px 4px rgba(0,0,0,0.1)',
        large: '0 4px 8px rgba(0,0,0,0.1)',
      },
      cssVariables: {},
      // ThemeVars (CSS variable references)
      vars: {
        colors: mockColorsVars,
        spacing: {unit: '8px'},
        borderRadius: {small: '2px', medium: '4px', large: '8px'},
        shadows: {
          small: '0 1px 2px rgba(0,0,0,0.1)',
          medium: '0 2px 4px rgba(0,0,0,0.1)',
          large: '0 4px 8px rgba(0,0,0,0.1)',
        },
        typography: mockTypographyVars,
      },
    },
    colorScheme: 'light',
    direction: (document.documentElement.getAttribute('dir') as 'ltr' | 'rtl') || 'ltr',
  }),
}));

vi.mock('../../../hooks/useTranslation', () => ({
  default: () => ({
    t: (key: string) => key,
    currentLanguage: 'en',
    setLanguage: vi.fn(),
    availableLanguages: ['en'],
  }),
}));

const mockOrganizations: Organization[] = [
  {
    id: '1',
    name: 'Organization 1',
    avatar: 'https://example.com/avatar1.jpg',
    memberCount: 10,
    role: 'admin',
  },
  {
    id: '2',
    name: 'Organization 2',
    avatar: 'https://example.com/avatar2.jpg',
    memberCount: 5,
    role: 'member',
  },
];

describe('BaseOrganizationSwitcher RTL Support', () => {
  beforeEach(() => {
    document.documentElement.removeAttribute('dir');
  });

  afterEach(() => {
    cleanup();
    document.documentElement.removeAttribute('dir');
  });

  it('should render correctly in LTR mode', () => {
    document.documentElement.setAttribute('dir', 'ltr');
    const handleSwitch = vi.fn();

    render(
      <BaseOrganizationSwitcher
        organizations={mockOrganizations}
        currentOrganization={mockOrganizations[0]}
        onOrganizationSwitch={handleSwitch}
      />,
    );

    expect(screen.getByText('Organization 1')).toBeDefined();
  });

  it('should render correctly in RTL mode', () => {
    document.documentElement.setAttribute('dir', 'rtl');
    const handleSwitch = vi.fn();

    render(
      <BaseOrganizationSwitcher
        organizations={mockOrganizations}
        currentOrganization={mockOrganizations[0]}
        onOrganizationSwitch={handleSwitch}
      />,
    );

    expect(screen.getByText('Organization 1')).toBeDefined();
  });

  it('should flip chevron icon in RTL mode', async () => {
    document.documentElement.setAttribute('dir', 'rtl');
    const handleSwitch = vi.fn();

    const {container} = render(
      <BaseOrganizationSwitcher
        organizations={mockOrganizations}
        currentOrganization={mockOrganizations[0]}
        onOrganizationSwitch={handleSwitch}
      />,
    );

    await waitFor(() => {
      const chevronIcon = container.querySelector('svg');
      expect(chevronIcon).toBeTruthy();
      if (chevronIcon) {
        // The transform is on the parent span, not the SVG itself
        const parentSpan = chevronIcon.parentElement;
        expect(parentSpan?.style.transform).toContain('scaleX(-1)');
      }
    });
  });

  it('should not flip chevron icon in LTR mode', async () => {
    document.documentElement.setAttribute('dir', 'ltr');
    const handleSwitch = vi.fn();

    const {container} = render(
      <BaseOrganizationSwitcher
        organizations={mockOrganizations}
        currentOrganization={mockOrganizations[0]}
        onOrganizationSwitch={handleSwitch}
      />,
    );

    await waitFor(() => {
      const chevronIcon = container.querySelector('svg');
      expect(chevronIcon).toBeTruthy();
      if (chevronIcon) {
        // The transform is on the parent span, not the SVG itself
        // In LTR mode, the transform should be 'none'
        const parentSpan = chevronIcon.parentElement;
        expect(parentSpan?.style.transform).toBe('none');
      }
    });
  });

  it('should update icon flip when direction changes', async () => {
    document.documentElement.setAttribute('dir', 'ltr');
    const handleSwitch = vi.fn();

    const {container, rerender} = render(
      <BaseOrganizationSwitcher
        organizations={mockOrganizations}
        currentOrganization={mockOrganizations[0]}
        onOrganizationSwitch={handleSwitch}
      />,
    );

    // Initially LTR - style.transform is 'none' on parent span
    let chevronIcon = container.querySelector('svg');
    let parentSpan = chevronIcon?.parentElement;
    expect(parentSpan?.style.transform).toBe('none');

    // Change to RTL
    document.documentElement.setAttribute('dir', 'rtl');

    // Force re-render
    rerender(
      <BaseOrganizationSwitcher
        organizations={mockOrganizations}
        currentOrganization={mockOrganizations[0]}
        onOrganizationSwitch={handleSwitch}
      />,
    );

    await waitFor(() => {
      chevronIcon = container.querySelector('svg');
      parentSpan = chevronIcon?.parentElement;
      expect(parentSpan?.style.transform).toContain('scaleX(-1)');
    });
  });
});
