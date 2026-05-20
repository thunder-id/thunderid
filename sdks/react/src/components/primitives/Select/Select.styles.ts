/**
 * Copyright (c) 2024, WSO2 LLC. (https://www.wso2.com).
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

/**
 * Creates styles for the Select component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param disabled - Whether the component is disabled
 * @param hasError - Whether the component has an error
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string, disabled: boolean, hasError: boolean): Record<string, string> =>
  useMemo(() => {
    const dropdownArrowColor: string = theme.colors.text.secondary.replace('#', '');
    const dropdownArrow = `data:image/svg+xml;charset=US-ASCII,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%22292.4%22%20height%3D%22292.4%22%3E%3Cpath%20fill%3D%22%23${dropdownArrowColor}%22%20d%3D%22M287%2069.4a17.6%2017.6%200%200%200-13-5.4H18.4c-5%200-9.3%201.8-12.9%205.4A17.6%2017.6%200%200%200%200%2082.2c0%205%201.8%209.3%205.4%2012.9l128%20127.9c3.6%203.6%207.8%205.4%2012.8%205.4s9.2-1.8%2012.8-5.4L287%2095c3.5-3.5%205.4-7.8%205.4-12.8%200-5-1.9-9.2-5.5-12.8z%22%2F%3E%3C%2Fsvg%3E`;

    const select: string = css`
      width: 100%;
      padding: ${theme.vars.spacing.unit} calc(${theme.vars.spacing.unit} * 1.5);
      border: 1px solid ${hasError ? theme.vars.colors.error.main : theme.vars.colors.border};
      border-radius: ${theme.vars.components?.Field?.root?.borderRadius || theme.vars.borderRadius.medium};
      font-size: ${theme.vars.typography.fontSizes.md};
      font-family: ${theme.vars.typography.fontFamily};
      color: ${theme.vars.colors.text.primary};
      background-color: ${disabled ? theme.vars.colors.background.disabled : theme.vars.colors.background.surface};
      outline: none;
      transition:
        border-color 0.2s ease,
        box-shadow 0.2s ease;
      appearance: none;
      background-image: url('${dropdownArrow}');
      background-repeat: no-repeat;
      background-position: right 0.7em top 50%;
      background-size: 0.65em auto;
      cursor: ${disabled ? 'not-allowed' : 'pointer'};

      &:focus {
        border-color: ${hasError ? theme.vars.colors.error.main : theme.vars.colors.primary.main};
        box-shadow: 0 0 0 2px ${hasError ? `${theme.vars.colors.error.main}20` : `${theme.vars.colors.primary.main}20`};
      }

      &:disabled {
        opacity: 0.6;
        cursor: not-allowed;
      }

      &:hover:not(:disabled) {
        border-color: ${hasError ? theme.vars.colors.error.main : theme.vars.colors.primary.main};
      }
    `;

    const selectError: string = css`
      border-color: ${theme.vars.colors.error.main};

      &:focus {
        border-color: ${theme.vars.colors.error.main};
        box-shadow: 0 0 0 2px ${theme.vars.colors.error.main}20;
      }

      &:hover:not(:disabled) {
        border-color: ${theme.vars.colors.error.main};
      }
    `;

    const selectDisabled: string = css`
      background-color: ${theme.vars.colors.background.disabled};
      opacity: 0.6;
      cursor: not-allowed;
    `;

    const option: string = css`
      padding: calc(${theme.vars.spacing.unit} / 2) ${theme.vars.spacing.unit};
      color: ${theme.vars.colors.text.primary};
      background-color: ${theme.vars.colors.background.surface};

      &:hover {
        background-color: ${theme.vars.colors.action.hover};
      }

      &:checked {
        background-color: ${theme.vars.colors.primary.main};
        color: ${theme.vars.colors.primary.contrastText};
      }
    `;

    return {
      option,
      select,
      selectDisabled,
      selectError,
    };
  }, [theme, colorScheme, disabled, hasError]);

export default useStyles;
