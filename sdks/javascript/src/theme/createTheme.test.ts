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

import {describe, it, expect} from 'vitest';
import createTheme from './createTheme';
import {Theme} from './types';

describe('createTheme', () => {
  it('should include vars property with CSS variable references', () => {
    const theme: Theme = createTheme();

    expect(theme.vars).toBeDefined();
    expect(theme.vars.colors.primary.main).toBe('var(--thunder-color-primary-main)');
    expect(theme.vars.colors.primary.contrastText).toBe('var(--thunder-color-primary-contrastText)');
    expect(theme.vars.spacing.unit).toBe('var(--thunder-spacing-unit)');
    expect(theme.vars.borderRadius.small).toBe('var(--thunder-border-radius-small)');
    expect(theme.vars.shadows.medium).toBe('var(--thunder-shadow-medium)');
  });

  it('should have matching structure between cssVariables and vars', () => {
    const theme: Theme = createTheme();

    // Check that cssVariables has corresponding entries for vars
    expect(theme.cssVariables['--thunder-color-primary-main']).toBeDefined();
    expect(theme.cssVariables['--thunder-spacing-unit']).toBeDefined();
    expect(theme.cssVariables['--thunder-border-radius-small']).toBeDefined();
    expect(theme.cssVariables['--thunder-shadow-medium']).toBeDefined();
  });

  it('should work with custom theme configurations', () => {
    const customTheme: Theme = createTheme({
      colors: {
        primary: {
          main: '#custom-color',
        },
      },
    });

    // vars should still reference CSS variables, not the actual values
    expect(customTheme.vars.colors.primary.main).toBe('var(--thunder-color-primary-main)');
    // but cssVariables should have the custom value
    expect(customTheme.cssVariables['--thunder-color-primary-main']).toBe('#custom-color');
  });

  it('should work with dark theme', () => {
    const darkTheme: Theme = createTheme({}, true);

    expect(darkTheme.vars.colors.primary.main).toBe('var(--thunder-color-primary-main)');
    expect(darkTheme.vars.colors.background.surface).toBe('var(--thunder-color-background-surface)');

    // Should have dark theme values in cssVariables
    expect(darkTheme.cssVariables['--thunder-color-background-surface']).toBe('#121212');
  });

  it('should use custom CSS variable prefix when provided', () => {
    const customTheme: Theme = createTheme({
      colors: {
        primary: {
          main: '#custom-color',
        },
      },
      cssVarPrefix: 'custom-app',
    });

    // Should use custom prefix in CSS variables
    expect(customTheme.cssVariables['--custom-app-color-primary-main']).toBe('#custom-color');
    expect(customTheme.cssVariables['--custom-app-spacing-unit']).toBe('8px');

    // Should use custom prefix in vars
    expect(customTheme.vars.colors.primary.main).toBe('var(--custom-app-color-primary-main)');
    expect(customTheme.vars.spacing.unit).toBe('var(--custom-app-spacing-unit)');

    // Should not have old thunderid prefixed variables
    expect(customTheme.cssVariables['--thunder-color-primary-main']).toBeUndefined();
  });

  it('should use VendorConstants.VENDOR_PREFIX as default prefix', () => {
    const theme: Theme = createTheme();

    // Should use default prefix from VendorConstants
    expect(theme.cssVariables['--thunder-color-primary-main']).toBeDefined();
    expect(theme.vars.colors.primary.main).toBe('var(--thunder-color-primary-main)');
  });
});
