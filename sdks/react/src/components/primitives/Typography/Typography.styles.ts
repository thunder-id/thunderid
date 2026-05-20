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

export type TypographyVariant =
  | 'h1'
  | 'h2'
  | 'h3'
  | 'h4'
  | 'h5'
  | 'h6'
  | 'subtitle1'
  | 'subtitle2'
  | 'body1'
  | 'body2'
  | 'caption'
  | 'overline'
  | 'button';

export type TypographyAlign = 'left' | 'center' | 'right' | 'justify';

export type TypographyColor =
  | 'primary'
  | 'secondary'
  | 'error'
  | 'success'
  | 'warning'
  | 'info'
  | 'textPrimary'
  | 'textSecondary'
  | 'inherit';

/**
 * Creates styles for the Typography component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param variant - The typography variant
 * @param align - Text alignment
 * @param color - Color variant
 * @param noWrap - Whether text should be truncated with ellipsis
 * @param inline - Whether text should be displayed inline
 * @param gutterBottom - Whether to add bottom margin
 * @param fontWeight - Custom font weight
 * @param fontSize - Custom font size
 * @param lineHeight - Custom line height
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  variant: TypographyVariant,
  align: TypographyAlign,
  color: TypographyColor,
  noWrap: boolean,
  inline: boolean,
  gutterBottom: boolean,
  fontWeight?: 'normal' | 'medium' | 'semibold' | 'bold' | number,
  fontSize?: string | number,
  lineHeight?: string | number,
): Record<string, string> =>
  useMemo(() => {
    const getColorValue = (colorVariant: TypographyColor): string => {
      switch (colorVariant) {
        case 'primary':
          return theme.colors.primary.main;
        case 'secondary':
          return theme.colors.secondary.main;
        case 'error':
          return theme.colors.error.main;
        case 'textPrimary':
          return theme.colors.text.primary;
        case 'textSecondary':
          return theme.colors.text.secondary;
        case 'inherit':
          return 'inherit';
        default:
          return theme.colors.text.primary;
      }
    };

    const getVariantStyles = (variantName: TypographyVariant): Record<string, string | number> => {
      switch (variantName) {
        case 'h1':
          return {
            fontSize: theme.vars.typography.fontSizes['3xl'],
            fontWeight: 600,
            letterSpacing: '-0.00735em',
            lineHeight: 1.235,
          };
        case 'h2':
          return {
            fontSize: theme.vars.typography.fontSizes['2xl'],
            fontWeight: 600,
            letterSpacing: '0em',
            lineHeight: 1.334,
          };
        case 'h3':
          return {
            fontSize: theme.vars.typography.fontSizes.xl,
            fontWeight: 600,
            letterSpacing: '0.0075em',
            lineHeight: 1.6,
          };
        case 'h4':
          return {
            fontSize: theme.vars.typography.fontSizes.lg,
            fontWeight: 600,
            letterSpacing: '0.00938em',
            lineHeight: 1.5,
          };
        case 'h5':
          return {
            fontSize: theme.vars.typography.fontSizes.md,
            fontWeight: 600,
            letterSpacing: '0em',
            lineHeight: 1.334,
          };
        case 'h6':
          return {
            fontSize: theme.vars.typography.fontSizes.sm,
            fontWeight: 500,
            letterSpacing: '0.0075em',
            lineHeight: 1.6,
          };
        case 'subtitle1':
          return {
            fontSize: theme.vars.typography.fontSizes.md,
            fontWeight: 400,
            letterSpacing: '0.00938em',
            lineHeight: 1.75,
          };
        case 'subtitle2':
          return {
            fontSize: theme.vars.typography.fontSizes.sm,
            fontWeight: 500,
            letterSpacing: '0.00714em',
            lineHeight: 1.57,
          };
        case 'body1':
          return {
            fontSize: theme.vars.typography.fontSizes.md,
            fontWeight: 400,
            letterSpacing: '0.00938em',
            lineHeight: 1.5,
          };
        case 'body2':
          return {
            fontSize: theme.vars.typography.fontSizes.sm,
            fontWeight: 400,
            letterSpacing: '0.01071em',
            lineHeight: 1.43,
          };
        case 'caption':
          return {
            fontSize: theme.vars.typography.fontSizes.xs,
            fontWeight: 400,
            letterSpacing: '0.03333em',
            lineHeight: 1.66,
          };
        case 'overline':
          return {
            fontSize: theme.vars.typography.fontSizes.xs,
            fontWeight: 400,
            letterSpacing: '0.08333em',
            lineHeight: 2.66,
            textTransform: 'uppercase' as const,
          };
        case 'button':
          return {
            fontSize: theme.vars.typography.fontSizes.sm,
            fontWeight: 500,
            letterSpacing: '0.02857em',
            lineHeight: 1.75,
            textTransform: 'uppercase' as const,
          };
        default:
          return {};
      }
    };

    const variantStyles: Record<string, string | number> = getVariantStyles(variant);
    const colorValue: string = getColorValue(color);

    const typography: string = css`
      margin: 0;
      font-family: ${theme.vars.typography.fontFamily};
      color: ${colorValue};
      text-align: ${align};
      display: ${inline ? 'inline' : 'block'};
      ${variantStyles['fontSize'] ? `font-size: ${variantStyles['fontSize']};` : ''}
      ${variantStyles['fontWeight'] ? `font-weight: ${variantStyles['fontWeight']};` : ''}
      ${variantStyles['lineHeight'] ? `line-height: ${variantStyles['lineHeight']};` : ''}
      ${variantStyles['letterSpacing'] ? `letter-spacing: ${variantStyles['letterSpacing']};` : ''}
      ${variantStyles['textTransform'] ? `text-transform: ${variantStyles['textTransform']};` : ''}

      /* Custom overrides */
      ${fontWeight ? `font-weight: ${fontWeight} !important;` : ''}
      ${fontSize ? `font-size: ${typeof fontSize === 'number' ? `${fontSize}px` : fontSize} !important;` : ''}
      ${lineHeight ? `line-height: ${lineHeight} !important;` : ''}

      /* Conditional styles */
      ${noWrap
        ? `
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      `
        : ''}

      ${gutterBottom
        ? `
        margin-bottom: ${theme.spacing.unit}px;
      `
        : ''}
    `;

    const typographyNoWrap: string = css`
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `;

    const typographyInline: string = css`
      display: inline;
    `;

    const typographyGutterBottom: string = css`
      margin-bottom: ${theme.spacing.unit}px;
    `;

    const typographyH1: string = css`
      font-size: ${theme.vars.typography.fontSizes['3xl']};
      font-weight: 600;
      line-height: 1.235;
      letter-spacing: -0.00735em;
    `;

    const typographyH2: string = css`
      font-size: ${theme.vars.typography.fontSizes['2xl']};
      font-weight: 600;
      line-height: 1.334;
      letter-spacing: 0em;
    `;

    const typographyH3: string = css`
      font-size: ${theme.vars.typography.fontSizes.xl};
      font-weight: 600;
      line-height: 1.6;
      letter-spacing: 0.0075em;
    `;

    const typographyH4: string = css`
      font-size: ${theme.vars.typography.fontSizes.lg};
      font-weight: 600;
      line-height: 1.5;
      letter-spacing: 0.00938em;
    `;

    const typographyH5: string = css`
      font-size: ${theme.vars.typography.fontSizes.md};
      font-weight: 600;
      line-height: 1.334;
      letter-spacing: 0em;
    `;

    const typographyH6: string = css`
      font-size: ${theme.vars.typography.fontSizes.sm};
      font-weight: 500;
      line-height: 1.6;
      letter-spacing: 0.0075em;
    `;

    const typographySubtitle1: string = css`
      font-size: ${theme.vars.typography.fontSizes.md};
      font-weight: 400;
      line-height: 1.75;
      letter-spacing: 0.00938em;
    `;

    const typographySubtitle2: string = css`
      font-size: ${theme.vars.typography.fontSizes.sm};
      font-weight: 500;
      line-height: 1.57;
      letter-spacing: 0.00714em;
    `;

    const typographyBody1: string = css`
      font-size: ${theme.vars.typography.fontSizes.md};
      font-weight: 400;
      line-height: 1.5;
      letter-spacing: 0.00938em;
    `;

    const typographyBody2: string = css`
      font-size: ${theme.vars.typography.fontSizes.sm};
      font-weight: 400;
      line-height: 1.43;
      letter-spacing: 0.01071em;
    `;

    const typographyCaption: string = css`
      font-size: ${theme.vars.typography.fontSizes.xs};
      font-weight: 400;
      line-height: 1.66;
      letter-spacing: 0.03333em;
    `;

    const typographyOverline: string = css`
      font-size: ${theme.vars.typography.fontSizes.xs};
      font-weight: 400;
      line-height: 2.66;
      letter-spacing: 0.08333em;
      text-transform: uppercase;
    `;

    const typographyButton: string = css`
      font-size: ${theme.vars.typography.fontSizes.sm};
      font-weight: 500;
      line-height: 1.75;
      letter-spacing: 0.02857em;
      text-transform: uppercase;
    `;

    return {
      typography,
      typographyBody1,
      typographyBody2,
      typographyButton,
      typographyCaption,
      typographyGutterBottom,
      typographyH1,
      typographyH2,
      typographyH3,
      typographyH4,
      typographyH5,
      typographyH6,
      typographyInline,
      typographyNoWrap,
      typographyOverline,
      typographySubtitle1,
      typographySubtitle2,
    };
  }, [theme, colorScheme, variant, align, color, noWrap, inline, gutterBottom, fontWeight, fontSize, lineHeight]);

export default useStyles;
