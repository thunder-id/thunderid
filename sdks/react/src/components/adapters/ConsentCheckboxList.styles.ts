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
  useMemo(
    () => ({
      bullet: css`
        width: 5px;
        height: 5px;
        border-radius: 50%;
        background-color: #9e9e9e;
        flex-shrink: 0;
      `,
      divider: css`
        opacity: 0.5;
        margin: 0.25rem 0;
      `,
      labelContainer: css`
        display: flex;
        align-items: center;
        gap: 0.4rem;
      `,
      listContainer: css`
        display: flex;
        flex-direction: column;
      `,
      listItem: css`
        padding: 0 0.25rem;
      `,
      listRow: css`
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.125rem 0;
      `,
      typography: css`
        margin: 0;
      `,
    }),
    [theme, colorScheme],
  );

export default useStyles;
