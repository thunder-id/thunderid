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

import {useCopyToClipboard} from '@thunderid/hooks';
import {IconButton, Stack, Typography} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

/**
 * Props for the {@link CopyableListRow} component.
 *
 * @public
 */
export interface CopyableListRowProps {
  /**
   * The read-only value displayed and copied
   */
  value: string;

  /**
   * The `aria-label` for the copy button
   */
  copyAriaLabel: string;
}

/**
 * Read-only monospace row (no label) with a trailing copy button, for list-shaped values.
 * Shared by the mcp-client template's create-flow Connect completion screen for the
 * registered redirect URIs list.
 *
 * @param props - The component props
 * @param props.value - The read-only value displayed and copied
 * @param props.copyAriaLabel - The `aria-label` for the copy button
 *
 * @returns JSX element displaying a read-only copyable list row
 *
 * @example
 * ```tsx
 * <CopyableListRow value="http://127.0.0.1:8080/callback" copyAriaLabel="Copy redirect URI" />
 * ```
 *
 * @public
 */
export default function CopyableListRow({value, copyAriaLabel}: CopyableListRowProps): JSX.Element {
  const {copied, copy} = useCopyToClipboard({resetDelay: 2000});

  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1}
      sx={{border: '1px solid', borderColor: 'divider', borderRadius: 1, px: 1.5, py: 1}}
    >
      <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem', flex: 1, wordBreak: 'break-all'}}>
        {value}
      </Typography>
      <IconButton
        aria-label={copyAriaLabel}
        size="small"
        onClick={() => {
          copy(value).catch(() => {
            // Error already handled in copy
          });
        }}
      >
        {copied ? <Check size={16} /> : <Copy size={16} />}
      </IconButton>
    </Stack>
  );
}
