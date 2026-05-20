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

/**
 * Creates styles for the DatePicker component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param hasError - Whether the date picker has an error state
 * @param disabled - Whether the date picker is disabled
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string, hasError: boolean, disabled: boolean): Record<string, string> =>
  useMemo(() => {
    const inputStyles: string = css`
      width: 100%;
      padding: ${theme.vars.spacing.unit} calc(${theme.vars.spacing.unit} * 1.5);
      border: 1px solid ${theme.vars.colors.border};
      border-radius: ${theme.vars.components?.Field?.root?.borderRadius || theme.vars.borderRadius.medium};
      font-size: 1rem;
      font-family: ${theme.vars.typography.fontFamily};
      color: ${theme.vars.colors.text.primary};
      background-color: ${theme.vars.colors.background.surface};
      outline: none;
      transition:
        border-color 0.2s ease,
        box-shadow 0.2s ease;

      &:focus {
        border-color: ${theme.vars.colors.primary.main};
        box-shadow: 0 0 0 2px ${theme.vars.colors.primary.main}20;
      }

      &:hover:not(:disabled) {
        border-color: ${theme.vars.colors.primary.main};
      }

      &::placeholder {
        color: ${theme.vars.colors.text.secondary};
      }
    `;

    const errorInputStyles: string = css`
      border-color: ${theme.vars.colors.error.main};

      &:focus {
        border-color: ${theme.vars.colors.error.main};
        box-shadow: 0 0 0 2px ${theme.vars.colors.error.main}20;
      }

      &:hover:not(:disabled) {
        border-color: ${theme.vars.colors.error.main};
      }
    `;

    const disabledInputStyles: string = css`
      background-color: ${theme.vars.colors.background.disabled};
      color: ${theme.vars.colors.text.secondary};
      cursor: not-allowed;
      opacity: 0.6;

      &:hover,
      &:focus {
        border-color: ${theme.vars.colors.border};
        box-shadow: none;
      }
    `;

    const labelStyles: string = css`
      /* Label styles will be handled by InputLabel component */
    `;

    return {
      disabledInput: disabled ? disabledInputStyles : '',
      errorInput: hasError ? errorInputStyles : '',
      input: inputStyles,
      label: labelStyles,
    };
  }, [theme, colorScheme, hasError, disabled]);

export default useStyles;
