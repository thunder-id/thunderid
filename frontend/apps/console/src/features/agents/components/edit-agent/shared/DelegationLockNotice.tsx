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

import {Alert, Box} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';

interface DelegationLockNoticeProps {
  /** Whether Delegated mode is on for this agent. */
  isUnlocked: boolean;
  /** Explains where/how to turn on Delegated mode — differs by caller (see Flows/Tokens). */
  message: ReactNode;
  children: ReactNode;
}

/**
 * Wraps Flows/Tokens tab content with an info banner explaining that the settings shown below
 * aren't in effect for this agent yet, and gives the content a frozen look (dimmed, no pointer
 * interaction) rather than a blur — the values stay fully legible. The caller is still
 * responsible for disabling the inputs themselves (e.g. by forcing `isReadOnly` on the
 * agent/application it passes down) when `isUnlocked` is false.
 */
export default function DelegationLockNotice({isUnlocked, message, children}: DelegationLockNoticeProps): ReactNode {
  if (isUnlocked) {
    return children;
  }

  return (
    <Box>
      <Alert severity="info" icon={<Lock size={20} />} sx={{mb: 3}}>
        {message}
      </Alert>
      <Box sx={{opacity: 0.6, pointerEvents: 'none', cursor: 'not-allowed'}}>{children}</Box>
    </Box>
  );
}
