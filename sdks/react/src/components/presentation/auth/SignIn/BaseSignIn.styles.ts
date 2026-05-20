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
 * Creates styles for the BaseSignIn component
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string): Record<string, string> =>
  useMemo(() => {
    const signIn: string = css`
      min-width: 420px;
      margin: 0 auto;
      font-family: ${theme.vars.typography.fontFamily};
    `;

    const card: string = css`
      background: ${theme.vars.colors.background.surface};
      border-radius: ${theme.vars.borderRadius.large};
      gap: calc(${theme.vars.spacing.unit} * 2);
      min-width: 420px;
    `;

    const logoContainer: string = css`
      display: flex;
      flex-direction: column;
      align-items: center;
      margin-bottom: calc(${theme.vars.spacing.unit} * 2);
    `;

    const header: string = css`
      gap: 0;
      align-items: center;
    `;

    const title: string = css`
      margin: 0 0 calc(${theme.vars.spacing.unit} * 1) 0;
      color: ${theme.vars.colors.text.primary};
    `;

    const subtitle: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 1);
      color: ${theme.vars.colors.text.secondary};
    `;

    const messagesContainer: string = css`
      margin-top: calc(${theme.vars.spacing.unit} * 2);
    `;

    const messageItem: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 1);
    `;

    const errorContainer: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 2);
    `;

    const contentContainer: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} * 2);
    `;

    const loadingContainer: string = css`
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: calc(${theme.vars.spacing.unit} * 4);
    `;

    const loadingText: string = css`
      margin-top: calc(${theme.vars.spacing.unit} * 2);
      color: ${theme.vars.colors.text.secondary};
    `;

    const divider: string = css`
      margin: calc(${theme.vars.spacing.unit} * 1) 0;
    `;

    const centeredContainer: string = css`
      text-align: center;
      padding: calc(${theme.vars.spacing.unit} * 4);
    `;

    const passkeyContainer: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 2);
    `;

    const passkeyText: string = css`
      margin-top: calc(${theme.vars.spacing.unit} * 1);
      color: ${theme.vars.colors.text.secondary};
    `;

    const form: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} * 2);
    `;

    const formDivider: string = css`
      margin: calc(${theme.vars.spacing.unit} * 1) 0;
    `;

    const authenticatorSection: string = css`
      display: flex;
      flex-direction: column;
      gap: calc(${theme.vars.spacing.unit} * 1);
    `;

    const authenticatorItem: string = css`
      width: 100%;
    `;

    const noAuthenticatorCard: string = css`
      background: ${theme.vars.colors.background.surface};
      border-radius: ${theme.vars.borderRadius.large};
      padding: calc(${theme.vars.spacing.unit} * 2);
    `;

    const errorAlert: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 2);
    `;

    const messagesAlert: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 1);
    `;

    const flowMessagesContainer: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 2);
    `;

    const flowMessageItem: string = css`
      margin-bottom: calc(${theme.vars.spacing.unit} * 1);
    `;

    return {
      authenticatorItem,
      authenticatorSection,
      card,
      centeredContainer,
      contentContainer,
      divider,
      errorAlert,
      errorContainer,
      flowMessageItem,
      flowMessagesContainer,
      form,
      formDivider,
      header,
      loadingContainer,
      loadingText,
      logoContainer,
      messageItem,
      messagesAlert,
      messagesContainer,
      noAuthenticatorCard,
      passkeyContainer,
      passkeyText,
      signIn,
      subtitle,
      title,
    };
  }, [
    theme.vars.colors.background.surface,
    theme.vars.colors.text.primary,
    theme.vars.colors.text.secondary,
    theme.vars.borderRadius.large,
    theme.vars.spacing.unit,
    theme.vars.typography.fontFamily,
    colorScheme,
  ]);

export default useStyles;
