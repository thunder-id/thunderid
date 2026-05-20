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

export type AvatarVariant = 'circular' | 'square';

/**
 * Creates styles for the Avatar component using BEM methodology
 * @param theme - The theme object containing design tokens
 * @param colorScheme - The current color scheme (used for memoization)
 * @param size - The size of the avatar in pixels
 * @param variant - The avatar variant
 * @param backgroundColor - The background color for the avatar
 * @returns Object containing CSS class names for component styling
 */
const useStyles = (
  theme: Theme,
  colorScheme: string,
  size: number,
  variant: AvatarVariant,
  backgroundColor?: string,
): Record<string, string> =>
  useMemo(() => {
    const baseAvatar: string = css`
      align-items: center;
      background: ${backgroundColor || theme.vars.colors.background.surface};
      border: ${backgroundColor ? 'none' : `1px solid ${theme.vars.colors.border}`};
      border-radius: ${variant === 'circular' ? '50%' : '8px'};
      color: ${backgroundColor ? '#ffffff' : theme.vars.colors.text.primary};
      display: flex;
      font-size: ${size * 0.4}px;
      font-family: ${theme.vars.typography.fontFamily};
      font-weight: 600;
      height: ${size}px;
      justify-content: center;
      overflow: hidden;
      text-shadow: ${backgroundColor ? '0 1px 2px rgba(0, 0, 0, 0.1)' : 'none'};
      width: ${size}px;
    `;

    const variantStyles: Record<string, string> = {
      circular: css`
        border-radius: 50%;
      `,
      square: css`
        border-radius: 8px;
      `,
    };

    const imageStyles: string = css`
      height: 100%;
      object-fit: cover;
      width: 100%;
    `;

    const skeletonStyles: string = css`
      width: 100%;
      height: 100%;
      background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
      background-size: 200% 100%;
      animation: skeleton-loading 1.5s infinite;
      border-radius: ${variant === 'circular' ? '50%' : '8px'};

      @keyframes skeleton-loading {
        0% {
          background-position: -200% 0;
        }
        100% {
          background-position: 200% 0;
        }
      }
    `;

    const iconStyles: string = css`
      width: 60%;
      height: 60%;
      fill: ${backgroundColor ? '#ffffff' : theme.vars.colors.text.secondary};
      opacity: 0.8;
    `;

    return {
      avatar: baseAvatar,
      icon: iconStyles,
      image: imageStyles,
      skeleton: skeletonStyles,
      variant: variantStyles[variant],
    };
  }, [theme, colorScheme, size, variant, backgroundColor]);

export default useStyles;
