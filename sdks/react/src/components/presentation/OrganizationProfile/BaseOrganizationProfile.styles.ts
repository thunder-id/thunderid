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
 * Creates styles for the BaseOrganizationProfile component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string): Record<string, string> =>
  useMemo(
    () => ({
      attributeItem: css`
        display: flex;
        gap: ${theme.vars.spacing.unit};
        padding: calc(${theme.vars.spacing.unit} / 4) 0;
        align-items: center;
      `,
      attributeKey: css`
        font-size: 0.75rem;
        font-weight: 500;
        color: ${theme.vars.colors.text.secondary};
        min-width: 80px;
        flex-shrink: 0;
      `,
      attributeValue: css`
        font-size: 0.75rem;
        color: ${theme.vars.colors.text.primary};
        word-break: break-word;
        flex: 1;
      `,
      attributesList: css`
        display: flex;
        flex-direction: column;
        gap: calc(${theme.vars.spacing.unit} / 4);
      `,
      card: css`
        background: ${theme.vars.colors.background.surface};
        border-radius: ${theme.vars.borderRadius.large};
      `,
      editButton: css`
        min-width: auto;
        padding: calc(${theme.vars.spacing.unit} / 2);
        min-height: auto;
      `,
      field: css`
        display: flex;
        align-items: flex-start;
        padding: calc(${theme.vars.spacing.unit} / 2) 0;
        border-bottom: 1px solid ${theme.vars.colors.border};
        min-height: 28px;
        gap: ${theme.vars.spacing.unit};
      `,
      fieldActions: css`
        display: flex;
        align-items: center;
        gap: calc(${theme.vars.spacing.unit} / 2);
      `,
      fieldContent: css`
        flex: 1;
        display: flex;
        align-items: center;
        gap: ${theme.vars.spacing.unit};
      `,
      fieldInput: css`
        margin-bottom: 0;
      `,
      fieldLast: css`
        border-bottom: none;
      `,
      handle: css`
        font-size: 1rem;
        color: ${theme.vars.colors.text.secondary};
        margin: 0;
        font-family: monospace;
      `,
      header: css`
        display: flex;
        align-items: center;
        gap: calc(${theme.vars.spacing.unit} * 2);
        margin-bottom: calc(${theme.vars.spacing.unit} * 3);
        padding-bottom: calc(${theme.vars.spacing.unit} * 2);
      `,
      infoContainer: css`
        display: flex;
        flex-direction: column;
        gap: ${theme.vars.spacing.unit};
      `,
      label: css`
        font-size: 0.875rem;
        font-weight: 500;
        color: ${theme.vars.colors.text.secondary};
        width: 120px;
        flex-shrink: 0;
        line-height: 28px;
      `,
      name: css`
        font-size: 1.5rem;
        font-weight: 600;
        margin: 0 0 8px 0;
        color: ${theme.vars.colors.text.primary};
      `,
      orgInfo: css`
        flex: 1;
      `,
      permissionBadge: css`
        padding: calc(${theme.vars.spacing.unit} / 4) ${theme.vars.spacing.unit};
        border-radius: ${theme.vars.borderRadius.small};
        font-size: 0.75rem;
        background-color: ${theme.vars.colors.primary.main};
        color: ${theme.vars.colors.primary.contrastText};
        border: 1px solid ${theme.vars.colors.border};
      `,
      permissionsList: css`
        display: flex;
        flex-wrap: wrap;
        gap: calc(${theme.vars.spacing.unit} / 2);
      `,
      placeholderButton: css`
        font-style: italic;
        text-decoration: underline;
        opacity: 0.7;
        padding: 0;
        min-height: auto;
      `,
      popup: css`
        padding: calc(${theme.vars.spacing.unit} * 2);
      `,
      root: css`
        padding: calc(${theme.vars.spacing.unit} * 4);
        min-width: 600px;
        margin: 0 auto;
        font-family: ${theme.vars.typography.fontFamily};
      `,
      statusBadge: css`
        padding: calc(${theme.vars.spacing.unit} / 2) ${theme.vars.spacing.unit};
        border-radius: ${theme.vars.borderRadius.small};
        font-size: 0.75rem;
        font-weight: 500;
        color: white;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      `,
      value: css`
        color: ${theme.vars.colors.text.primary};
        flex: 1;
        display: flex;
        align-items: center;
        gap: ${theme.vars.spacing.unit};
        overflow: hidden;
        min-height: 28px;
        line-height: 28px;
        word-break: break-word;
      `,
      valueEmpty: css`
        font-style: italic;
        opacity: 0.7;
      `,
    }),
    [theme, colorScheme],
  );

export default useStyles;
