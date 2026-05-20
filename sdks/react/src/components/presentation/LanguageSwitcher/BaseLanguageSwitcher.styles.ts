/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
      gap: calc(${theme.vars.spacing.unit} * 0.5);
      padding: calc(${theme.vars.spacing.unit} * 0.75) ${theme.vars.spacing.unit};
      border: 1px solid ${theme.vars.colors.border};
      background: ${theme.vars.colors.background.surface};
      cursor: pointer;
      border-radius: ${theme.vars.borderRadius.medium};
      min-width: 120px;
      font-size: 0.875rem;
      color: ${theme.vars.colors.text.primary};

      &:hover {
        background-color: ${theme.vars.colors.action?.hover || 'rgba(0, 0, 0, 0.04)'};
      }
    `;

    const triggerEmoji: string = css`
      font-size: 1rem;
      line-height: 1;
    `;

    const triggerLabel: string = css`
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 500;
    `;

    const content: string = css`
      min-width: 200px;
      max-width: 320px;
      background-color: ${theme.vars.colors.background.surface};
      border-radius: ${theme.vars.borderRadius.medium};
      box-shadow: ${theme.vars.shadows.medium};
      border: 1px solid ${theme.vars.colors.border};
      outline: none;
      z-index: 1000;
      padding: calc(${theme.vars.spacing.unit} * 0.5) 0;
    `;

    const option: string = css`
      display: flex;
      align-items: center;
      gap: ${theme.vars.spacing.unit};
      padding: calc(${theme.vars.spacing.unit} * 1) calc(${theme.vars.spacing.unit} * 1.5);
      width: 100%;
      border: none;
      background-color: transparent;
      cursor: pointer;
      font-size: 0.875rem;
      text-align: start;
      color: ${theme.vars.colors.text.primary};
      transition: background-color 0.15s ease-in-out;

      &:hover {
        background-color: ${theme.vars.colors.action?.hover || 'rgba(0, 0, 0, 0.04)'};
      }
    `;

    const optionActive: string = css`
      font-weight: 600;
      color: ${theme.vars.colors.primary?.main || theme.vars.colors.text.primary};
    `;

    const optionEmoji: string = css`
      font-size: 1rem;
      line-height: 1;
      flex-shrink: 0;
    `;

    const optionLabel: string = css`
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `;

    const checkIcon: string = css`
      color: ${theme.vars.colors.primary?.main || theme.vars.colors.text.primary};
      flex-shrink: 0;
      margin-inline-start: auto;
    `;

    return {
      checkIcon,
      content,
      option,
      optionActive,
      optionEmoji,
      optionLabel,
      root,
      trigger,
      triggerEmoji,
      triggerLabel,
    };
  }, [
    theme.vars.colors.background.surface,
    theme.vars.colors.text.primary,
    theme.vars.colors.border,
    theme.vars.borderRadius.medium,
    theme.vars.shadows.medium,
    theme.vars.spacing.unit,
    theme.vars.colors.action?.hover,
    theme.vars.typography.fontFamily,
    theme.vars.colors.primary?.main,
    colorScheme,
  ]);

export default useStyles;
