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

export type LogoSize = 'small' | 'medium' | 'large';

/**
 * Creates styles for the Logo component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param size - The size of the logo
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string, size: LogoSize): Record<string, string> =>
  useMemo(() => {
    const baseLogo: string = css`
      width: auto;
      object-fit: contain;
      display: block;
    `;

    const smallLogo: string = css`
      height: 32px;
      max-width: 120px;
    `;

    const mediumLogo: string = css`
      height: 48px;
      max-width: 180px;
    `;

    const largeLogo: string = css`
      height: 64px;
      max-width: 240px;
    `;

    const sizeStyles: Record<string, string> = {
      large: largeLogo,
      medium: mediumLogo,
      small: smallLogo,
    };

    return {
      large: largeLogo,
      logo: baseLogo,
      medium: mediumLogo,
      size: sizeStyles[size],
      small: smallLogo,
    };
  }, [theme, colorScheme, size]);

export default useStyles;
