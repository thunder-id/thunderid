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

export type DividerOrientation = 'horizontal' | 'vertical';
export type DividerVariant = 'solid' | 'dashed' | 'dotted';

/**
 * Creates styles for the Divider component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param orientation - The divider orientation
 * @param variant - The divider variant
 * @param color - Custom color for the divider
 * @param hasChildren - Whether the divider has children (text)
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  orientation: DividerOrientation,
  variant: DividerVariant,
  color?: string,
  hasChildren?: boolean,
): Record<string, string> =>
  useMemo(() => {
    const baseColor: string = color || theme.colors.border;
    let borderStyle: string;
    if (variant === 'solid') {
      borderStyle = 'solid';
    } else if (variant === 'dashed') {
      borderStyle = 'dashed';
    } else {
      borderStyle = 'dotted';
    }

    const baseDivider: string = css`
      margin: calc(${theme.vars.spacing.unit} * 2) 0;
    `;

    const verticalDivider: string = css`
      display: inline-block;
      height: 100%;
      min-height: calc(${theme.vars.spacing.unit} * 2);
      width: 1px;
      border-inline-start: 1px ${borderStyle} ${baseColor};
      margin-block: 0;
      margin-inline: calc(${theme.vars.spacing.unit} * 1);
    `;

    const horizontalDivider: string = css`
      display: flex;
      align-items: center;
      width: 100%;
      ${!hasChildren &&
      css`
        height: 1px;
        border-top: 1px ${borderStyle} ${baseColor};
      `}
    `;

    const dividerLine: string = css`
      flex: 1;
      height: 1px;
      border-top: 1px ${borderStyle} ${baseColor};
    `;

    const dividerText: string = css`
      background-color: ${theme.vars.colors.background.surface};
      font-family: ${theme.vars.typography.fontFamily};
      padding: 0 calc(${theme.vars.spacing.unit} * 1);
      white-space: nowrap;
    `;

    return {
      divider: baseDivider,
      horizontal: horizontalDivider,
      line: dividerLine,
      text: dividerText,
      vertical: verticalDivider,
    };
  }, [theme, colorScheme, orientation, variant, color, hasChildren]);

export default useStyles;
