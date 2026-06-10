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

const useStyles = (theme: Theme): Record<string, string> =>
  useMemo(() => {
    const container: string = css`
      display: flex;
      flex-direction: column;
      width: 100%;
      max-height: 320px;
      overflow-y: auto;
      border: 1px solid ${theme.vars.colors.border};
      border-radius: ${theme.vars.borderRadius.medium};
      font-family: ${theme.vars.typography.fontFamily};
    `;

    const node: string = css`
      display: flex;
      align-items: center;
      padding: calc(${theme.vars.spacing.unit} * 1) calc(${theme.vars.spacing.unit} * 1.5);
      cursor: pointer;
      user-select: none;
      transition: background-color 0.15s ease;

      &:hover {
        background-color: ${theme.vars.colors.action.hover};
      }
    `;

    const nodeSelected: string = css`
      background-color: ${theme.vars.colors.action.selected};

      &:hover {
        background-color: ${theme.vars.colors.action.selected};
      }
    `;

    const toggleButton: string = css`
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      border: none;
      background: none;
      cursor: pointer;
      padding: 0;
      margin-right: calc(${theme.vars.spacing.unit} * 0.5);
      color: ${theme.vars.colors.text.secondary};
      font-size: 12px;
      flex-shrink: 0;
    `;

    const togglePlaceholder: string = css`
      width: 20px;
      height: 20px;
      margin-right: calc(${theme.vars.spacing.unit} * 0.5);
      flex-shrink: 0;
    `;

    const nodeName: string = css`
      font-size: 14px;
      color: ${theme.vars.colors.text.primary};
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    `;

    const loadMoreButton: string = css`
      display: flex;
      align-items: center;
      padding: calc(${theme.vars.spacing.unit} * 0.75) calc(${theme.vars.spacing.unit} * 1.5);
      border: none;
      background: none;
      cursor: pointer;
      color: ${theme.vars.colors.primary.main};
      font-size: 13px;
      font-family: ${theme.vars.typography.fontFamily};

      &:hover {
        text-decoration: underline;
      }
    `;

    const loadingPlaceholder: string = css`
      display: flex;
      align-items: center;
      padding: calc(${theme.vars.spacing.unit} * 1) calc(${theme.vars.spacing.unit} * 1.5);
      gap: calc(${theme.vars.spacing.unit} * 1);
    `;

    const skeleton: string = css`
      height: 14px;
      border-radius: ${theme.vars.borderRadius.small};
      background-color: ${theme.vars.colors.background.disabled};
      animation: pulse 1.5s ease-in-out infinite;

      @keyframes pulse {
        0%,
        100% {
          opacity: 1;
        }
        50% {
          opacity: 0.4;
        }
      }
    `;

    return {
      container,
      loadMoreButton,
      loadingPlaceholder,
      node,
      nodeName,
      nodeSelected,
      skeleton,
      toggleButton,
      togglePlaceholder,
    };
  }, [
    theme.vars.colors.action.hover,
    theme.vars.colors.action.selected,
    theme.vars.colors.background.disabled,
    theme.vars.colors.border,
    theme.vars.colors.primary.main,
    theme.vars.colors.text.primary,
    theme.vars.colors.text.secondary,
    theme.vars.borderRadius.medium,
    theme.vars.borderRadius.small,
    theme.vars.spacing.unit,
    theme.vars.typography.fontFamily,
  ]);

export default useStyles;
