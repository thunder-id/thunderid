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

export type MultiInputType = 'text' | 'email' | 'tel' | 'url' | 'password' | 'date' | 'boolean';
export type MultiInputFieldType = 'STRING' | 'DATE_TIME' | 'BOOLEAN';

/**
 * Creates styles for the MultiInput component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param disabled - Whether the component is disabled
 * @param hasError - Whether the component has an error
 * @param canAddMore - Whether more items can be added
 * @param canRemove - Whether items can be removed
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  disabled: boolean,
  hasError: boolean,
  canAddMore: boolean,
  canRemove: boolean,
): Record<string, string> =>
  useMemo(() => {
    const container: string = css`
      display: flex;
      flex-direction: column;
      gap: ${theme.vars.spacing.unit};
    `;

    const inputRow: string = css`
      display: flex;
      align-items: center;
      gap: ${theme.vars.spacing.unit};
      position: relative;
    `;

    const inputWrapper: string = css`
      flex: 1;
    `;

    const plusIcon: string = css`
      background: ${theme.vars.colors.secondary.main};
      border-radius: 50%;
      outline: 4px ${theme.vars.colors.secondary.main} auto;
      color: ${theme.vars.colors.secondary.contrastText};
    `;

    const listContainer: string = css`
      display: flex;
      flex-direction: column;
      gap: 0;
    `;

    const listItem: string = css`
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: ${theme.vars.spacing.unit} calc(${theme.vars.spacing.unit} * 1.5);
      background-color: ${theme.vars.colors.background.surface};
      border-radius: ${theme.vars.borderRadius.medium};
      font-size: 1rem;
      font-family: ${theme.vars.typography.fontFamily};
      color: ${theme.vars.colors.text.primary};
      margin-bottom: calc(${theme.vars.spacing.unit} / 2);

      &:last-child {
        margin-bottom: 0;
      }
    `;

    const listItemText: string = css`
      flex: 1;
      word-break: break-word;
    `;

    const removeButton: string = css`
      padding: calc(${theme.vars.spacing.unit} / 2);
      min-width: auto;
      color: ${theme.vars.colors.error.main};
      background: transparent;
      border: none;
      border-radius: ${theme.vars.borderRadius.small};
      cursor: ${disabled ? 'not-allowed' : 'pointer'};
      display: flex;
      align-items: center;
      justify-content: center;

      &:hover:not(:disabled) {
        background-color: ${theme.vars.colors.action.hover};
      }

      &:disabled {
        opacity: 0.6;
      }
    `;

    const icon: string = css`
      width: 16px;
      height: 16px;
      stroke: currentColor;
      stroke-width: 2;
      stroke-linecap: round;
      stroke-linejoin: round;
      fill: none;
    `;

    return {
      container,
      icon,
      inputRow,
      inputWrapper,
      listContainer,
      listItem,
      listItemText,
      plusIcon,
      removeButton,
    };
  }, [theme, colorScheme, disabled, hasError, canAddMore, canRemove]);

export default useStyles;
