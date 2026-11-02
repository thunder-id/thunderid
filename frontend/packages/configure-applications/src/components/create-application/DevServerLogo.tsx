// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';

/**
 * Props for the {@link DevServerLogo} component.
 *
 * @public
 */
export interface DevServerLogoProps {
  /** The dev server tool identifier, e.g. 'vite', 'nextjs', 'nuxt'. */
  id: string;
  size?: number;
}

/**
 * Renders the logo for a template's local development server tool, used in the Configuration
 * step's dev-server quick-add banner. Falls back to nothing for unrecognized ids so unknown
 * `devServer.id` values don't render a broken icon.
 */
export default function DevServerLogo({id, size = 18}: DevServerLogoProps): JSX.Element | null {
  switch (id) {
    case 'vite':
      return (
        <svg width={size} height={size} viewBox="0 0 257 257">
          <defs>
            <linearGradient id="devServerLogoViteA" x1="6%" y1="33%" x2="92%" y2="78%">
              <stop offset="0%" stopColor="#41D1FF" />
              <stop offset="100%" stopColor="#BD34FE" />
            </linearGradient>
            <linearGradient id="devServerLogoViteB" x1="22%" y1="2%" x2="79%" y2="90%">
              <stop offset="0%" stopColor="#FFEA83" />
              <stop offset="8%" stopColor="#FFDD35" />
              <stop offset="100%" stopColor="#FFA800" />
            </linearGradient>
          </defs>
          <path
            fill="url(#devServerLogoViteA)"
            d="M255.2 45.9L134.5 252.8c-2.4 4.1-8.4 4.1-10.8-.1L2.9 45.9c-2.6-4.6 1.2-10.2 6.5-9.4l117.4 18.6c.6.1 1.2.1 1.8 0L244.7 36.5c5.3-.8 9.1 4.8 6.5 9.4z"
          />
          <path
            fill="url(#devServerLogoViteB)"
            d="M186.1 3.5L98 20.1c-1.6.3-2.8 1.6-3 3.3l-5.4 90.6c-.2 2.6 2.2 4.7 4.8 4.1l24.4-5.6c2.8-.6 5.3 1.8 4.9 4.6l-7.3 48.8c-.5 3 2.5 5.4 5.3 4.2l15.1-6.4c2.8-1.2 5.9 1.1 5.4 4.1l-11.6 68.7c-.7 4.3 4.9 6.6 7.4 3l1.6-2.4 71.6-142.9c1.4-2.9-1-6.2-4.2-5.6l-25.1 4.6c-2.9.5-5.3-2.2-4.5-5l16.4-56.9c.8-2.9-1.7-5.6-4.7-5z"
          />
        </svg>
      );
    case 'nextjs':
      return (
        <svg width={size} height={size} viewBox="0 0 16 16" fill="none">
          <path d="M8 15.733A7.733 7.733 0 1 0 8 .267a7.733 7.733 0 0 0 0 15.466Z" fill="#000" />
          <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M8 .533a7.467 7.467 0 1 0 0 14.934A7.467 7.467 0 0 0 8 .533ZM0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Z"
            fill="#fff"
          />
          <path d="M13.29 14.002 6.146 4.8H4.8v6.397h1.077v-5.03l6.567 8.486c.297-.198.58-.416.846-.651Z" fill="#fff" />
          <path d="M11.289 4.8h-1.067v6.4h1.067V4.8Z" fill="#fff" />
        </svg>
      );
    case 'nuxt':
      return (
        <svg width={size} height={size} viewBox="0 0 221 120">
          <path
            d="M120.81 120H212.7c1.903 0 3.773-.498 5.408-1.442a10.827 10.827 0 003.977-3.92 10.657 10.657 0 001.458-5.36c0-1.889-.5-3.745-1.458-5.36L166.037 19.2a10.827 10.827 0 00-3.977-3.92 10.978 10.978 0 00-10.816 0 10.827 10.827 0 00-3.977 3.92l-9.684 16.704-18.664-32.28A10.827 10.827 0 00114.942 0a10.978 10.978 0 00-10.816 0 10.827 10.827 0 00-3.977 3.92L1.458 104.008A10.697 10.697 0 000 109.278a10.657 10.657 0 001.458 5.36 10.827 10.827 0 003.977 3.92A10.978 10.978 0 0010.843 120H67.89c21.187 0 36.72-9.152 47.248-26.88L140.47 51.2l12.94 22.4-21.6 37.36C125.097 118.18 113.433 120 120.81 120zm-58.168-21.28l-36.19-.08 72.368-125.2 18.096 31.28-25.936 44.8c-8.784 14.56-18.37 49.2-28.338 49.2z"
            fill="#00dc82"
          />
        </svg>
      );
    default:
      return null;
  }
}
