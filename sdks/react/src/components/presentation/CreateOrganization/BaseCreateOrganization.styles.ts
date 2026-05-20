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
 * Creates styles for the BaseCreateOrganization component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string): Record<string, string> =>
  useMemo(() => {
    const root: string = css`
      padding: calc(${theme.vars.spacing.unit} * 4);
      min-width: 600px;
      margin: 0 auto;
      font-family: ${theme.vars.typography.fontFamily};
    `;

    const card: string = css`
      background: ${theme.vars.colors.background.surface};
      border-radius: ${theme.vars.borderRadius.large};
      padding: calc(${theme.vars.spacing.unit} * 4);
    `;

    const content: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} * 2);
    `;

    const form: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} * 2);
      width: 100%;
    `;

    const header: string = css`
      display: flex;
      align-items: center;
      gap: calc(${theme.vars.spacing.unit} * 1.5);
      margin-bottom: calc(${theme.vars.spacing.unit} * 1.5);
    `;

    const field: string = css`
      display: flex;
      align-items: center;
      padding: ${theme.vars.spacing.unit} 0;
      border-bottom: 1px solid ${theme.vars.colors.border};
      min-height: 32px;
    `;

    const fieldGroup: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} * 0.5);
    `;

    const textarea: string = css`
      width: 100%;
      padding: ${theme.vars.spacing.unit} calc(${theme.vars.spacing.unit} * 1.5);
      border: 1px solid ${theme.vars.colors.border};
      border-radius: ${theme.vars.borderRadius.medium};
      font-size: ${theme.vars.typography.fontSizes.md};
      color: ${theme.vars.colors.text.primary};
      background-color: ${theme.vars.colors.background.surface};
      font-family: ${theme.vars.typography.fontFamily};
      min-height: 80px;
      resize: vertical;
      outline: none;
      &:focus {
        border-color: ${theme.vars.colors.primary.main};
        box-shadow: 0 0 0 2px ${theme.vars.colors.primary.main}20;
      }
      &:disabled {
        background-color: ${theme.vars.colors.background.disabled};
        color: ${theme.vars.colors.text.secondary};
        cursor: not-allowed;
      }
    `;

    const textareaError: string = css`
      border-color: ${theme.vars.colors.error.main};
    `;

    const input: string = css``;

    const avatarContainer: string = css`
      align-items: flex-start;
      display: flex;
      gap: calc(${theme.vars.spacing.unit} * 2);
      margin-bottom: ${theme.vars.spacing.unit};
    `;

    const actions: string = css`
      display: flex;
      gap: ${theme.vars.spacing.unit};
      justify-content: flex-end;
      padding-top: calc(${theme.vars.spacing.unit} * 2);
    `;

    const infoContainer: string = css`
      display: flex;
      flex-direction: column;
      gap: ${theme.vars.spacing.unit};
    `;

    const value: string = css`
      color: ${theme.vars.colors.text.primary};
      flex: 1;
      display: flex;
      align-items: center;
      gap: ${theme.vars.spacing.unit};
      overflow: hidden;
      min-height: 32px;
      line-height: 32px;
    `;

    const popup: string = css`
      padding: calc(${theme.vars.spacing.unit} * 2);
    `;

    const errorAlert: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 2);
    `;

    return {
      actions,
      avatarContainer,
      card,
      content,
      errorAlert,
      field,
      fieldGroup,
      form,
      header,
      infoContainer,
      input,
      popup,
      root,
      textarea,
      textareaError,
      value,
    };
  }, [
    theme.vars.spacing.unit,
    theme.vars.colors.background.surface,
    theme.vars.colors.border,
    theme.vars.borderRadius.large,
    theme.vars.borderRadius.medium,
    theme.vars.typography.fontSizes.md,
    theme.vars.colors.text.primary,
    theme.vars.colors.primary.main,
    theme.vars.colors.background.disabled,
    theme.vars.colors.text.secondary,
    theme.vars.colors.error.main,
    theme.vars.typography.fontFamily,
    colorScheme,
  ]);

export default useStyles;
