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
 * Creates styles for the BaseOrganizationSwitcher component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @returns Object containing CSS class names for component styling
 */

const useStyles = (theme: Theme, colorScheme: string): Record<string, string> =>
  useMemo(() => {
    const root: string = css`
      display: inline-block;
      position: relative;
      font-family: ${theme.vars.typography.fontFamily};
    `;

    const trigger: string = css`
      display: inline-flex;
      align-items: center;
      gap: ${theme.vars.spacing.unit};
      padding: calc(${theme.vars.spacing.unit} * 0.75) ${theme.vars.spacing.unit};
      border: 1px solid ${theme.vars.colors.border};
      background: ${theme.vars.colors.background.surface};
      cursor: pointer;
      border-radius: ${theme.vars.borderRadius.medium};
      min-width: 160px;

      > span {
        width: 100%;
        gap: ${theme.vars.spacing.unit};
      }

      &:hover {
        background-color: ${theme.vars.colors.background.surface};
      }
    `;

    const triggerLabel: string = css`
      color: ${theme.vars.colors.text.primary};
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      flex: 1;
    `;

    const content: string = css`
      min-width: 280px;
      max-width: 400px;
      background-color: ${theme.vars.colors.background.surface};
      border-radius: ${theme.vars.borderRadius.medium};
      box-shadow: ${theme.vars.shadows.medium};
      border: 1px solid ${theme.vars.colors.border};
      outline: none;
      z-index: 1000;
    `;

    const header: string = css`
      display: flex;
      align-items: center;
      gap: ${theme.vars.spacing.unit};
      padding: ${theme.vars.spacing.unit} calc(${theme.vars.spacing.unit} * 2);
    `;

    const headerInfo: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} / 4);
      flex: 1;
      min-width: 0;
      overflow: hidden;
    `;

    const headerName: string = css`
      color: ${theme.vars.colors.text.primary};
      font-size: 0.875rem;
      font-weight: 500;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `;

    const headerMeta: string = css`
      color: ${theme.vars.colors.text.secondary};
      font-size: 0.75rem;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `;

    const headerRole: string = css`
      color: ${theme.vars.colors.text.secondary};
      font-size: 0.75rem;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      text-transform: capitalize;
    `;

    const manageButton: string = css`
      min-width: auto;
      margin-inline-start: auto;
    `;

    const menu: string = css`
      display: flex;
      flex-direction: column;
      width: 100%;
    `;

    const menuItem: string = css`
      display: flex;
      align-items: center;
      justify-content: flex-start;
      gap: ${theme.vars.spacing.unit};
      padding: calc(${theme.vars.spacing.unit} * 1.5) calc(${theme.vars.spacing.unit} * 2);
      width: 100%;
      color: ${theme.vars.colors.text.primary};
      text-decoration: none;
      border: none;
      background-color: transparent;
      cursor: pointer;
      font-size: 0.875rem;
      text-align: start;
      border-radius: ${theme.vars.borderRadius.medium};
      transition: background-color 0.15s ease-in-out;

      > span {
        gap: ${theme.vars.spacing.unit};
      }

      &:hover {
        background-color: ${theme.vars.colors.action?.hover || 'rgba(0, 0, 0, 0.04)'};
      }
    `;

    const menuDivider: string = css`
      margin: calc(${theme.vars.spacing.unit} * 0.5) 0;
      border-bottom: 1px solid ${theme.vars.colors.border};
    `;

    const organizationInfo: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} / 4);
      flex: 1;
      min-width: 0;
      overflow: hidden;
    `;

    const organizationName: string = css`
      color: ${theme.vars.colors.text.primary};
      font-size: 0.875rem;
      font-weight: 500;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `;

    const organizationMeta: string = css`
      color: ${theme.vars.colors.text.secondary};
      font-size: 0.75rem;
      margin: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `;

    const loadingContainer: string = css`
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 80px;
      gap: ${theme.vars.spacing.unit};
    `;

    const loadingText: string = css`
      color: ${theme.vars.colors.text.secondary};
      font-size: 0.875rem;
    `;

    const errorContainer: string = css`
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 80px;
      padding: calc(${theme.vars.spacing.unit} * 2);
    `;

    const errorText: string = css`
      color: ${theme.vars.colors.text.secondary};
      font-size: 0.875rem;
      text-align: center;
    `;

    const sectionHeader: string = css`
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: ${theme.vars.colors.text.secondary};
    `;

    const sectionHeaderContainer: string = css`
      border-top: none;
      border-bottom: none;
      padding-bottom: calc(${theme.vars.spacing.unit} / 2);
    `;

    const roleCapitalized: string = css`
      text-transform: capitalize;
    `;

    return {
      content,
      errorContainer,
      errorText,
      header,
      headerInfo,
      headerMeta,
      headerName,
      headerRole,
      loadingContainer,
      loadingText,
      manageButton,
      menu,
      menuDivider,
      menuItem,
      organizationInfo,
      organizationMeta,
      organizationName,
      roleCapitalized,
      root,
      sectionHeader,
      sectionHeaderContainer,
      trigger,
      triggerLabel,
    };
  }, [
    theme.vars.colors.background.surface,
    theme.vars.colors.text.primary,
    theme.vars.colors.text.secondary,
    theme.vars.colors.border,
    theme.vars.borderRadius.medium,
    theme.vars.shadows.medium,
    theme.vars.spacing.unit,
    theme.vars.colors.action?.hover,
    theme.vars.typography.fontFamily,
    colorScheme,
  ]);

export default useStyles;
