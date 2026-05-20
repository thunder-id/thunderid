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
  useMemo(
    () => ({
      container: css`
        display: flex;
        flex-direction: column;
        gap: calc(${theme.vars.spacing.unit} * 0.5);
        width: 100%;
      `,
      copyButton: css`
        flex-shrink: 0;
        white-space: nowrap;
      `,
      label: css`
        color: ${theme.vars.colors.text.secondary};
        font-size: 0.875rem;
        font-weight: 500;
      `,
      valueBox: css`
        align-items: center;
        background-color: ${theme.vars.colors.background.surface};
        border: 1px solid ${theme.vars.colors.border};
        border-radius: ${theme.vars.borderRadius.small};
        display: flex;
        gap: calc(${theme.vars.spacing.unit} * 1);
        padding: calc(${theme.vars.spacing.unit} * 0.75) calc(${theme.vars.spacing.unit} * 1);
      `,
      valueText: css`
        color: ${theme.vars.colors.text.primary};
        flex: 1;
        font-family: monospace;
        font-size: 0.85rem;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        word-break: break-all;
      `,
    }),
    [theme],
  );

export default useStyles;
