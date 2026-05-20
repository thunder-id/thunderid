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

export type OtpFieldType = 'text' | 'number' | 'password';

/**
 * Creates styles for the OtpField component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param disabled - Whether the component is disabled
 * @param hasError - Whether the component has an error
 * @param length - Number of OTP input fields
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  disabled: boolean,
  hasError: boolean,
  length: number,
): Record<string, string> =>
  useMemo(() => {
    const inputContainer: string = css`
      display: flex;
      gap: ${theme.vars.spacing.unit};
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
    `;

    const input: string = css`
      width: calc(${theme.vars.spacing.unit} * 6);
      height: calc(${theme.vars.spacing.unit} * 6);
      text-align: center;
      font-size: ${theme.vars.typography.fontSizes.xl};
      font-family: ${theme.vars.typography.fontFamily};
      font-weight: 500;
      border: 2px solid ${hasError ? theme.vars.colors.error.main : theme.vars.colors.border};
      border-radius: ${theme.vars.components?.Field?.root?.borderRadius || theme.vars.borderRadius.medium};
      color: ${theme.vars.colors.text.primary};
      background-color: ${disabled ? theme.vars.colors.background.disabled : theme.vars.colors.background.surface};
      outline: none;
      transition:
        border-color 0.2s ease,
        box-shadow 0.2s ease;

      &:focus {
        border-color: ${hasError ? theme.vars.colors.error.main : theme.vars.colors.primary.main};
        box-shadow: 0 0 0 2px ${hasError ? `${theme.vars.colors.error.main}20` : `${theme.vars.colors.primary.main}20`};
      }

      &:disabled {
        cursor: not-allowed;
        opacity: 0.6;
      }

      &::placeholder {
        color: ${theme.vars.colors.text.secondary};
        opacity: 0.7;
      }
    `;

    const inputError: string = css`
      border-color: ${theme.vars.colors.error.main};

      &:focus {
        border-color: ${theme.vars.colors.error.main};
        box-shadow: 0 0 0 2px ${theme.vars.colors.error.main}20;
      }
    `;

    const inputDisabled: string = css`
      background-color: ${theme.vars.colors.background.disabled};
      cursor: not-allowed;
      opacity: 0.6;
    `;

    return {
      input,
      inputContainer,
      inputDisabled,
      inputError,
    };
  }, [theme, colorScheme, disabled, hasError, length]);

export default useStyles;
