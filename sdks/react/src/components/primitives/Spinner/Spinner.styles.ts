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

import {css, keyframes} from '@emotion/css';
import {Theme} from '@thunderid/browser';
import {useMemo} from 'react';

export type SpinnerSize = 'small' | 'medium' | 'large';

/**
 * Creates styles for the Spinner component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param size - The size of the spinner
 * @param color - The color of the spinner
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (theme: Theme, colorScheme: string, size: SpinnerSize, color?: string): Record<string, string> =>
  useMemo(() => {
    const spinnerColor: string = color || theme.vars.colors.primary.main;

    const spinnerSizes: Record<string, string> = {
      large: '32px',
      medium: '20px',
      small: '16px',
    };

    const spinnerSize: string = spinnerSizes[size];

    const spinAnimation: string = keyframes`
      0% {
        transform: rotate(0deg);
      }
      100% {
        transform: rotate(360deg);
      }
    `;

    const spinner: string = css`
      width: ${spinnerSize};
      height: ${spinnerSize};
      border: 2px solid transparent;
      border-top: 2px solid ${spinnerColor};
      border-radius: 50%;
      animation: ${spinAnimation} 1s linear infinite;
      display: inline-block;
    `;

    const spinnerSmall: string = css`
      width: ${spinnerSizes['small']};
      height: ${spinnerSizes['small']};
    `;

    const spinnerMedium: string = css`
      width: ${spinnerSizes['medium']};
      height: ${spinnerSizes['medium']};
    `;

    const spinnerLarge: string = css`
      width: ${spinnerSizes['large']};
      height: ${spinnerSizes['large']};
    `;

    return {
      spinner,
      spinnerLarge,
      spinnerMedium,
      spinnerSmall,
    };
  }, [theme, colorScheme, size, color]);

export default useStyles;
