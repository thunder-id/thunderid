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

import {Box, IconButton, LinearProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';

interface ConnectionFullPageLayoutProps {
  label: string;
  onClose: () => void;
  /** When provided, renders a determinate top progress bar (wizard). */
  progress?: number;
  /** Optional breadcrumb rendered in the header instead of the plain label. */
  breadcrumb?: ReactNode;
  children: ReactNode;
}

/**
 * Full-screen chrome shared by the connection configure/edit form and the add-custom-connection
 * wizard: an optional top progress bar, an X-close button + label header, and a left-aligned
 * constrained content column. Mounted under FullScreenLayout (no console sidebar).
 */
export default function ConnectionFullPageLayout({
  label,
  onClose,
  progress = undefined,
  breadcrumb = undefined,
  children,
}: ConnectionFullPageLayoutProps): JSX.Element {
  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      {progress !== undefined && <LinearProgress variant="determinate" value={progress} sx={{height: 6}} />}

      <Box sx={{p: 4, display: 'flex', flexDirection: 'column', flex: 1}}>
        <Stack direction="row" spacing={2} alignItems="center" sx={{mb: 4}}>
          <IconButton
            onClick={onClose}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
            aria-label="close"
            data-testid="connection-fullpage-close"
          >
            <X size={24} />
          </IconButton>
          {breadcrumb ?? (
            <Typography variant="subtitle1" fontWeight={600}>
              {label}
            </Typography>
          )}
        </Stack>

        <Box
          sx={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            py: {xs: 4, md: 8},
            px: {xs: 0, md: 10},
            width: '100%',
          }}
        >
          <Box data-testid="connection-fullpage-content" sx={{width: '100%', maxWidth: 920}}>
            {children}
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
