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
 * Creates styles for the PasswordField component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param showPassword - Whether the password is currently visible
 * @param disabled - Whether the component is disabled
 * @param hasError - Whether the component has an error
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  showPassword: boolean,
  disabled: boolean,
  hasError: boolean,
): Record<string, string> =>
  useMemo(() => {
    const toggleIcon: string = css`
      cursor: ${disabled ? 'not-allowed' : 'pointer'};
      color: ${theme.vars.colors.text.secondary};
      opacity: ${disabled ? 0.6 : 1};
      transition: color 0.2s ease;

      &:hover {
        color: ${!disabled ? theme.vars.colors.text.primary : theme.vars.colors.text.secondary};
      }
    `;

    const visibleIcon: string = css`
      color: ${theme.vars.colors.primary.main};
    `;

    const hiddenIcon: string = css`
      color: ${theme.vars.colors.text.secondary};
    `;

    return {
      hiddenIcon,
      toggleIcon,
      visibleIcon,
    };
  }, [theme, colorScheme, showPassword, disabled, hasError]);

export default useStyles;
