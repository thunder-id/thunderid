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
 * Creates styles for the KeyValueInput component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param disabled - Whether the component is disabled
 * @param readOnly - Whether the component is read-only
 * @param hasError - Whether the component has an error
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  disabled: boolean,
  readOnly: boolean,
  hasError: boolean,
): Record<string, string> =>
  useMemo(() => {
    const container: string = css`
      display: flex;
      flex-direction: column;
      font-family: ${theme.vars.typography.fontFamily};
      gap: calc(${theme.vars.spacing.unit} / 2);
    `;

    const label: string = css`
      font-size: 0.875rem;
      font-weight: 500;
      color: ${theme.vars.colors.text.primary};
      margin-bottom: calc(${theme.vars.spacing.unit} / 2);
    `;

    const requiredIndicator: string = css`
      color: ${theme.vars.colors.error.main};
    `;

    const pairsList: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} / 4);
    `;

    const pairRow: string = css`
      display: flex;
      align-items: center;
      gap: calc(${theme.vars.spacing.unit} / 2);
      padding: calc(${theme.vars.spacing.unit} / 2);
      border-radius: ${theme.vars.borderRadius.small};
      background-color: transparent;
      border: none;

      &:hover {
        background-color: ${theme.vars.colors.action.hover};
      }
    `;

    const pairInput: string = css`
      flex: 1;
      min-width: 0;
    `;

    const addRow: string = css`
      display: flex;
      align-items: center;
      gap: calc(${theme.vars.spacing.unit} / 2);
      padding: calc(${theme.vars.spacing.unit} / 2);
      border: none;
      border-radius: ${theme.vars.borderRadius.small};
      background-color: transparent;
      margin-top: calc(${theme.vars.spacing.unit} / 2);
    `;

    const removeButton: string = css`
      min-width: auto;
      width: 24px;
      height: 24px;
      padding: 0;
      background-color: transparent;
      color: ${theme.vars.colors.text.secondary};
      border: none;
      border-radius: ${theme.vars.borderRadius.small};
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: ${disabled ? 'not-allowed' : 'pointer'};

      &:hover:not(:disabled) {
        background-color: ${theme.vars.colors.action.hover};
        color: ${theme.vars.colors.error.main};
      }

      &:disabled {
        opacity: 0.6;
      }
    `;

    const addButton: string = css`
      min-width: auto;
      width: 24px;
      height: 24px;
      padding: 0;
      background-color: transparent;
      color: ${theme.vars.colors.primary.main};
      border: none;
      border-radius: ${theme.vars.borderRadius.small};
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: ${disabled ? 'not-allowed' : 'pointer'};

      &:hover:not(:disabled) {
        background-color: ${theme.vars.colors.primary.main};
        color: ${theme.vars.colors.primary.contrastText};
      }

      &:disabled {
        opacity: 0.6;
      }
    `;

    const helperText: string = css`
      font-size: 0.75rem;
      color: ${hasError ? theme.vars.colors.error.main : theme.vars.colors.text.secondary};
      margin-top: calc(${theme.vars.spacing.unit} / 2);
    `;

    const emptyState: string = css`
      padding: ${theme.vars.spacing.unit};
      text-align: center;
      color: ${theme.vars.colors.text.secondary};
      font-style: italic;
      font-size: 0.75rem;
    `;

    const readOnlyPair: string = css`
      display: flex;
      align-items: center;
      gap: calc(${theme.vars.spacing.unit} / 2);
      padding: calc(${theme.vars.spacing.unit} / 4) 0;
      min-height: 20px;
    `;

    const readOnlyKey: string = css`
      font-size: 0.75rem;
      font-weight: 500;
      color: ${theme.vars.colors.text.secondary};
      min-width: 80px;
      flex-shrink: 0;
    `;

    const readOnlyValue: string = css`
      font-size: 0.75rem;
      color: ${theme.vars.colors.text.primary};
      word-break: break-word;
      flex: 1;
    `;

    const counterText: string = css`
      font-size: 0.75rem;
      color: ${theme.vars.colors.text.secondary};
      margin-top: calc(${theme.vars.spacing.unit} / 2);
    `;

    return {
      addButton,
      addRow,
      container,
      counterText,
      emptyState,
      helperText,
      label,
      pairInput,
      pairRow,
      pairsList,
      readOnlyKey,
      readOnlyPair,
      readOnlyValue,
      removeButton,
      requiredIndicator,
    };
  }, [theme, colorScheme, disabled, readOnly, hasError]);

export default useStyles;
