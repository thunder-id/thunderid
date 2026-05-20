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

import {css} from '@emotion/css';
import {Theme} from '@thunderid/browser';
import {useMemo} from 'react';

export type ButtonColor = 'primary' | 'secondary' | 'tertiary' | string;
export type ButtonVariant = 'solid' | 'outline' | 'text' | 'icon';
export type ButtonSize = 'small' | 'medium' | 'large';

/**
 * Creates styles for the Button component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param color - The button color
 * @param variant - The button variant
 * @param size - The button size
 * @param fullWidth - Whether the button should take full width
 * @param disabled - Whether the button is disabled
 * @param loading - Whether the button is in loading state
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  color: ButtonColor,
  variant: ButtonVariant,
  size: ButtonSize,
  fullWidth: boolean,
  disabled: boolean,
  loading: boolean,
  shape: 'square' | 'round' = 'square',
): Record<string, string | null> =>
  useMemo(() => {
    const iconSizeMap: Record<string, string> = {
      large: `calc(${theme.vars.spacing.unit} * 5)`,
      medium: `calc(${theme.vars.spacing.unit} * 4)`,
      small: `calc(${theme.vars.spacing.unit} * 3)`,
    };

    const iconDimension: string = iconSizeMap[size] || iconSizeMap['medium'];

    const baseButton: string = css`
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: calc(${theme.vars.spacing.unit} * 1);
      border-radius: ${shape === 'round'
        ? '50%'
        : theme.vars.components?.Button?.root?.borderRadius || theme.vars.borderRadius.medium};
      font-weight: 500;
      cursor: ${disabled || loading ? 'not-allowed' : 'pointer'};
      outline: none;
      text-decoration: none;
      white-space: nowrap;
      width: ${fullWidth ? '100%' : 'auto'};
      opacity: ${disabled || loading ? 0.6 : 1};
      font-family: ${theme.vars.typography.fontFamily};
      border-width: 1px;
      border-style: solid;
      ${variant === 'icon'
        ? `
        padding: 0;
        min-width: unset;
        min-height: unset;
        width: ${iconDimension};
        height: ${iconDimension};
        justify-content: center;
        align-items: center;
      `
        : ''}
    `;

    const sizeStyles: Record<string, string> = {
      large: css`
        ${variant === 'icon'
          ? `font-size: ${theme.vars.typography.fontSizes.lg};`
          : `padding: calc(${theme.vars.spacing.unit} * 1.5) calc(${theme.vars.spacing.unit} * 3);
             font-size: ${theme.vars.typography.fontSizes.lg};
             min-height: calc(${theme.vars.spacing.unit} * 5);`}
      `,
      medium: css`
        ${variant === 'icon'
          ? `font-size: ${theme.vars.typography.fontSizes.md};`
          : `padding: calc(${theme.vars.spacing.unit} * 1) calc(${theme.vars.spacing.unit} * 2);
             font-size: ${theme.vars.typography.fontSizes.md};
             min-height: calc(${theme.vars.spacing.unit} * 4);`}
      `,
      small: css`
        ${variant === 'icon'
          ? `font-size: ${theme.vars.typography.fontSizes.sm};`
          : `padding: calc(${theme.vars.spacing.unit} * 0.5) calc(${theme.vars.spacing.unit} * 1);
             font-size: ${theme.vars.typography.fontSizes.sm};
             min-height: calc(${theme.vars.spacing.unit} * 3);`}
      `,
    };

    const variantStyles: Record<string, string> = {
      'primary-icon': css`
        background-color: transparent;
        color: ${theme.vars.colors.primary.main};
        border-color: transparent;
        &:hover:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.hover};
          color: ${theme.vars.colors.primary.dark};
        }
        &:active:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.selected};
          color: ${theme.vars.colors.primary.dark};
        }
        &:focus:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.focus};
          color: ${theme.vars.colors.primary.dark};
          outline: none;
        }
      `,
      'primary-outline': css`
        background-color: transparent;
        color: ${theme.vars.colors.primary.main};
        border-color: ${theme.vars.colors.primary.main};
        &:hover:not(:disabled) {
          background-color: ${theme.vars.colors.primary.main};
          color: ${theme.vars.colors.primary.contrastText};
        }
        &:active:not(:disabled) {
          background-color: ${theme.vars.colors.primary.main};
          color: ${theme.vars.colors.primary.contrastText};
          opacity: 0.9;
        }
        &:focus:not(:disabled) {
          background-color: ${theme.vars.colors.primary.main};
          color: ${theme.vars.colors.primary.contrastText};
          opacity: 0.9;
        }
      `,
      'primary-solid': css`
        background-color: ${theme.vars.colors.primary.main};
        color: ${theme.vars.colors.primary.contrastText};
        border-color: ${theme.vars.colors.primary.main};
        &:hover:not(:disabled) {
          background-color: ${theme.vars.colors.primary.main};
          opacity: 0.9;
        }
        &:active:not(:disabled) {
          background-color: ${theme.vars.colors.primary.main};
          opacity: 0.8;
        }
        &:focus:not(:disabled) {
          background-color: ${theme.vars.colors.primary.main};
          opacity: 0.8;
        }
      `,
      'primary-text': css`
        background-color: transparent;
        color: ${theme.vars.colors.primary.main};
        border-color: transparent;
        &:hover:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.hover};
        }
        &:active:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.selected};
        }
        &:focus:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.focus};
          outline: none;
        }
      `,
      'secondary-icon': css`
        background-color: transparent;
        color: ${theme.vars.colors.secondary.main};
        border-color: transparent;
        &:hover:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.hover};
          color: ${theme.vars.colors.secondary.dark};
        }
        &:active:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.selected};
          color: ${theme.vars.colors.secondary.dark};
        }
        &:focus:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.focus};
          color: ${theme.vars.colors.secondary.dark};
          outline: none;
        }
      `,
      'secondary-outline': css`
        background-color: transparent;
        color: ${theme.vars.colors.secondary.main};
        border-color: ${theme.vars.colors.secondary.main};
        &:hover:not(:disabled) {
          background-color: ${theme.vars.colors.secondary.main};
          color: ${theme.vars.colors.secondary.contrastText};
        }
        &:active:not(:disabled) {
          background-color: ${theme.vars.colors.secondary.main};
          color: ${theme.vars.colors.secondary.contrastText};
          opacity: 0.9;
        }
        &:focus:not(:disabled) {
          background-color: ${theme.vars.colors.secondary.main};
          color: ${theme.vars.colors.secondary.contrastText};
          opacity: 0.9;
        }
      `,
      'secondary-solid': css`
        background-color: ${theme.vars.colors.secondary.main};
        color: ${theme.vars.colors.secondary.contrastText};
        border-color: ${theme.vars.colors.secondary.main};
        &:hover:not(:disabled) {
          background-color: ${theme.vars.colors.secondary.main};
          opacity: 0.9;
        }
        &:active:not(:disabled) {
          background-color: ${theme.vars.colors.secondary.main};
          opacity: 0.8;
        }
        &:focus:not(:disabled) {
          background-color: ${theme.vars.colors.secondary.main};
          opacity: 0.8;
        }
      `,
      'secondary-text': css`
        background-color: transparent;
        color: ${theme.vars.colors.secondary.main};
        border-color: transparent;
        &:hover:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.hover};
        }
        &:active:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.selected};
        }
        &:focus:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.focus};
          outline: none;
        }
      `,
      'tertiary-icon': css`
        background-color: transparent;
        color: ${theme.vars.colors.text.secondary};
        border-color: transparent;
        &:hover:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.hover};
          color: ${theme.vars.colors.text.primary};
        }
        &:active:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.selected};
          color: ${theme.vars.colors.text.primary};
        }
        &:focus:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.focus};
          color: ${theme.vars.colors.text.primary};
          outline: none;
        }
      `,
      'tertiary-outline': css`
        background-color: transparent;
        color: ${theme.vars.colors.text.secondary};
        border-color: ${theme.vars.colors.border};
        &:hover:not(:disabled) {
          background-color: ${theme.vars.colors.action.hover};
          border-color: ${theme.vars.colors.text.secondary};
        }
        &:active:not(:disabled) {
          background-color: ${theme.vars.colors.action.selected};
          border-color: ${theme.vars.colors.text.primary};
        }
        &:focus:not(:disabled) {
          background-color: ${theme.vars.colors.action.focus};
          border-color: ${theme.vars.colors.text.primary};
        }
      `,
      'tertiary-solid': css`
        background-color: ${theme.vars.colors.text.secondary};
        color: ${theme.vars.colors.background.surface};
        border-color: ${theme.vars.colors.text.secondary};
        &:hover:not(:disabled) {
          background-color: ${theme.vars.colors.text.primary};
          color: ${theme.vars.colors.background.surface};
        }
        &:active:not(:disabled) {
          background-color: ${theme.vars.colors.text.primary};
          color: ${theme.vars.colors.background.surface};
          opacity: 0.9;
        }
        &:focus:not(:disabled) {
          background-color: ${theme.vars.colors.text.primary};
          color: ${theme.vars.colors.background.surface};
          opacity: 0.9;
        }
      `,
      'tertiary-text': css`
        background-color: transparent;
        color: ${theme.vars.colors.text.secondary};
        border-color: transparent;
        &:hover:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.hover};
          color: ${theme.vars.colors.text.primary};
        }
        &:active:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.selected};
          color: ${theme.vars.colors.text.primary};
        }
        &:focus:not(:disabled) {
          border-color: transparent;
          background-color: ${theme.vars.colors.action.focus};
          color: ${theme.vars.colors.text.primary};
          outline: none;
        }
      `,
    };

    const spinnerStyles: string = css`
      display: flex;
      align-items: center;
      justify-content: center;
    `;

    const iconStyles: string = css`
      display: flex;
      align-items: center;
      justify-content: center;
    `;

    const contentStyles: string = css`
      display: flex;
      align-items: center;
      justify-content: center;
    `;

    return {
      button: baseButton,
      content: contentStyles,
      endIcon: iconStyles,
      fullWidth: fullWidth
        ? css`
            width: 100%;
          `
        : null,
      icon: iconStyles,
      loading: loading
        ? css`
            pointer-events: none;
          `
        : null,
      shape:
        shape === 'round'
          ? css`
              border-radius: 50%;
            `
          : null,
      size: sizeStyles[size],
      spinner: spinnerStyles,
      startIcon: iconStyles,
      variant: variantStyles[`${color}-${variant}`] || variantStyles['primary-solid'],
    };
  }, [theme, colorScheme, color, variant, size, fullWidth, disabled, loading]);

export default useStyles;
