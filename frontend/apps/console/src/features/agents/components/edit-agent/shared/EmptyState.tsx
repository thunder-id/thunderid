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

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

interface EmptyStateProps {
  icon: ReactNode;
  message: string;
}

/**
 * Centered icon + message used for empty lists across the agent edit page (groups, roles,
 * attributes), instead of a plain left-aligned line of text.
 */
export default function EmptyState({icon, message}: EmptyStateProps): JSX.Element {
  return (
    <Stack alignItems="center" spacing={1} sx={{py: 3}}>
      <Box sx={{color: 'text.disabled', display: 'inline-flex'}}>{icon}</Box>
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
    </Stack>
  );
}
